package controller

import (
	"github.com/deanxv/CycleTLS/cycletls"
	"github.com/gin-gonic/gin"
)

// ChatForOpenAI 处理OpenAI聊天请求
func InitModelChatMap(c *gin.Context) {
	client := cycletls.Init()
	defer safeClose(client)

	// TODO
}
