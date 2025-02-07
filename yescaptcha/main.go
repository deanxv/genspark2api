package yescaptcha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	defaultAPIEndpoint = "https://api.yescaptcha.com"
	createTaskPath     = "/createTask"
	getResultPath      = "/getTaskResult"
	maxRetries         = 20
	pollingInterval    = 3 * time.Second
)

// Client represents a YesCaptcha API client
type Client struct {
	clientKey   string
	apiEndpoint string
	httpClient  *http.Client
}

// Options contains configuration options for the YesCaptcha client
type Options struct {
	APIEndpoint string
	HTTPClient  *http.Client
}

// NewClient creates a new YesCaptcha client with the given client key and options
func NewClient(clientKey string, opts *Options) *Client {
	client := &Client{
		clientKey:   clientKey,
		apiEndpoint: defaultAPIEndpoint,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}

	if opts != nil {
		if opts.APIEndpoint != "" {
			client.apiEndpoint = opts.APIEndpoint
		}
		if opts.HTTPClient != nil {
			client.httpClient = opts.HTTPClient
		}
	}

	return client
}

// RecaptchaV3Request contains the parameters for solving a ReCaptcha V3 challenge
type RecaptchaV3Request struct {
	WebsiteURL string
	WebsiteKey string
	PageAction string
	MinScore   float64
	SoftID     string
	Callback   string
}

type createTaskRequest struct {
	ClientKey string `json:"clientKey"`
	Task      task   `json:"task"`
}

type task struct {
	Type       string  `json:"type"`
	WebsiteURL string  `json:"websiteURL"`
	WebsiteKey string  `json:"websiteKey"`
	PageAction string  `json:"pageAction"`
	MinScore   float64 `json:"minScore,omitempty"`
	SoftID     string  `json:"softId,omitempty"`
	Callback   string  `json:"callback,omitempty"`
}

type createTaskResponse struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
	TaskID           string `json:"taskId"`
}

type getResultRequest struct {
	ClientKey string `json:"clientKey"`
	TaskID    string `json:"taskId"`
}

type getResultResponse struct {
	ErrorID          int      `json:"errorId"`
	ErrorCode        string   `json:"errorCode"`
	ErrorDescription string   `json:"errorDescription"`
	Status           string   `json:"status"`
	Solution         solution `json:"solution"`
}

type solution struct {
	GRecaptchaResponse string `json:"gRecaptchaResponse"`
}

// SolveRecaptchaV3 solves a ReCaptcha V3 challenge with context
func (c *Client) SolveRecaptchaV3(ctx context.Context, req RecaptchaV3Request) (string, error) {
	taskID, err := c.createTask(ctx, req)
	if err != nil {
		return "", err
	}

	return c.waitForResult(ctx, taskID)
}

func (c *Client) createTask(ctx context.Context, req RecaptchaV3Request) (string, error) {
	request := createTaskRequest{
		ClientKey: c.clientKey,
		Task: task{
			Type:       "RecaptchaV3TaskProxyless",
			WebsiteURL: req.WebsiteURL,
			WebsiteKey: req.WebsiteKey,
			PageAction: req.PageAction,
			MinScore:   req.MinScore,
			SoftID:     req.SoftID,
			Callback:   req.Callback,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint+createTaskPath, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response createTaskResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if response.ErrorID != 0 {
		return "", errors.New(response.ErrorDescription)
	}

	return response.TaskID, nil
}

func (c *Client) waitForResult(ctx context.Context, taskID string) (string, error) {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			result, err := c.getTaskResult(ctx, taskID)
			if err != nil {
				return "", err
			}

			if result.Status == "ready" {
				return result.Solution.GRecaptchaResponse, nil
			}

			if result.ErrorID != 0 {
				return "", errors.New(result.ErrorDescription)
			}
		}
	}

	return "", errors.New("timeout waiting for captcha solution")
}

func (c *Client) getTaskResult(ctx context.Context, taskID string) (*getResultResponse, error) {
	request := getResultRequest{
		ClientKey: c.clientKey,
		TaskID:    taskID,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint+getResultPath, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response getResultResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
