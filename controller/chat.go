package controller

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"genspark2api/common"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"genspark2api/model"
	"github.com/deanxv/CycleTLS/cycletls"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	errNoValidCookies = "No valid cookies available"
)

const (
	baseURL          = "https://www.genspark.ai"
	apiEndpoint      = baseURL + "/api/copilot/ask"
	deleteEndpoint   = baseURL + "/api/project/delete?project_id=%s"
	uploadEndpoint   = baseURL + "/api/get_upload_personal_image_url"
	chatType         = "COPILOT_MOA_CHAT"
	imageType        = "COPILOT_MOA_IMAGE"
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
							Message: model.OpenAIMessage{
								Role:    "assistant",
								Content: strings.Join(content, "\n"),
							},
							FinishReason: &finishReason,
						},
					},
					Usage: model.OpenAIUsage{
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

	return io.ReadAll(resp.Body)
}

func createRequestBody(c *gin.Context, client cycletls.CycleTLS, cookie string, openAIReq *model.OpenAIChatCompletionRequest) (map[string]interface{}, error) {
	openAIReq.SystemMessagesProcess(openAIReq.Model)

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
	} else if openAIReq.Model == "deep-seek-r1" {
		openAIReq.FilterUserMessage()
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

	//gRecaptchaToken := ""
	//if config.YesCaptchaClientKey != "" {
	//	// 创建上下文，设置超时
	//	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	//	defer cancel()
	//	// 准备请求参数
	//	req := yescaptcha.RecaptchaV3Request{
	//		WebsiteURL: "https://www.genspark.ai/",
	//		WebsiteKey: "6Leq7KYqAAAAAGdd1NaUBJF9dHTPAKP7DcnaRc66",
	//		PageAction: "copilot",
	//	}
	//
	//	// 解决验证码
	//	response, err := config.YescaptchaClient.SolveRecaptchaV3(ctx, req)
	//	if err != nil {
	//		return map[string]interface{}{}, err
	//	}
	//
	//	gRecaptchaToken = response
	//}

	// 创建请求体
	return map[string]interface{}{
		"type": chatType,
		//"current_query_string": fmt.Sprintf("&type=%s", chatType),
		"current_query_string": currentQueryString,
		"messages":             openAIReq.Messages,
		//"user_s_input":  "我刚刚问了什么问题？",
		"action_params": map[string]interface{}{},
		"extra_data": map[string]interface{}{
			"models":                 models,
			"run_with_another_model": false,
			"writingContent":         nil,
			"request_web_knowledge":  requestWebKnowledge,
		},
		//"g_recaptcha_token": gRecaptchaToken,
	}, nil
}
func createImageRequestBody(c *gin.Context, cookie string, openAIReq *model.OpenAIImagesGenerationRequest, chatId string) (map[string]interface{}, error) {
	//gRecaptchaToken := ""
	//if config.YesCaptchaClientKey != "" {
	//	// 创建上下文，设置超时
	//	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	//	defer cancel()
	//	// 准备请求参数
	//	req := yescaptcha.RecaptchaV3Request{
	//		WebsiteURL: "https://www.genspark.ai/",
	//		WebsiteKey: "6Leq7KYqAAAAAGdd1NaUBJF9dHTPAKP7DcnaRc66",
	//		PageAction: "copilot",
	//	}
	//
	//	// 解决验证码
	//	response, err := config.YescaptchaClient.SolveRecaptchaV3(ctx, req)
	//	if err != nil {
	//		return map[string]interface{}{}, err
	//	}
	//
	//	gRecaptchaToken = response
	//}

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
		currentQueryString = fmt.Sprintf("id=%s&type=%s", chatId, chatType)
	} else {
		currentQueryString = fmt.Sprintf("type=%s", chatId, chatType)
	}

	// 创建请求体
	return map[string]interface{}{
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
		//"g_recaptcha_token": gRecaptchaToken,
	}, nil
}

// createStreamResponse 创建流式响应
func createStreamResponse(responseId, modelName string, jsonData []byte, delta model.OpenAIDelta, finishReason *string) model.OpenAIChatCompletionResponse {
	promptTokens := common.CountTokenText(string(jsonData), modelName)
	completionTokens := common.CountTokenText(delta.Content, modelName)
	return model.OpenAIChatCompletionResponse{
		ID:      responseId,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.OpenAIChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
		Usage: model.OpenAIUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
}

// handleMessageFieldDelta 处理消息字段增量
func handleMessageFieldDelta(c *gin.Context, event map[string]interface{}, responseId, modelName string, jsonData []byte) error {
	fieldName, ok := event["field_name"].(string)
	if !ok || (fieldName != "session_state.answer" &&
		!strings.Contains(fieldName, "session_state.streaming_detail_answer") &&
		//fieldName != "session_state.reflection" &&
		fieldName != "session_state.streaming_markmap") {
		return nil
	}

	var delta string
	if (modelName == "o1" || modelName == "o3-mini-high") && fieldName == "session_state.answer" {
		delta, ok = event["field_value"].(string)
	} else {
		delta, ok = event["delta"].(string)
	}
	if !ok {
		return nil
	}

	streamResp := createStreamResponse(responseId, modelName, jsonData, model.OpenAIDelta{Content: delta, Role: "assistant"}, nil)
	return sendSSEvent(c, streamResp)
}

type Content struct {
	DetailAnswer string `json:"detailAnswer"`
}

// 然后这样解析
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

	return client.Do(apiEndpoint, cycletls.Options{
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
		},
	}, "POST")
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

