package controller

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"genspark2api/common"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"genspark2api/model"
	"genspark2api/tooluse"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/deanxv/CycleTLS/cycletls"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

const (
	errNoValidCookies = "No valid cookies available"
)

const (
	baseURL          = "https://www.genspark.ai"
	apiEndpoint      = baseURL + "/api/copilot/ask"
	loginEndpoint    = baseURL + "/api/is_login"
	deleteEndpoint   = baseURL + "/api/project/delete?project_id=%s"
	uploadEndpoint   = baseURL + "/api/get_upload_personal_image_url"
	chatType         = "COPILOT_MOA_CHAT"
	imageType        = "COPILOT_MOA_IMAGE"
	videoType        = "COPILOT_MOA_VIDEO"
	responseIDFormat = "chatcmpl-%s"
)

type OpenAIChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type OpenAIChatCompletionRequest struct {
	Messages []OpenAIChatMessage
	Model    string
}

// ChatForOpenAI 处理OpenAI聊天请求
func ChatForOpenAI(c *gin.Context) {
	client := cycletls.Init()
	defer safeClose(client)

	var openAIReq model.OpenAIChatCompletionRequest

	// Log request body if enabled
	if config.DebugLogBody {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Errorf(c.Request.Context(), "Failed to read request body: %v", err)
		} else {
			logger.Debugf(c.Request.Context(), "Request Body: %s", string(bodyBytes))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	if err := c.BindJSON(&openAIReq); err != nil {
		logger.Errorf(c.Request.Context(), err.Error())
		c.JSON(http.StatusInternalServerError, model.OpenAIErrorResponse{
			OpenAIError: model.OpenAIError{
				Message: "Invalid request parameters",
				Type:    "request_error",
				Code:    "500",
			},
		})
		return
	}

	// 模型映射
	if strings.HasPrefix(openAIReq.Model, "deepseek") {
		openAIReq.Model = strings.Replace(openAIReq.Model, "deepseek", "deep-seek", 1)
	}

	// 初始化cookie

	cookieManager := config.NewCookieManager()
	cookie, err := cookieManager.GetRandomCookie()
	if err != nil {
		logger.Errorf(c.Request.Context(), "Failed to get initial cookie: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
		return
	}

	// Check login status
	checkLogin(c, client, cookie)

	if lo.Contains(common.ImageModelList, openAIReq.Model) {
		responseId := fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405"))

		if len(openAIReq.GetUserContent()) == 0 {
			logger.Errorf(c.Request.Context(), "user content is null")
			c.JSON(http.StatusInternalServerError, model.OpenAIErrorResponse{
				OpenAIError: model.OpenAIError{
					Message: "Invalid request parameters",
					Type:    "request_error",
					Code:    "500",
				},
			})
			return
		}

		jsonData, err := json.Marshal(openAIReq.GetUserContent()[0])
		if err != nil {
			logger.Errorf(c.Request.Context(), err.Error())
			c.JSON(500, gin.H{"error": "Failed to marshal request body"})
			return
		}
		resp, err := ImageProcess(c, client, model.OpenAIImagesGenerationRequest{
			Model:  openAIReq.Model,
			Prompt: openAIReq.GetUserContent()[0],
		})

		if err != nil {
			logger.Errorf(c.Request.Context(), err.Error())
			c.JSON(http.StatusInternalServerError, model.OpenAIErrorResponse{
				OpenAIError: model.OpenAIError{
					Message: err.Error(),
					Type:    "request_error",
					Code:    "500",
				},
			})
			return
		} else {
			data := resp.Data
			var content []string
			for _, item := range data {
				content = append(content, fmt.Sprintf("![Image](%s)", item.URL))
			}

			if openAIReq.Stream {
				streamResp := createStreamResponse(responseId, openAIReq.Model, jsonData, model.OpenAIDelta{Content: strings.Join(content, "\n"), Role: "assistant"}, nil)
				err := sendSSEvent(c, streamResp)
				if err != nil {
					logger.Errorf(c.Request.Context(), err.Error())
					c.JSON(http.StatusInternalServerError, model.OpenAIErrorResponse{
						OpenAIError: model.OpenAIError{
							Message: err.Error(),
							Type:    "request_error",
							Code:    "500",
						},
					})
					return
				}
				c.SSEvent("", " [DONE]")
				return
			} else {

				jsonBytes, _ := json.Marshal(openAIReq.Messages)
				promptTokens := common.CountTokenText(string(jsonBytes), openAIReq.Model)
				completionTokens := common.CountTokenText(strings.Join(content, "\n"), openAIReq.Model)

				finishReason := "stop"
				// 创建并返回 OpenAIChatCompletionResponse 结构
				resp := model.OpenAIChatCompletionResponse{
					ID:      fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405")),
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   openAIReq.Model,
					Choices: []model.OpenAIChoice{
						{
							Message: &model.OpenAIMessage{
								Role:    "assistant",
								Content: strings.Join(content, "\n"),
							},
							FinishReason: &finishReason,
						},
					},
					Usage: &model.OpenAIUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				}
				c.JSON(200, resp)
				return
			}

		}
	}

	var isSearchModel bool
	if strings.HasSuffix(openAIReq.Model, "-search") {
		isSearchModel = true
	}

	// Check if tools are provided and handle tool-use mode
	hasTools := len(openAIReq.Tools) > 0
	if hasTools {
		handleToolUseRequest(c, client, cookie, cookieManager, &openAIReq, isSearchModel)
		return
	}

	requestBody, err := createRequestBody(c, client, cookie, &openAIReq)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	//jsonData, err := json.Marshal(requestBody)
	//if err != nil {
	//	c.JSON(500, gin.H{"error": "Failed to marshal request body"})
	//	return
	//}

	if openAIReq.Stream {
		handleStreamRequest(c, client, cookie, cookieManager, requestBody, openAIReq.Model, isSearchModel)
	} else {
		handleNonStreamRequest(c, client, cookie, cookieManager, requestBody, openAIReq.Model, isSearchModel)
	}

}

func processMessages(c *gin.Context, client cycletls.CycleTLS, cookie string, messages []model.OpenAIChatMessage) error {
	//client := cycletls.Init()
	//defer client.Close()

	for i, message := range messages {
		if contentArray, ok := message.Content.([]interface{}); ok {
			for j, content := range contentArray {
				if contentMap, ok := content.(map[string]interface{}); ok {
					if contentType, ok := contentMap["type"].(string); ok && contentType == "image_url" {
						if imageMap, ok := contentMap["image_url"].(map[string]interface{}); ok {
							if url, ok := imageMap["url"].(string); ok {
								err := processUrl(c, client, cookie, url, imageMap, j, contentArray)
								if err != nil {
									logger.Errorf(c.Request.Context(), fmt.Sprintf("processUrl err  %v\n", err))
									return fmt.Errorf("processUrl err: %v", err)
								}
							}
						}
					}
				}
			}
			messages[i].Content = contentArray
		}
	}
	return nil
}
func processUrl(c *gin.Context, client cycletls.CycleTLS, cookie string, url string, imageMap map[string]interface{}, index int, contentArray []interface{}) error {
	// 判断是否为URL
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// 下载文件
		bytes, err := fetchImageBytes(url)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("fetchImageBytes err  %v\n", err))
			return fmt.Errorf("fetchImageBytes err  %v\n", err)
		}

		err = processBytes(c, client, cookie, bytes, imageMap, index, contentArray)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("processBytes err  %v\n", err))
			return fmt.Errorf("processBytes err  %v\n", err)
		}
	} else {
		// 尝试解析base64
		var bytes []byte
		var err error

		// 处理可能包含 data:image/ 前缀的base64
		base64Str := url
		if strings.Contains(url, ";base64,") {
			base64Str = strings.Split(url, ";base64,")[1]
		}

		bytes, err = base64.StdEncoding.DecodeString(base64Str)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("base64.StdEncoding.DecodeString err  %v\n", err))
			return fmt.Errorf("base64.StdEncoding.DecodeString err: %v\n", err)
		}

		err = processBytes(c, client, cookie, bytes, imageMap, index, contentArray)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("processBytes err  %v\n", err))
			return fmt.Errorf("processBytes err: %v\n", err)
		}
	}
	return nil
}

func processBytes(c *gin.Context, client cycletls.CycleTLS, cookie string, bytes []byte, imageMap map[string]interface{}, index int, contentArray []interface{}) error {
	// 检查是否为图片类型
	contentType := http.DetectContentType(bytes)
	if strings.HasPrefix(contentType, "image/") {
		// 是图片类型，转换为base64
		base64Data := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes)
		imageMap["url"] = base64Data
	} else {
		response, err := makeGetUploadUrlRequest(client, cookie)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("makeGetUploadUrlRequest err  %v\n", err))
			return fmt.Errorf("makeGetUploadUrlRequest err: %v\n", err)
		}

		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(response.Body), &jsonResponse); err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("Unmarshal err  %v\n", err))
			return fmt.Errorf("Unmarshal err: %v\n", err)
		}

		uploadImageUrl, ok := jsonResponse["data"].(map[string]interface{})["upload_image_url"].(string)
		privateStorageUrl, ok := jsonResponse["data"].(map[string]interface{})["private_storage_url"].(string)

		if !ok {
			//fmt.Println("Failed to extract upload_image_url")
			return fmt.Errorf("Failed to extract upload_image_url")
		}

		// 发送OPTIONS预检请求
		//_, err = makeOptionsRequest(client, uploadImageUrl)
		//if err != nil {
		//	return
		//}
		// 上传文件
		_, err = makeUploadRequest(client, uploadImageUrl, bytes)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("makeUploadRequest err  %v\n", err))
			return fmt.Errorf("makeUploadRequest err: %v\n", err)
		}
		//fmt.Println(resp)

		// 创建新的 private_file 格式的内容
		privateFile := map[string]interface{}{
			"type": "private_file",
			"private_file": map[string]interface{}{
				"name":                "file", // 你可能需要从原始文件名或其他地方获取
				"type":                contentType,
				"size":                len(bytes),
				"ext":                 strings.Split(contentType, "/")[1], // 简单处理，可能需要更复杂的逻辑
				"private_storage_url": privateStorageUrl,
			},
		}

		// 替换数组中的元素
		contentArray[index] = privateFile
	}
	return nil
}

