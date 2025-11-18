package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// APIKeyValidator validates API keys with enhanced security
func APIKeyValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip validation for health and metrics endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// Get API key from header
		apiKey := c.GetHeader("Authorization")
		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}

		// Remove "Bearer " prefix if present
		if strings.HasPrefix(apiKey, "Bearer ") {
			apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		}

		// Check if API key is required
		if config.ApiSecret != "" {
			if !isValidAPIKey(apiKey, config.ApiSecret) {
				logger.SecurityLogf("Invalid API key attempt from IP: %s, Path: %s", c.ClientIP(), c.Request.URL.Path)
				
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
					"code":  "INVALID_API_KEY",
					"timestamp": time.Now(),
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// isValidAPIKey validates the API key using constant-time comparison
func isValidAPIKey(providedKey, validKey string) bool {
	if providedKey == "" || validKey == "" {
		return false
	}

	// Support multiple API keys separated by comma
	validKeys := strings.Split(validKey, ",")
	for _, key := range validKeys {
		key = strings.TrimSpace(key)
		if key != "" && subtle.ConstantTimeCompare([]byte(providedKey), []byte(key)) == 1 {
			return true
		}
	}

	return false
}

// RequestSizeLimiter limits the size of incoming requests
func RequestSizeLimiter(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			logger.SecurityLogf("Request too large from IP: %s, Size: %d bytes (max: %d)", 
				c.ClientIP(), c.Request.ContentLength, maxSize)
			
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("Request too large. Maximum size is %d bytes", maxSize),
				"code": "REQUEST_TOO_LARGE",
				"timestamp": time.Now(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestTimeout adds timeout protection to requests
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Wrap the request context with timeout
		timeoutCtx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// Update the request with timeout context
		c.Request = c.Request.WithContext(timeoutCtx)

		// Process request with timeout
		done := make(chan bool)
		go func() {
			c.Next()
			done <- true
		}()

		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				logger.SysLogf("Request timeout for %s %s", c.Request.Method, c.Request.URL.Path)
				c.JSON(http.StatusRequestTimeout, gin.H{
					"error": "Request timeout",
					"code": "REQUEST_TIMEOUT",
					"timestamp": time.Now(),
				})
				c.Abort()
			}
		case <-done:
			// Request completed successfully
		}
	}
}

// IPRateLimiter provides IP-based rate limiting
func IPRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		
		// Check if IP is rate limited
		if config.IsIPRateLimited(clientIP) {
			logger.SecurityLogf("Rate limit exceeded for IP: %s", clientIP)
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"code": "RATE_LIMIT_EXCEEDED",
				"retry_after": config.GetRateLimitResetTime(clientIP),
				"timestamp": time.Now(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CORSMiddleware provides enhanced CORS handling
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Validate origin against whitelist
		if !isOriginAllowed(origin) {
			logger.SecurityLogf("CORS request from unauthorized origin: %s", origin)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Origin not allowed",
				"code": "CORS_ORIGIN_NOT_ALLOWED",
				"timestamp": time.Now(),
			})
			c.Abort()
			return
		}

		// Set CORS headers
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID, X-Response-Time")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks if the origin is in the whitelist
func isOriginAllowed(origin string) bool {
	// Allow empty origin (same-origin requests)
	if origin == "" {
		return true
	}

	// Check against configured allowed origins
	allowedOrigins := []string{
		"http://localhost:*",
		"https://localhost:*",
		"http://127.0.0.1:*",
		"https://127.0.0.1:*",
	}

	for _, allowed := range allowedOrigins {
		if matchOrigin(origin, allowed) {
			return true
		}
	}

	// Add more specific origins from config if needed
	return false
}

// matchOrigin matches origin against pattern with wildcards
func matchOrigin(origin, pattern string) bool {
	if pattern == origin {
		return true
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		matched, _ := regexp.MatchString(pattern, origin)
		return matched
	}

	return false
}

// SecurityLogger logs security-related events
func SecurityLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		
		// Process request
		c.Next()
		
		// Log security events
		if c.Writer.Status() >= 400 {
			logger.SecurityLogf("Security event - Method: %s, Path: %s, Status: %d, IP: %s, UserAgent: %s, Duration: %v",
				c.Request.Method,
				c.Request.URL.Path,
				c.Writer.Status(),
				c.ClientIP(),
				c.Request.UserAgent(),
				time.Since(startTime),
			)
		}
	}
}

// SanitizeInput removes potentially harmful content from input
func SanitizeInput() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for GET requests
		if c.Request.Method == "GET" {
			c.Next()
			return
		}

		// Parse and sanitize request body
		var requestData map[string]interface{}
		if err := c.ShouldBindJSON(&requestData); err == nil {
			sanitizedData := sanitizeRequestData(requestData)
			
			// Store sanitized data in context for later use
			c.Set("sanitized_request", sanitizedData)
		}

		c.Next()
	}
}

// sanitizeRequestData removes potentially harmful content
func sanitizeRequestData(data map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	
	for key, value := range data {
		switch v := value.(type) {
		case string:
			// Remove potential XSS payloads
			sanitized[key] = sanitizeString(v)
		case map[string]interface{}:
			// Recursively sanitize nested objects
			sanitized[key] = sanitizeRequestData(v)
		case []interface{}:
			// Sanitize array elements
			sanitizedArray := make([]interface{}, len(v))
			for i, item := range v {
				if str, ok := item.(string); ok {
					sanitizedArray[i] = sanitizeString(str)
				} else {
					sanitizedArray[i] = item
				}
			}
			sanitized[key] = sanitizedArray
		default:
			sanitized[key] = value
		}
	}
	
	return sanitized
}

// sanitizeString removes potentially harmful content from strings
func sanitizeString(input string) string {
	// Remove script tags and other potentially harmful content
	output := input
	
	// Basic XSS prevention
	harmfulPatterns := []string{
		"<script", "</script>", "javascript:", "data:text/html",
		"onload=", "onerror=", "onclick=", "onmouseover=",
	}
	
	for _, pattern := range harmfulPatterns {
		output = strings.ReplaceAll(output, pattern, "")
	}
	
	return output
}