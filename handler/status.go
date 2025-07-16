package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"os"
	"path/filepath"
	"strings"
)

func UploadStatus(c *gin.Context) {
	fileID := c.Query("file_id")
	
	if fileID == "" {
		c.JSON(400, gin.H{"error": "缺少file_id参数"})
		return
	}

	// 优先从存储管理器获取状态
	if utils.Storage != nil {
		task, exists := utils.Storage.GetTask(fileID)
		if exists {
			uploaded := utils.Storage.GetUploadedChunks(fileID)
			
			c.JSON(200, gin.H{
				"uploaded_chunks": uploaded,
				"total_chunks":    task.TotalChunks,
				"file_size":       task.FileSize,
				"status":          task.Status,
				"created_at":      task.CreatedAt,
				"updated_at":      task.UpdatedAt,
				"completion_rate": float64(len(uploaded)) / float64(task.TotalChunks) * 100,
			})
			return
		}
	}

	// 回退到文件系统检查（兼容旧版本）
	// 使用安全的文件ID作为目录名，适应扁平化存储
	safeFileID := utils.SanitizeFileID(fileID)
	dir := filepath.Join(utils.Config.UploadDir, safeFileID)
	files, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(200, gin.H{
			"uploaded_chunks": []int{},
			"status":          "not_found",
		})
		return
	}

	uploaded := []int{}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".part") {
			name := strings.TrimSuffix(f.Name(), ".part")
			var idx int
			fmt.Sscanf(name, "%d", &idx)
			uploaded = append(uploaded, idx)
		}
	}

	c.JSON(200, gin.H{
		"uploaded_chunks": uploaded,
		"status":          "uploading",
	})
}
