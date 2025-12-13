package model

import "encoding/json"

type OpenAIChatCompletionRequest struct {
	Model      string              `json:"model"`
	Stream     bool                `json:"stream"`
	Messages   []OpenAIChatMessage `json:"messages"`
	Tools      []OpenAITool        `json:"tools,omitempty"`
	ToolChoice interface{}         `json:"tool_choice,omitempty"`
	OpenAIChatCompletionExtraRequest
}

// OpenAITool represents a tool definition in the OpenAI API format
type OpenAITool struct {
	Type     string             `json:"type"` // "function"
	Function OpenAIToolFunction `json:"function"`
}

// OpenAIToolFunction represents the function definition within a tool
type OpenAIToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"` // JSON Schema object
}

// OpenAIToolCall represents a tool call in the response
type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // "function"
	Function OpenAIToolCallFunction `json:"function"`
}

// OpenAIToolCallFunction represents the function call details
type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type OpenAIChatCompletionExtraRequest struct {
	ChannelId *string `json:"channelId"`
}

type SessionState struct {
	Models           []string `json:"models"`
	Layers           int      `json:"layers"`
	Answer           string   `json:"answer"`
	AnswerIsFinished bool     `json:"answer_is_finished"`
}
type OpenAIChatMessage struct {
	Role         string           `json:"role"`
	Content      interface{}      `json:"content"`
	IsPrompt     bool             `json:"is_prompt"`
	SessionState *SessionState    `json:"session_state"`
	ToolCalls    []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID   string           `json:"tool_call_id,omitempty"`
}

func (r *OpenAIChatCompletionRequest) AddMessage(message OpenAIChatMessage) {
	r.Messages = append([]OpenAIChatMessage{message}, r.Messages...)
}

func (r *OpenAIChatCompletionRequest) PrependMessagesFromJSON(jsonString string) error {
	var newMessages []OpenAIChatMessage
	err := json.Unmarshal([]byte(jsonString), &newMessages)
	if err != nil {
		return err
	}

	// 查找最后一个 system role 的索引
	var insertIndex int
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].Role == "system" {
			insertIndex = i + 1
			break
		}
	}

	// 将 newMessages 插入到找到的索引后面
	r.Messages = append(r.Messages[:insertIndex], append(newMessages, r.Messages[insertIndex:]...)...)
	return nil
}

func (r *OpenAIChatCompletionRequest) SystemMessagesProcess(model string) {
	if r.Messages == nil {
		return
	}

	if model == "deep-seek-r1" {
		for i := range r.Messages {
			if r.Messages[i].Role == "system" {
				r.Messages[i].Role = "user"
			}
			if r.Messages[i].Role == "assistant" {
				r.Messages[i].IsPrompt = false
				r.Messages[i].SessionState = &SessionState{
					Models: []string{model},
				}
			}
		}
	}
}

func (r *OpenAIChatCompletionRequest) FilterUserMessage() {
	if r.Messages == nil {
		return
	}

	// Find the last user message
	var lastUserIdx int = -1
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx == -1 {
		return
	}

	// Keep system messages and the last user message
	var filtered []OpenAIChatMessage
	for i, msg := range r.Messages {
		if msg.Role == "system" || i >= lastUserIdx {
			filtered = append(filtered, msg)
		}
	}
	r.Messages = filtered
}

type OpenAIErrorResponse struct {
	OpenAIError OpenAIError `json:"error"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

type OpenAIChatCompletionResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []OpenAIChoice `json:"choices"`
	Usage             OpenAIUsage    `json:"usage"`
	SystemFingerprint *string        `json:"system_fingerprint"`
	Suggestions       []string       `json:"suggestions"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	LogProbs     *string       `json:"logprobs"`
	FinishReason *string       `json:"finish_reason"`
	Delta        OpenAIDelta   `json:"delta"`
}

type OpenAIMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIDelta struct {
	Content          string                `json:"content,omitempty"`
	Role             string                `json:"role,omitempty"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	ToolCalls        []OpenAIDeltaToolCall `json:"tool_calls,omitempty"`
}

// OpenAIDeltaToolCall represents a tool call chunk in streaming response
type OpenAIDeltaToolCall struct {
	Index    int                         `json:"index"`
	ID       string                      `json:"id,omitempty"`
	Type     string                      `json:"type,omitempty"` // "function"
	Function OpenAIDeltaToolCallFunction `json:"function,omitempty"`
}

// OpenAIDeltaToolCallFunction represents function call chunk in streaming
type OpenAIDeltaToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type OpenAIImagesGenerationRequest struct {
	OpenAIChatCompletionExtraRequest
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	ResponseFormat string `json:"response_format"`
	Image          string `json:"image"`
}

type VideosGenerationRequest struct {
	ResponseFormat string `json:"response_format"`
	Model          string `json:"model"`
	AspectRatio    string `json:"aspect_ratio"`
	Duration       int    `json:"duration"`
	Prompt         string `json:"prompt"`
	AutoPrompt     bool   `json:"auto_prompt"`
	Image          string `json:"image"`
}

type VideosGenerationResponse struct {
	Created int64                           `json:"created"`
	Data    []*VideosGenerationDataResponse `json:"data"`
}

type VideosGenerationDataResponse struct {
	URL           string `json:"url"`
	RevisedPrompt string `json:"revised_prompt"`
	B64Json       string `json:"b64_json"`
}

type OpenAIImagesGenerationResponse struct {
	Created     int64                                 `json:"created"`
	DailyLimit  bool                                  `json:"dailyLimit"`
	Data        []*OpenAIImagesGenerationDataResponse `json:"data"`
	Suggestions []string                              `json:"suggestions"`
}

type OpenAIImagesGenerationDataResponse struct {
	URL           string `json:"url"`
	RevisedPrompt string `json:"revised_prompt"`
	B64Json       string `json:"b64_json"`
}

type OpenAIGPT4VImagesReq struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

type GetUserContent interface {
	GetUserContent() []string
}

type OpenAIModerationRequest struct {
	Input string `json:"input"`
}

type OpenAIModerationResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Results []struct {
		Flagged        bool               `json:"flagged"`
		Categories     map[string]bool    `json:"categories"`
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
}

type OpenaiModelResponse struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	//Created time.Time `json:"created"`
	//OwnedBy string    `json:"owned_by"`
}

// ModelList represents a list of models.
type OpenaiModelListResponse struct {
	Object string                `json:"object"`
	Data   []OpenaiModelResponse `json:"data"`
}

func (r *OpenAIChatCompletionRequest) GetUserContent() []string {
	var userContent []string

	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].Role == "user" {
			switch contentObj := r.Messages[i].Content.(type) {
			case string:
				userContent = append(userContent, contentObj)
			}
			break
		}
	}

	return userContent
}
