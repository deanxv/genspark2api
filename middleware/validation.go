package middleware

import (
	"encoding/json"
	"fmt"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// ValidationRule represents a validation rule
type ValidationRule struct {
	Field    string
	Required bool
	Type     string
	Min      interface{}
	Max      interface{}
	Pattern  string
	Custom   func(interface{}) error
}

// ValidationMiddleware provides request validation
func ValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip validation for GET requests
		if c.Request.Method == "GET" {
			c.Next()
			return
		}

		// Get validation rules based on endpoint
		rules := getValidationRules(c.Request.URL.Path, c.Request.Method)
		if len(rules) == 0 {
			c.Next()
			return
		}

		// Parse request body
		var requestData map[string]interface{}
		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate request data
		validationErrors := validateData(requestData, rules)
		if len(validationErrors) > 0 {
			logger.SysLogf("Validation failed for %s %s: %v", c.Request.Method, c.Request.URL.Path, validationErrors)
			
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Validation failed",
				"details": validationErrors,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getValidationRules returns validation rules based on endpoint
func getValidationRules(path, method string) []ValidationRule {
	switch {
	case strings.Contains(path, "/chat/completions"):
		return getChatCompletionRules()
	case strings.Contains(path, "/images/generations"):
		return getImageGenerationRules()
	case strings.Contains(path, "/videos/generations"):
		return getVideoGenerationRules()
	default:
		return []ValidationRule{}
	}
}

// getChatCompletionRules returns validation rules for chat completions
func getChatCompletionRules() []ValidationRule {
	return []ValidationRule{
		{
			Field:    "model",
			Required: true,
			Type:     "string",
			Min:      1,
			Max:      100,
		},
		{
			Field:    "messages",
			Required: true,
			Type:     "array",
			Min:      1,
		},
		{
			Field: "temperature",
			Type:  "number",
			Min:   0.0,
			Max:   2.0,
		},
		{
			Field: "max_tokens",
			Type:  "integer",
			Min:   1,
			Max:   8192,
		},
		{
			Field: "stream",
			Type:  "boolean",
		},
	}
}

// getImageGenerationRules returns validation rules for image generation
func getImageGenerationRules() []ValidationRule {
	return []ValidationRule{
		{
			Field:    "model",
			Required: true,
			Type:     "string",
			Min:      1,
			Max:      100,
		},
		{
			Field:    "prompt",
			Required: true,
			Type:     "string",
			Min:      1,
			Max:      4000,
		},
		{
			Field: "n",
			Type:  "integer",
			Min:   1,
			Max:   10,
		},
		{
			Field: "size",
			Type:  "string",
			Pattern: "^(256x256|512x512|1024x1024)$",
		},
	}
}

// getVideoGenerationRules returns validation rules for video generation
func getVideoGenerationRules() []ValidationRule {
	return []ValidationRule{
		{
			Field:    "model",
			Required: true,
			Type:     "string",
			Min:      1,
			Max:      100,
		},
		{
			Field:    "prompt",
			Required: true,
			Type:     "string",
			Min:      1,
			Max:      2000,
		},
		{
			Field: "aspect_ratio",
			Type:  "string",
			Pattern: "^(16:9|9:16|4:3|3:4|1:1)$",
		},
		{
			Field: "duration",
			Type:  "integer",
			Min:   2,
			Max:   60,
		},
		{
			Field: "auto_prompt",
			Type:  "boolean",
		},
	}
}

// validateData validates request data against rules
func validateData(data map[string]interface{}, rules []ValidationRule) map[string]string {
	errors := make(map[string]string)

	for _, rule := range rules {
		value, exists := data[rule.Field]

		// Check required fields
		if rule.Required && !exists {
			errors[rule.Field] = fmt.Sprintf("%s is required", rule.Field)
			continue
		}

		// Skip if field doesn't exist and not required
		if !exists {
			continue
		}

		// Validate field type
		if err := validateFieldType(value, rule.Type); err != nil {
			errors[rule.Field] = fmt.Sprintf("%s must be %s: %v", rule.Field, rule.Type, err)
			continue
		}

		// Validate field constraints
		if err := validateFieldConstraints(value, rule); err != nil {
			errors[rule.Field] = err.Error()
			continue
		}

		// Run custom validation if provided
		if rule.Custom != nil {
			if err := rule.Custom(value); err != nil {
				errors[rule.Field] = err.Error()
			}
		}
	}

	return errors
}

// validateFieldType validates the type of a field
func validateFieldType(value interface{}, expectedType string) error {
	if value == nil {
		return fmt.Errorf("value is nil")
	}

	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "integer", "int":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
		// JSON numbers are float64, check if it's a whole number
		if float64(int64(value.(float64))) != value.(float64) {
			return fmt.Errorf("expected integer, got float")
		}
	case "number", "float":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
	case "boolean", "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	default:
		return fmt.Errorf("unknown type: %s", expectedType)
	}

	return nil
}

// validateFieldConstraints validates field constraints (min, max, pattern)
func validateFieldConstraints(value interface{}, rule ValidationRule) error {
	switch rule.Type {
	case "string":
		strValue := value.(string)
		
		// Check minimum length
		if rule.Min != nil {
			if minLength, ok := rule.Min.(int); ok && len(strValue) < minLength {
				return fmt.Errorf("must be at least %d characters", minLength)
			}
		}

		// Check maximum length
		if rule.Max != nil {
			if maxLength, ok := rule.Max.(int); ok && len(strValue) > maxLength {
				return fmt.Errorf("must be at most %d characters", maxLength)
			}
		}

		// Check pattern
		if rule.Pattern != "" {
			matched, err := regexp.MatchString(rule.Pattern, strValue)
			if err != nil || !matched {
				return fmt.Errorf("must match pattern: %s", rule.Pattern)
			}
		}

	case "integer", "int":
		intValue := int(value.(float64))
		
		// Check minimum value
		if rule.Min != nil {
			switch min := rule.Min.(type) {
			case int:
				if intValue < min {
					return fmt.Errorf("must be at least %d", min)
				}
			case float64:
				if float64(intValue) < min {
					return fmt.Errorf("must be at least %v", min)
				}
			}
		}

		// Check maximum value
		if rule.Max != nil {
			switch max := rule.Max.(type) {
			case int:
				if intValue > max {
					return fmt.Errorf("must be at most %d", max)
				}
			case float64:
				if float64(intValue) > max {
					return fmt.Errorf("must be at most %v", max)
				}
			}
		}

	case "number", "float":
		floatValue := value.(float64)
		
		// Check minimum value
		if rule.Min != nil {
			if minFloat, ok := rule.Min.(float64); ok && floatValue < minFloat {
				return fmt.Errorf("must be at least %v", minFloat)
			}
		}

		// Check maximum value
		if rule.Max != nil {
			if maxFloat, ok := rule.Max.(float64); ok && floatValue > maxFloat {
				return fmt.Errorf("must be at most %v", maxFloat)
			}
		}

	case "array":
		arrayValue := value.([]interface{})
		
		// Check minimum length
		if rule.Min != nil {
			if minLength, ok := rule.Min.(int); ok && len(arrayValue) < minLength {
				return fmt.Errorf("must have at least %d items", minLength)
			}
		}

		// Check maximum length
		if rule.Max != nil {
			if maxLength, ok := rule.Max.(int); ok && len(arrayValue) > maxLength {
				return fmt.Errorf("must have at most %d items", maxLength)
			}
		}
	}

	return nil
}

// LogValidationErrors logs validation errors for monitoring
func LogValidationErrors(errors map[string]string, c *gin.Context) {
	if len(errors) > 0 {
		logger.SysLogf("Validation errors for %s %s: %v", c.Request.Method, c.Request.URL.Path, errors)
	}
}