package config

import (
	"errors"
	"genspark2api/common/env"
	"genspark2api/yescaptcha"
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

// var IpBlackList = os.Getenv("IP_BLACK_LIST")
var IpBlackList = strings.Split(os.Getenv("IP_BLACK_LIST"), ",")

var AutoDelChat = env.Int("AUTO_DEL_CHAT", 0)
var ProxyUrl = env.String("PROXY_URL", "")
var AutoModelChatMapType = env.Int("AUTO_MODEL_CHAT_MAP_TYPE", 1)
var YesCaptchaClientKey = env.String("YES_CAPTCHA_CLIENT_KEY", "")

// 路由前缀
var RoutePrefix = env.String("ROUTE_PREFIX", "")
var ModelChatMapStr = env.String("MODEL_CHAT_MAP", "")
var ModelChatMap = make(map[string]string)
var SessionImageChatMapStr = env.String("SESSION_IMAGE_CHAT_MAP", "")
var SessionImageChatMap = make(map[string]string)
var GlobalSessionManager *SessionManager
var YescaptchaClient *yescaptcha.Client

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
var (
	FreeLimitDisableCookieNum      = env.Int("FREE_LIMIT_DISABLE_COOKIE_NUM", 0)
	FreeLimitDisableCookieDuration = time.Duration(FreeLimitDisableCookieNum) * time.Second
)

type CookieManager struct {
	Cookies      []string
	currentIndex int
	mu           sync.Mutex
}

func NewCookieManager() (cm *CookieManager) {
	cookies := strings.Split(os.Getenv("GS_COOKIE"), ",")
	// 过滤空字符串
	var validCookies []string
	for _, cookie := range cookies {
		if strings.TrimSpace(cookie) != "" {

			if !strings.Contains(cookie, "session_id=") {
				cookie = "session_id=" + cookie

			}

			validCookies = append(validCookies, cookie)
		}
	}
	cm = &CookieManager{
		Cookies:      validCookies,
		currentIndex: 0,
	}
	cm.GetNoLimitCookie()
	return cm
}

func (cm *CookieManager) RemoveCookie(cookieToRemove string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.Cookies) == 0 {
		return errors.New("no cookies available")
	}

	// 查找要删除的cookie的索引
	index := -1
	for i, cookie := range cm.Cookies {
		if cookie == cookieToRemove {
			index = i
			break
		}
	}

	// 如果没找到要删除的cookie
	if index == -1 {
		return errors.New("RemoveCookie -> cookie not found")
	}

	// 从切片中删除cookie
	cm.Cookies = append(cm.Cookies[:index], cm.Cookies[index+1:]...)

	// 如果当前索引大于或等于删除后的切片长度，重置为0
	if cm.currentIndex >= len(cm.Cookies) {
		cm.currentIndex = 0
	}

	return nil
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

// GetChatIDsByCookie 获取指定cookie关联的所有chatID列表(读操作,使用读锁)
func (sm *SessionManager) GetChatIDsByCookie(cookie string) []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var chatIDs []string
	for key, chatID := range sm.sessions {
		if key.Cookie == cookie {
			chatIDs = append(chatIDs, chatID)
		}
	}
	return chatIDs
}

type SessionMapManager struct {
	sessionMap   map[string]string
	keys         []string
	currentIndex int
	mu           sync.Mutex
}

func NewSessionMapManager() *SessionMapManager {
	// 从初始map中提取所有的key
	keys := make([]string, 0, len(SessionImageChatMap))
	for k := range SessionImageChatMap {
		keys = append(keys, k)
	}

	return &SessionMapManager{
		sessionMap:   SessionImageChatMap,
		keys:         keys,
		currentIndex: 0,
	}
}

// GetCurrentKeyValue 获取当前索引对应的键值对
func (sm *SessionMapManager) GetCurrentKeyValue() (string, string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.keys) == 0 {
		return "", "", errors.New("no sessions available")
	}

	currentKey := sm.keys[sm.currentIndex]
	return currentKey, sm.sessionMap[currentKey], nil
}

// GetNextKeyValue 获取下一个键值对
func (sm *SessionMapManager) GetNextKeyValue() (string, string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.keys) == 0 {
		return "", "", errors.New("no sessions available")
	}

	sm.currentIndex = (sm.currentIndex + 1) % len(sm.keys)
	currentKey := sm.keys[sm.currentIndex]
	return currentKey, sm.sessionMap[currentKey], nil
}

// GetRandomKeyValue 随机获取一个键值对
func (sm *SessionMapManager) GetRandomKeyValue() (string, string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.keys) == 0 {
		return "", "", errors.New("no sessions available")
	}

	randomIndex := rand.Intn(len(sm.keys))
	sm.currentIndex = randomIndex
	currentKey := sm.keys[randomIndex]
	return currentKey, sm.sessionMap[currentKey], nil
}

// AddKeyValue 添加新的键值对
func (sm *SessionMapManager) AddKeyValue(key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 如果key不存在，则添加到keys切片中
	if _, exists := sm.sessionMap[key]; !exists {
		sm.keys = append(sm.keys, key)
	}
	sm.sessionMap[key] = value
}

// RemoveKey 删除指定的键值对
func (sm *SessionMapManager) RemoveKey(key string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessionMap[key]; !exists {
		return
	}

	// 从map中删除
	delete(sm.sessionMap, key)

	// 从keys切片中删除
	for i, k := range sm.keys {
		if k == key {
			sm.keys = append(sm.keys[:i], sm.keys[i+1:]...)
			break
		}
	}

	// 调整currentIndex如果需要
	if sm.currentIndex >= len(sm.keys) && len(sm.keys) > 0 {
		sm.currentIndex = len(sm.keys) - 1
	}
}

// GetSize 获取当前map的大小
func (sm *SessionMapManager) GetSize() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.keys)
}
