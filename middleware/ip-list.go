package middleware

import (
	"genspark2api/common/config"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// IPBlacklistMiddleware 检查请求的IP是否在黑名单中
func IPBlacklistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取请求的IP地址
		clientIP := c.ClientIP()

		// 检查IP是否在黑名单中
		for _, blockedIP := range config.IpBlackList {
			if strings.TrimSpace(blockedIP) == clientIP {
				// 如果在黑名单中，返回403 Forbidden
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
				return
			}
		}

		// 如果不在黑名单中，继续处理请求
		c.Next()
	}
}