// 获取文件字节数组的函数
func fetchImageBytes(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http.Get err: %v\n", err)
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func createRequestBody(c *gin.Context, client cycletls.CycleTLS, cookie string, openAIReq *model.OpenAIChatCompletionRequest) (map[string]interface{}, error) {
	openAIReq.SystemMessagesProcess(openAIReq.Model)
	if config.PRE_MESSAGES_JSON != "" {
		err := openAIReq.PrependMessagesFromJSON(config.PRE_MESSAGES_JSON)
		if err != nil {
			return nil, fmt.Errorf("PrependMessagesFromJSON err: %v PrependMessagesFromJSON:", err, config.PRE_MESSAGES_JSON)
		}
	}

	// 处理消息中的图像 URL
	err := processMessages(c, client, cookie, openAIReq.Messages)
	if err != nil {
		logger.Errorf(c.Request.Context(), "processMessages err: %v", err)
		return nil, fmt.Errorf("processMessages err: %v", err)
	}

	currentQueryString := fmt.Sprintf("type=%s", chatType)
	//查找 key 对应的 value
	if chatId, ok := config.ModelChatMap[openAIReq.Model]; ok {
		currentQueryString = fmt.Sprintf("id=%s&type=%s", chatId, chatType)
	} else if chatId, ok := config.GlobalSessionManager.GetChatID(cookie, openAIReq.Model); ok {
		currentQueryString = fmt.Sprintf("id=%s&type=%s", chatId, chatType)
	} else {
		// Don't filter messages for tool-use requests - we need full history
		if len(openAIReq.Tools) == 0 {
			openAIReq.FilterUserMessage()
		}
	}
	requestWebKnowledge := false
	models := []string{openAIReq.Model}
	if strings.HasSuffix(openAIReq.Model, "-search") {
		openAIReq.Model = strings.Replace(openAIReq.Model, "-search", "", 1)
		requestWebKnowledge = true
		models = []string{openAIReq.Model}
	}
	if !lo.Contains(common.TextModelList, openAIReq.Model) {
		models = common.MixtureModelList
	}

	// 创建请求体
	requestBody := map[string]interface{}{
		"type":                 chatType,
		"current_query_string": currentQueryString,
		"messages":             openAIReq.Messages,
		"action_params":        map[string]interface{}{},
		"extra_data": map[string]interface{}{
			"models":                 models,
			"run_with_another_model": false,
			"writingContent":         nil,
			"request_web_knowledge":  requestWebKnowledge,
		},
	}

	// Log request body if DEBUG_LOG_BODY is enabled
	logger.LogRequestBody(c.Request.Context(), requestBody)

	return requestBody, nil
}

func createImageRequestBody(c *gin.Context, cookie string, openAIReq *model.OpenAIImagesGenerationRequest, chatId string) (map[string]interface{}, error) {

	if openAIReq.Model == "dall-e-3" {
		openAIReq.Model = "dalle-3"
	}
	// 创建模型配置
	modelConfigs := []map[string]interface{}{
		{
			"model":                   openAIReq.Model,
			"aspect_ratio":            "auto",
			"use_personalized_models": false,
			"fashion_profile_id":      nil,
			"hd":                      false,
			"reflection_enabled":      false,
			"style":                   "auto",
		},
	}

	// 创建消息数组
	var messages []map[string]interface{}

	if openAIReq.Image != "" {
		var base64Data string

		if strings.HasPrefix(openAIReq.Image, "http://") || strings.HasPrefix(openAIReq.Image, "https://") {
			// 下载文件
			bytes, err := fetchImageBytes(openAIReq.Image)
			if err != nil {
				logger.Errorf(c.Request.Context(), fmt.Sprintf("fetchImageBytes err  %v\n", err))
				return nil, fmt.Errorf("fetchImageBytes err  %v\n", err)
			}

			contentType := http.DetectContentType(bytes)
			if strings.HasPrefix(contentType, "image/") {
				// 是图片类型，转换为base64
				base64Data = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes)
			}
		} else if common.IsImageBase64(openAIReq.Image) {
			// 如果已经是 base64 格式
			if !strings.HasPrefix(openAIReq.Image, "data:image") {
				base64Data = "data:image/jpeg;base64," + openAIReq.Image
			} else {
				base64Data = openAIReq.Image
			}
		}

		// 构建包含图片的消息
		if base64Data != "" {
			messages = []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{
							"type": "image_url",
							"image_url": map[string]interface{}{
								"url": base64Data,
							},
						},
						{
							"type": "text",
							"text": openAIReq.Prompt,
						},
					},
				},
			}
		}
	}

	// 如果没有图片或处理图片失败，使用纯文本消息
	if len(messages) == 0 {
		messages = []map[string]interface{}{
			{
				"role":    "user",
				"content": openAIReq.Prompt,
			},
		}
	}
	var currentQueryString string
	if len(chatId) != 0 {
		currentQueryString = fmt.Sprintf("id=%s&type=%s", chatId, imageType)
	} else {
		currentQueryString = fmt.Sprintf("type=%s", imageType)
	}

	// 创建请求体
	requestBody := map[string]interface{}{
		"type": "COPILOT_MOA_IMAGE",
		//"current_query_string": "type=COPILOT_MOA_IMAGE",
		"current_query_string": currentQueryString,
		"messages":             messages,
		"user_s_input":         openAIReq.Prompt,
		"action_params":        map[string]interface{}{},
		"extra_data": map[string]interface{}{
			"model_configs":  modelConfigs,
			"llm_model":      "gpt-4o",
			"imageModelMap":  map[string]interface{}{},
			"writingContent": nil,
		},
	}

	logger.Debug(c.Request.Context(), fmt.Sprintf("RequestBody: %v", requestBody))

	if strings.TrimSpace(config.RecaptchaProxyUrl) == "" ||
		(!strings.HasPrefix(config.RecaptchaProxyUrl, "http://") &&
			!strings.HasPrefix(config.RecaptchaProxyUrl, "https://")) {
		return requestBody, nil
	} else {

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}

		// 检查并补充 RecaptchaProxyUrl 的末尾斜杠
		if !strings.HasSuffix(config.RecaptchaProxyUrl, "/") {
			config.RecaptchaProxyUrl += "/"
		}

		// 创建请求
		req, err := http.NewRequest("GET", fmt.Sprintf("%sgenspark", config.RecaptchaProxyUrl), nil)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("创建/genspark请求失败   %v\n", err))
			return nil, err
		}

		// 设置请求头
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", cookie)

		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("发送/genspark请求失败   %v\n", err))
			return nil, err
		}
		defer resp.Body.Close()

		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("读取/genspark响应失败   %v\n", err))
			return nil, err
		}

		type Response struct {
			Code    int    `json:"code"`
			Token   string `json:"token"`
			Message string `json:"message"`
		}

		if resp.StatusCode == 200 {
			var response Response
			if err := json.Unmarshal(body, &response); err != nil {
				logger.Errorf(c.Request.Context(), fmt.Sprintf("读取/genspark JSON 失败   %v\n", err))
				return nil, err
			}

			if response.Code == 200 {
				logger.Debugf(c.Request.Context(), fmt.Sprintf("g_recaptcha_token: %v\n", response.Token))
				requestBody["g_recaptcha_token"] = response.Token
				logger.Infof(c.Request.Context(), fmt.Sprintf("cheat success!"))
				return requestBody, nil
			} else {
				logger.Errorf(c.Request.Context(), fmt.Sprintf("读取/genspark token 失败   %v\n", err))
				return nil, err
			}
		} else {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("请求/genspark失败   %v\n", err))
			return nil, err
		}
	}
}

// createStreamResponse 创建流式响应
func createStreamResponse(responseId, modelName string, jsonData []byte, delta model.OpenAIDelta, finishReason *string) model.OpenAIChatCompletionResponse {
	var usage *model.OpenAIUsage
	if finishReason != nil {
		promptTokens := common.CountTokenText(string(jsonData), modelName)
		completionTokens := common.CountTokenText(delta.Content, modelName)
		reasoningTokens := common.CountTokenText(delta.ReasoningContent, modelName)
		usage = &model.OpenAIUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			CompletionTokensDetails: &model.OpenAICompletionTokensDetails{
				ReasoningTokens: reasoningTokens,
			},
		}
	}
	return model.OpenAIChatCompletionResponse{
		ID:      responseId,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.OpenAIChoice{
			{
				Index:        0,
				Delta:        &delta,
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}
}

// handleMessageFieldDelta 处理消息字段增量
func handleMessageFieldDelta(c *gin.Context, event map[string]interface{}, responseId, modelName string, jsonData []byte, totalContent, totalReasoningContent *string) error {
	fieldName, ok := event["field_name"].(string)
	if !ok {
		return nil
	}

	// 基础允许列表（所有配置下都需要处理的字段）
	baseAllowed := fieldName == "session_state.answer" ||
		strings.Contains(fieldName, "session_state.streaming_detail_answer") ||
		fieldName == "session_state.streaming_markmap" ||
		strings.HasPrefix(fieldName, "session_state.layer_")

	// 需要显示思考过程时需要额外处理的字段
	if config.ReasoningHide != 1 {
		baseAllowed = baseAllowed ||
			fieldName == "session_state.answerthink" ||
			fieldName == "session_state.answerthink_is_started" ||
			fieldName == "session_state.answerthink_is_finished"
	}

	if !baseAllowed {
		return nil
	}

	// 获取 delta 内容
	var delta string
	var err error
	// 优先尝试获取 delta
	if d, ok := event["delta"].(string); ok && len(d) > 0 {
		delta = d
	} else if v, ok := event["field_value"].(string); ok {
		// 如果没有 delta，尝试获取 field_value
		delta = v
	}

	_ = err // fix unused

	// 处理思考过程 - 使用 reasoning_content 字段 (OpenAI API 格式)
	if config.ReasoningHide != 1 {
		switch fieldName {
		case "session_state.answerthink_is_started":
			// 发送空的reasoning_content开始标记，客户端会知道reasoning开始了
			return nil
		case "session_state.answerthink":
			// 发送reasoning内容到reasoning_content字段
			if totalReasoningContent != nil {
				*totalReasoningContent += delta
			}
			streamResp := createStreamResponse(
				responseId,
				modelName,
				jsonData,
				model.OpenAIDelta{ReasoningContent: delta, Reasoning: delta, Role: "assistant"},
				nil,
			)
			return sendSSEvent(c, streamResp)
		case "session_state.answerthink_is_finished":
			// 发送空的标记表示reasoning结束
			return nil
		}
	}

	// 发送普通content
	// 发送普通content
	var deltaRole string
	var deltaContent string
	var deltaReasoningContent string

	if strings.HasPrefix(fieldName, "session_state.layer_") {
		deltaReasoningContent = delta
		if totalReasoningContent != nil {
			*totalReasoningContent += delta
		}
	} else {
		deltaContent = delta
		if totalContent != nil {
			*totalContent += delta
		}
	}

	streamResp := createStreamResponse(
		responseId,
		modelName,
		jsonData,
		model.OpenAIDelta{
			Content:          deltaContent,
			ReasoningContent: deltaReasoningContent,
			Reasoning:        deltaReasoningContent,
			Role:             deltaRole, // Role defaults to "" which is fine, usually "assistant" is sent once or inferred
		},
		nil,
	)
	// Ensure Role is set if needed, though usually Delta just updates content.
	// Actually, upstream logic had Role: "assistant" separately. Let's keep it safe.
	if streamResp.Choices[0].Delta.Role == "" {
		streamResp.Choices[0].Delta.Role = "assistant"
	}

	return sendSSEvent(c, streamResp)
}

type Content struct {
	DetailAnswer string `json:"detailAnswer"`
}

func getDetailAnswer(eventMap map[string]interface{}) (string, error) {
	// 获取 content 字段的值
	contentStr, ok := eventMap["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is not a string")
	}

	// 解析内层的 JSON
	var content Content
	if err := json.Unmarshal([]byte(contentStr), &content); err != nil {
		return "", err
	}

	return content.DetailAnswer, nil
}

