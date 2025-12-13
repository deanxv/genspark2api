package tooluse

import (
	"encoding/json"
	"fmt"
	"genspark2api/model"
	"strings"

	"github.com/google/uuid"
)

// ToolCallResponse represents the expected JSON format from the model when calling a tool
type ToolCallResponse struct {
	Type      string                 `json:"type"`                // "tool_call" or "response"
	Tool      string                 `json:"tool,omitempty"`      // function name
	Arguments map[string]interface{} `json:"arguments,omitempty"` // function arguments
	Content   string                 `json:"content,omitempty"`   // final response content
}

// GenerateToolSystemPrompt creates a system prompt that instructs the model
// to use a specific JSON format for tool calls
func GenerateToolSystemPrompt(tools []model.OpenAITool) string {
	if len(tools) == 0 {
		return ""
	}

	var toolDescriptions []string
	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}

		desc := fmt.Sprintf("- %s", tool.Function.Name)
		if tool.Function.Description != "" {
			desc += fmt.Sprintf(": %s", tool.Function.Description)
		}

		// Add parameters info if available
		if tool.Function.Parameters != nil {
			if paramsBytes, err := json.Marshal(tool.Function.Parameters); err == nil {
				desc += fmt.Sprintf("\n  Parameters: %s", string(paramsBytes))
			}
		}
		toolDescriptions = append(toolDescriptions, desc)
	}

	if len(toolDescriptions) == 0 {
		return ""
	}

	prompt := `You are a function-calling AI. You have access to external tools and MUST use them.

AVAILABLE TOOLS:
` + strings.Join(toolDescriptions, "\n") + `

STRICT RULES - FOLLOW EXACTLY:

1. You MUST call a tool when the user's request requires external data (weather, time, calculations, web search, etc.)

2. Your response MUST be ONLY this JSON format, nothing else:
{"type":"tool_call","tool":"<TOOL_NAME>","arguments":{<ARGS>}}

3. If you already have tool results (shown as [Tool Result for ...]), use them to answer:
{"type":"response","content":"<your answer based on tool results>"}

4. If no tool is needed and you can answer from your knowledge:
{"type":"response","content":"<your answer>"}

5. FORBIDDEN:
   - Do NOT explain why you can't get data
   - Do NOT say "I don't have access to..."
   - Do NOT write anything except the JSON
   - Do NOT use markdown or code blocks
   - Do NOT apologize

6. The user asks about weather and no tool result is available? CALL get_weather.
   If [Tool Result] with weather data is present? Use it to respond.

EXAMPLE - User asks "What's the weather in Paris?" (no tool result yet):
{"type":"tool_call","tool":"get_weather","arguments":{"city":"Paris"}}

EXAMPLE - After tool result is received:
{"type":"response","content":"The weather in Paris is sunny, 22°C."}

YOUR RESPONSE MUST START WITH { AND END WITH } - NOTHING ELSE.`

	return prompt
}

// ParseToolCallFromText attempts to parse a tool call from the model's text response
func ParseToolCallFromText(text string) (*ToolCallResponse, error) {
	text = strings.TrimSpace(text)

	// Try to find JSON in the text
	startIdx := strings.Index(text, "{")
	endIdx := strings.LastIndex(text, "}")

	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := text[startIdx : endIdx+1]

	var response ToolCallResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate the response
	if response.Type != "tool_call" && response.Type != "response" {
		return nil, fmt.Errorf("invalid response type: %s (expected 'tool_call' or 'response')", response.Type)
	}

	if response.Type == "tool_call" && response.Tool == "" {
		return nil, fmt.Errorf("tool_call missing tool name")
	}

	return &response, nil
}

// ConvertToOpenAIToolCall converts our ToolCallResponse to OpenAI format
func ConvertToOpenAIToolCall(toolResp *ToolCallResponse) (*model.OpenAIToolCall, error) {
	if toolResp.Type != "tool_call" {
		return nil, fmt.Errorf("not a tool call response")
	}

	argsJSON, err := json.Marshal(toolResp.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal arguments: %w", err)
	}

	return &model.OpenAIToolCall{
		ID:   "call_" + uuid.New().String()[:8],
		Type: "function",
		Function: model.OpenAIToolCallFunction{
			Name:      toolResp.Tool,
			Arguments: string(argsJSON),
		},
	}, nil
}

// IsToolCallResponse checks if the parsed response is a tool call
func IsToolCallResponse(resp *ToolCallResponse) bool {
	return resp != nil && resp.Type == "tool_call"
}

// IsContentResponse checks if the parsed response is a final content response
func IsContentResponse(resp *ToolCallResponse) bool {
	return resp != nil && resp.Type == "response"
}

// ValidateToolCall checks if a tool call is valid against the available tools
func ValidateToolCall(toolResp *ToolCallResponse, tools []model.OpenAITool) error {
	if toolResp.Type != "tool_call" {
		return nil // not a tool call, nothing to validate
	}

	for _, tool := range tools {
		if tool.Type == "function" && tool.Function.Name == toolResp.Tool {
			return nil // found matching tool
		}
	}

	return fmt.Errorf("unknown tool: %s", toolResp.Tool)
}

