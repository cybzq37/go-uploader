package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/handler"
	"go-uploader/utils"
	"log"
	"time"
)

const configFile = "./config.json"

func main() {
	// 加载配置文件
	if err := utils.LoadConfig(configFile); err != nil {
		log.Printf("加载配置文件失败: %v，将使用默认配置", err)
	}
	
	// 初始化配置目录
	if err := utils.InitDirectories(); err != nil {
		log.Fatalf("初始化目录失败: %v", err)
	}
	
	// 初始化存储管理器
	if err := utils.InitStorage(); err != nil {
		log.Fatalf("初始化存储管理器失败: %v", err)
	}
	
	// 启动清理任务
	go startCleanupRoutine()
	
	r := gin.Default()
	
	// 配置HTML模板
	r.LoadHTMLGlob("static/*.html")

	// 创建 go-uploader 路由组
	goUploader := r.Group("/go-uploader")
	{
		// 配置静态文件服务
		goUploader.Static("/static", "./static")
		
		// 设置根路径重定向到index.html
		goUploader.GET("/", func(c *gin.Context) {
			c.HTML(200, "index.html", nil)
		})

		// 认证相关路由（不需要验证）
		goUploader.POST("/auth/login", handler.Login)
		goUploader.POST("/auth/logout", handler.Logout)
		goUploader.GET("/auth/check", handler.CheckAuth)

		// 应用认证中间件到所有其他API路由
		api := goUploader.Group("")
		api.Use(utils.AuthMiddleware())
		{
			// API路由
			api.POST("/upload_chunk", handler.UploadChunk)
			api.POST("/merge_chunks", handler.MergeChunks)
			api.GET("/upload_status", handler.UploadStatus)
			
			// 任务管理API
			api.GET("/tasks", handler.GetAllTasks)
			api.GET("/tasks/:file_id", handler.GetTask)
			api.DELETE("/tasks/:file_id", handler.DeleteTask)
			api.POST("/tasks/:file_id/pause", handler.PauseTask)
			api.POST("/tasks/:file_id/resume", handler.ResumeTask)
			api.POST("/tasks/cleanup", handler.CleanupTasks)
			api.POST("/tasks/resume_all_failed", handler.ResumeAllFailedTasks)
			api.GET("/tasks/failed", handler.GetFailedTasks)
			
			// 文件夹任务API
			api.POST("/folder_tasks", handler.CreateFolderTask)
			api.GET("/folder_tasks/:folder_task_id/summary", handler.GetFolderTaskSummary)
			api.GET("/folder_tasks/:folder_task_id/sub_tasks", handler.GetSubTasks)
			
			// 监控和健康检查API
			api.GET("/health", handler.HealthCheck)
			api.GET("/system", handler.SystemInfo)
			api.GET("/metrics", handler.GetMetrics)
		}
	}

	// 使用配置中的端口
	port := fmt.Sprintf(":%s", utils.Config.Port)
	log.Printf("服务器启动，监听端口: %s", utils.Config.Port)
	if utils.Config.EnableAuth {
		log.Printf("密钥验证已启用，密钥: %s", utils.Config.SecretKey)
	} else {
		log.Printf("密钥验证已禁用")
	}
	r.Run(port) // 监听端口
}

// startCleanupRoutine 启动定期清理任务
func startCleanupRoutine() {
	ticker := time.NewTicker(time.Duration(utils.Config.CleanupInterval) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := utils.Storage.CleanupExpiredTasks(); err != nil {
				log.Printf("清理过期任务失败: %v", err)
			} else {
				log.Printf("定期清理任务完成")
			}
		}
	}
}
