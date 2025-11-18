package middleware

import (
	"genspark2api/controller"
	logger "genspark2api/common/loggger"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsMiddleware collects request metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		
		// Get model from request if available
		model := extractModelFromRequest(c)
		
		// Process request
		c.Next()
		
		// Calculate response time
		responseTime := time.Since(startTime).Milliseconds()
		statusCode := c.Writer.Status()
		
		// Determine if request was successful
		success := statusCode >= 200 && statusCode < 400
		
		// Create endpoint identifier
		endpoint := method + " " + path
		
		// Record metrics
		controller.GlobalMetrics.RecordRequest(endpoint, model, statusCode, float64(responseTime), success)
		
		// Add response time header for debugging
		c.Header("X-Response-Time", strconv.FormatInt(responseTime, 10)+"ms")
		
		logger.SysLogf("Request: %s %s - Status: %d - Time: %dms", method, path, statusCode, responseTime)
	}
}

// extractModelFromRequest extracts model from request body or query params
func extractModelFromRequest(c *gin.Context) string {
	// Try to get model from query parameters first
	if model := c.Query("model"); model != "" {
		return model
	}
	
	// Try to get model from form data
	if model := c.PostForm("model"); model != "" {
		return model
	}
	
	// Try to parse JSON body for model (common for OpenAI API)
	if c.Request.Header.Get("Content-Type") == "application/json" {
		// Create a temporary struct to parse just the model field
		var requestData struct {
			Model string `json:"model"`
		}
		
		// Use ShouldBindJSON but don't consume the original body
		if err := c.ShouldBindJSON(&requestData); err == nil && requestData.Model != "" {
			return requestData.Model
		}
	}
	
	return "unknown"
}

// RequestLoggingMiddleware provides detailed request logging
func RequestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		
		// Log request details
		logger.SysLogf("Incoming Request: %s %s from %s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
		
		// Process request
		c.Next()
		
		// Log response details
		duration := time.Since(startTime)
		logger.SysLogf("Response: %d %s in %v", c.Writer.Status(), c.Request.URL.Path, duration)
		
		// Log any errors
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				logger.SysLogf("Request Error: %s - %s", c.Request.URL.Path, err.Error())
			}
		}
	}
}