// HasTools checks if the request contains any tools
func HasTools(req *model.OpenAIChatCompletionRequest) bool {
	return len(req.Tools) > 0
}

// PrependToolSystemMessage adds the tool system prompt to the messages
// It respects existing system messages by appending to them
func PrependToolSystemMessage(messages []model.OpenAIChatMessage, tools []model.OpenAITool) []model.OpenAIChatMessage {
	toolPrompt := GenerateToolSystemPrompt(tools)
	if toolPrompt == "" {
		return messages
	}

	// First, convert any tool-related messages to text format
	messages = ConvertToolMessagesToText(messages)

	// Check if there's already a system message
	hasSystemMessage := false
	for i, msg := range messages {
		if msg.Role == "system" {
			hasSystemMessage = true
			// Append tool instructions to existing system message
			if content, ok := msg.Content.(string); ok {
				messages[i].Content = content + "\n\n" + toolPrompt
			}
			break
		}
	}

	// If no system message exists, prepend one
	if !hasSystemMessage {
		systemMsg := model.OpenAIChatMessage{
			Role:    "system",
			Content: toolPrompt,
		}
		messages = append([]model.OpenAIChatMessage{systemMsg}, messages...)
	}

	return messages
}

// ConvertToolMessagesToText converts assistant messages with tool_calls and tool result messages
// to a text format that Genspark can understand
func ConvertToolMessagesToText(messages []model.OpenAIChatMessage) []model.OpenAIChatMessage {
	var result []model.OpenAIChatMessage

	for _, msg := range messages {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Convert assistant tool_calls to text representation
			var toolCallsText strings.Builder
			toolCallsText.WriteString("[Assistant called tools]:\n")
			for _, tc := range msg.ToolCalls {
				toolCallsText.WriteString(fmt.Sprintf("- %s(%s)\n", tc.Function.Name, tc.Function.Arguments))
			}
			// Also include content if present
			if msg.Content != nil {
				if s, ok := msg.Content.(string); ok && s != "" {
					toolCallsText.WriteString("\nAssistant message: " + s)
				}
			}
			result = append(result, model.OpenAIChatMessage{
				Role:    "assistant",
				Content: toolCallsText.String(),
			})
		} else if msg.Role == "tool" {
			// Convert tool result to user message with context
			content := ""
			if msg.Content != nil {
				if s, ok := msg.Content.(string); ok {
					content = s
				}
			}
			result = append(result, model.OpenAIChatMessage{
				Role:    "user",
				Content: fmt.Sprintf("[Tool Result for %s]: %s", msg.ToolCallID, content),
			})
		} else if msg.Role == "user" || msg.Role == "system" {
			// Always include user and system messages
			result = append(result, msg)
		} else if msg.Role == "assistant" {
			// Include regular assistant messages (without tool_calls)
			result = append(result, msg)
		} else {
			// Include any other messages
			result = append(result, msg)
		}
	}

	return result
}

// StreamBuffer helps accumulate streaming chunks for JSON validation
type StreamBuffer struct {
	buffer       strings.Builder
	braceCount   int
	bracketCount int
	inString     bool
	escapeNext   bool
	hasStarted   bool
}

// NewStreamBuffer creates a new StreamBuffer
func NewStreamBuffer() *StreamBuffer {
	return &StreamBuffer{}
}

// Append adds content to the buffer and returns true if we might have complete JSON
func (sb *StreamBuffer) Append(content string) bool {
	for _, ch := range content {
		sb.buffer.WriteRune(ch)

		if sb.escapeNext {
			sb.escapeNext = false
			continue
		}

		if ch == '\\' && sb.inString {
			sb.escapeNext = true
			continue
		}

		if ch == '"' {
			sb.inString = !sb.inString
			continue
		}

		if sb.inString {
			continue
		}

		switch ch {
		case '{':
			sb.hasStarted = true
			sb.braceCount++
		case '}':
			sb.braceCount--
		case '[':
			sb.bracketCount++
		case ']':
			sb.bracketCount--
		}
	}

	// JSON might be complete if we've started and all braces/brackets are balanced
	return sb.hasStarted && sb.braceCount == 0 && sb.bracketCount == 0
}

// GetContent returns the accumulated content
func (sb *StreamBuffer) GetContent() string {
	return sb.buffer.String()
}

// Reset clears the buffer
func (sb *StreamBuffer) Reset() {
	sb.buffer.Reset()
	sb.braceCount = 0
	sb.bracketCount = 0
	sb.inString = false
	sb.escapeNext = false
	sb.hasStarted = false
}

// IsValidStart checks if the buffer starts with a valid JSON object
func (sb *StreamBuffer) IsValidStart() bool {
	content := strings.TrimSpace(sb.buffer.String())
	return strings.HasPrefix(content, "{")
}
