package utils

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// AuthMiddleware 密钥验证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未启用验证，直接通过
		if !Config.EnableAuth {
			c.Next()
			return
		}

		// 检查是否是静态文件请求，静态文件不需要验证
		if strings.HasPrefix(c.Request.URL.Path, "/go-uploader/static/") {
			c.Next()
			return
		}

		// 检查是否是根路径（登录页面），不需要验证
		if c.Request.URL.Path == "/go-uploader/" || c.Request.URL.Path == "/go-uploader" {
			c.Next()
			return
		}

		// 获取密钥，支持多种方式
		secretKey := ""
		
		// 1. 从请求头获取
		secretKey = c.GetHeader("X-Secret-Key")
		
		// 2. 从查询参数获取
		if secretKey == "" {
			secretKey = c.Query("secret_key")
		}
		
		// 3. 从Cookie获取
		if secretKey == "" {
			if cookie, err := c.Cookie("secret_key"); err == nil {
				secretKey = cookie
			}
		}

		// 验证密钥
		if secretKey == "" || secretKey != Config.SecretKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "未授权访问",
				"message": "请提供有效的访问密钥",
				"code":    401,
			})
			c.Abort()
			return
		}

		// 验证通过，继续处理请求
		c.Next()
	}
}

// ValidateSecretKey 验证密钥是否有效
func ValidateSecretKey(key string) bool {
	if !Config.EnableAuth {
		return true
	}
	return key == Config.SecretKey
}

// SetAuthCookie 设置认证Cookie
func SetAuthCookie(c *gin.Context, secretKey string) {
	c.SetCookie("secret_key", secretKey, 24*60*60, "/go-uploader", "", false, true) // 24小时过期
}

// ClearAuthCookie 清除认证Cookie
func ClearAuthCookie(c *gin.Context) {
	c.SetCookie("secret_key", "", -1, "/go-uploader", "", false, true)
} 