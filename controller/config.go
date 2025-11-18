package controller

import (
	"encoding/json"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ConfigManager handles dynamic configuration management
type ConfigManager struct {
	mu              sync.RWMutex
	currentConfig   *RuntimeConfig
	configHistory   []ConfigChange
	configValidator *ConfigValidator
}

// RuntimeConfig represents the current runtime configuration
type RuntimeConfig struct {
	// API Configuration
	RateLimitRPS     int    `json:"rate_limit_rps"`
	RateLimitBurst    int    `json:"rate_limit_burst"`
	MaxRequestSize   int64  `json:"max_request_size"`
	RequestTimeout     int    `json:"request_timeout"`
	
	// Cache Configuration
	CacheEnabled      bool   `json:"cache_enabled"`
	CacheTTL          int    `json:"cache_ttl"`
	CacheMaxSize      int    `json:"cache_max_size"`
	
	// Security Configuration
	SecurityHeaders   bool   `json:"security_headers"`
	CORSOrigins       []string `json:"cors_origins"`
	IPWhitelist       []string `json:"ip_whitelist"`
	IPBlacklist       []string `json:"ip_blacklist"`
	
	// Logging Configuration
	LogLevel          string `json:"log_level"`
	LogRequests       bool   `json:"log_requests"`
	LogResponses      bool   `json:"log_responses"`
	
	// Feature Flags
	MetricsEnabled    bool   `json:"metrics_enabled"`
	ValidationEnabled bool   `json:"validation_enabled"`
	DebugMode         bool   `json:"debug_mode"`
	
	// Model Configuration
	DefaultModel      string `json:"default_model"`
	MaxTokens        int    `json:"max_tokens"`
	Temperature      float64 `json:"temperature"`
	
	// Performance Configuration
	WorkerPoolSize   int    `json:"worker_pool_size"`
	MaxConcurrent    int    `json:"max_concurrent"`
	QueueSize        int    `json:"queue_size"`
	
	// Health Configuration
	HealthCheckInterval int `json:"health_check_interval"`
	HealthCheckTimeout  int `json:"health_check_timeout"`
}

// ConfigChange represents a configuration change
type ConfigChange struct {
	Timestamp   time.Time              `json:"timestamp"`
	User        string                 `json:"user"`
	Action      string                 `json:"action"`
	Key         string                 `json:"key"`
	OldValue    interface{}            `json:"old_value"`
	NewValue    interface{}            `json:"new_value"`
	Description string                 `json:"description"`
}

// ConfigValidator validates configuration changes
type ConfigValidator struct {
	rules map[string][]ValidationRule
}

// ValidationRule represents a validation rule for configuration
type ValidationRule struct {
	Type        string
	Min         interface{}
	Max         interface{}
	Options     []interface{}
	Required    bool
	CustomFunc  func(interface{}) error
}

// GlobalConfigManager is the global configuration manager instance
var GlobalConfigManager *ConfigManager

// Initialize configuration manager
func init() {
	GlobalConfigManager = NewConfigManager()
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	defaultConfig := &RuntimeConfig{
		RateLimitRPS:      60,
		RateLimitBurst:    100,
		MaxRequestSize:    10 * 1024 * 1024, // 10MB
		RequestTimeout:    30,
		CacheEnabled:      true,
		CacheTTL:        300,
		CacheMaxSize:    1000,
		SecurityHeaders: true,
		CORSOrigins:     []string{"*"},
		LogLevel:        "info",
		LogRequests:     true,
		LogResponses:    false,
		MetricsEnabled:  true,
		ValidationEnabled: true,
		DebugMode:       false,
		DefaultModel:    "gpt-4o",
		MaxTokens:      4096,
		Temperature:    0.7,
		WorkerPoolSize: 10,
		MaxConcurrent:  100,
		QueueSize:      1000,
		HealthCheckInterval: 30,
		HealthCheckTimeout:  5,
	}

	return &ConfigManager{
		currentConfig:   defaultConfig,
		configHistory:   make([]ConfigChange, 0),
		configValidator: NewConfigValidator(),
	}
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		rules: map[string][]ValidationRule{
			"rate_limit_rps": {
				{Type: "int", Min: 1, Max: 10000},
			},
			"max_request_size": {
				{Type: "int", Min: 1024, Max: 100 * 1024 * 1024}, // 1KB to 100MB
			},
			"cache_ttl": {
				{Type: "int", Min: 1, Max: 86400}, // 1 second to 24 hours
			},
			"log_level": {
				{Type: "string", Options: []interface{}{"debug", "info", "warn", "error"}},
			},
			"temperature": {
				{Type: "float", Min: 0.0, Max: 2.0},
			},
			"max_tokens": {
				{Type: "int", Min: 1, Max: 32768},
			},
		},
	}
}