// handleMessageResult 处理消息结果
func handleMessageResult(c *gin.Context, event map[string]interface{}, responseId, modelName string, jsonData []byte, searchModel bool) bool {
	finishReason := "stop"
	var delta string
	var err error
	if modelName == "o1" && searchModel {
		delta, err = getDetailAnswer(event)
		if err != nil {
			logger.Errorf(c.Request.Context(), "getDetailAnswer err: %v", err)
			return false
		}
	}

	streamResp := createStreamResponse(responseId, modelName, jsonData, model.OpenAIDelta{Content: delta, Role: "assistant"}, &finishReason)
	if err := sendSSEvent(c, streamResp); err != nil {
		logger.Warnf(c.Request.Context(), "sendSSEvent err: %v", err)
		return false
	}
	c.SSEvent("", " [DONE]")
	return false
}

// sendSSEvent 发送SSE事件
func sendSSEvent(c *gin.Context, response model.OpenAIChatCompletionResponse) error {
	jsonResp, err := json.Marshal(response)
	if err != nil {
		logger.Errorf(c.Request.Context(), "Failed to marshal response: %v", err)
		return err
	}
	c.SSEvent("", " "+string(jsonResp))
	c.Writer.Flush()
	return nil
}

// makeRequest 发送HTTP请求
func makeRequest(client cycletls.CycleTLS, jsonData []byte, cookie string, isStream bool) (cycletls.Response, error) {
	accept := "application/json"
	if isStream {
		accept = "text/event-stream"
	}

	options := cycletls.Options{
		Timeout: 10 * 60 * 60,
		Proxy:   config.ProxyUrl, // 在每个请求中设置代理
		Body:    string(jsonData),
		Method:  "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       accept,
			"Origin":       baseURL,
			"Referer":      baseURL + "/",
			"Cookie":       cookie,
			"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
		},
	}

	if config.DebugLogNetwork {
		logger.Debugf(context.Background(), "\n=== OUTGOING REQUEST ===\nURL: %s\nHeaders: %v\nBody: %s\n========================", apiEndpoint, options.Headers, options.Body)
	}

	response, err := client.Do(apiEndpoint, options, "POST")
	if err != nil {
		return response, err
	}

	if config.DebugLogNetwork {
		logger.Debugf(context.Background(), "\n=== INCOMING RESPONSE ===\nStatus: %d\nBody: %s\n=========================", response.Status, response.Body)
	}

	return response, nil
}

// makeRequest 发送HTTP请求
func makeImageRequest(client cycletls.CycleTLS, jsonData []byte, cookie string) (cycletls.Response, error) {

	accept := "*/*"

	return client.Do(apiEndpoint, cycletls.Options{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
		Timeout:   10 * 60 * 60,
		Proxy:     config.ProxyUrl, // 在每个请求中设置代理
		Body:      string(jsonData),
		Method:    "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       accept,
			"Origin":       baseURL,
			"Referer":      baseURL + "/",
			"Cookie":       cookie,
			"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
		},
	}, "POST")
}

func makeDeleteRequest(c *gin.Context, client cycletls.CycleTLS, cookie, projectId string) (cycletls.Response, error) {
	ctx := c.Request.Context()

	// Проверка на пустой projectId - критическая проблема
	if strings.TrimSpace(projectId) == "" {
		logger.Warnf(ctx, "[DELETE] SKIP: projectId is empty, cannot delete anything")
		return cycletls.Response{}, fmt.Errorf("projectId is empty")
	}

	logger.Infof(ctx, "[DELETE] ATTEMPT: Trying to delete chat projectId=%s", projectId)

	// 不删除环境变量中的map中的对话 (Don't delete chats from configured maps)

	for _, v := range config.ModelChatMap {
		if v == projectId {
			logger.Infof(ctx, "[DELETE] SKIP: projectId=%s found in MODEL_CHAT_MAP (configured to keep)", projectId)
			return cycletls.Response{}, nil
		}
	}
	for _, v := range config.GlobalSessionManager.GetChatIDsByCookie(cookie) {
		if v == projectId {
			logger.Infof(ctx, "[DELETE] SKIP: projectId=%s found in GlobalSessionManager (configured to keep)", projectId)
			return cycletls.Response{}, nil
		}
	}
	for _, v := range config.SessionImageChatMap {
		if v == projectId {
			logger.Infof(ctx, "[DELETE] SKIP: projectId=%s found in SESSION_IMAGE_CHAT_MAP (configured to keep)", projectId)
			return cycletls.Response{}, nil
		}
	}

	accept := "application/json"
	deleteURL := fmt.Sprintf(deleteEndpoint, projectId)

	logger.Infof(ctx, "[DELETE] SENDING: HTTP GET to %s", deleteURL)

	response, err := client.Do(deleteURL, cycletls.Options{
		Timeout: 10 * 60 * 60,
		Proxy:   config.ProxyUrl,
		Method:  "GET",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       accept,
			"Origin":       baseURL,
			"Referer":      baseURL + "/",
			"Cookie":       cookie,
			"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
		},
	}, "GET")

	if err != nil {
		logger.Errorf(ctx, "[DELETE] ERROR: Failed to delete projectId=%s, error=%v", projectId, err)
		return response, err
	}

	// Детальное логирование результата
	if response.Status == 200 {
		logger.Debugf(ctx, "[DELETE] SUCCESS: projectId=%s deleted successfully, Status=%d", projectId, response.Status)
	} else {
		logger.Warnf(ctx, "[DELETE] FAILED: projectId=%s, Status=%d, Body=%s", projectId, response.Status, strings.TrimSpace(response.Body))
	}

	return response, nil
}

func makeGetUploadUrlRequest(client cycletls.CycleTLS, cookie string) (cycletls.Response, error) {

	accept := "*/*"

	return client.Do(fmt.Sprintf(uploadEndpoint), cycletls.Options{
		Timeout: 10 * 60 * 60,
		Proxy:   config.ProxyUrl, // 在每个请求中设置代理
		Method:  "GET",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       accept,
			"Origin":       baseURL,
			"Referer":      baseURL + "/",
			"Cookie":       cookie,
			"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
		},
	}, "GET")
}

//func makeOptionsRequest(client cycletls.CycleTLS, uploadUrl string) (cycletls.Response, error) {
//	return client.Do(uploadUrl, cycletls.Options{
//		Method: "OPTIONS",
//		Headers: map[string]string{
//			"Accept":                         "*/*",
//			"Access-Control-Request-Headers": "x-ms-blob-type",
//			"Access-Control-Request-Method":  "PUT",
//			"Origin":                         "https://www.genspark.ai",
//			"Sec-Fetch-Dest":                 "empty",
//			"Sec-Fetch-Mode":                 "cors",
//			"Sec-Fetch-Site":                 "cross-site",
//		},
//		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
//	}, "OPTIONS")
//}

func makeUploadRequest(client cycletls.CycleTLS, uploadUrl string, fileBytes []byte) (cycletls.Response, error) {
	return client.Do(uploadUrl, cycletls.Options{
		Timeout: 10 * 60 * 60,
		Proxy:   config.ProxyUrl, // 在每个请求中设置代理
		Method:  "PUT",
		Body:    string(fileBytes),
		Headers: map[string]string{
			"Accept":         "*/*",
			"x-ms-blob-type": "BlockBlob",
			"Content-Type":   "application/octet-stream",
			"Content-Length": fmt.Sprintf("%d", len(fileBytes)),
			"Origin":         "https://www.genspark.ai",
			"Sec-Fetch-Dest": "empty",
			"Sec-Fetch-Mode": "cors",
			"Sec-Fetch-Site": "cross-site",
		},
	}, "PUT")
}

// handleStreamRequest 处理流式请求
//func handleStreamRequest(c *gin.Context, client cycletls.CycleTLS, cookie string, jsonData []byte, model string) {
//	c.Header("Content-Type", "text/event-stream")
//	c.Header("Cache-Control", "no-cache")
//	c.Header("Connection", "keep-alive")
//
//	responseId := fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405"))
//
//	c.Stream(func(w io.Writer) bool {
//		sseChan, err := makeStreamRequest(c, client, jsonData, cookie)
//		if err != nil {
//			logger.Errorf(c.Request.Context(), "makeStreamRequest err: %v", err)
//			return false
//		}
//
//		return handleStreamResponse(c, sseChan, responseId, cookie, model, jsonData)
//	})
//}

