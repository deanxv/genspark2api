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

//var GSCookies = strings.Split(os.Getenv("GS_COOKIE"), ",")

// var IpBlackList = os.Getenv("IP_BLACK_LIST")
var IpBlackList = strings.Split(os.Getenv("IP_BLACK_LIST"), ",")

var AutoDelChat = env.Int("AUTO_DEL_CHAT", 0)
var ProxyUrl = env.String("PROXY_URL", "")
var AutoModelChatMapType = env.Int("AUTO_MODEL_CHAT_MAP_TYPE", 1)
var YesCaptchaClientKey = env.String("YES_CAPTCHA_CLIENT_KEY", "")

// var CheatUrl = env.String("CHEAT_URL", "https://gs-cheat.aytsao.cn/genspark/create/req/body")
var RecaptchaProxyUrl = env.String("RECAPTCHA_PROXY_URL", "")

// 隐藏思考过程
var ReasoningHide = env.Int("REASONING_HIDE", 0)

// 前置message
var PRE_MESSAGES_JSON = env.String("PRE_MESSAGES_JSON", "")

var RateLimitCookieLockDuration = env.Int("RATE_LIMIT_COOKIE_LOCK_DURATION", 10*60)

// 路由前缀
var RoutePrefix = env.String("ROUTE_PREFIX", "")
var ModelChatMapStr = env.String("MODEL_CHAT_MAP", "")
var ModelChatMap = make(map[string]string)
var SessionImageChatMap = make(map[string]string)
var GlobalSessionManager *SessionManager

var SessionImageChatMapStr = env.String("SESSION_IMAGE_CHAT_MAP", "")
var YescaptchaClient *yescaptcha.Client

var AllDialogRecordEnable = os.Getenv("ALL_DIALOG_RECORD_ENABLE")
var RequestOutTime = os.Getenv("REQUEST_OUT_TIME")
var StreamRequestOutTime = os.Getenv("STREAM_REQUEST_OUT_TIME")
var SwaggerEnable = os.Getenv("SWAGGER_ENABLE")
var OnlyOpenaiApi = os.Getenv("ONLY_OPENAI_API")

var DebugEnabled = os.Getenv("DEBUG") == "true"

// Debug logging levels
// DEBUG_LOG_BODY=true - логировать тело запроса/ответа
// DEBUG_LOG_HEADERS=true - логировать заголовки
// DEBUG_SAVE_PAYLOADS=true - сохранять детали в файлы logs/debug/
// DEBUG_LOG_LEVEL=verbose|normal|minimal
var DebugLogBody = os.Getenv("DEBUG_LOG_BODY") == "true"
var DebugLogHeaders = os.Getenv("DEBUG_LOG_HEADERS") == "true"
var DebugLogNetwork = os.Getenv("DEBUG_LOG_NETWORK") == "true"
var DebugSavePayloads = os.Getenv("DEBUG_SAVE_PAYLOADS") == "true"
var DebugLogLevel = env.String("DEBUG_LOG_LEVEL", "normal") // verbose, normal, minimal

var RateLimitKeyExpirationDuration = 20 * time.Minute

var RequestOutTimeDuration = 5 * time.Minute

var (
	RequestRateLimitNum            = env.Int("REQUEST_RATE_LIMIT", 60)
	RequestRateLimitDuration int64 = 1 * 60
)

type RateLimitCookie struct {
	ExpirationTime time.Time // 过期时间
}

var (
	rateLimitCookies sync.Map // 使用 sync.Map 管理限速 Cookie
)

func AddRateLimitCookie(cookie string, expirationTime time.Time) {
	rateLimitCookies.Store(cookie, RateLimitCookie{
		ExpirationTime: expirationTime,
	})
	//fmt.Printf("Storing cookie: %s with value: %+v\n", cookie, RateLimitCookie{ExpirationTime: expirationTime})
}

type CookieManager struct {
	Cookies      []string
	currentIndex int
	mu           sync.Mutex
}

var (
	GSCookies    []string   // 存储所有的 cookies
	cookiesMutex sync.Mutex // 保护 GSCookies 的互斥锁
)

// InitGSCookies 初始化 GSCookies
func InitGSCookies() {
	cookiesMutex.Lock()
	defer cookiesMutex.Unlock()

	GSCookies = []string{}

	// 从环境变量中读取 GS_COOKIE 并拆分为切片
	cookieStr := os.Getenv("GS_COOKIE")
	if cookieStr != "" {

		for _, cookie := range strings.Split(cookieStr, ",") {
			// 如果 cookie 不包含 "session_id="，则添加前缀
			if !strings.Contains(cookie, "session_id=") {
				cookie = "session_id=" + cookie
			}
			GSCookies = append(GSCookies, cookie)
		}
	}
}

// RemoveCookie 删除指定的 cookie（支持并发）
func RemoveCookie(cookieToRemove string) {
	cookiesMutex.Lock()
	defer cookiesMutex.Unlock()

	// 创建一个新的切片，过滤掉需要删除的 cookie
	var newCookies []string
	for _, cookie := range GetGSCookies() {
		if cookie != cookieToRemove {
			newCookies = append(newCookies, cookie)
		}
	}

	// 更新 GSCookies
	GSCookies = newCookies
}

