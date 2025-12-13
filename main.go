package main

import (
	"fmt"
	"genspark2api/check"
	"genspark2api/common"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"genspark2api/middleware"
	"genspark2api/router"
	"genspark2api/yescaptcha"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

func main() {
	// Pass log directory from command line flag to logger
	if *common.LogDir != "" {
		logger.LogDir = *common.LogDir
	}
	logger.SetupLogger()
	logger.SysLog(fmt.Sprintf("genspark2api %s starting...", common.Version))

	check.CheckEnvVariable()

	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	var err error

	common.InitTokenEncoders()
	config.InitGSCookies()
	config.YescaptchaClient = yescaptcha.NewClient(config.YesCaptchaClientKey, nil)

	config.GlobalSessionManager = config.NewSessionManager()

	// Log configuration summary (without secrets)
	for _, line := range config.GetConfigSummary() {
		logger.SysLog(line)
	}

	// 定时任务 每天9点整重载GS_COOKIES
	//go job.LoadCookieTask()

	server := gin.New()
	server.Use(gin.Recovery())
	server.Use(middleware.RequestId())
	middleware.SetUpLogger(server)

	router.SetRouter(server)
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}

	if config.DebugEnabled {
		logger.SysLog("running in DEBUG mode.")
	}

	logger.SysLog("genspark2api start success. enjoy it! ^_^\n")

	err = server.Run(":" + port)

	if err != nil {
		logger.FatalLog("failed to start HTTP server: " + err.Error())
	}

}
