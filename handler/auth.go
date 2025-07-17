package handler

import (
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"net/http"
)

// LoginRequest 登录请求结构
type LoginRequest struct {
	SecretKey string `json:"secret_key" binding:"required"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
	AuthToken string `json:"auth_token,omitempty"`
}

// Login 处理登录请求
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"code":    400,
		})
		return
	}

	// 验证密钥
	if !utils.ValidateSecretKey(req.SecretKey) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "密钥验证失败",
			"code":    401,
		})
		return
	}

	// 设置认证Cookie
	utils.SetAuthCookie(c, req.SecretKey)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "登录成功",
		"code":      200,
		"auth_token": req.SecretKey,
	})
}

// Logout 处理登出请求
func Logout(c *gin.Context) {
	// 清除认证Cookie
	utils.ClearAuthCookie(c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "登出成功",
		"code":    200,
	})
}

// CheckAuth 检查认证状态
func CheckAuth(c *gin.Context) {
	// 如果未启用验证，直接返回成功
	if !utils.Config.EnableAuth {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "验证已禁用",
			"code":    200,
			"auth_enabled": false,
		})
		return
	}

	// 获取密钥
	secretKey := c.GetHeader("X-Secret-Key")
	if secretKey == "" {
		if cookie, err := c.Cookie("secret_key"); err == nil {
			secretKey = cookie
		}
	}

	// 验证密钥
	if utils.ValidateSecretKey(secretKey) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "认证有效",
			"code":    200,
			"auth_enabled": true,
		})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "认证无效",
			"code":    401,
			"auth_enabled": true,
		})
	}
} 