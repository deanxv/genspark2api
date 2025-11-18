package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// RedisRateLimiter provides distributed rate limiting using Redis
type RedisRateLimiter struct {
	client       *redis.Client
	script       *redis.Script
	defaultLimit RateLimitConfig
}

// RateLimitConfig defines rate limiting parameters
type RateLimitConfig struct {
	Requests  int           // Number of requests allowed
	Window    time.Duration // Time window for rate limiting
	KeyPrefix string        // Redis key prefix
}

// slidingWindowRateLimitScript is a Lua script for sliding window rate limiting
const slidingWindowRateLimitScript = `
local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local current = tonumber(ARGV[3])

-- Clean up old entries
redis.call('ZREMRANGEBYSCORE', key, '-inf', current - window)

-- Count current entries
local count = redis.call('ZCARD', key)

if count >= limit then
    -- Rate limit exceeded
    return {0, count}
end

-- Add current request
redis.call('ZADD', key, current, current)
redis.call('EXPIRE', key, window)

return {1, count + 1}
`

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(redisAddr string, password string, db int) (*RedisRateLimiter, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	limiter := &RedisRateLimiter{
		client: client,
		defaultLimit: RateLimitConfig{
			Requests:  60,              // 60 requests
			Window:    1 * time.Minute,   // per minute
			KeyPrefix: "rate_limit:",
		},
	}

	return limiter, nil
}

// RateLimitByIP implements IP-based rate limiting with sliding window
func (rl *RedisRateLimiter) RateLimitByIP(ip string, limit *RateLimitConfig) (bool, int, error) {
	if limit == nil {
		limit = &rl.defaultLimit
	}

	key := limit.KeyPrefix + "ip:" + ip
	now := time.Now().Unix()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Execute Lua script for atomic sliding window rate limiting
	result, err := rl.client.Eval(
		ctx,
		slidingWindowRateLimitScript,
		[]string{key},
		int64(limit.Window.Seconds()),
		limit.Requests,
		now,
	).Result()

	if err != nil {
		logger.SysLogf("Redis rate limit error for IP %s: %v", ip, err)
		return false, 0, err
	}

	// Parse result
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) != 2 {
		return false, 0, fmt.Errorf("unexpected result format from Redis script")
	}

	allowed, ok1 := resultArray[0].(int64)
	currentCount, ok2 := resultArray[1].(int64)

	if !ok1 || !ok2 {
		return false, 0, fmt.Errorf("failed to parse Redis result")
	}

	return allowed == 1, int(currentCount), nil
}

// RateLimitByAPIKey implements API key-based rate limiting
func (rl *RedisRateLimiter) RateLimitByAPIKey(apiKey string, limit *RateLimitConfig) (bool, int, error) {
	if limit == nil {
		limit = &rl.defaultLimit
	}

	key := limit.KeyPrefix + "api_key:" + apiKey
	now := time.Now().Unix()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := rl.client.Eval(
		ctx,
		slidingWindowRateLimitScript,
		[]string{key},
		int64(limit.Window.Seconds()),
		limit.Requests,
		now,
	).Result()

	if err != nil {
		logger.SysLogf("Redis rate limit error for API key: %v", err)
		return false, 0, err
	}

	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) != 2 {
		return false, 0, fmt.Errorf("unexpected result format from Redis script")
	}

	allowed, ok1 := resultArray[0].(int64)
	currentCount, ok2 := resultArray[1].(int64)

	if !ok1 || !ok2 {
		return false, 0, fmt.Errorf("failed to parse Redis result")
	}

	return allowed == 1, int(currentCount), nil
}

