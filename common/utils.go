package common

import (
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	_ "github.com/pkoukk/tiktoken-go"
	"math/rand"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// splitStringByBytes 将字符串按照指定的字节数进行切割
func SplitStringByBytes(s string, size int) []string {
	var result []string

	for len(s) > 0 {
		// 初始切割点
		l := size
		if l > len(s) {
			l = len(s)
		}

		// 确保不在字符中间切割
		for l > 0 && !utf8.ValidString(s[:l]) {
			l--
		}

		// 如果 l 减到 0，说明 size 太小，无法容纳一个完整的字符
		if l == 0 {
			l = len(s)
			for l > 0 && !utf8.ValidString(s[:l]) {
				l--
			}
		}

		result = append(result, s[:l])
		s = s[l:]
	}

	return result
}

func Obj2Bytes(obj interface{}) ([]byte, error) {
	// 创建一个jsonIter的Encoder
	configCompatibleWithStandardLibrary := jsoniter.ConfigCompatibleWithStandardLibrary
	// 将结构体转换为JSON文本并保持顺序
	bytes, err := configCompatibleWithStandardLibrary.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func GetUUID() string {
	code := uuid.New().String()
	code = strings.Replace(code, "-", "", -1)
	return code
}

// RandomElement 返回给定切片中的随机元素
func RandomElement[T any](slice []T) (T, error) {
	if len(slice) == 0 {
		var zero T
		return zero, fmt.Errorf("empty slice")
	}

	// 确保每次随机都不一样
	rand.Seed(time.Now().UnixNano())

	// 随机选择一个索引
	index := rand.Intn(len(slice))
	return slice[index], nil
}

func SliceContains(slice []string, str string) bool {
	for _, item := range slice {
		if strings.Contains(str, item) {
			return true
		}
	}
	return false
}

func IsImageBase64(s string) bool {
	// 检查字符串是否符合数据URL的格式
	if !strings.HasPrefix(s, "data:image/") || !strings.Contains(s, ";base64,") {
		return false
	}

	if !strings.Contains(s, ";base64,") {
		return false
	}

	// 获取";base64,"后的Base64编码部分
	dataParts := strings.Split(s, ";base64,")
	if len(dataParts) != 2 {
		return false
	}
	base64Data := dataParts[1]

	// 尝试Base64解码
	_, err := base64.StdEncoding.DecodeString(base64Data)
	return err == nil
}

func IsBase64(s string) bool {
	// 检查字符串是否符合数据URL的格式
	//if !strings.HasPrefix(s, "data:image/") || !strings.Contains(s, ";base64,") {
	//	return false
	//}

	if !strings.Contains(s, ";base64,") {
		return false
	}

	// 获取";base64,"后的Base64编码部分
	dataParts := strings.Split(s, ";base64,")
	if len(dataParts) != 2 {
		return false
	}
	base64Data := dataParts[1]

	// 尝试Base64解码
	_, err := base64.StdEncoding.DecodeString(base64Data)
	return err == nil
}

func IsCloudflareChallenge(data string) bool {
	// 检查基本的 HTML 结构
	htmlPattern := `^<!DOCTYPE html><html.*?><head>.*?</head><body.*?>.*?</body></html>$`

	// 检查 Cloudflare 特征
	cfPatterns := []string{
		`<title>Just a moment\.\.\.</title>`,          // 标题特征
		`window\._cf_chl_opt`,                         // CF 配置对象
		`challenge-platform/h/b/orchestrate/chl_page`, // CF challenge 路径
		`cdn-cgi/challenge-platform`,                  // CDN 路径特征
		`<meta http-equiv="refresh" content="\d+">`,   // 刷新 meta 标签
	}

	// 首先检查整体 HTML 结构
	matched, _ := regexp.MatchString(htmlPattern, strings.TrimSpace(data))
	if !matched {
		return false
	}

	// 检查是否包含 Cloudflare 特征
	for _, pattern := range cfPatterns {
		if matched, _ := regexp.MatchString(pattern, data); matched {
			return true
		}
	}

	return false
}

func IsRateLimit(data string) bool {
	if data == "Rate limit exceeded cf1" || data == "Rate limit exceeded cf2" {
		return true
	}

	return false
}

func IsServerError(data string) bool {
	if data == "Internal Server Error" {
		return true
	}

	return false
}

func IsServerOverloaded(data string) bool {
	if strings.Contains(data, `data: {"id": "", "role": "assistant", "content": "Server overloaded, please try again later.", "action": null, "recommend_actions": null, "is_prompt": false, "render_template": null, "session_state": null, "message_type": null, "type": "message_result"}`) {
		return true
	}

	return false
}

func IsFreeLimit(data string) bool {
	if strings.Contains(data, `data: {"id": "", "role": "assistant", "content": "You've reached your free usage limit today", "action": {"type": "ACTION_QUOTA_EXCEEDED", "query_string": null, "update_flow_data": null, "label": null, "user_s_input": null, "action_params": null}, "recommend_actions": null, "is_prompt": true, "render_template": null, "session_state": {"consume_usage_quota_exceeded": true}, "message_type": null, "type": "message_result"}`) {
		return true
	}

	return false
}

func IsServiceUnavailablePage(data string) bool {
	// 检查基本的 HTML 结构
	htmlPattern := `^<!doctype html><html.*?><head>.*?</head><body.*?>.*?</body></html>`

	// 检查 Service Unavailable 页面特征
	suPatterns := []string{
		`<title>Genspark</title>`,                           // 标题特征
		`Service\s+Unavailable`,                             // 错误信息
		`class="bb".*?class="s1".*?class="s2".*?class="s3"`, // 特征性类名结构
		`genspark_logo\.png`,                                // Logo 图片
		`gensparkpublicblob-cdn.*?\.azurefd\.net`,           // CDN 域名
		`<div class="tt">Service Unavailable</div>`,         // 错误信息容器
	}

	// 首先检查整体 HTML 结构
	matched, _ := regexp.MatchString(htmlPattern, strings.TrimSpace(data))
	if !matched {
		return false
	}

	// 检查特征模式，至少匹配其中的 3 个才认为是目标页面
	matchCount := 0
	for _, pattern := range suPatterns {
		if matched, _ := regexp.MatchString(pattern, data); matched {
			matchCount++
		}
	}

	return matchCount >= 3
}

//<!doctype html><html><head><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no"><title>Genspark</title><link rel="icon" href="https://gensparkpublicblob-cdn-e6g4btgjavb5a7gh.z03.azurefd.net/user-upload-image/manual/favicon.ico"><style>body,html{margin:0;padding:0;font-family:Arial}.bb{width:100vw;height:100vh;position:absolute;overflow:hidden}.logo img{margin:20px 0 0 24px;height:24px}.iw{display:flex;flex-direction:column;height:100vh;width:100%}.s1{position:absolute;top:0;left:0;margin-top:-5%;margin-left:15%;width:289px;height:289px;border-radius:289px;opacity:.6;background:radial-gradient(55.64% 49.84%,#2c10d6 0,rgba(44,16,214,.36) 100%);filter:blur(120px)}.s2{position:absolute;top:0;left:0;margin-top:10%;margin-left:50%;width:204.845px;height:204.845px;transform:rotate(-131.346deg);flex-shrink:0;background:radial-gradient(55.64% 49.84%,#7fd1ff 0,rgba(44,16,214,.36) 100%);filter:blur(120px)}.s3{position:absolute;bottom:0;right:0;margin-bottom:10%;margin-right:10%;width:251px;height:251px;border-radius:289.093px;background:radial-gradient(88.27% 88.27% at 90.98% 61.04%,#ce7fff 0,#ffe4af 100%);filter:blur(120px)}.cc{display:flex;justify-content:center;align-items:center;height:100%;width:100%}.hh{align-items:center;display:flex;width:100vw}.dd{margin-top:-200px}.tt{color:#000;text-align:center;font-size:40px;font-style:normal;font-weight:700}@media (max-width:800px){.tt{font-size:30px}}</style></head><body><div class="bb"><div class="s1"></div><div class="s2"></div><div class="s3"></div></div><div class="iw"><div class="hh"><div class="logo"><img src="https://gensparkpublicblob-cdn-e6g4btgjavb5a7gh.z03.azurefd.net/user-upload-image/manual/genspark_logo.png" alt="logo"></div></div><div class="cc"><div class="dd"><div class="tt">Service Unavailable</div></div></div></div></body></html>
//<!doctype html><html><head><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no"><title>Genspark</title><link rel="icon" href="https://gensparkpublicblob-cdn-e6g4btgjavb5a7gh.z03.azurefd.net/user-upload-image/manual/favicon.ico"><style>body,html{margin:0;padding:0;font-family:Arial}.bb{width:100vw;height:100vh;position:absolute;overflow:hidden}.logo img{margin:20px 0 0 24px;height:24px}.iw{display:flex;flex-direction:column;height:100vh;width:100%!}(MISSING).s1{position:absolute;top:0;left:0;margin-top:-5%!;(MISSING)margin-left:15%!;(MISSING)width:289px;height:289px;border-radius:289px;opacity:.6;background:radial-gradient(55.64%,#2c10d6 0,rgba(44,16,214,.36) 100%!)(MISSING);filter:blur(120px)}.s2{position:absolute;top:0;left:0;margin-top:10%!;(MISSING)margin-left:50%!;(MISSING)width:204.845px;height:204.845px;transform:rotate(-131.346deg);flex-shrink:0;background:radial-gradient(55.64%,#7fd1ff 0,rgba(44,16,214,.36) 100%!)(MISSING);filter:blur(120px)}.s3{position:absolute;bottom:0;right:0;margin-bottom:10%!;(MISSING)margin-right:10%!;(MISSING)width:251px;height:251px;border-radius:289.093px;background:radial-gradient(88.27% at 90.98%,#ce7fff 0,#ffe4af 100%!)(MISSING);filter:blur(120px)}.cc{display:flex;justify-content:center;align-items:center;height:100%!;(MISSING)width:100%!}(MISSING).hh{align-items:center;display:flex;width:100vw}.dd{margin-top:-200px}.tt{color:#000;text-align:center;font-size:40px;font-style:normal;font-weight:700}@media (max-width:800px){.tt{font-size:30px}}</style></head><body><div class="bb"><div class="s1"></div><div class="s2"></div><div class="s3"></div></div><div class="iw"><div class="hh"><div class="logo"><img src="https://gensparkpublicblob-cdn-e6g4btgjavb5a7gh.z03.azurefd.net/user-upload-image/manual/genspark_logo.png" alt="logo"></div></div><div class="cc"><div class="dd"><div class="tt">Service Unavailable</div></div></div></div></body></html>
