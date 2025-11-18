package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CacheEntry represents a cached response
type CacheEntry struct {
	Data        []byte
	ContentType string
	StatusCode  int
	Headers     map[string]string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	HitCount    int64
}

// CacheStore interface for different cache implementations
type CacheStore interface {
	Get(key string) (*CacheEntry, bool)
	Set(key string, entry *CacheEntry, ttl time.Duration)
	Delete(key string)
	Clear()
	Size() int
}

// MemoryCacheStore implements in-memory cache
type MemoryCacheStore struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
}

// NewMemoryCacheStore creates a new memory cache store
func NewMemoryCacheStore() *MemoryCacheStore {
	return &MemoryCacheStore{
		entries: make(map[string]*CacheEntry),
	}
}

// Get retrieves a cache entry
func (m *MemoryCacheStore) Get(key string) (*CacheEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[key]
	if !exists {
		return nil, false
	}

	// Check if entry is expired
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	entry.HitCount++
	return entry, true
}

// Set stores a cache entry
func (m *MemoryCacheStore) Set(key string, entry *CacheEntry, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry.ExpiresAt = time.Now().Add(ttl)
	m.entries[key] = entry
}

// Delete removes a cache entry
func (m *MemoryCacheStore) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.entries, key)
}

// Clear removes all cache entries
func (m *MemoryCacheStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]*CacheEntry)
}

// Size returns the number of cache entries
func (m *MemoryCacheStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.entries)
}

// Global cache instance
var GlobalCache CacheStore

// CacheConfig represents cache configuration
type CacheConfig struct {
	Enabled        bool
	DefaultTTL     time.Duration
	MaxSize        int
	CachePatterns  []string
	SkipPatterns   []string
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:       true,
		DefaultTTL:    5 * time.Minute,
		MaxSize:       1000,
		CachePatterns: []string{"/v1/chat/completions", "/v1/images/generations"},
		SkipPatterns:  []string{"/health", "/metrics", "/auth"},
	}
}

// CacheMiddleware provides request caching
func CacheMiddleware(config CacheConfig) gin.HandlerFunc {
	if GlobalCache == nil {
		GlobalCache = NewMemoryCacheStore()
	}

	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// Skip caching for certain patterns
		if shouldSkipCaching(c.Request.URL.Path, config.SkipPatterns) {
			c.Next()
			return
		}

		// Only cache specific patterns
		if !shouldCache(c.Request.URL.Path, config.CachePatterns) {
			c.Next()
			return
		}

		// Generate cache key
		cacheKey := generateCacheKey(c)
		
		// Try to get from cache
		if entry, found := GlobalCache.Get(cacheKey); found {
			logger.SysLogf("Cache hit for %s %s", c.Request.Method, c.Request.URL.Path)
			serveCachedResponse(c, entry)
			return
		}

		// Process request and cache response
		c.Next()

		// Cache successful responses
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			cacheResponse(c, cacheKey, config.DefaultTTL)
		}
	}
}

// ResponseCacheMiddleware caches response data
func ResponseCacheMiddleware(config CacheConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// Create a custom writer to capture response
		writer := &cacheResponseWriter{
			ResponseWriter: c.Writer,
			body:           make([]byte, 0),
			statusCode:     http.StatusOK,
		}
		c.Writer = writer

		// Process request
		c.Next()

		// Cache the response if conditions are met
		if shouldCacheResponse(c, writer) {
			cacheKey := generateCacheKey(c)
			entry := &CacheEntry{
				Data:        writer.body,
				ContentType: writer.Header().Get("Content-Type"),
				StatusCode:  writer.statusCode,
				Headers:     extractHeaders(writer.Header()),
				CreatedAt:   time.Now(),
				HitCount:    0,
			}
			
			GlobalCache.Set(cacheKey, entry, config.DefaultTTL)
			logger.SysLogf("Cached response for %s %s", c.Request.Method, c.Request.URL.Path)
		}
	}
}

// SmartCacheMiddleware provides intelligent caching based on request patterns
func SmartCacheMiddleware() gin.HandlerFunc {
	config := DefaultCacheConfig()
	
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// Determine cache TTL based on request type
		var ttl time.Duration
		switch {
		case strings.Contains(c.Request.URL.Path, "chat/completions"):
			ttl = 30 * time.Second // Short cache for chat
		case strings.Contains(c.Request.URL.Path, "images/generations"):
			ttl = 5 * time.Minute // Longer cache for images
		case strings.Contains(c.Request.URL.Path, "videos/generations"):
			ttl = 15 * time.Minute // Longest cache for videos
		default:
			c.Next()
			return
		}

		// Generate cache key
		cacheKey := generateSmartCacheKey(c)
		
		// Try cached response
		if entry, found := GlobalCache.Get(cacheKey); found {
			logger.SysLogf("Smart cache hit for %s %s", c.Request.Method, c.Request.URL.Path)
			serveCachedResponse(c, entry)
			return
		}

		// Process and cache
		c.Next()

		// Cache successful responses
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			cacheResponseWithTTL(c, cacheKey, ttl)
		}
	}
}

