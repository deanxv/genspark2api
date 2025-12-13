package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"genspark2api/common/config"
	"genspark2api/common/helper"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Subsystems for categorizing logs
const (
	SubHTTP     = "HTTP"
	SubTool     = "TOOL"
	SubCookie   = "COOKIE"
	SubGenspark = "GENSPARK"
	SubSchedule = "SCHED"
	SubSystem   = "SYS"
)

// DebugPayload stores request/response details for verbose debugging
type DebugPayload struct {
	RequestID   string      `json:"request_id"`
	Timestamp   string      `json:"timestamp"`
	Subsystem   string      `json:"subsystem"`
	Phase       string      `json:"phase"`
	Model       string      `json:"model,omitempty"`
	Messages    interface{} `json:"messages,omitempty"`
	Tools       interface{} `json:"tools,omitempty"`
	RawResponse string      `json:"raw_response,omitempty"`
	ParsedData  interface{} `json:"parsed_data,omitempty"`
	Duration    string      `json:"duration,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// LogEvent represents a structured log event
type LogEvent struct {
	Timestamp   time.Time
	Level       string
	Subsystem   string
	RequestID   string
	Message     string
	Phase       string
	DurationMs  int64
	ExtraFields map[string]interface{}
}

// StructuredDebug logs with subsystem and phase info
func StructuredDebug(ctx context.Context, subsystem, phase, msg string) {
	if !config.DebugEnabled {
		return
	}
	id := getRequestID(ctx)
	now := time.Now()
	formatted := fmt.Sprintf("[DEBUG] %v | %s | %s | %s | %s",
		now.Format("2006/01/02 - 15:04:05"), id, subsystem, phase, msg)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

// StructuredInfo logs info with subsystem
func StructuredInfo(ctx context.Context, subsystem, msg string) {
	id := getRequestID(ctx)
	now := time.Now()
	formatted := fmt.Sprintf("[INFO] %v | %s | %s | %s",
		now.Format("2006/01/02 - 15:04:05"), id, subsystem, msg)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

// StructuredWarn logs warning with subsystem
func StructuredWarn(ctx context.Context, subsystem, msg string) {
	id := getRequestID(ctx)
	now := time.Now()
	formatted := fmt.Sprintf("[WARN] %v | %s | %s | %s",
		now.Format("2006/01/02 - 15:04:05"), id, subsystem, msg)
	_, _ = fmt.Fprintln(gin.DefaultErrorWriter, formatted)
}

// StructuredError logs error with subsystem
func StructuredError(ctx context.Context, subsystem, msg string) {
	id := getRequestID(ctx)
	now := time.Now()
	formatted := fmt.Sprintf("[ERR] %v | %s | %s | %s",
		now.Format("2006/01/02 - 15:04:05"), id, subsystem, msg)
	_, _ = fmt.Fprintln(gin.DefaultErrorWriter, formatted)
}

// SaveDebugPayload saves detailed debug info to a JSON file
func SaveDebugPayload(ctx context.Context, payload *DebugPayload) error {
	if !config.DebugEnabled || !config.DebugSavePayloads || LogDir == "" {
		return nil
	}

	// Create debug subdirectory
	debugDir := filepath.Join(LogDir, "debug")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return err
	}

	// Mask sensitive data
	payload = maskSensitiveData(payload)

	// Generate filename
	filename := fmt.Sprintf("%s-%s-%s.json",
		time.Now().Format("20060102-150405"),
		payload.RequestID,
		payload.Phase)
	filepath := filepath.Join(debugDir, filename)

	// Marshal and write
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

// LogRequestBody logs the request body in a readable format
func LogRequestBody(ctx context.Context, body interface{}) {
	if !config.DebugEnabled || !config.DebugLogBody {
		return
	}

	id := getRequestID(ctx)
	now := time.Now()

	var bodyStr string
	switch v := body.(type) {
	case string:
		bodyStr = v
	case []byte:
		bodyStr = string(v)
	case map[string]interface{}:
		// Format messages separately for readability
		if messages, ok := v["messages"]; ok {
			msgJson, _ := json.MarshalIndent(messages, "  ", "  ")
			bodyStr = fmt.Sprintf("messages:\n  %s", string(msgJson))

			// Add other fields
			for k, val := range v {
				if k != "messages" {
					bodyStr += fmt.Sprintf("\n%s: %v", k, val)
				}
			}
		} else {
			data, _ := json.MarshalIndent(v, "", "  ")
			bodyStr = string(data)
		}
	default:
		data, _ := json.MarshalIndent(body, "", "  ")
		bodyStr = string(data)
	}

	// Truncate if too long for console
	if len(bodyStr) > 2000 && config.DebugLogLevel != "verbose" {
		bodyStr = bodyStr[:2000] + "\n...(truncated, use DEBUG_LOG_LEVEL=verbose for full output)"
	}

	formatted := fmt.Sprintf("[DEBUG] %v | %s | %s | %s | \n%s",
		now.Format("2006/01/02 - 15:04:05"), id, SubHTTP, "REQUEST_BODY", bodyStr)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

// LogResponseBody logs the response body
func LogResponseBody(ctx context.Context, body string, statusCode int) {
	if !config.DebugEnabled || !config.DebugLogBody {
		return
	}

	id := getRequestID(ctx)
	now := time.Now()

	bodyStr := body
	// Truncate if too long
	if len(bodyStr) > 1000 && config.DebugLogLevel != "verbose" {
		bodyStr = bodyStr[:1000] + "...(truncated)"
	}

	formatted := fmt.Sprintf("[DEBUG] %v | %s | %s | %s | status=%d, body=%s",
		now.Format("2006/01/02 - 15:04:05"), id, SubHTTP, "RESPONSE_BODY", statusCode, bodyStr)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

// LogModelRawResponse logs the raw response from the model
func LogModelRawResponse(ctx context.Context, response string) {
	if !config.DebugEnabled {
		return
	}

	id := getRequestID(ctx)
	now := time.Now()

	// Only log if verbose or body logging enabled
	if config.DebugLogLevel == "minimal" {
		return
	}

	respStr := response
	if len(respStr) > 500 && config.DebugLogLevel != "verbose" {
		respStr = respStr[:500] + "...(truncated)"
	}

	formatted := fmt.Sprintf("[DEBUG] %v | %s | %s | %s | %s",
		now.Format("2006/01/02 - 15:04:05"), id, SubGenspark, "MODEL_RAW_RESPONSE", respStr)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

// ShouldLogVerbose returns true if verbose logging is enabled
func ShouldLogVerbose() bool {
	return config.DebugEnabled && config.DebugLogLevel == "verbose"
}

// ShouldLogBody returns true if body logging is enabled
func ShouldLogBody() bool {
	return config.DebugEnabled && config.DebugLogBody
}

// LogToolEvent logs tool-related events with structured format
func LogToolEvent(ctx context.Context, event string, details map[string]interface{}) {
	if !config.DebugEnabled {
		return
	}

	id := getRequestID(ctx)
	now := time.Now()

	// Build details string
	var detailParts []string
	for k, v := range details {
		detailParts = append(detailParts, fmt.Sprintf("%s=%v", k, v))
	}
	detailStr := strings.Join(detailParts, ", ")

	formatted := fmt.Sprintf("[DEBUG] %v | %s | %s | %s | %s",
		now.Format("2006/01/02 - 15:04:05"), id, SubTool, event, detailStr)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

// LogRequestStart logs the start of a request with timing
func LogRequestStart(ctx context.Context, model string, hasTools bool) {
	if !config.DebugEnabled {
		return
	}

	id := getRequestID(ctx)
	now := time.Now()

	toolsInfo := "no"
	if hasTools {
		toolsInfo = "yes"
	}

	formatted := fmt.Sprintf("[DEBUG] %v | %s | %s | %s | model=%s, tools=%s",
		now.Format("2006/01/02 - 15:04:05"), id, SubHTTP, "REQ_START", model, toolsInfo)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

// LogRequestComplete logs request completion with duration
func LogRequestComplete(ctx context.Context, status int, duration time.Duration) {
	id := getRequestID(ctx)
	now := time.Now()

	formatted := fmt.Sprintf("[INFO] %v | %s | %s | %s | status=%d, duration=%s",
		now.Format("2006/01/02 - 15:04:05"), id, SubHTTP, "REQ_COMPLETE", status, duration)
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatted)
}

func getRequestID(ctx context.Context) string {
	if ctx == nil {
		return helper.GenRequestID()
	}
	id := ctx.Value(helper.RequestIdKey)
	if id == nil {
		return helper.GenRequestID()
	}
	if s, ok := id.(string); ok {
		return s
	}
	return helper.GenRequestID()
}

// maskSensitiveData masks tokens and cookies in the payload
func maskSensitiveData(payload *DebugPayload) *DebugPayload {
	// Create a copy to avoid modifying original
	masked := *payload

	// Mask raw response if it contains sensitive patterns
	if masked.RawResponse != "" {
		masked.RawResponse = maskString(masked.RawResponse)
	}

	return &masked
}

var sensitivePatterns = regexp.MustCompile(`(?i)(session_id|api_key|token|cookie|bearer|authorization)[=:\s]*[^\s,}"]+`)

func maskString(s string) string {
	return sensitivePatterns.ReplaceAllStringFunc(s, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "=***MASKED***"
		}
		parts = strings.SplitN(match, ":", 2)
		if len(parts) == 2 {
			return parts[0] + ":***MASKED***"
		}
		return "***MASKED***"
	})
}

// PrettyPrintMessages formats messages for readable logging
func PrettyPrintMessages(messages interface{}) string {
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", messages)
	}
	// Truncate if too long
	s := string(data)
	if len(s) > 500 {
		return s[:500] + "...(truncated)"
	}
	return s
}
