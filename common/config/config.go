package config

import (
	"errors"
	"genspark2api/common/env"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

var ApiSecret = os.Getenv("API_SECRET")
var ApiSecrets = strings.Split(os.Getenv("API_SECRET"), ",")
var GSCookie = os.Getenv("GS_COOKIE")
var GSCookies = strings.Split(os.Getenv("GS_COOKIE"), ",")
var AutoDelChat = env.Int("AUTO_DEL_CHAT", 0)
var ProxyUrl = env.String("PROXY_URL", "")
var ModelChatMapStr = env.String("MODEL_CHAT_MAP", "")
var AutoModelChatMapType = env.Int("AUTO_MODEL_CHAT_MAP_TYPE", 1)
var ModelChatMap = make(map[string]string)
var GlobalSessionManager *SessionManager

var AllDialogRecordEnable = os.Getenv("ALL_DIALOG_RECORD_ENABLE")
var RequestOutTime = os.Getenv("REQUEST_OUT_TIME")
var StreamRequestOutTime = os.Getenv("STREAM_REQUEST_OUT_TIME")
var SwaggerEnable = os.Getenv("SWAGGER_ENABLE")
var OnlyOpenaiApi = os.Getenv("ONLY_OPENAI_API")

var DebugEnabled = os.Getenv("DEBUG") == "true"

var RateLimitKeyExpirationDuration = 20 * time.Minute

var RequestOutTimeDuration = 5 * time.Minute

var (
	RequestRateLimitNum            = env.Int("REQUEST_RATE_LIMIT", 60)
	RequestRateLimitDuration int64 = 1 * 60
)

type CookieManager struct {
	Cookies      []string
	currentIndex int
	mu           sync.Mutex
}

func NewCookieManager() *CookieManager {
	cookies := strings.Split(os.Getenv("GS_COOKIE"), ",")
	// 过滤空字符串
	var validCookies []string
	for _, cookie := range cookies {
		if strings.TrimSpace(cookie) != "" {
			validCookies = append(validCookies, cookie)
		}
	}

	return &CookieManager{
		Cookies:      validCookies,
		currentIndex: 0,
	}
}

func (cm *CookieManager) GetNextCookie() (string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.Cookies) == 0 {
		return "", errors.New("no cookies available")
	}

	cm.currentIndex = (cm.currentIndex + 1) % len(cm.Cookies)
	return cm.Cookies[cm.currentIndex], nil
}

func (cm *CookieManager) GetRandomCookie() (string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.Cookies) == 0 {
		return "", errors.New("no cookies available")
	}

	// 生成随机索引
	randomIndex := rand.Intn(len(cm.Cookies))
	// 更新当前索引
	cm.currentIndex = randomIndex

	return cm.Cookies[randomIndex], nil
}

// SessionKey 定义复合键结构
type SessionKey struct {
	Cookie string
	Model  string
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions map[SessionKey]string
	mutex    sync.RWMutex
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[SessionKey]string),
	}
}

// AddSession 添加会话记录（写操作，需要写锁）
func (sm *SessionManager) AddSession(cookie string, model string, chatID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	key := SessionKey{
		Cookie: cookie,
		Model:  model,
	}
	sm.sessions[key] = chatID
}

// GetChatID 获取会话ID（读操作，使用读锁）
func (sm *SessionManager) GetChatID(cookie string, model string) (string, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	key := SessionKey{
		Cookie: cookie,
		Model:  model,
	}
	chatID, exists := sm.sessions[key]
	return chatID, exists
}

// DeleteSession 删除会话记录（写操作，需要写锁）
func (sm *SessionManager) DeleteSession(cookie string, model string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	key := SessionKey{
		Cookie: cookie,
		Model:  model,
	}
	delete(sm.sessions, key)
}
