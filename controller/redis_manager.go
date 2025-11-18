package controller

import (
	"context"
	"encoding/json"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// RedisManager handles Redis operations for rate limiting and caching
type RedisManager struct {
	client   *redis.Client
	ctx      context.Context
	config   *RedisConfig
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Enabled  bool   `json:"enabled"`
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	PoolSize int    `json:"pool_size"`
}

// RateLimitStats represents rate limiting statistics
type RateLimitStats struct {
	TotalRequests    int64                  `json:"total_requests"`
	BlockedRequests  int64                  `json:"blocked_requests"`
	CurrentRates     map[string]RateInfo    `json:"current_rates"`
	RedisConnected   bool                   `json:"redis_connected"`
	LastUpdate       time.Time              `json:"last_update"`
}

// RateInfo contains rate limit information for a specific key
type RateInfo struct {
	Key          string  `json:"key"`
	CurrentCount int     `json:"current_count"`
	Limit        int     `json:"limit"`
	Window       string  `json:"window"`
	ResetTime    int64   `json:"reset_time"`
}

var GlobalRedisManager *RedisManager

// InitializeRedisManager initializes the global Redis manager
func InitializeRedisManager() error {
	redisConfig := &RedisConfig{
		Enabled:  config.RedisAddr != "",
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
		PoolSize: 100,
	}

	if !redisConfig.Enabled {
		logger.SysLog("Redis is disabled, using memory-based rate limiting")
		return nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     redisConfig.PoolSize,
		MinIdleConns: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.SysLogf("Failed to connect to Redis at %s: %v", redisConfig.Addr, err)
		return err
	}

	GlobalRedisManager = &RedisManager{
		client: client,
		ctx:    context.Background(),
		config: redisConfig,
	}

	logger.SysLogf("Redis manager initialized successfully at %s", redisConfig.Addr)
	return nil
}

// GetRedisStatus returns Redis connection status
func GetRedisStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled": GlobalRedisManager != nil && GlobalRedisManager.config.Enabled,
	}

	if GlobalRedisManager != nil && GlobalRedisManager.config.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := GlobalRedisManager.client.Ping(ctx).Err()
		status["connected"] = err == nil
		status["addr"] = GlobalRedisManager.config.Addr
		status["db"] = GlobalRedisManager.config.DB
		status["pool_size"] = GlobalRedisManager.config.PoolSize

		if err != nil {
			status["error"] = err.Error()
		}
	}

	return status
}

// GetRateLimitStats returns current rate limiting statistics
func GetRateLimitStats() *RateLimitStats {
	stats := &RateLimitStats{
		CurrentRates:   make(map[string]RateInfo),
		RedisConnected: GlobalRedisManager != nil && GlobalRedisManager.config.Enabled,
		LastUpdate:     time.Now(),
	}

	if !stats.RedisConnected {
		return stats
	}

	// Get rate limit information from Redis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Scan for rate limit keys
	pattern := "rate_limit:*"
	var cursor uint64
	var keys []string

	for {
		var scanKeys []string
		var err error
		scanKeys, cursor, err = GlobalRedisManager.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			logger.SysLogf("Failed to scan Redis keys: %v", err)
			break
		}

		keys = append(keys, scanKeys...)
		if cursor == 0 {
			break
		}
	}

	// Get information for each rate limit key
	for _, key := range keys {
		pipe := GlobalRedisManager.client.Pipeline()
		countCmd := pipe.ZCard(ctx, key)
		ttlCmd := pipe.TTL(ctx, key)

		_, err := pipe.Exec(ctx)
		if err != nil {
			continue
		}

		// Extract key type and identifier
		var keyType, identifier string
		if len(key) > 11 && key[:11] == "rate_limit:" {
			remaining := key[11:]
			parts := strings.Split(remaining, ":")
			if len(parts) >= 2 {
				keyType = parts[0]
				identifier = parts[1]
			}
		}

		info := RateInfo{
			Key:          key,
			CurrentCount: int(countCmd.Val()),
			Limit:        60, // Default limit, should be configurable
			Window:       "1m",
			ResetTime:    time.Now().Add(time.Minute).Unix(),
		}

		stats.CurrentRates[identifier] = info
	}

	return stats
}

// ClearRateLimit clears rate limit for a specific key
func ClearRateLimit(key string) error {
	if GlobalRedisManager == nil || !GlobalRedisManager.config.Enabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := GlobalRedisManager.client.Del(ctx, key).Err()
	if err != nil {
		logger.SysLogf("Failed to clear rate limit for key %s: %v", key, err)
		return err
	}

	logger.SysLogf("Rate limit cleared for key: %s", key)
	return nil
}

// ConfigureRateLimit updates rate limit configuration
func ConfigureRateLimit(endpoint string, requests int, window time.Duration) error {
	if GlobalRedisManager == nil || !GlobalRedisManager.config.Enabled {
		return nil
	}

	// Update configuration in global config manager
	config := GlobalConfigManager.GetCurrentConfig()
	config.RateLimitRPS = requests
	
	// This would need to be extended to support per-endpoint configuration
	logger.SysLogf("Rate limit configuration updated: %d requests per %v", requests, window)
	return nil
}

// RedisStatusHandler returns Redis connection status
func RedisStatusHandler(c *gin.Context) {
	status := GetRedisStatus()
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"redis":  status,
		"timestamp": time.Now(),
	})
}

// RateLimitStatsHandler returns rate limiting statistics
func RateLimitStatsHandler(c *gin.Context) {
	stats := GetRateLimitStats()
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"stats":  stats,
		"timestamp": time.Now(),
	})
}

// ClearRateLimitHandler clears rate limit for a specific key
func ClearRateLimitHandler(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing key parameter",
			"message": "Please provide a rate limit key to clear",
		})
		return
	}

	err := ClearRateLimit(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to clear rate limit",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Rate limit cleared",
		"key": key,
	})
}

// ConfigureRateLimitHandler updates rate limit configuration
func ConfigureRateLimitHandler(c *gin.Context) {
	var config struct {
		Endpoint string `json:"endpoint" binding:"required"`
		Requests int    `json:"requests" binding:"required,min=1"`
		Window   int    `json:"window" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	err := ConfigureRateLimit(config.Endpoint, config.Requests, time.Duration(config.Window)*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to configure rate limit",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Rate limit configuration updated",
		"endpoint": config.Endpoint,
		"requests": config.Requests,
		"window_seconds": config.Window,
	})
}