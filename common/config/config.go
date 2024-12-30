package config

import (
	"genspark2api/common/env"
	"os"
	"strings"
	"time"
)

var ApiSecret = os.Getenv("API_SECRET")
var ApiSecrets = strings.Split(os.Getenv("API_SECRET"), ",")
var GSCookie = os.Getenv("GS_COOKIE")
var GSCookies = strings.Split(os.Getenv("GS_COOKIE"), ",")
var AutoDelChat = env.Int("AUTO_DEL_CHAT", 0)
var ProxyUrl = env.String("PROXY_URL", "")
var ModelChatMapStr = env.String("MODEL_CHAT_MAP", "")
var ModelChatMap = make(map[string]string)

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
