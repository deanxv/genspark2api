package middleware

import (
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// AdminAuth creates middleware for admin authentication
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get admin key from header or query parameter
		adminKey := c.GetHeader("X-Admin-Key")
		if adminKey == "" {
			adminKey = c.Query("admin_key")
		}

		// Check if admin authentication is enabled
		if config.AdminKey == "" {
			// Admin authentication is disabled, allow access
			c.Set("user", "admin")
			c.Next()
			return
		}

		// Validate admin key
		if adminKey == "" {
			logger.SysLog("Admin access denied: missing admin key")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Admin authentication required",
				"message": "Missing X-Admin-Key header or admin_key query parameter",
			})
			c.Abort()
			return
		}

		// Check against configured admin keys (support multiple keys)
		adminKeys := strings.Split(config.AdminKey, ",")
		validKey := false
		for _, key := range adminKeys {
			if strings.TrimSpace(key) == adminKey {
				validKey = true
				break
			}
		}

		if !validKey {
			logger.SysLogf("Admin access denied: invalid admin key provided")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid admin key",
				"message": "The provided admin key is not valid",
			})
			c.Abort()
			return
		}

		// Set user context for audit logging
		c.Set("user", "admin")
		logger.SysLog("Admin access granted")
		
		c.Next()
	}
}

// RequireAdminOrAPIKey creates middleware that requires either admin key or valid API key
func RequireAdminOrAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try admin authentication first
		adminKey := c.GetHeader("X-Admin-Key")
		if adminKey != "" && config.AdminKey != "" {
			adminKeys := strings.Split(config.AdminKey, ",")
			for _, key := range adminKeys {
				if strings.TrimSpace(key) == adminKey {
					c.Set("user", "admin")
					c.Set("auth_type", "admin")
					c.Next()
					return
				}
			}
		}

		// Fall back to API key validation
		apiKey := c.GetHeader("Authorization")
		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}

		if strings.HasPrefix(apiKey, "Bearer ") {
			apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		}

		// Check against configured API secrets
		if apiKey != "" && len(config.ApiSecrets) > 0 {
			for _, secret := range config.ApiSecrets {
				if secret == apiKey {
					c.Set("user", "api_user")
					c.Set("auth_type", "api_key")
					c.Next()
					return
				}
			}
		}

		// Neither admin key nor API key is valid
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
			"message": "Valid X-Admin-Key or Authorization header required",
		})
		c.Abort()
	}
}