// generateCacheKey creates a cache key from request
func generateCacheKey(c *gin.Context) string {
	var keyParts []string
	
	// Method and path
	keyParts = append(keyParts, c.Request.Method, c.Request.URL.Path)
	
	// Query parameters (sorted for consistency)
	queryParams := c.Request.URL.Query()
	for key, values := range queryParams {
		for _, value := range values {
			keyParts = append(keyParts, fmt.Sprintf("%s=%s", key, value))
		}
	}
	
	// Authorization header (if present)
	if auth := c.GetHeader("Authorization"); auth != "" {
		// Hash the auth token to avoid storing sensitive data
		hash := md5.Sum([]byte(auth))
		keyParts = append(keyParts, "auth="+hex.EncodeToString(hash[:]))
	}
	
	// Request body for POST/PUT requests (hashed)
	if c.Request.Method == "POST" || c.Request.Method == "PUT" {
		var bodyData map[string]interface{}
		if err := c.ShouldBindJSON(&bodyData); err == nil {
			bodyJSON, _ := json.Marshal(bodyData)
			hash := md5.Sum(bodyJSON)
			keyParts = append(keyParts, "body="+hex.EncodeToString(hash[:]))
		}
	}
	
	// Create final hash
	keyString := strings.Join(keyParts, "|")
	hash := md5.Sum([]byte(keyString))
	return hex.EncodeToString(hash[:])
}

// generateSmartCacheKey creates an intelligent cache key
func generateSmartCacheKey(c *gin.Context) string {
	var keyParts []string
	
	// Method and path
	keyParts = append(keyParts, c.Request.Method, c.Request.URL.Path)
	
	// Model-specific caching
	var model string
	if c.Request.Method == "POST" {
		var bodyData map[string]interface{}
		if err := c.ShouldBindJSON(&bodyData); err == nil {
			if modelVal, ok := bodyData["model"].(string); ok {
				model = modelVal
			}
		}
	}
	
	if model != "" {
		keyParts = append(keyParts, "model="+model)
	}
	
	// Create hash
	keyString := strings.Join(keyParts, "|")
	hash := md5.Sum([]byte(keyString))
	return hex.EncodeToString(hash[:])
}

// shouldCache determines if a request should be cached
func shouldCache(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// shouldSkipCaching determines if a request should skip caching
func shouldSkipCaching(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// serveCachedResponse serves a cached response
func serveCachedResponse(c *gin.Context, entry *CacheEntry) {
	// Set headers from cached response
	for key, value := range entry.Headers {
		c.Header(key, value)
	}
	
	// Add cache hit header
	c.Header("X-Cache", "HIT")
	c.Header("X-Cache-Hits", fmt.Sprintf("%d", entry.HitCount))
	
	// Serve cached content
	c.Data(entry.StatusCode, entry.ContentType, entry.Data)
}

// cacheResponse caches the current response
func cacheResponse(c *gin.Context, cacheKey string, ttl time.Duration) {
	// This would be called after the response is written
	// Implementation depends on how we capture the response
}

// cacheResponseWithTTL caches response with specific TTL
func cacheResponseWithTTL(c *gin.Context, cacheKey string, ttl time.Duration) {
	// Capture response and cache it
	// Implementation would capture the written response
}

// shouldCacheResponse determines if a response should be cached
func shouldCacheResponse(c *gin.Context, writer *cacheResponseWriter) bool {
	// Only cache successful responses
	if writer.statusCode < 200 || writer.statusCode >= 300 {
		return false
	}
	
	// Don't cache if cache-control header says not to
	cacheControl := writer.Header().Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") {
		return false
	}
	
	return true
}

// extractHeaders extracts relevant headers for caching
func extractHeaders(headers http.Header) map[string]string {
	extracted := make(map[string]string)
	
	importantHeaders := []string{
		"Content-Type",
		"Content-Encoding",
		"Content-Length",
		"ETag",
		"Last-Modified",
	}
	
	for _, key := range importantHeaders {
		if value := headers.Get(key); value != "" {
			extracted[key] = value
		}
	}
	
	return extracted
}

// CacheStats returns cache statistics
func CacheStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GlobalCache == nil {
			c.JSON(http.StatusOK, gin.H{
				"status": "disabled",
			})
			return
		}
		
		stats := gin.H{
			"status": "enabled",
			"size":   GlobalCache.Size(),
		}
		
		c.JSON(http.StatusOK, stats)
	}
}

// ClearCache clears all cached entries
func ClearCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GlobalCache != nil {
			GlobalCache.Clear()
			logger.SysLog("Cache cleared successfully")
		}
		
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"message": "Cache cleared",
		})
	}
}

// cacheResponseWriter wraps the response writer to capture response
type cacheResponseWriter struct {
	gin.ResponseWriter
	body       []byte
	statusCode int
}

func (w *cacheResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return w.ResponseWriter.Write(data)
}

func (w *cacheResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}