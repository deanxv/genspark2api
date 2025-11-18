package router

import (
	"fmt"
	"genspark2api/common/config"
	"genspark2api/controller"
	"genspark2api/middleware"
	"github.com/gin-gonic/gin"
	"strings"
)

func SetApiRouter(router *gin.Engine) {
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.SecurityLogger())
	router.Use(middleware.IPBlacklistMiddleware())
	router.Use(middleware.RequestRateLimit())
	router.Use(middleware.RequestSizeLimiter(10 * 1024 * 1024)) // 10MB limit
	router.Use(middleware.RecoveryMiddleware())
	router.Use(middleware.ErrorMiddleware())
	router.Use(middleware.ValidationMiddleware())
	router.Use(middleware.SanitizeInput())
	router.Use(middleware.MetricsMiddleware())
	router.Use(middleware.RequestLoggingMiddleware())

	// Add API key validation for protected routes
	router.Use(middleware.APIKeyValidator())
	router.GET("/health", controller.HealthCheck)
	router.GET("/metrics", controller.MetricsHandler)
	router.POST("/metrics/reset", controller.ResetMetricsHandler)

	//router.GET("/api/init/model/chat/map", controller.InitModelChatMap)
	//https://api.openai.com/v1/images/generations
	v1Router := router.Group(fmt.Sprintf("%s/v1", ProcessPath(config.RoutePrefix)))
	v1Router.Use(middleware.OpenAIAuth())
	v1Router.POST("/chat/completions", controller.ChatForOpenAI)
	v1Router.POST("/images/generations", controller.ImagesForOpenAI)
	v1Router.POST("/videos/generations", controller.VideosForOpenAI)
	v1Router.GET("/models", controller.OpenaiModels)
}

func ProcessPath(path string) string {
	// 判断字符串是否为空
	if path == "" {
		return ""
	}

	// 判断开头是否为/，不是则添加
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 判断结尾是否为/，是则去掉
	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	return path
}
