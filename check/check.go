package check

import (
	"genspark2api/common"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"github.com/samber/lo"
	"regexp"
	"strings"
)

func CheckEnvVariable() {
	logger.SysLog("environment variable checking...")

	if config.GSCookie == "" {
		logger.FatalLog("环境变量 GS_COOKIE 未设置")
	}
	if config.ModelChatMapStr != "" {
		pattern := `^([a-zA-Z0-9\-\/]+=([a-zA-Z0-9\-]+))(,[a-zA-Z0-9\-\/]+=([a-zA-Z0-9\-]+))*$`
		match, _ := regexp.MatchString(pattern, config.ModelChatMapStr)
		if !match {
			logger.FatalLog("环境变量 MODEL_CHAT_MAP 设置有误")
		} else {
			modelChatMap := make(map[string]string)
			pairs := strings.Split(config.ModelChatMapStr, ",")

			for _, pair := range pairs {
				kv := strings.Split(pair, "=")
				if !lo.Contains(common.DefaultOpenaiModelList, kv[0]) {
					logger.FatalLog("环境变量 MODEL_CHAT_MAP 中 MODEL 有误")
				}
				modelChatMap[kv[0]] = kv[1]
			}

			config.ModelChatMap = modelChatMap

			//if config.AutoDelChat == 1 {
			//	logger.FatalLog("环境变量 MODEL_CHAT_MAP 有值时,环境变量 AUTO_DEL_CHAT 不能设置为1")
			//}

			cookies := strings.Split(config.GSCookie, ",")
			if len(cookies) > 1 {
				logger.FatalLog("环境变量 MODEL_CHAT_MAP 有值时,环境变量 GS_COOKIE 仅能设置一个")
			}
		}
	}
	logger.SysLog("environment variable check passed.")
}
