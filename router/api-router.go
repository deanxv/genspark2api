package router

import (
	"genspark2api/controller"
	"genspark2api/middleware"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	router.Use(middleware.CORS())
	//router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.RequestRateLimit())

	router.GET("/")

	//router.GET("/api/init/model/chat/map", controller.InitModelChatMap)
	//https://api.openai.com/v1/images/generations
	v1Router := router.Group("/v1")
	v1Router.Use(middleware.OpenAIAuth())
	v1Router.POST("/chat/completions", controller.ChatForOpenAI)
	v1Router.POST("/images/generations", controller.ImagesForOpenAI)
	v1Router.GET("/models", controller.OpenaiModels)
}