func handleStreamRequest(c *gin.Context, client cycletls.CycleTLS, cookie string, cookieManager *config.CookieManager, requestBody map[string]interface{}, modelName string, searchModel bool) {
	const (
		errNoValidCookies         = "No valid cookies available"
		errCloudflareChallengeMsg = "Detected Cloudflare Challenge Page"
		errCloudflareBlock        = "CloudFlare: Sorry, you have been blocked"
		errServerErrMsg           = "An error occurred with the current request, please try again."
		errServiceUnavailable     = "Genspark Service Unavailable"
	)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	responseId := fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405"))
	ctx := c.Request.Context()
	maxRetries := len(cookieManager.Cookies)

	c.Stream(func(w io.Writer) bool {
		for attempt := 0; attempt < maxRetries; attempt++ {

			requestBody, err := cheat(requestBody, c, cookie)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return false
			}
			jsonData, err := json.Marshal(requestBody)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to marshal request body"})
				return false
			}
			sseChan, err := makeStreamRequest(c, client, jsonData, cookie)
			if err != nil {
				logger.Errorf(ctx, "makeStreamRequest err on attempt %d: %v", attempt+1, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return false
			}

			var projectId string
			isRateLimit := false
			var totalContent string
			var totalReasoningContent string

		SSELoop:
			for response := range sseChan {
				if response.Done {
					logger.Debugf(ctx, response.Data)
					return false
				}

				data := response.Data
				if data == "" {
					continue
				}

				logger.Debug(ctx, strings.TrimSpace(data))

				switch {
				case common.IsCloudflareChallenge(data):
					logger.Errorf(ctx, errCloudflareChallengeMsg)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errCloudflareChallengeMsg})
					return false
				case common.IsCloudflareBlock(data):
					logger.Errorf(ctx, errCloudflareBlock)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errCloudflareBlock})
					return false
				case common.IsServiceUnavailablePage(data):
					logger.Errorf(ctx, errServiceUnavailable)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errServiceUnavailable})
					return false
				case common.IsServerError(data):
					logger.Errorf(ctx, errServerErrMsg)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errServerErrMsg})
					return false
				case common.IsRateLimit(data):
					isRateLimit = true
					logger.Warnf(ctx, "Cookie rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
					config.AddRateLimitCookie(cookie, time.Now().Add(time.Duration(config.RateLimitCookieLockDuration)*time.Second))
					break SSELoop // 使用 label 跳出 SSE 循环
				case common.IsFreeLimit(data):
					isRateLimit = true
					logger.Warnf(ctx, "Cookie free rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
					config.AddRateLimitCookie(cookie, time.Now().Add(24*60*60*time.Second))
					// 删除cookie
					//config.RemoveCookie(cookie)
					break SSELoop // 使用 label 跳出 SSE 循环
				case common.IsNotLogin(data):
					isRateLimit = true
					logger.Warnf(ctx, "Cookie Not Login, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
					// 删除cookie
					config.RemoveCookie(cookie)
					break SSELoop // 使用 label 跳出 SSE 循环
				}

				// 处理事件流数据
				if shouldContinue := processStreamData(c, data, &projectId, cookie, responseId, modelName, jsonData, searchModel, &totalContent, &totalReasoningContent); !shouldContinue {
					return false
				}
			}

			// Send final usage
			promptTokens := common.CountTokenText(string(jsonData), modelName)
			completionTokens := common.CountTokenText(totalContent, modelName)
			reasoningTokens := common.CountTokenText(totalReasoningContent, modelName)

			usageResp := model.OpenAIChatCompletionResponse{
				ID:      responseId,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   modelName,
				Choices: []model.OpenAIChoice{},
				Usage: &model.OpenAIUsage{
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      promptTokens + completionTokens,
					CompletionTokensDetails: &model.OpenAICompletionTokensDetails{
						ReasoningTokens: reasoningTokens,
					},
				},
			}
			sendSSEvent(c, usageResp)

			if !isRateLimit {
				return true
			}

			// 获取下一个可用的cookie继续尝试
			cookie, err = cookieManager.GetNextCookie()
			if err != nil {
				logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
				return false
			}

			// requestBody重制chatId
			currentQueryString := fmt.Sprintf("type=%s", chatType)
			if chatId, ok := config.GlobalSessionManager.GetChatID(cookie, modelName); ok {
				currentQueryString = fmt.Sprintf("id=%s&type=%s", chatId, chatType)
			}
			requestBody["current_query_string"] = currentQueryString
		}

		logger.Errorf(ctx, "All cookies exhausted after %d attempts", maxRetries)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "All cookies are temporarily unavailable."})
		return false
	})
}

func cheat(requestBody map[string]interface{}, c *gin.Context, cookie string) (map[string]interface{}, error) {
	if strings.TrimSpace(config.RecaptchaProxyUrl) == "" ||
		(!strings.HasPrefix(config.RecaptchaProxyUrl, "http://") &&
			!strings.HasPrefix(config.RecaptchaProxyUrl, "https://")) {
		return requestBody, nil
	} else {

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}

		// 检查并补充 RecaptchaProxyUrl 的末尾斜杠
		if !strings.HasSuffix(config.RecaptchaProxyUrl, "/") {
			config.RecaptchaProxyUrl += "/"
		}

		// 创建请求
		req, err := http.NewRequest("GET", fmt.Sprintf("%sgenspark", config.RecaptchaProxyUrl), nil)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("创建/genspark请求失败   %v\n", err))
			return nil, err
		}

		// 设置请求头
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", cookie)

		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("发送/genspark请求失败   %v\n", err))
			return nil, err
		}
		defer resp.Body.Close()

		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("读取/genspark响应失败   %v\n", err))
			return nil, err
		}

		type Response struct {
			Code    int    `json:"code"`
			Token   string `json:"token"`
			Message string `json:"message"`
		}

		if resp.StatusCode == 200 {
			var response Response
			if err := json.Unmarshal(body, &response); err != nil {
				logger.Errorf(c.Request.Context(), fmt.Sprintf("读取/genspark JSON 失败   %v\n", err))
				return nil, err
			}

			if response.Code == 200 {
				logger.Debugf(c.Request.Context(), fmt.Sprintf("g_recaptcha_token: %v\n", response.Token))
				requestBody["g_recaptcha_token"] = response.Token
				logger.Infof(c.Request.Context(), fmt.Sprintf("cheat success!"))
				return requestBody, nil
			} else {
				logger.Errorf(c.Request.Context(), fmt.Sprintf("读取/genspark token 失败,查看 playwright-proxy log"))
				return nil, err
			}
		} else {
			logger.Errorf(c.Request.Context(), fmt.Sprintf("请求/genspark失败,查看 playwright-proxy log"))
			return nil, err
		}
	}
}

