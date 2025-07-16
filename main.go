package main

import (
	"github.com/gin-gonic/gin"
	"go-uploader/handler"
)

func main() {
	r := gin.Default()

	// 创建 go-uploader 路由组
	goUploader := r.Group("/go-uploader")
	{
		// 配置静态文件服务
		goUploader.Static("/static", "./static")
		goUploader.LoadHTMLGlob("static/*.html")
		
		// 设置根路径重定向到index.html
		goUploader.GET("/", func(c *gin.Context) {
			c.HTML(200, "index.html", nil)
		})

		// API路由
		goUploader.POST("/upload_chunk", handler.UploadChunk)
		goUploader.POST("/merge_chunks", handler.MergeChunks)
		goUploader.GET("/upload_status", handler.UploadStatus)
	}

	r.Run(":18101") // 监听端口
}