// GetCurrentConfig returns the current runtime configuration
func (cm *ConfigManager) GetCurrentConfig() *RuntimeConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// Return a copy to prevent external modifications
	configCopy := *cm.currentConfig
	return &configCopy
}

// UpdateConfig updates a configuration value
func (cm *ConfigManager) UpdateConfig(key string, value interface{}, user, description string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Validate the configuration change
	if err := cm.configValidator.Validate(key, value); err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	// Get old value
	oldValue := cm.getConfigValue(key)

	// Update the configuration
	if err := cm.setConfigValue(key, value); err != nil {
		return fmt.Errorf("failed to update config: %v", err)
	}

	// Log the change
	change := ConfigChange{
		Timestamp:   time.Now(),
		User:        user,
		Action:      "update",
		Key:         key,
		OldValue:    oldValue,
		NewValue:    value,
		Description: description,
	}

	cm.configHistory = append(cm.configHistory, change)
	
	// Keep only last 100 changes
	if len(cm.configHistory) > 100 {
		cm.configHistory = cm.configHistory[len(cm.configHistory)-100:]
	}

	logger.SysLogf("Configuration updated: %s = %v (by %s)", key, value, user)
	
	return nil
}

// getConfigValue gets a configuration value by key
func (cm *ConfigManager) getConfigValue(key string) interface{} {
	configValue := reflect.ValueOf(cm.currentConfig).Elem()
	field := configValue.FieldByName(key)
	
	if !field.IsValid() {
		return nil
	}
	
	return field.Interface()
}

// setConfigValue sets a configuration value by key
func (cm *ConfigManager) setConfigValue(key string, value interface{}) error {
	configValue := reflect.ValueOf(cm.currentConfig).Elem()
	field := configValue.FieldByName(key)
	
	if !field.IsValid() {
		return fmt.Errorf("invalid configuration key: %s", key)
	}
	
	if !field.CanSet() {
		return fmt.Errorf("cannot set configuration key: %s", key)
	}
	
	// Convert value to appropriate type
	convertedValue := reflect.ValueOf(value)
	if convertedValue.Type().ConvertibleTo(field.Type()) {
		field.Set(convertedValue.Convert(field.Type()))
	} else {
		return fmt.Errorf("cannot convert %v to %s", value, field.Type())
	}
	
	return nil
}

// GetConfigHistory returns configuration change history
func (cm *ConfigManager) GetConfigHistory(limit int) []ConfigChange {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if limit <= 0 || limit > len(cm.configHistory) {
		limit = len(cm.configHistory)
	}

	start := len(cm.configHistory) - limit
	return cm.configHistory[start:]
}

// ResetToDefaults resets configuration to default values
func (cm *ConfigManager) ResetToDefaults(user string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Create new default config
	defaultConfig := &RuntimeConfig{
		RateLimitRPS:      60,
		RateLimitBurst:    100,
		MaxRequestSize:    10 * 1024 * 1024,
		RequestTimeout:    30,
		CacheEnabled:      true,
		CacheTTL:        300,
		CacheMaxSize:    1000,
		SecurityHeaders: true,
		CORSOrigins:     []string{"*"},
		LogLevel:        "info",
		LogRequests:     true,
		LogResponses:    false,
		MetricsEnabled:  true,
		ValidationEnabled: true,
		DebugMode:       false,
		DefaultModel:    "gpt-4o",
		MaxTokens:      4096,
		Temperature:    0.7,
		WorkerPoolSize: 10,
		MaxConcurrent:  100,
		QueueSize:      1000,
		HealthCheckInterval: 30,
		HealthCheckTimeout:  5,
	}

	cm.currentConfig = defaultConfig

	change := ConfigChange{
		Timestamp:   time.Now(),
		User:        user,
		Action:      "reset",
		Description: "Configuration reset to default values",
	}

	cm.configHistory = append(cm.configHistory, change)

	logger.SysLogf("Configuration reset to defaults by %s", user)
	return nil
}

