package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"strconv"
)

// GetAllTasks 获取所有任务
func GetAllTasks(c *gin.Context) {
	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	tasks := utils.Storage.GetAllTasks()
	
	// 转换为响应格式
	taskList := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		uploadedChunks := utils.Storage.GetUploadedChunks(task.FileID)
		completionRate := float64(len(uploadedChunks)) / float64(task.TotalChunks) * 100

		taskList = append(taskList, gin.H{
			"file_id":         task.FileID,
			"filename":        task.FileName,
			"relative_path":   task.RelativePath,
			"total_chunks":    task.TotalChunks,
			"uploaded_chunks": len(uploadedChunks),
			"file_size":       task.FileSize,
			"status":          task.Status,
			"created_at":      task.CreatedAt,
			"updated_at":      task.UpdatedAt,
			"completion_rate": completionRate,
			"retry_count":     task.RetryCount,
		})
	}

	c.JSON(200, gin.H{
		"tasks": taskList,
		"total": len(taskList),
	})
}

// GetTask 获取单个任务详情
func GetTask(c *gin.Context) {
	fileID := c.Param("file_id")
	if fileID == "" {
		c.JSON(400, gin.H{"error": "缺少file_id参数"})
		return
	}

	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	task, exists := utils.Storage.GetTask(fileID)
	if !exists {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	uploadedChunks := utils.Storage.GetUploadedChunks(fileID)
	completionRate := float64(len(uploadedChunks)) / float64(task.TotalChunks) * 100

	// 获取分片详细信息
	chunkDetails := make([]gin.H, 0, len(task.Chunks))
	for index, chunk := range task.Chunks {
		chunkDetails = append(chunkDetails, gin.H{
			"index":       index,
			"size":        chunk.Size,
			"md5":         chunk.MD5,
			"status":      chunk.Status,
			"uploaded_at": chunk.UploadedAt,
			"retry_count": chunk.RetryCount,
		})
	}

	c.JSON(200, gin.H{
		"file_id":         task.FileID,
		"filename":        task.FileName,
		"relative_path":   task.RelativePath,
		"total_chunks":    task.TotalChunks,
		"uploaded_chunks": uploadedChunks,
		"file_size":       task.FileSize,
		"file_md5":        task.FileMD5,
		"status":          task.Status,
		"created_at":      task.CreatedAt,
		"updated_at":      task.UpdatedAt,
		"completion_rate": completionRate,
		"retry_count":     task.RetryCount,
		"chunks":          chunkDetails,
	})
}

// DeleteTask 删除任务
func DeleteTask(c *gin.Context) {
	fileID := c.Param("file_id")
	if fileID == "" {
		c.JSON(400, gin.H{"error": "缺少file_id参数"})
		return
	}

	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	// 检查任务是否存在
	_, exists := utils.Storage.GetTask(fileID)
	if !exists {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	// 删除任务
	if err := utils.Storage.DeleteTask(fileID); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("删除任务失败: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "任务删除成功",
	})
}

// CleanupTasks 清理任务
func CleanupTasks(c *gin.Context) {
	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	// 获取查询参数
	statusFilter := c.Query("status")      // 可选：只清理特定状态的任务
	olderThanStr := c.Query("older_than")  // 可选：清理N天前的任务

	var olderThanDays int
	if olderThanStr != "" {
		var err error
		olderThanDays, err = strconv.Atoi(olderThanStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "无效的older_than参数"})
			return
		}
	}

	cleanedCount := 0
	
	if statusFilter == "" && olderThanDays == 0 {
		// 执行默认清理（过期任务）
		if err := utils.Storage.CleanupExpiredTasks(); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("清理失败: %v", err)})
			return
		}
		cleanedCount = -1 // 表示使用默认清理策略
	} else {
		// 根据条件清理
		tasks := utils.Storage.GetAllTasks()
		for _, task := range tasks {
			shouldClean := false
			
			// 检查状态过滤器
			if statusFilter != "" && task.Status == statusFilter {
				shouldClean = true
			}
			
			// 检查时间过滤器
			if olderThanDays > 0 {
				daysDiff := int(task.UpdatedAt.Sub(task.UpdatedAt).Hours() / 24)
				if daysDiff >= olderThanDays {
					shouldClean = true
				}
			}
			
			if shouldClean {
				if err := utils.Storage.DeleteTask(task.FileID); err == nil {
					cleanedCount++
				}
			}
		}
	}

	message := "清理完成"
	if cleanedCount >= 0 {
		message = fmt.Sprintf("清理了 %d 个任务", cleanedCount)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"message":       message,
		"cleaned_count": cleanedCount,
	})
}

// PauseTask 暂停任务
func PauseTask(c *gin.Context) {
	fileID := c.Param("file_id")
	if fileID == "" {
		c.JSON(400, gin.H{"error": "缺少file_id参数"})
		return
	}

	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	task, exists := utils.Storage.GetTask(fileID)
	if !exists {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status == "completed" {
		c.JSON(400, gin.H{"error": "已完成的任务不能暂停"})
		return
	}

	// 更新任务状态
	task.Status = "paused"
	if err := utils.Storage.SaveTask(task); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("暂停任务失败: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "任务已暂停",
	})
}

// ResumeTask 恢复任务
func ResumeTask(c *gin.Context) {
	fileID := c.Param("file_id")
	if fileID == "" {
		c.JSON(400, gin.H{"error": "缺少file_id参数"})
		return
	}

	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	task, exists := utils.Storage.GetTask(fileID)
	if !exists {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	if task.Status != "paused" && task.Status != "failed" {
		c.JSON(400, gin.H{"error": "只有暂停或失败的任务可以恢复"})
		return
	}

	// 更新任务状态
	task.Status = "uploading"
	task.RetryCount++
	if err := utils.Storage.SaveTask(task); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("恢复任务失败: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "任务已恢复",
	})
} 