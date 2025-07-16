package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/handler"
	"go-uploader/utils"
	"log"
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

		// API路由
		goUploader.POST("/upload_chunk", handler.UploadChunk)
		goUploader.POST("/merge_chunks", handler.MergeChunks)
		goUploader.GET("/upload_status", handler.UploadStatus)
	}

	// 使用配置中的端口
	port := fmt.Sprintf(":%s", utils.Config.Port)
	log.Printf("服务器启动，监听端口: %s", utils.Config.Port)
	r.Run(port) // 监听端口
}