// Validate validates a configuration value
func (cv *ConfigValidator) Validate(key string, value interface{}) error {
	rules, exists := cv.rules[key]
	if !exists {
		return nil // No validation rules for this key
	}

	for _, rule := range rules {
		if err := cv.validateRule(value, rule); err != nil {
			return err
		}
	}

	return nil
}

// validateRule validates a single rule
func (cv *ConfigValidator) validateRule(value interface{}, rule ValidationRule) error {
	// Type validation
	switch rule.Type {
	case "int":
		if _, ok := value.(int); !ok {
			return fmt.Errorf("value must be an integer")
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("value must be a string")
		}
	case "float", "float64":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("value must be a float")
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("value must be a boolean")
		}
	}

	// Min/Max validation
	if rule.Min != nil {
		if err := cv.validateMin(value, rule.Min); err != nil {
			return err
		}
	}

	if rule.Max != nil {
		if err := cv.validateMax(value, rule.Max); err != nil {
			return err
		}
	}

	// Options validation
	if len(rule.Options) > 0 {
		found := false
		for _, option := range rule.Options {
			if value == option {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value must be one of: %v", rule.Options)
		}
	}

	// Custom validation
	if rule.CustomFunc != nil {
		if err := rule.CustomFunc(value); err != nil {
			return err
		}
	}

	return nil
}

// validateMin validates minimum value
func (cv *ConfigValidator) validateMin(value, min interface{}) error {
	switch v := value.(type) {
	case int:
		if minInt, ok := min.(int); ok && v < minInt {
			return fmt.Errorf("value must be >= %d", minInt)
		}
	case float64:
		if minFloat, ok := min.(float64); ok && v < minFloat {
			return fmt.Errorf("value must be >= %f", minFloat)
		}
	}
	return nil
}

// validateMax validates maximum value
func (cv *ConfigValidator) validateMax(value, max interface{}) error {
	switch v := value.(type) {
	case int:
		if maxInt, ok := max.(int); ok && v > maxInt {
			return fmt.Errorf("value must be <= %d", maxInt)
		}
	case float64:
		if maxFloat, ok := max.(float64); ok && v > maxFloat {
			return fmt.Errorf("value must be <= %f", maxFloat)
		}
	}
	return nil
}

// HTTP Handlers
func GetCurrentConfig(c *gin.Context) {
	config := GlobalConfigManager.GetCurrentConfig()
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"config": config,
		"timestamp": time.Now(),
	})
}

func UpdateConfig(c *gin.Context) {
	var updateRequest struct {
		Key         string      `json:"key" binding:"required"`
		Value       interface{} `json:"value" binding:"required"`
		Description string      `json:"description"`
	}
	
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}
	
	// Get user from context (would be set by authentication middleware)
	user := c.GetString("user")
	if user == "" {
		user = "anonymous"
	}
	
	err := GlobalConfigManager.UpdateConfig(
		updateRequest.Key,
		updateRequest.Value,
		user,
		updateRequest.Description,
	)
	
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to update configuration",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Configuration updated successfully",
		"key": updateRequest.Key,
		"value": updateRequest.Value,
		"timestamp": time.Now(),
	})
}

func GetConfigHistory(c *gin.Context) {
	limit := 50 // Default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	
	history := GlobalConfigManager.GetConfigHistory(limit)
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"history": history,
		"count": len(history),
	})
}

func ResetConfig(c *gin.Context) {
	var resetRequest struct {
		Description string `json:"description"`
	}
	
	c.ShouldBindJSON(&resetRequest)
	
	// Get user from context
	user := c.GetString("user")
	if user == "" {
		user = "anonymous"
	}
	
	err := GlobalConfigManager.ResetToDefaults(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to reset configuration",
			"details": err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Configuration reset to defaults",
		"description": resetRequest.Description,
	})
}