// RateLimitByEndpoint implements endpoint-based rate limiting
func (rl *RedisRateLimiter) RateLimitByEndpoint(endpoint string, identifier string, limit *RateLimitConfig) (bool, int, error) {
	if limit == nil {
		limit = &rl.defaultLimit
	}

	key := limit.KeyPrefix + "endpoint:" + endpoint + ":" + identifier
	now := time.Now().Unix()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := rl.client.Eval(
		ctx,
		slidingWindowRateLimitScript,
		[]string{key},
		int64(limit.Window.Seconds()),
		limit.Requests,
		now,
	).Result()

	if err != nil {
		logger.SysLogf("Redis rate limit error for endpoint %s: %v", endpoint, err)
		return false, 0, err
	}

	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) != 2 {
		return false, 0, fmt.Errorf("unexpected result format from Redis script")
	}

	allowed, ok1 := resultArray[0].(int64)
	currentCount, ok2 := resultArray[1].(int64)

	if !ok1 || !ok2 {
		return false, 0, fmt.Errorf("failed to parse Redis result")
	}

	return allowed == 1, int(currentCount), nil
}

// GetRateLimitInfo returns current rate limit information
func (rl *RedisRateLimiter) GetRateLimitInfo(key string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Get current count and TTL
	pipe := rl.client.Pipeline()
	countCmd := pipe.ZCard(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	info := map[string]interface{}{
		"key":        key,
		"count":      countCmd.Val(),
		"ttl_seconds": ttlCmd.Val().Seconds(),
		"timestamp":  time.Now().Unix(),
	}

	return info, nil
}

// Close closes the Redis connection
func (rl *RedisRateLimiter) Close() error {
	return rl.client.Close()
}

// RedisRateLimitMiddleware creates a Gin middleware for Redis-based rate limiting
func RedisRateLimitMiddleware() gin.HandlerFunc {
	// Initialize Redis rate limiter
	redisAddr := config.RedisAddr
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rateLimiter, err := NewRedisRateLimiter(redisAddr, config.RedisPassword, config.RedisDB)
	if err != nil {
		logger.SysLogf("Failed to initialize Redis rate limiter: %v", err)
		// Fall back to memory-based rate limiting
		return RequestRateLimit()
	}

	return func(c *gin.Context) {
		// Get client IP
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		// Get current rate limit config from runtime config
		currentConfig := getCurrentRateLimitConfig()

		// Apply rate limiting
		allowed, currentCount, err := rateLimiter.RateLimitByIP(ip, currentConfig)
		if err != nil {
			logger.SysLogf("Rate limiting error: %v", err)
			// Continue with request on rate limit error
			c.Next()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(currentConfig.Requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(currentConfig.Requests-currentCount))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(currentConfig.Window).Unix(), 10))

		if !allowed {
			// Rate limit exceeded
			c.Header("Retry-After", strconv.Itoa(int(currentConfig.Window.Seconds())))
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"message": fmt.Sprintf("Too many requests. Limit: %d requests per %v", currentConfig.Requests, currentConfig.Window),
				"retry_after": int(currentConfig.Window.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getCurrentRateLimitConfig gets the current rate limit configuration
func getCurrentRateLimitConfig() *RateLimitConfig {
	// This would integrate with the configuration management system
	// For now, use default values
	return &RateLimitConfig{
		Requests:  config.GlobalConfigManager.GetCurrentConfig().RateLimitRPS,
		Window:    time.Minute,
		KeyPrefix: "rate_limit:",
	}
}

// AdvancedRateLimitMiddleware provides advanced rate limiting with multiple strategies
func AdvancedRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting for health and metrics endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// Get client identifier (IP or API key)
		identifier := c.ClientIP()
		if identifier == "" {
			identifier = "unknown"
		}

		// Check for API key in header
		apiKey := c.GetHeader("Authorization")
		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}
		if apiKey != "" {
			identifier = "api_" + apiKey[:10] // Use first 10 chars of API key
		}

		// Apply different rate limits based on endpoint
		endpoint := c.Request.Method + " " + c.Request.URL.Path
		
		// Check if Redis is available
		if config.RedisAddr != "" {
			// Use Redis rate limiting
			RedisRateLimitMiddleware()(c)
		} else {
			// Fall back to memory-based rate limiting
			RequestRateLimit()(c)
		}
	}
}