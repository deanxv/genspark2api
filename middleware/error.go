package middleware

import (
	"encoding/json"
	"fmt"
	"genspark2api/common"
	logger "genspark2api/common/loggger"
	"genspark2api/common/config"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error       string                 `json:"error"`
	Message     string                 `json:"message"`
	Code        string                 `json:"code"`
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id"`
	Details     map[string]interface{} `json:"details,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
}

// ErrorMiddleware provides comprehensive error handling and logging
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a custom response writer to capture the status
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			statusCode:     http.StatusOK,
			body:           make([]byte, 0),
		}
		c.Writer = writer

		// Process the request
		c.Next()

		// Handle errors after request processing
		if len(c.Errors) > 0 {
			handleErrors(c, writer)
		}

		// Log request details for debugging
		if writer.statusCode >= 400 {
			logErrorRequest(c, writer)
		}
	}
}

// RecoveryMiddleware recovers from panics and logs detailed error information
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get request ID for tracking
				requestID := c.GetString("request_id")
				
				// Log the panic with detailed information
				logger.ErrorLogf("Panic recovered - RequestID: %s, Error: %v, Stack: %s", 
					requestID, err, string(debug.Stack()))

				// Send error response
				errorResponse := ErrorResponse{
					Error:      "Internal Server Error",
					Message:    "An unexpected error occurred while processing your request",
					Code:       "INTERNAL_ERROR",
					Timestamp:  time.Now(),
					RequestID:  requestID,
				}

				// Add stack trace in debug mode
				if config.DebugEnabled {
					errorResponse.StackTrace = string(debug.Stack())
				}

				c.JSON(http.StatusInternalServerError, errorResponse)
				c.Abort()
			}
		}()

		c.Next()
	}
}

// handleErrors processes and logs errors from the request
func handleErrors(c *gin.Context, writer *responseWriter) {
	for _, err := range c.Errors {
		requestID := c.GetString("request_id")
		
		// Determine error type and status code
		statusCode, errorType, errorCode := classifyError(err.Err)
		
		// Log the error with context
		logger.ErrorLogf("Request Error - RequestID: %s, Type: %s, Code: %s, Error: %v, Path: %s, Method: %s, Status: %d",
			requestID, errorType, errorCode, err.Err, c.Request.URL.Path, c.Request.Method, statusCode)

		// Create error response
		errorResponse := ErrorResponse{
			Error:     errorType,
			Message:   getErrorMessage(err.Err, errorCode),
			Code:      errorCode,
			Timestamp: time.Now(),
			RequestID: requestID,
		}

		// Add additional details based on error type
		if statusCode == http.StatusBadRequest {
			errorResponse.Details = getValidationDetails(err.Err)
		}

		// Add stack trace in debug mode for internal errors
		if config.DebugEnabled && statusCode >= 500 {
			errorResponse.StackTrace = string(debug.Stack())
		}

		// Send error response
		c.JSON(statusCode, errorResponse)
		c.Abort()
		return
	}
}

// classifyError determines the error type and appropriate HTTP status code
func classifyError(err error) (int, string, string) {
	if err == nil {
		return http.StatusInternalServerError, "Unknown Error", "UNKNOWN_ERROR"
	}

	errStr := err.Error()

	// Classify based on error message patterns
	switch {
	case contains(errStr, "validation") || contains(errStr, "invalid"):
		return http.StatusBadRequest, "Validation Error", "VALIDATION_ERROR"
	case contains(errStr, "authentication") || contains(errStr, "unauthorized"):
		return http.StatusUnauthorized, "Authentication Error", "AUTH_ERROR"
	case contains(errStr, "rate limit") || contains(errStr, "too many requests"):
		return http.StatusTooManyRequests, "Rate Limit Exceeded", "RATE_LIMIT_ERROR"
	case contains(errStr, "not found"):
		return http.StatusNotFound, "Not Found", "NOT_FOUND_ERROR"
	case contains(errStr, "timeout"):
		return http.StatusRequestTimeout, "Request Timeout", "TIMEOUT_ERROR"
	case contains(errStr, "cookie") || contains(errStr, "session"):
		return http.StatusUnauthorized, "Session Error", "SESSION_ERROR"
	case contains(errStr, "cloudflare") || contains(errStr, "captcha"):
		return http.StatusServiceUnavailable, "Service Unavailable", "CLOUDFLARE_ERROR"
	default:
		return http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR"
	}
}

// getErrorMessage returns user-friendly error messages
func getErrorMessage(err error, code string) string {
	if err == nil {
		return "An unknown error occurred"
	}

	// Map error codes to user-friendly messages
	messages := map[string]string{
		"VALIDATION_ERROR":     "The request data is invalid. Please check your input.",
		"AUTH_ERROR":           "Authentication failed. Please check your API key.",
		"RATE_LIMIT_ERROR":     "Too many requests. Please slow down and try again later.",
		"NOT_FOUND_ERROR":      "The requested resource was not found.",
		"TIMEOUT_ERROR":        "The request timed out. Please try again.",
		"SESSION_ERROR":        "Session expired or invalid. Please check your cookie configuration.",
		"CLOUDFLARE_ERROR":     "Service temporarily unavailable due to Cloudflare protection. Please try again.",
		"INTERNAL_ERROR":       "An internal server error occurred. Please try again later.",
	}

	if msg, exists := messages[code]; exists {
		return msg
	}

	return "An error occurred while processing your request"
}

// getValidationDetails extracts validation error details
func getValidationDetails(err error) map[string]interface{} {
	details := make(map[string]interface{})
	
	// Try to parse validation errors
	if jsonErr := json.Unmarshal([]byte(err.Error()), &details); jsonErr == nil {
		return details
	}

	return map[string]interface{}{
		"error": err.Error(),
	}
}

// logErrorRequest logs detailed error information for debugging
func logErrorRequest(c *gin.Context, writer *responseWriter) {
	requestID := c.GetString("request_id")
	
	// Create detailed error log
	errorLog := map[string]interface{}{
		"timestamp":    time.Now(),
		"request_id": requestID,
		"method":     c.Request.Method,
		"path":       c.Request.URL.Path,
		"status":     writer.statusCode,
		"client_ip":  c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
		"latency":    time.Since(c.GetTime("request_start")),
	}

	// Add request headers (excluding sensitive data)
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if key != "Authorization" && key != "Cookie" && key != "X-Api-Key" {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}
	errorLog["headers"] = headers

	// Log the error details
	logger.ErrorLogf("HTTP Error - %v", errorLog)
}

// responseWriter wraps the response writer to capture status and body
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
	body       []byte
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return w.ResponseWriter.Write(data)
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr ||
		      containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}