// 处理流式数据的辅助函数，返回bool表示是否继续处理
func processStreamData(c *gin.Context, data string, projectId *string, cookie, responseId, model string, jsonData []byte, searchModel bool, totalContent, totalReasoningContent *string) bool {
	data = strings.TrimSpace(data)
	//if !strings.HasPrefix(data, "data: ") {
	//	return true
	//}
	data = strings.TrimPrefix(data, "data: ")
	if !strings.HasPrefix(data, "{\"id\":") && !strings.HasPrefix(data, "{\"message_id\":") {
		return true
	}
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		logger.Errorf(c.Request.Context(), "Failed to unmarshal event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}

	eventType, ok := event["type"].(string)
	if !ok {
		return true
	}

	switch eventType {
	case "project_start":
		*projectId, _ = event["id"].(string)
	case "message_field":
		if err := handleMessageFieldDelta(c, event, responseId, model, jsonData, totalContent, totalReasoningContent); err != nil {
			logger.Errorf(c.Request.Context(), "handleMessageFieldDelta err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return false
		}
	case "message_field_delta":
		if err := handleMessageFieldDelta(c, event, responseId, model, jsonData, totalContent, totalReasoningContent); err != nil {
			logger.Errorf(c.Request.Context(), "handleMessageFieldDelta err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return false
		}

	case "message_result":
		// Сохраняем значения для горутины
		currentProjectId := ""
		if projectId != nil {
			currentProjectId = *projectId
		}
		go func(pid string, ck string, mdl string, gc *gin.Context) {
			ctx := gc.Request.Context()
			if config.AutoModelChatMapType == 1 {
				logger.Debugf(ctx, "[DELETE] STREAM: Saving session instead of deleting, projectId=%s, model=%s", pid, mdl)
				config.GlobalSessionManager.AddSession(ck, mdl, pid)
			} else {
				if config.AutoDelChat == 1 {
					logger.Debugf(ctx, "[DELETE] STREAM: Auto-delete enabled, projectId=%s, model=%s", pid, mdl)
					client := cycletls.Init()
					defer safeClose(client)
					if _, err := makeDeleteRequest(gc, client, ck, pid); err != nil {
						logger.Errorf(ctx, "[DELETE] STREAM: Delete failed for projectId=%s, error=%v", pid, err)
					}
				} else {
					logger.Debugf(ctx, "[DELETE] STREAM: Auto-delete disabled, skipping projectId=%s", pid)
				}
			}
		}(currentProjectId, cookie, model, c)

		return handleMessageResult(c, event, responseId, model, jsonData, searchModel)
	}

	return true
}

func makeStreamRequest(c *gin.Context, client cycletls.CycleTLS, jsonData []byte, cookie string) (<-chan cycletls.SSEResponse, error) {

	options := cycletls.Options{
		Timeout: 10 * 60 * 60,
		Proxy:   config.ProxyUrl, // 在每个请求中设置代理
		Body:    string(jsonData),
		Method:  "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "text/event-stream",
			"Origin":       baseURL,
			"Referer":      baseURL + "/",
			"Cookie":       cookie,
			"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
		},
	}

	logger.Debug(c.Request.Context(), fmt.Sprintf("cookie: %v", cookie))

	if config.DebugLogNetwork {
		logger.Debugf(c.Request.Context(), "\n=== OUTGOING STREAM REQUEST ===\nURL: %s\nHeaders: %v\nBody: %s\n===============================", apiEndpoint, options.Headers, options.Body)
	}

	sseChan, err := client.DoSSE(apiEndpoint, options, "POST")
	if err != nil {
		logger.Errorf(c, "Failed to make stream request: %v", err)
		return nil, fmt.Errorf("Failed to make stream request: %v", err)
	}
	return sseChan, nil
}

// handleNonStreamRequest 处理非流式请求
//
//	func handleNonStreamRequest(c *gin.Context, client cycletls.CycleTLS, cookie string, jsonData []byte, modelName string) {
//		response, err := makeRequest(client, jsonData, cookie, false)
//		if err != nil {
//			logger.Errorf(c.Request.Context(), "makeRequest err: %v", err)
//			c.JSON(500, gin.H{"error": err.Error()})
//			return
//		}
//
//		reader := strings.NewReader(response.Body)
//		scanner := bufio.NewScanner(reader)
//
//		var content string
//		var firstline string
//		for scanner.Scan() {
//			line := scanner.Text()
//			firstline = line
//			logger.Debug(c.Request.Context(), strings.TrimSpace(line))
//
//			if common.IsCloudflareChallenge(line) {
//				logger.Errorf(c.Request.Context(), "Detected Cloudflare Challenge Page")
//				c.JSON(500, gin.H{"error": "Detected Cloudflare Challenge Page"})
//				return
//			}
//
//			if common.IsRateLimit(line) {
//				logger.Errorf(c.Request.Context(), "Cookie has reached the rate Limit")
//				c.JSON(500, gin.H{"error": "Cookie has reached the rate Limit"})
//				return
//			}
//
//			if strings.HasPrefix(line, "data: ") {
//				data := strings.TrimPrefix(line, "data: ")
//				var parsedResponse struct {
//					Type      string `json:"type"`
//					FieldName string `json:"field_name"`
//					Content   string `json:"content"`
//				}
//				if err := json.Unmarshal([]byte(data), &parsedResponse); err != nil {
//					logger.Warnf(c.Request.Context(), "Failed to unmarshal response: %v", err)
//					continue
//				}
//				if parsedResponse.Type == "message_result" {
//					content = parsedResponse.Content
//					break
//				}
//			}
//		}
//
//		if content == "" {
//			logger.Errorf(c.Request.Context(), firstline)
//			c.JSON(500, gin.H{"error": "No valid response content"})
//			return
//		}
//
//		promptTokens := common.CountTokenText(string(jsonData), modelName)
//		completionTokens := common.CountTokenText(content, modelName)
//
//		finishReason := "stop"
//		// 创建并返回 OpenAIChatCompletionResponse 结构
//		resp := model.OpenAIChatCompletionResponse{
//			ID:      fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405")),
//			Object:  "chat.completion",
//			Created: time.Now().Unix(),
//			Model:   modelName,
//			Choices: []model.OpenAIChoice{
//				{
//					Message: model.OpenAIMessage{
//						Role:    "assistant",
//						Content: content,
//					},
//					FinishReason: &finishReason,
//				},
//			},
//			Usage: model.OpenAIUsage{
//				PromptTokens:     promptTokens,
//				CompletionTokens: completionTokens,
//				TotalTokens:      promptTokens + completionTokens,
//			},
//		}
//
//		c.JSON(200, resp)
//	}
func handleNonStreamRequest(c *gin.Context, client cycletls.CycleTLS, cookie string, cookieManager *config.CookieManager, requestBody map[string]interface{}, modelName string, searchModel bool) {
	const (
		errCloudflareChallengeMsg = "Detected Cloudflare Challenge Page"
		errCloudflareBlock        = "CloudFlare: Sorry, you have been blocked"
		errServerErrMsg           = "An error occurred with the current request, please try again."
		errServiceUnavailable     = "Genspark Service Unavailable"
		errNoValidResponseContent = "No valid response content"
	)

	ctx := c.Request.Context()
	maxRetries := len(cookieManager.Cookies)

	for attempt := 0; attempt < maxRetries; attempt++ {
		requestBody, err := cheat(requestBody, c, cookie)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to marshal request body"})
			return
		}
		response, err := makeRequest(client, jsonData, cookie, false)
		if err != nil {
			logger.Errorf(ctx, "makeRequest err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		scanner := bufio.NewScanner(strings.NewReader(response.Body))
		var content string
		var reasoningContent string
		var firstLine string
		var projectId string
		isRateLimit := false
		_ = errNoValidResponseContent // Suppress unused variable warning

		for scanner.Scan() {
			line := scanner.Text()
			if firstLine == "" {
				firstLine = line
			}
			if line == "" {
				continue
			}
			logger.Debug(ctx, strings.TrimSpace(line))

			switch {
			case common.IsCloudflareChallenge(line):
				logger.Errorf(ctx, errCloudflareChallengeMsg)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errCloudflareChallengeMsg})
				return
			case common.IsCloudflareBlock(line):
				logger.Errorf(ctx, errCloudflareBlock)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errCloudflareBlock})
				return
			case common.IsRateLimit(line):
				isRateLimit = true
				logger.Warnf(ctx, "Cookie rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
				config.AddRateLimitCookie(cookie, time.Now().Add(time.Duration(config.RateLimitCookieLockDuration)*time.Second))
				break
			case common.IsFreeLimit(line):
				isRateLimit = true
				logger.Warnf(ctx, "Cookie free rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
				config.AddRateLimitCookie(cookie, time.Now().Add(24*60*60*time.Second))
				break
			case common.IsNotLogin(line):
				isRateLimit = true
				logger.Warnf(ctx, "Cookie Not Login, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
				config.RemoveCookie(cookie)
				break
			case common.IsServiceUnavailablePage(line):
				logger.Errorf(ctx, errServiceUnavailable)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errServiceUnavailable})
				return
			case common.IsServerError(line):
				logger.Errorf(ctx, errServerErrMsg)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errServerErrMsg})
				return
			case strings.HasPrefix(line, "data: "):
				data := strings.TrimPrefix(line, "data: ")
				var parsedResponse struct {
					Type       string `json:"type"`
					FieldName  string `json:"field_name"`
					FieldValue string `json:"field_value"`
					Content    string `json:"content"`
					Id         string `json:"id"`
					Delta      string `json:"delta"`
				}
				if err := json.Unmarshal([]byte(data), &parsedResponse); err != nil {
					// Игнорируем ошибки парсинга для некоторых событий (field_value может быть объектом)
					continue
				}
				if parsedResponse.Type == "project_start" {
					projectId = parsedResponse.Id
				}
				if parsedResponse.Type == "message_field_delta" {
					// Собираем контент из session_state.answer (для thinking моделей)
					if parsedResponse.FieldName == "session_state.answer" {
						content = content + parsedResponse.Delta
					}
					// Собираем контент из streaming_detail_answer
					if strings.Contains(parsedResponse.FieldName, "session_state.streaming_detail_answer") {
						content = content + parsedResponse.Delta
					}
					// Собираем reasoning/thinking контент
					if strings.HasPrefix(parsedResponse.FieldName, "session_state.layer_") {
						reasoningContent = reasoningContent + parsedResponse.Delta
					}

					if config.ReasoningHide != 1 {
						if parsedResponse.FieldName == "session_state.answerthink" {
							reasoningContent = reasoningContent + parsedResponse.Delta
						}
					}
				}
				// Для моделей типа o1, o3-mini-high - field_value содержит весь ответ
				if parsedResponse.Type == "message_field" {
					if (modelName == "o1" || modelName == "o3-mini-high") && parsedResponse.FieldName == "session_state.answer" {
						content = parsedResponse.FieldValue
					}
				}
				if parsedResponse.Type == "message_result" {
					// Удаление/сохранение сессии
					go func(pid string, ck string, mdl string, gc *gin.Context) {
						ctx := gc.Request.Context()
						if config.AutoModelChatMapType == 1 {
							logger.Infof(ctx, "[DELETE] NON-STREAM: Saving session instead of deleting, projectId=%s, model=%s", pid, mdl)
							config.GlobalSessionManager.AddSession(ck, mdl, pid)
						} else {
							if config.AutoDelChat == 1 {
								logger.Infof(ctx, "[DELETE] NON-STREAM: Auto-delete enabled, projectId=%s, model=%s", pid, mdl)
								client := cycletls.Init()
								defer safeClose(client)
								if _, err := makeDeleteRequest(gc, client, ck, pid); err != nil {
									logger.Errorf(ctx, "[DELETE] NON-STREAM: Delete failed for projectId=%s, error=%v", pid, err)
								}
							} else {
								logger.Debugf(ctx, "[DELETE] NON-STREAM: Auto-delete disabled, skipping projectId=%s", pid)
							}
						}
					}(projectId, cookie, modelName, c)
					// Если content пустой, попробуем взять из message_result
					if content == "" {
						if modelName == "o1" && searchModel {
							var contentObj Content
							if err := json.Unmarshal([]byte(parsedResponse.Content), &contentObj); err == nil {
								content = contentObj.DetailAnswer
							}
						} else {
							content = strings.TrimSpace(parsedResponse.Content)
						}
					}
					break
				}
			}
		}

		if !isRateLimit {
			if content == "" {
				logger.Warnf(ctx, firstLine)
				//c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidResponseContent})
			} else {
				promptTokens := common.CountTokenText(string(jsonData), modelName)
				completionTokens := common.CountTokenText(content, modelName)
				finishReason := "stop"

				c.JSON(http.StatusOK, model.OpenAIChatCompletionResponse{
					ID:      fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405")),
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   modelName,
					Choices: []model.OpenAIChoice{{
						Message: &model.OpenAIMessage{
							Role:             "assistant",
							Content:          content,
							ReasoningContent: reasoningContent,
							Reasoning:        reasoningContent,
						},
						FinishReason: &finishReason,
					}},
					Usage: &model.OpenAIUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
						CompletionTokensDetails: &model.OpenAICompletionTokensDetails{
							ReasoningTokens: common.CountTokenText(reasoningContent, modelName),
						},
					},
				})
				return
			}
		}

		cookie, err = cookieManager.GetNextCookie()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No more valid cookies available"})
			return
		}
		// requestBody重制chatId
		currentQueryString := fmt.Sprintf("type=%s", chatType)
		if chatId, ok := config.GlobalSessionManager.GetChatID(cookie, modelName); ok {
			currentQueryString = fmt.Sprintf("id=%s&type=%s", chatId, chatType)
		}
		requestBody["current_query_string"] = currentQueryString
	}

	logger.Errorf(ctx, "All cookies exhausted after %d attempts", maxRetries)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "All cookies are temporarily unavailable."})
}

func OpenaiModels(c *gin.Context) {
	var modelsResp []string

	modelsResp = common.DefaultOpenaiModelList

	var openaiModelListResponse model.OpenaiModelListResponse
	var openaiModelResponse []model.OpenaiModelResponse
	openaiModelListResponse.Object = "list"

	for _, modelResp := range modelsResp {
		openaiModelResponse = append(openaiModelResponse, model.OpenaiModelResponse{
			ID:     modelResp,
			Object: "model",
		})
	}
	openaiModelListResponse.Data = openaiModelResponse
	c.JSON(http.StatusOK, openaiModelListResponse)
	return
}

func ImagesForOpenAI(c *gin.Context) {

	client := cycletls.Init()
	defer safeClose(client)

	var openAIReq model.OpenAIImagesGenerationRequest
	if err := c.BindJSON(&openAIReq); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// 初始化cookie
	//cookieManager := config.NewCookieManager()
	//cookie, err := cookieManager.GetRandomCookie()
	//
	//if err != nil {
	//	logger.Errorf(c.Request.Context(), "Failed to get initial cookie: %v", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
	//	return
	//}

	resp, err := ImageProcess(c, client, openAIReq)
	if err != nil {
		logger.Errorf(c.Request.Context(), fmt.Sprintf("ImageProcess err  %v\n", err))
		c.JSON(http.StatusInternalServerError, model.OpenAIErrorResponse{
			OpenAIError: model.OpenAIError{
				Message: err.Error(),
				Type:    "request_error",
				Code:    "500",
			},
		})
		return
	} else {
		c.JSON(200, resp)
	}

}

