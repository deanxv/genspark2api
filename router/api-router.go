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
	router.Use(middleware.AdvancedRateLimitMiddleware()) // Updated to use Redis rate limiting
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

	// Redis and Rate Limit Management (Admin only)
	router.GET("/admin/redis/status", controller.RedisStatusHandler)
	router.GET("/admin/rate-limit/stats", controller.RateLimitStatsHandler)
	router.POST("/admin/rate-limit/clear", controller.ClearRateLimitHandler)
	router.PUT("/admin/rate-limit/config", controller.ConfigureRateLimitHandler)

	// Configuration Management Routes (Admin only)
	adminRouter := router.Group("/admin")
	adminRouter.Use(middleware.AdminAuth())
	adminRouter.GET("/config", controller.GetCurrentConfig)
	adminRouter.PUT("/config", controller.UpdateConfig)
	adminRouter.GET("/config/history", controller.GetConfigHistory)
	adminRouter.POST("/config/reset", controller.ResetConfig)

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