func makeDeleteRequest(client cycletls.CycleTLS, cookie, projectId string) (cycletls.Response, error) {

	// 不删除环境变量中的map中的对话

	for _, v := range config.ModelChatMap {
		if v == projectId {
			return cycletls.Response{}, nil
		}
	}
	for _, v := range config.GlobalSessionManager.GetChatIDsByCookie(cookie) {
		if v == projectId {
			return cycletls.Response{}, nil
		}
	}
	for _, v := range config.SessionImageChatMap {
		if v == projectId {
			return cycletls.Response{}, nil
		}
	}

	accept := "application/json"

	return client.Do(fmt.Sprintf(deleteEndpoint, projectId), cycletls.Options{
		Timeout: 10 * 60 * 60,
		Proxy:   config.ProxyUrl, // 在每个请求中设置代理
		Method:  "GET",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       accept,
			"Origin":       baseURL,
			"Referer":      baseURL + "/",
			"Cookie":       cookie,
		},
	}, "GET")
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
		SSELoop:
			for response := range sseChan {
				if response.Done {
					logger.Warnf(ctx, response.Data)
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
					break SSELoop // 使用 label 跳出 SSE 循环
				case common.IsFreeLimit(data):
					isRateLimit = true
					logger.Warnf(ctx, "Cookie free rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
					config.CookieLimit(cookie)
					break SSELoop // 使用 label 跳出 SSE 循环
				case common.IsNotLogin(data):
					isRateLimit = true
					logger.Warnf(ctx, "Cookie Not Login, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
					//err := cookieManager.RemoveCookie(cookie)
					//if err != nil {
					//	logger.Errorf(ctx, "Failed to remove cookie: %v", err)
					//}
					break SSELoop // 使用 label 跳出 SSE 循环
				}

				// 处理事件流数据
				if shouldContinue := processStreamData(c, data, &projectId, cookie, responseId, modelName, jsonData, searchModel); !shouldContinue {
					return false
				}
			}

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

// 处理流式数据的辅助函数，返回bool表示是否继续处理
func processStreamData(c *gin.Context, data string, projectId *string, cookie, responseId, model string, jsonData []byte, searchModel bool) bool {
	data = strings.TrimSpace(data)
	if !strings.HasPrefix(data, "data: ") {
		return true
	}
	data = strings.TrimPrefix(data, "data: ")

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
		if err := handleMessageFieldDelta(c, event, responseId, model, jsonData); err != nil {
			logger.Errorf(c.Request.Context(), "handleMessageFieldDelta err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return false
		}
	case "message_field_delta":
		if err := handleMessageFieldDelta(c, event, responseId, model, jsonData); err != nil {
			logger.Errorf(c.Request.Context(), "handleMessageFieldDelta err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return false
		}
	case "message_result":
		go func() {
			if config.AutoModelChatMapType == 1 {
				// 保存映射
				config.GlobalSessionManager.AddSession(cookie, model, *projectId)
			} else {
				if config.AutoDelChat == 1 {
					client := cycletls.Init()
					defer safeClose(client)
					makeDeleteRequest(client, cookie, *projectId)
				}
			}
		}()

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
			"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
		},
	}

	logger.Debug(c.Request.Context(), fmt.Sprintf("options: %v", options))

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
		errNoValidCookies         = "No valid cookies available"
		errCloudflareChallengeMsg = "Detected Cloudflare Challenge Page"
		errCloudflareBlock        = "CloudFlare: Sorry, you have been blocked"
		errServerErrMsg           = "An error occurred with the current request, please try again."
		errServiceUnavailable     = "Genspark Service Unavailable"
		errNoValidResponseContent = "No valid response content"
	)

	ctx := c.Request.Context()
	maxRetries := len(cookieManager.Cookies)

	for attempt := 0; attempt < maxRetries; attempt++ {
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
				break
			case common.IsFreeLimit(line):
				isRateLimit = true
				logger.Warnf(ctx, "Cookie free rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
				config.CookieLimit(cookie)
				break
			case common.IsNotLogin(line):
				isRateLimit = true
				logger.Warnf(ctx, "Cookie Not Login, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
				//err := cookieManager.RemoveCookie(cookie)
				//if err != nil {
				//	logger.Errorf(ctx, "Failed to remove cookie: %v", err)
				//}
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
					Type      string `json:"type"`
					FieldName string `json:"field_name"`
					Content   string `json:"content"`
					Id        string `json:"id"`
				}
				if err := json.Unmarshal([]byte(data), &parsedResponse); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				if parsedResponse.Type == "project_start" {
					projectId = parsedResponse.Id
				}
				if parsedResponse.Type == "message_result" {
					// 删除临时会话
					go func() {
						if config.AutoModelChatMapType == 1 {
							// 保存映射
							config.GlobalSessionManager.AddSession(cookie, modelName, projectId)
						} else {
							if config.AutoDelChat == 1 {
								client := cycletls.Init()
								defer safeClose(client)
								makeDeleteRequest(client, cookie, projectId)
							}
						}
					}()
					if modelName == "o1" && searchModel {
						// 解析内层的 JSON
						var content Content
						if err := json.Unmarshal([]byte(parsedResponse.Content), &content); err != nil {
							logger.Errorf(ctx, "Failed to unmarshal response content: %v err %s", parsedResponse.Content, err.Error())
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal response content"})
							return
						}
						parsedResponse.Content = content.DetailAnswer
					}
					content = parsedResponse.Content
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
						Message: model.OpenAIMessage{
							Role:    "assistant",
							Content: content,
						},
						FinishReason: &finishReason,
					}},
					Usage: model.OpenAIUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
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
		logger.Warnf(ctx, "未配置环境变量 SESSION_IMAGE_CHAT_MAP, 可能会生图失败!")
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
			if sessionImageChatManager != nil {
				cookie, chatId, err = sessionImageChatManager.GetNextKeyValue()
				if err != nil {
					logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
					return nil, fmt.Errorf(errNoValidCookies)
				}
			} else {
				//cookieManager := config.NewCookieManager()
				cookie, err = cookieManager.GetNextCookie()
				if err != nil {
					logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
					return nil, fmt.Errorf(errNoValidCookies)
				}
			}
			continue
		case common.IsFreeLimit(body):
			logger.Warnf(ctx, "Cookie free rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
			if sessionImageChatManager != nil {
				cookie, chatId, err = sessionImageChatManager.GetNextKeyValue()
				if err != nil {
					logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
					return nil, fmt.Errorf(errNoValidCookies)
				}
			} else {
				//cookieManager := config.NewCookieManager()
				cookie, err = cookieManager.GetNextCookie()
				if err != nil {
					logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
					return nil, fmt.Errorf(errNoValidCookies)
				}
			}
			continue
		case common.IsNotLogin(body):
			logger.Warnf(ctx, "Cookie free rate limited, switching to next cookie, attempt %d/%d, COOKIE:%s", attempt+1, maxRetries, cookie)
			if sessionImageChatManager != nil {
				//sessionImageChatManager.RemoveKey(cookie)
				cookie, chatId, err = sessionImageChatManager.GetNextKeyValue()
				if err != nil {
					logger.Errorf(ctx, "No more valid cookies available after attempt %d", attempt+1)
					c.JSON(http.StatusInternalServerError, gin.H{"error": errNoValidCookies})
					return nil, fmt.Errorf(errNoValidCookies)
				}
			} else {
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
				}

			}
			continue
		case common.IsServerError(body):
			logger.Errorf(ctx, errServerErrMsg)
			return nil, fmt.Errorf(errServerErrMsg)
		case common.IsServerOverloaded(body):
			logger.Errorf(ctx, fmt.Sprintf("Server overloaded, please try again later.%s", "官方服务超载或环境变量 SESSION_IMAGE_CHAT_MAP 未配置"))
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
				go func() {
					client := cycletls.Init()
					defer safeClose(client)
					makeDeleteRequest(client, cookie, projectId)
				}()
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

	for _, taskID := range taskIDs {
		for {
			// 构建请求URL
			url := fmt.Sprintf("https://www.genspark.ai/api/spark/image_generation_task_status?task_id=%s", taskID)

			// 发送请求
			response, err := client.Do(url, cycletls.Options{
				Timeout: 10 * 60 * 60,
				Proxy:   config.ProxyUrl, // 在每个请求中设置代理
				Method:  "GET",
				Headers: map[string]string{
					"Cookie": cookie,
				},
			}, "GET")

			if err != nil {
				continue
			}

			var result struct {
				Data struct {
					ImageURLsNowatermark []string `json:"image_urls_nowatermark"`
					Status               string   `json:"status"`
				}
			}

			if err := json.Unmarshal([]byte(response.Body), &result); err != nil {
				continue
			}

			// 如果状态成功且有图片URL
			if result.Data.Status == "SUCCESS" && len(result.Data.ImageURLsNowatermark) > 0 {
				imageURLs = append(imageURLs, result.Data.ImageURLsNowatermark...)
				break
			}

			// 等待1秒后重试
			time.Sleep(500 * time.Millisecond)
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

func safeClose(client cycletls.CycleTLS) {
	if client.ReqChan != nil {
		close(client.ReqChan)
	}
	if client.RespChan != nil {
		close(client.RespChan)
	}
}