func ImageProcess(c *gin.Context, client cycletls.CycleTLS, openAIReq model.OpenAIImagesGenerationRequest) (*model.OpenAIImagesGenerationResponse, error) {
	const (
		errNoValidCookies = "No valid cookies available"
		errRateLimitMsg   = "Rate limit reached, please try again later"
		errServerErrMsg   = "An error occurred with the current request, please try again"
		errNoValidTaskIDs = "No valid task IDs received"
	)

	var (
		sessionImageChatManager *config.SessionMapManager
		maxRetries              int
		cookie                  string
		chatId                  string
	)

	cookieManager := config.NewCookieManager()
	sessionImageChatManager = config.NewSessionMapManager()
	ctx := c.Request.Context()

	// Initialize session manager and get initial cookie
	if len(config.SessionImageChatMap) == 0 {
		//logger.Warnf(ctx, "未配置环境变量 SESSION_IMAGE_CHAT_MAP, 可能会生图失败!")
		maxRetries = len(cookieManager.Cookies)

		var err error
		cookie, err = cookieManager.GetRandomCookie()
		if err != nil {
			logger.Errorf(ctx, "Failed to get initial cookie: %v", err)
			return nil, fmt.Errorf(errNoValidCookies)
		}
	} else {
		maxRetries = sessionImageChatManager.GetSize()
		cookie, chatId, _ = sessionImageChatManager.GetRandomKeyValue()
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create request body
		requestBody, err := createImageRequestBody(c, cookie, &openAIReq, chatId)
		if err != nil {
			logger.Errorf(ctx, "Failed to create request body: %v", err)
			return nil, err
		}

		// Marshal request body
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			logger.Errorf(ctx, "Failed to marshal request body: %v", err)
			return nil, err
		}

		// Make request
		response, err := makeImageRequest(client, jsonData, cookie)
		if err != nil {
			logger.Errorf(ctx, "Failed to make image request: %v", err)
			return nil, err
		}

		body := response.Body

		// Handle different response cases
		switch {
		case common.IsRateLimit(body):
			logger.Warnf(ctx, "Cookie rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
			//if sessionImageChatManager != nil {
			//	cookie, chatId, err = sessionImageChatManager.GetNextKeyValue()
			//	if err != nil {
			//		logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
			//		c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
			//		return nil, fmt.Errorf(errNoValidCookies)
			//	}
			//} else {
			//cookieManager := config.NewCookieManager()
			config.AddRateLimitCookie(cookie, time.Now().Add(time.Duration(config.RateLimitCookieLockDuration)*time.Second))
			cookie, err = cookieManager.GetNextCookie()
			if err != nil {
				logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
				return nil, fmt.Errorf(errNoValidCookies)
				//}
			}
			continue
		case common.IsFreeLimit(body):
			logger.Warnf(ctx, "Cookie free rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
			//if sessionImageChatManager != nil {
			//	cookie, chatId, err = sessionImageChatManager.GetNextKeyValue()
			//	if err != nil {
			//		logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
			//		c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
			//		return nil, fmt.Errorf(errNoValidCookies)
			//	}
			//} else {
			//cookieManager := config.NewCookieManager()
			config.AddRateLimitCookie(cookie, time.Now().Add(24*60*60*time.Second))
			// 删除cookie
			//config.RemoveCookie(cookie)
			cookie, err = cookieManager.GetNextCookie()
			if err != nil {
				logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
				return nil, fmt.Errorf(errNoValidCookies)
				//}
			}
			continue
		case common.IsNotLogin(body):
			logger.Warnf(ctx, "Cookie Not Login, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
			//if sessionImageChatManager != nil {
			//	//sessionImageChatManager.RemoveKey(cookie)
			//	cookie, chatId, err = sessionImageChatManager.GetNextKeyValue()
			//	if err != nil {
			//		logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
			//		c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
			//		return nil, fmt.Errorf(errNoValidCookies)
			//	}
			//} else {
			//cookieManager := config.NewCookieManager()
			//err := cookieManager.RemoveCookie(cookie)
			//if err != nil {
			//	logger.Errorf(ctx, "Failed to remove cookie: %v", err)
			//}
			cookie, err = cookieManager.GetNextCookie()
			if err != nil {
				logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
				return nil, fmt.Errorf(errNoValidCookies)
				//}

			}
			continue
		case common.IsServerError(body):
			logger.Errorf(ctx, errServerErrMsg)
			return nil, fmt.Errorf(errServerErrMsg)
		case common.IsServerOverloaded(body):
			//logger.Errorf(ctx, fmt.Sprintf("Server overloaded, please try again later.%s", "官方服务超载或环境变量 SESSION_IMAGE_CHAT_MAP 未配置"))
			logger.Errorf(ctx, fmt.Sprintf("Server overloaded, please try again later.%s", "官方服务超载"))
			return nil, fmt.Errorf("Server overloaded, please try again later.")
		}

		// Extract task IDs
		projectId, taskIDs := extractTaskIDs(response.Body)
		if len(taskIDs) == 0 {
			logger.Errorf(ctx, "Response body: %s", response.Body)
			return nil, fmt.Errorf(errNoValidTaskIDs)
		}

		// Poll for image URLs
		imageURLs := pollTaskStatus(c, client, taskIDs, cookie)
		if len(imageURLs) == 0 {
			logger.Warnf(ctx, "No image URLs received, retrying with next cookie")
			continue
		}

		// Create response object
		result := &model.OpenAIImagesGenerationResponse{
			Created: time.Now().Unix(),
			Data:    make([]*model.OpenAIImagesGenerationDataResponse, 0, len(imageURLs)),
		}

		// Process image URLs
		for _, url := range imageURLs {
			data := &model.OpenAIImagesGenerationDataResponse{
				URL:           url,
				RevisedPrompt: openAIReq.Prompt,
			}

			if openAIReq.ResponseFormat == "b64_json" {
				base64Str, err := getBase64ByUrl(data.URL)
				if err != nil {
					logger.Errorf(ctx, "getBase64ByUrl error: %v", err)
					continue
				}
				data.B64Json = "data:image/webp;base64," + base64Str
			}

			result.Data = append(result.Data, data)
		}

		// Handle successful case
		if len(result.Data) > 0 {
			// Delete temporary session if needed
			if config.AutoDelChat == 1 {
				go func(pid string, ck string, gc *gin.Context) {
					ctx := gc.Request.Context()
					logger.Infof(ctx, "[DELETE] IMAGE: Auto-delete enabled, projectId=%s", pid)
					delClient := cycletls.Init()
					defer safeClose(delClient)
					if _, err := makeDeleteRequest(gc, delClient, ck, pid); err != nil {
						logger.Errorf(ctx, "[DELETE] IMAGE: Delete failed for projectId=%s, error=%v", pid, err)
					}
				}(projectId, cookie, c)
			}
			return result, nil
		}
	}

	// All retries exhausted
	logger.Errorf(ctx, "All cookies exhausted after %d attempts", maxRetries)
	return nil, fmt.Errorf("all cookies are temporarily unavailable")
}
func extractTaskIDs(responseBody string) (string, []string) {
	var taskIDs []string
	var projectId string

	// 分行处理响应
	lines := strings.Split(responseBody, "\n")
	for _, line := range lines {

		// 找到包含project_id的行
		if strings.Contains(line, "project_start") {
			// 去掉"data: "前缀
			jsonStr := strings.TrimPrefix(line, "data: ")

			// 解析JSON
			var jsonResp struct {
				ProjectID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &jsonResp); err != nil {
				continue
			}

			// 保存project_id
			projectId = jsonResp.ProjectID
		}

		// 找到包含task_id的行
		if strings.Contains(line, "task_id") {
			// 去掉"data: "前缀
			jsonStr := strings.TrimPrefix(line, "data: ")

			// 解析外层JSON
			var outerJSON struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &outerJSON); err != nil {
				continue
			}

			// 解析内层JSON (content字段)
			var innerJSON struct {
				GeneratedImages []struct {
					TaskID string `json:"task_id"`
				} `json:"generated_images"`
			}
			if err := json.Unmarshal([]byte(outerJSON.Content), &innerJSON); err != nil {
				continue
			}

			// 提取所有task_id
			for _, img := range innerJSON.GeneratedImages {
				if img.TaskID != "" {
					taskIDs = append(taskIDs, img.TaskID)
				}
			}
		}
	}
	return projectId, taskIDs
}

func pollTaskStatus(c *gin.Context, client cycletls.CycleTLS, taskIDs []string, cookie string) []string {
	var imageURLs []string

	requestData := map[string]interface{}{
		"task_ids": taskIDs,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal request data"})
		return imageURLs
	}

	// Log outgoing request if network logging is enabled
	if config.DebugLogNetwork {
		logger.Debugf(context.Background(), "\n=== OUTGOING REQUEST ===\nURL: %s\nMethod: POST\nHeaders: %v\nBody: %s\n========================",
			"https://www.genspark.ai/api/ig_tasks_status", // Assuming this URL or checking helper
			map[string]string{
				"Content-Type": "application/json",
				"Accept":       "*/*",
				"Origin":       baseURL,
				"Referer":      baseURL + "/",
				"Cookie":       cookie,
				"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
			},
			string(jsonData))
	}

	sseChan, err := client.DoSSE("https://www.genspark.ai/api/ig_tasks_status", cycletls.Options{
		Timeout: 10 * 60 * 60,
		Proxy:   config.ProxyUrl, // 在每个请求中设置代理
		Body:    string(jsonData),
		Method:  "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "*/*",
			"Origin":       baseURL,
			"Referer":      baseURL + "/",
			"Cookie":       cookie,
			"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
		},
	}, "POST")
	if err != nil {
		logger.Errorf(c, "Failed to make stream request: %v", err)
		return imageURLs
	}
	for response := range sseChan {
		if response.Done {
			//logger.Warnf(c.Request.Context(), response.Data)
			return imageURLs
		}

		data := response.Data
		if data == "" {
			continue
		}

		logger.Debug(c.Request.Context(), strings.TrimSpace(data))

		var responseData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &responseData); err != nil {
			continue
		}

		if responseData["type"] == "TASKS_STATUS_COMPLETE" {
			if finalStatus, ok := responseData["final_status"].(map[string]interface{}); ok {
				for _, taskID := range taskIDs {
					if task, exists := finalStatus[taskID].(map[string]interface{}); exists {
						if status, ok := task["status"].(string); ok && status == "SUCCESS" {
							if urls, ok := task["image_urls"].([]interface{}); ok && len(urls) > 0 {
								if imageURL, ok := urls[0].(string); ok {
									imageURLs = append(imageURLs, imageURL)
								}
							}
						}
					}
				}
			}
		}
	}

	return imageURLs
}

func getBase64ByUrl(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	// Encode the image data to Base64
	base64Str := base64.StdEncoding.EncodeToString(imgData)
	return base64Str, nil
}

// handleToolUseRequest handles requests with tools - injects meta-prompt and parses tool calls from response
func handleToolUseRequest(c *gin.Context, client cycletls.CycleTLS, cookie string, cookieManager *config.CookieManager, openAIReq *model.OpenAIChatCompletionRequest, isSearchModel bool) {
	ctx := c.Request.Context()

	// Log request start with tool info
	logger.LogRequestStart(ctx, openAIReq.Model, true)
	logger.LogToolEvent(ctx, "TOOL_PROMPT_PREPARING", map[string]interface{}{
		"tools_count": len(openAIReq.Tools),
	})

	// Add tool system prompt to messages
	openAIReq.Messages = tooluse.PrependToolSystemMessage(openAIReq.Messages, openAIReq.Tools)
	logger.LogToolEvent(ctx, "TOOL_PROMPT_INJECTED", map[string]interface{}{
		"messages_count": len(openAIReq.Messages),
	})

	// Create request body (without tools - genspark doesn't support them)
	requestBody, err := createRequestBody(c, client, cookie, openAIReq)
	if err != nil {
		logger.StructuredError(ctx, logger.SubTool, fmt.Sprintf("Failed to create request body: %v", err))
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// For tool-use, we always need to get the full response first to parse it
	// So we handle it as non-stream internally, then convert to stream if needed
	if openAIReq.Stream {
		handleToolUseStreamRequest(c, client, cookie, cookieManager, requestBody, openAIReq, isSearchModel)
	} else {
		handleToolUseNonStreamRequest(c, client, cookie, cookieManager, requestBody, openAIReq, isSearchModel)
	}

	logger.StructuredDebug(ctx, logger.SubTool, "REQ_COMPLETE", "Tool use request completed")
}

// handleToolUseNonStreamRequest handles non-streaming tool use requests
func handleToolUseNonStreamRequest(c *gin.Context, client cycletls.CycleTLS, cookie string, cookieManager *config.CookieManager, requestBody map[string]interface{}, openAIReq *model.OpenAIChatCompletionRequest, searchModel bool) {
	ctx := c.Request.Context()
	maxRetries := len(cookieManager.Cookies)

	for attempt := 0; attempt < maxRetries; attempt++ {
		logger.Debugf(ctx, "Attempt %d/%d with cookie: %s...", attempt+1, maxRetries, cookie[:10])

		requestBody, err := cheat(requestBody, c, cookie)
		if err != nil {
			logger.Errorf(ctx, "cheat err: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			logger.Errorf(ctx, "json marshal err: %v", err)
			c.JSON(500, gin.H{"error": "Failed to marshal request body"})
			return
		}

		response, err := makeRequest(client, jsonData, cookie, false)
		if err != nil {
			logger.Errorf(ctx, "makeRequest err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		scanner := bufio.NewScanner(strings.NewReader(response.Body))
		var content string
		var firstLine string
		var projectId string
		isRateLimit := false

		for scanner.Scan() {
			line := scanner.Text()
			if firstLine == "" {
				firstLine = line
			}
			if line == "" {
				continue
			}

			switch {
			case common.IsCloudflareChallenge(line), common.IsCloudflareBlock(line):
				logger.Errorf(ctx, "Cloudflare blocked: %s", line)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Cloudflare blocked"})
				return
			case common.IsRateLimit(line), common.IsFreeLimit(line), common.IsNotLogin(line):
				logger.Warnf(ctx, "Rate limit/Auth error: %s", line)
				isRateLimit = true
				config.AddRateLimitCookie(cookie, time.Now().Add(time.Duration(config.RateLimitCookieLockDuration)*time.Second))
				break
			case common.IsServiceUnavailablePage(line), common.IsServerError(line):
				logger.Errorf(ctx, "Server error: %s", line)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Server error"})
				return
			case strings.HasPrefix(line, "data: "):
				data := strings.TrimPrefix(line, "data: ")
				var parsedResponse struct {
					Type       string `json:"type"`
					FieldName  string `json:"field_name"`
					FieldValue string `json:"field_value"`
					Content    string `json:"content"`
					Id         string `json:"id"`
					Delta      string `json:"delta"`
				}
				if err := json.Unmarshal([]byte(data), &parsedResponse); err != nil {
					continue
				}
				if parsedResponse.Type == "project_start" {
					projectId = parsedResponse.Id
					logger.Debugf(ctx, "Project started: %s", projectId)
				}
				if parsedResponse.Type == "message_field_delta" {
					logger.Debugf(ctx, "Field Delta: Name=%s, Delta=%s", parsedResponse.FieldName, parsedResponse.Delta)
					if parsedResponse.FieldName == "session_state.answer" ||
						strings.Contains(parsedResponse.FieldName, "session_state.streaming_detail_answer") ||
						parsedResponse.FieldName == "content" {
						content = content + parsedResponse.Delta
					}
				}
				if parsedResponse.Type == "message_field" {
					logger.Debugf(ctx, "Field Value: Name=%s, Value=%s", parsedResponse.FieldName, parsedResponse.FieldValue)
					if parsedResponse.FieldName == "session_state.answer" || parsedResponse.FieldName == "content" {
						content = parsedResponse.FieldValue
					}
				}
				if parsedResponse.Type == "message_result" {
					go func(pid string, ck string, mdl string, gc *gin.Context) {
						ctx := gc.Request.Context()
						if config.AutoDelChat == 1 {
							logger.Debugf(ctx, "[DELETE] TOOL-USE: Auto-delete enabled, projectId=%s, model=%s", pid, mdl)
							delClient := cycletls.Init()
							defer safeClose(delClient)
							if _, err := makeDeleteRequest(gc, delClient, ck, pid); err != nil {
								logger.Errorf(ctx, "[DELETE] TOOL-USE: Delete failed for projectId=%s, error=%v", pid, err)
							}
						} else {
							logger.Debugf(ctx, "[DELETE] TOOL-USE: Auto-delete disabled, skipping projectId=%s", pid)
						}
					}(projectId, cookie, openAIReq.Model, c)
					if content == "" {
						content = strings.TrimSpace(parsedResponse.Content)
					}
					break
				}
			}
		}

		if !isRateLimit && content != "" {
			// Log model response
			logger.LogToolEvent(ctx, "MODEL_RAW_RESPONSE", map[string]interface{}{
				"content_length": len(content),
				"content":        content,
			})

			// Save debug payload to file if enabled
			logger.SaveDebugPayload(ctx, &logger.DebugPayload{
				RequestID:   fmt.Sprintf("%v", ctx.Value("X-Request-Id")),
				Timestamp:   time.Now().Format(time.RFC3339),
				Subsystem:   logger.SubTool,
				Phase:       "MODEL_RESPONSE",
				Model:       openAIReq.Model,
				RawResponse: content,
			})

			// Parse the response to check for tool calls
			toolResp, err := tooluse.ParseToolCallFromText(content)
			if err != nil {
				// Model didn't follow the format - fallback to regular response
				logger.LogToolEvent(ctx, "PARSE_FALLBACK", map[string]interface{}{
					"reason": err.Error(),
				})

				promptTokens := common.CountTokenText(string(jsonData), openAIReq.Model)
				completionTokens := common.CountTokenText(content, openAIReq.Model)
				finishReason := "stop"

				c.JSON(http.StatusOK, model.OpenAIChatCompletionResponse{
					ID:      fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405")),
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   openAIReq.Model,
					Choices: []model.OpenAIChoice{{
						Message: &model.OpenAIMessage{
							Role:    "assistant",
							Content: content,
						},
						FinishReason: &finishReason,
					}},
					Usage: &model.OpenAIUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				})
				return
			}

			// Validate tool call if it's a tool call
			if err := tooluse.ValidateToolCall(toolResp, openAIReq.Tools); err != nil {
				c.JSON(http.StatusBadRequest, model.OpenAIErrorResponse{
					OpenAIError: model.OpenAIError{
						Message: err.Error(),
						Type:    "invalid_tool_call",
						Code:    "400",
					},
				})
				return
			}

			promptTokens := common.CountTokenText(string(jsonData), openAIReq.Model)
			completionTokens := common.CountTokenText(content, openAIReq.Model)

			if tooluse.IsToolCallResponse(toolResp) {
				// Convert to OpenAI tool call format
				toolCall, err := tooluse.ConvertToOpenAIToolCall(toolResp)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				finishReason := "tool_calls"
				c.JSON(http.StatusOK, model.OpenAIChatCompletionResponse{
					ID:      fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405")),
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   openAIReq.Model,
					Choices: []model.OpenAIChoice{{
						Message: &model.OpenAIMessage{
							Role:      "assistant",
							Content:   "",
							ToolCalls: []model.OpenAIToolCall{*toolCall},
						},
						FinishReason: &finishReason,
					}},
					Usage: &model.OpenAIUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				})
			} else {
				// Regular response
				finishReason := "stop"
				c.JSON(http.StatusOK, model.OpenAIChatCompletionResponse{
					ID:      fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405")),
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   openAIReq.Model,
					Choices: []model.OpenAIChoice{{
						Message: &model.OpenAIMessage{
							Role:    "assistant",
							Content: toolResp.Content,
						},
						FinishReason: &finishReason,
					}},
					Usage: &model.OpenAIUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				})
			}
			return
		}

		cookie, err = cookieManager.GetNextCookie()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No more valid cookies available"})
			return
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "All cookies are temporarily unavailable."})
}

// handleToolUseStreamRequest handles streaming tool use requests
func handleToolUseStreamRequest(c *gin.Context, client cycletls.CycleTLS, cookie string, cookieManager *config.CookieManager, requestBody map[string]interface{}, openAIReq *model.OpenAIChatCompletionRequest, searchModel bool) {
	ctx := c.Request.Context()
	maxRetries := len(cookieManager.Cookies)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	responseId := fmt.Sprintf(responseIDFormat, time.Now().Format("20060102150405"))

	c.Stream(func(w io.Writer) bool {
		logger.Debugf(ctx, "Starting stream loop. MaxRetries=%d", maxRetries)
		for attempt := 0; attempt < maxRetries; attempt++ {
			logger.Debugf(ctx, "Attempt %d/%d with cookie: %s...", attempt+1, maxRetries, cookie[:10])

			requestBody, err := cheat(requestBody, c, cookie)
			if err != nil {
				logger.Errorf(ctx, "cheat err: %v", err)
				return false
			}
			jsonData, err := json.Marshal(requestBody)
			if err != nil {
				logger.Errorf(ctx, "json marshal err: %v", err)
				return false
			}

			// Use incremental parser
			parser := tooluse.NewStreamParser()
			logger.Debugf(ctx, "Making stream request...")
			sseChan, err := makeStreamRequest(c, client, jsonData, cookie)
			if err != nil {
				logger.Errorf(ctx, "makeStreamRequest err: %v", err)
				return false
			}

			// Track if we sent any tool call ID (only need to send once)
			toolCallSent := false
			toolCallID := "call_" + uuid.New().String()[:8]

			var totalContent string
			var totalReasoning string

			isRateLimit := false
			var projectId string

			logger.Debugf(ctx, "Reading from sseChan...")
			for response := range sseChan {
				if response.Done {
					logger.Debugf(ctx, "SSE Channel closed/done.")
					break
				}

				data := strings.TrimSpace(response.Data)
				if data == "" {
					continue
				}
				logger.Debugf(ctx, "Received SSE Data: %s", data)

				if common.IsRateLimit(data) || common.IsFreeLimit(data) || common.IsNotLogin(data) {
					logger.Warnf(ctx, "Cookie invalid/rate-limited: %s", data)
					isRateLimit = true
					config.AddRateLimitCookie(cookie, time.Now().Add(time.Duration(config.RateLimitCookieLockDuration)*time.Second))
					break
				}

				data = strings.TrimPrefix(data, "data: ")
				if !strings.HasPrefix(data, "{") {
					continue
				}

				var chunk string

				// Re-implementing correctly:
				// Need to parse generic map first to check type and id
				var eventMap map[string]interface{}
				if err := json.Unmarshal([]byte(data), &eventMap); err != nil {
					logger.Errorf(c.Request.Context(), "[ERROR] Failed to unmarshal event: %v | Data: %s", err, data[6:])
					continue
				}

				if config.DebugLogBody {
					logger.Debugf(c.Request.Context(), "[DEBUG] Stream Event: %+v", eventMap)
				}

				eventType, _ := eventMap["type"].(string)
				// logger.Debugf(ctx, "Event Type: %s", eventType)

				if eventType == "project_start" {
					if id, ok := eventMap["id"].(string); ok {
						projectId = id
						logger.Debugf(ctx, "CHAT_CREATED: Stream started with projectId=%s", projectId)
					}
				} else if eventType == "message_result" {
					logger.Debugf(ctx, "STREAM_COMPLETE: Received message_result for projectId=%s", projectId)
					// Trigger deletion
					go func(pid string, ck string, mdl string, gc *gin.Context) {
						if pid == "" {
							return
						}
						// Create a new context for the goroutine to avoid using the cancelled request context
						// But for logging we might want original trace id?
						// Usually better to use Background or detached context if request ends.
						// For now, keeping as is but be aware of context cancellation.
						ctx := context.Background()

						if config.AutoDelChat == 1 {
							logger.Debugf(ctx, "[DELETE] TOOL-STREAM: Auto-delete enabled, projectId=%s, model=%s", pid, mdl)
							delClient := cycletls.Init()
							defer safeClose(delClient)
							// Note: makeDeleteRequest uses gc (gin.Context). If request is done, this might be issue.
							// But usually safe enough for quick calls.
							if _, err := makeDeleteRequest(gc, delClient, ck, pid); err != nil {
								logger.Errorf(ctx, "[DELETE] TOOL-STREAM: Delete failed for projectId=%s, error=%v", pid, err)
							} else {
								logger.Debugf(ctx, "[DELETE] TOOL-STREAM: Delete request sent for projectId=%s", pid)
							}
						} else {
							logger.Debugf(ctx, "[DELETE] TOOL-STREAM: Auto-delete disabled, skipping projectId=%s", pid)
						}
					}(projectId, cookie, openAIReq.Model, c)
				} else if eventType == "message_field" || eventType == "message_field_delta" {
					fieldName, _ := eventMap["field_name"].(string)
					delta, _ := eventMap["delta"].(string)

					// If delta is empty, try field_value (some models use this for content updates)
					if delta == "" {
						if val, ok := eventMap["field_value"].(string); ok {
							delta = val
						}
					}

					if fieldName == "session_state.answer" ||
						strings.Contains(fieldName, "session_state.streaming_detail_answer") ||
						fieldName == "content" { // Also check for direct "content" field
						chunk = delta
						totalContent += delta
					} else if strings.HasPrefix(fieldName, "session_state.layer_") ||
						(config.ReasoningHide != 1 && fieldName == "session_state.answerthink") {
						// Stream reasoning immediately
						totalReasoning += delta
						streamResp := model.OpenAIChatCompletionResponse{
							ID:      responseId,
							Object:  "chat.completion.chunk",
							Created: time.Now().Unix(),
							Model:   openAIReq.Model,
							Choices: []model.OpenAIChoice{{
								Index: 0,
								Delta: &model.OpenAIDelta{
									Role:             "assistant",
									ReasoningContent: delta,
									Reasoning:        delta,
								},
								FinishReason: nil,
							}},
						}
						sendSSEvent(c, streamResp)
					}
				}

				if chunk != "" {
					// Feed chunk to parser
					params, err := parser.Process(chunk)
					if err != nil {
						logger.Warnf(ctx, "Parser error: %v", err)
						continue
					}

					for _, p := range params {
						if p.Type == "content" {
							// finishReason := "stop" // unused
							streamResp := model.OpenAIChatCompletionResponse{
								ID:      responseId,
								Object:  "chat.completion.chunk",
								Created: time.Now().Unix(),
								Model:   openAIReq.Model,
								Choices: []model.OpenAIChoice{{
									Index: 0,
									Delta: &model.OpenAIDelta{
										Role:    "assistant",
										Content: p.Content,
									},
									FinishReason: nil,
								}},
							}
							sendSSEvent(c, streamResp)
						} else if p.Type == "tool_call_inc" {
							// Send tool call delta
							delta := model.OpenAIDelta{
								Role: "assistant", // only needed for first chunk? OpenAI handles it
							}

							toolDelta := model.OpenAIDeltaToolCall{
								Index: 0,
							}

							if !toolCallSent {
								toolDelta.ID = toolCallID
								toolDelta.Type = "function"
								toolDelta.Function = model.OpenAIDeltaToolCallFunction{
									Name:      p.Tool,
									Arguments: "", // First chunk might just be ID/Name?
									// StreamParser doesn't separate name emission cleanly from args start
									// But p.Tool is available.
									// Use p.Content as arguments delta
								}
								// If p.Content is the start of arguments, we include it
								toolDelta.Function.Arguments = p.Content
								toolCallSent = true
							} else {
								toolDelta.Function = model.OpenAIDeltaToolCallFunction{
									Arguments: p.Content,
								}
							}

							delta.ToolCalls = []model.OpenAIDeltaToolCall{toolDelta}

							streamResp := model.OpenAIChatCompletionResponse{
								ID:      responseId,
								Object:  "chat.completion.chunk",
								Created: time.Now().Unix(),
								Model:   openAIReq.Model,
								Choices: []model.OpenAIChoice{{
									Index:        0,
									Delta:        &delta,
									FinishReason: nil,
								}},
							}
							sendSSEvent(c, streamResp)
						}
					}
				}
			}

			if isRateLimit {
				cookie, _ = cookieManager.GetNextCookie()
				continue
			}

			// If we got here and sseChan closed without rate limit, check if we actually got anything?
			// Assuming successful completion if no rate limit/error triggers.

			// Done
			// We might want to send a final chunk with FinishReason if not sent?
			// But determining correct FinishReason (stop or tool_calls) depends on what we sent.
			// If we sent tool calls, verify if we finished.
			// Given it's streaming, client usually infers from [DONE].
			// Or we send an empty chunk with FinishReason.

			finishReason := "stop"
			if parser.ResponseType == "tool_call" {
				finishReason = "tool_calls"
			}

			streamResp := model.OpenAIChatCompletionResponse{
				ID:      responseId,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   openAIReq.Model,
				Choices: []model.OpenAIChoice{{
					Index:        0,
					Delta:        &model.OpenAIDelta{},
					FinishReason: &finishReason,
				}},
			}
			sendSSEvent(c, streamResp)
			// Send Usage
			promptTokens := common.CountTokenText(string(jsonData), openAIReq.Model)
			completionTokens := common.CountTokenText(totalContent, openAIReq.Model)
			reasoningTokens := common.CountTokenText(totalReasoning, openAIReq.Model)

			usageResp := model.OpenAIChatCompletionResponse{
				ID:      responseId,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   openAIReq.Model,
				Choices: []model.OpenAIChoice{},
				Usage: &model.OpenAIUsage{
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens + reasoningTokens, // Total completion tokens includes reasoning if we want to follow standard, or maybe just content? usually total = prompt + completion. OpenAI puts reasoning tokens INSIDE completion tokens count or purely separate?
					// OpenAI: completion_tokens includes reasoning_tokens.
					TotalTokens: promptTokens + completionTokens + reasoningTokens,
					CompletionTokensDetails: &model.OpenAICompletionTokensDetails{
						ReasoningTokens: reasoningTokens,
					},
				},
			}
			sendSSEvent(c, usageResp)
			c.SSEvent("", " [DONE]")

			return false
		}

		logger.Errorf(ctx, "All cookies exhausted in stream")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "All cookies are temporarily unavailable."})
		return false
	})
}

func safeClose(client cycletls.CycleTLS) {
	if client.ReqChan != nil {
		close(client.ReqChan)
	}
	if client.RespChan != nil {
		close(client.RespChan)
	}
}

func checkLogin(c *gin.Context, client cycletls.CycleTLS, cookie string) {
	logger.Debug(c.Request.Context(), "Checking login status...")
	resp, err := client.Do(loginEndpoint, cycletls.Options{
		Timeout: 30,
		Proxy:   config.ProxyUrl,
		Method:  "GET",
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome",
			"Cookie":     cookie,
		},
	}, "GET")

	if err != nil {
		// Log error but don't block the flow if check fails
		logger.Errorf(c.Request.Context(), "checkLogin request failed: %v", err)
		return
	}

	if resp.Status != 200 {
		logger.Warnf(c.Request.Context(), "checkLogin returned status: %d, body: %s", resp.Status, resp.Body)
		return
	}

	var loginResp model.GensparkLoginResponse
	if err := json.Unmarshal([]byte(resp.Body), &loginResp); err != nil {
		logger.Errorf(c.Request.Context(), "checkLogin unmarshal failed: %v, body: %s", err, resp.Body)
		return
	}

	cogenEmail := "unknown"
	if loginResp.Data.CogenEmail != "" {
		cogenEmail = loginResp.Data.CogenEmail
	} else if loginResp.Data.CogenName != "" {
		cogenEmail = loginResp.Data.CogenName
	}

	if loginResp.Data.IsLogin {
		logger.Debugf(c.Request.Context(), "Login check success. Account: %s, ID: %s", cogenEmail, loginResp.Data.CogenID)
	} else {
		logger.Warnf(c.Request.Context(), "Login check failed (not logged in). Account: %s, ID: %s", cogenEmail, loginResp.Data.CogenID)
	}
}