// GetGSCookies 获取 GSCookies 的副本
func GetGSCookies() []string {
	//cookiesMutex.Lock()
	//defer cookiesMutex.Unlock()

	// 返回 GSCookies 的副本，避免外部直接修改
	cookiesCopy := make([]string, len(GSCookies))
	copy(cookiesCopy, GSCookies)
	return cookiesCopy
}

// NewCookieManager 创建 CookieManager
func NewCookieManager() *CookieManager {
	var validCookies []string
	// 遍历 GSCookies
	for _, cookie := range GetGSCookies() {
		cookie = strings.TrimSpace(cookie)
		if cookie == "" {
			continue // 忽略空字符串
		}

		// 检查是否在 RateLimitCookies 中
		if value, ok := rateLimitCookies.Load(cookie); ok {
			rateLimitCookie, ok := value.(RateLimitCookie) // 正确转换为 RateLimitCookie
			if !ok {
				continue
			}
			if rateLimitCookie.ExpirationTime.After(time.Now()) {
				// 如果未过期，忽略该 cookie
				continue
			} else {
				// 如果已过期，从 RateLimitCookies 中删除
				rateLimitCookies.Delete(cookie)
			}
		}

		// 添加到有效 cookie 列表
		validCookies = append(validCookies, cookie)
	}

	return &CookieManager{
		Cookies:      validCookies,
		currentIndex: 0,
	}
}

func IsRateLimited(cookie string) bool {
	if value, ok := rateLimitCookies.Load(cookie); ok {
		rateLimitCookie := value.(RateLimitCookie)
		return rateLimitCookie.ExpirationTime.After(time.Now())
	}
	return false
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

// GetConfigSummary возвращает строку с конфигурацией для логирования (без секретов)
func GetConfigSummary() []string {
	var lines []string
	lines = append(lines, "========== Configuration Summary ==========")

	// Основные настройки
	if AutoDelChat == 1 {
		lines = append(lines, "AUTO_DEL_CHAT: enabled (1)")
	} else {
		lines = append(lines, "AUTO_DEL_CHAT: disabled (0)")
	}

	if AutoModelChatMapType == 1 {
		lines = append(lines, "AUTO_MODEL_CHAT_MAP_TYPE: save sessions (1)")
	} else {
		lines = append(lines, "AUTO_MODEL_CHAT_MAP_TYPE: delete sessions (0)")
	}

	if ReasoningHide == 1 {
		lines = append(lines, "REASONING_HIDE: hide reasoning (1)")
	} else {
		lines = append(lines, "REASONING_HIDE: show reasoning (0)")
	}

	if DebugEnabled {
		lines = append(lines, "DEBUG: enabled")
		lines = append(lines, "  DEBUG_LOG_LEVEL: "+DebugLogLevel)
		if DebugLogBody {
			lines = append(lines, "  DEBUG_LOG_BODY: enabled")
		}
		if DebugLogHeaders {
			lines = append(lines, "  DEBUG_LOG_HEADERS: enabled")
		}
		if DebugLogNetwork {
			lines = append(lines, "  DEBUG_LOG_NETWORK: enabled")
		}
		if DebugSavePayloads {
			lines = append(lines, "  DEBUG_SAVE_PAYLOADS: enabled")
		}
	} else {
		lines = append(lines, "DEBUG: disabled")
	}

	// Proxy
	if ProxyUrl != "" {
		lines = append(lines, "PROXY_URL: configured")
	} else {
		lines = append(lines, "PROXY_URL: not set")
	}

	// Recaptcha
	if RecaptchaProxyUrl != "" {
		lines = append(lines, "RECAPTCHA_PROXY_URL: configured")
	} else {
		lines = append(lines, "RECAPTCHA_PROXY_URL: not set")
	}

	// Route prefix
	if RoutePrefix != "" {
		lines = append(lines, "ROUTE_PREFIX: "+RoutePrefix)
	} else {
		lines = append(lines, "ROUTE_PREFIX: not set")
	}

	// Cookies count
	cookiesCount := len(GSCookies)
	if cookiesCount == 1 {
		lines = append(lines, "GS_COOKIES_COUNT: 1 cookie")
	} else {
		lines = append(lines, "GS_COOKIES_COUNT: multiple cookies configured")
	}

	// Model chat map
	if len(ModelChatMap) > 0 {
		lines = append(lines, "MODEL_CHAT_MAP: configured")
	} else {
		lines = append(lines, "MODEL_CHAT_MAP: not set")
	}

	// Session image chat map
	if len(SessionImageChatMap) > 0 {
		lines = append(lines, "SESSION_IMAGE_CHAT_MAP: configured")
	} else {
		lines = append(lines, "SESSION_IMAGE_CHAT_MAP: not set")
	}

	// Pre messages
	if PRE_MESSAGES_JSON != "" {
		lines = append(lines, "PRE_MESSAGES_JSON: configured")
	} else {
		lines = append(lines, "PRE_MESSAGES_JSON: not set")
	}

	// YesCaptcha
	if YesCaptchaClientKey != "" {
		lines = append(lines, "YES_CAPTCHA_CLIENT_KEY: configured")
	} else {
		lines = append(lines, "YES_CAPTCHA_CLIENT_KEY: not set")
	}

	// Timezone
	tz, _ := time.Now().Zone()
	lines = append(lines, "TIMEZONE: "+tz)

	lines = append(lines, "=============================================")
	return lines
}
