package token

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func getRecaptchaToken(siteKey string, action string) (string, error) {
	if action == "" {
		action = "copilot"
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 获取anchor token
	anchorURL := fmt.Sprintf("https://www.google.com/recaptcha/api2/anchor?ar=1&k=%s&co=aHR0cHM6Ly93d3cuZ29vZ2xlLmNvbTo0NDM.&hl=en&v=khH7Ei3klcvfRI74FvDcfuOo&size=invisible&cb=123456", siteKey)

	resp, err := client.Get(anchorURL)
	if err != nil {
		return "", fmt.Errorf("anchor request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading anchor response failed: %v", err)
	}

	// 使用正则表达式提取anchor token
	re := regexp.MustCompile(`recaptcha-token.+?value="(.+?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", fmt.Errorf("anchor token not found")
	}
	anchorToken := matches[1]

	// 构建reload请求
	reloadURL := "https://www.google.com/recaptcha/api2/reload?k=" + siteKey
	payload := fmt.Sprintf("v=khH7Ei3klcvfRI74FvDcfuOo&reason=q&c=%s&k=%s&co=aHR0cHM6Ly93d3cuZ29vZ2xlLmNvbTo0NDM.&sa=%s&chr=%%5B89%%2C64%%2C27%%5D",
		anchorToken, siteKey, action)

	// 创建POST请求
	req, err := http.NewRequest("POST", reloadURL, strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("creating reload request failed: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送请求
	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("reload request failed: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading reload response failed: %v", err)
	}

	// 提取最终token
	re = regexp.MustCompile(`rresp","(.+?)"`)
	matches = re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", fmt.Errorf("final token not found")
	}

	return matches[1], nil
}

func GetCopilotRecaptchaToken() string {
	siteKey := "6Leq7KYqAAAAAGdd1NaUBJF9dHTPAKP7DcnaRc66"
	token, err := getRecaptchaToken(siteKey, "copilot")
	if err != nil {
		fmt.Printf("Error getting reCAPTCHA token: %v\n", err)
		return ""
	}
	return token
}
