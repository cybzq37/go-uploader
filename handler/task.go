package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"strconv"
)

// CreateFolderTask 创建文件夹任务
func CreateFolderTask(c *gin.Context) {
	var req struct {
		FolderName string            `json:"folder_name" binding:"required"`
		Files      []utils.FileInfo  `json:"files" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	// 验证文件信息
	if len(req.Files) == 0 {
		c.JSON(400, gin.H{"error": "文件列表不能为空"})
		return
	}

	// 创建文件夹任务
	folderTask, err := utils.Storage.CreateFolderTask(req.FolderName, req.Files)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("创建文件夹任务失败: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"status":         "ok",
		"message":        "文件夹任务创建成功",
		"folder_task_id": folderTask.FileID,
		"folder_name":    folderTask.FolderName,
		"total_files":    len(folderTask.SubTasks),
		"total_size":     folderTask.FileSize,
		"sub_tasks":      folderTask.SubTasks,
	})
}

// GetFolderTaskSummary 获取文件夹任务摘要
func GetFolderTaskSummary(c *gin.Context) {
	folderTaskID := c.Param("folder_task_id")
	if folderTaskID == "" {
		c.JSON(400, gin.H{"error": "缺少folder_task_id参数"})
		return
	}

	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	summary, err := utils.Storage.GetFolderTaskSummary(folderTaskID)
	if err != nil {
		c.JSON(404, gin.H{"error": fmt.Sprintf("获取文件夹任务摘要失败: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"folder_task_id":  folderTaskID,
		"total_files":     summary.TotalFiles,
		"completed_files": summary.CompletedFiles,
		"failed_files":    summary.FailedFiles,
		"total_size":      summary.TotalSize,
		"uploaded_size":   summary.UploadedSize,
		"completion_rate": summary.CompletionRate,
		"status":          summary.Status,
	})
}

// GetSubTasks 获取文件夹的子任务列表
func GetSubTasks(c *gin.Context) {
	folderTaskID := c.Param("folder_task_id")
	if folderTaskID == "" {
		c.JSON(400, gin.H{"error": "缺少folder_task_id参数"})
		return
	}

	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	subTasks, err := utils.Storage.GetSubTasks(folderTaskID)
	if err != nil {
		c.JSON(404, gin.H{"error": fmt.Sprintf("获取子任务失败: %v", err)})
		return
	}

	// 转换为响应格式
	taskList := make([]gin.H, 0, len(subTasks))
	for _, task := range subTasks {
		uploadedChunks := utils.Storage.GetUploadedChunks(task.FileID)
		completionRate := float64(0)
		if task.TotalChunks > 0 {
			completionRate = float64(len(uploadedChunks)) / float64(task.TotalChunks) * 100
		}

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
			"parent_task_id":  task.ParentTaskID,
		})
	}

	c.JSON(200, gin.H{
		"folder_task_id": folderTaskID,
		"sub_tasks":      taskList,
		"total":          len(taskList),
	})
}

// GetAllTasks 获取所有主任务（修改为只显示主任务）
func GetAllTasks(c *gin.Context) {
	if utils.Storage == nil {
		c.JSON(500, gin.H{"error": "存储管理器未初始化"})
		return
	}

	// 只获取主任务（非子任务）
	tasks := utils.Storage.GetMainTasks()
	
	// 转换为响应格式
	taskList := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		var taskInfo gin.H
		
		if task.TaskType == "folder" {
			// 文件夹任务
			summary, err := utils.Storage.GetFolderTaskSummary(task.FileID)
			if err != nil {
				// 如果获取摘要失败，使用基本信息
				taskInfo = gin.H{
					"task_id":         task.FileID,
					"task_type":       task.TaskType,
					"folder_name":     task.FolderName,
					"filename":        task.FileName,
					"total_files":     len(task.SubTasks),
					"file_size":       task.FileSize,
					"status":          task.Status,
					"created_at":      task.CreatedAt,
					"updated_at":      task.UpdatedAt,
					"completion_rate": 0.0,
					"retry_count":     task.RetryCount,
				}
			} else {
				taskInfo = gin.H{
					"task_id":         task.FileID,
					"task_type":       task.TaskType,
					"folder_name":     task.FolderName,
					"filename":        task.FileName,
					"total_files":     summary.TotalFiles,
					"completed_files": summary.CompletedFiles,
					"failed_files":    summary.FailedFiles,
					"file_size":       task.FileSize,
					"uploaded_size":   summary.UploadedSize,
					"status":          summary.Status,
					"created_at":      task.CreatedAt,
					"updated_at":      task.UpdatedAt,
					"completion_rate": summary.CompletionRate,
					"retry_count":     task.RetryCount,
				}
			}
		} else {
			// 单文件任务
			uploadedChunks := utils.Storage.GetUploadedChunks(task.FileID)
			completionRate := float64(0)
			if task.TotalChunks > 0 {
				completionRate = float64(len(uploadedChunks)) / float64(task.TotalChunks) * 100
			}

			taskInfo = gin.H{
				"task_id":         task.FileID,
				"task_type":       task.TaskType,
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
			}
		}

		taskList = append(taskList, taskInfo)
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

	if task.TaskType == "folder" {
		// 文件夹任务详情
		summary, err := utils.Storage.GetFolderTaskSummary(fileID)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("获取文件夹任务摘要失败: %v", err)})
			return
		}

		subTasks, err := utils.Storage.GetSubTasks(fileID)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("获取子任务失败: %v", err)})
			return
		}

		// 转换子任务格式
		subTaskDetails := make([]gin.H, 0, len(subTasks))
		for _, subTask := range subTasks {
			uploadedChunks := utils.Storage.GetUploadedChunks(subTask.FileID)
			completionRate := float64(0)
			if subTask.TotalChunks > 0 {
				completionRate = float64(len(uploadedChunks)) / float64(subTask.TotalChunks) * 100
			}

			subTaskDetails = append(subTaskDetails, gin.H{
				"file_id":         subTask.FileID,
				"filename":        subTask.FileName,
				"relative_path":   subTask.RelativePath,
				"total_chunks":    subTask.TotalChunks,
				"uploaded_chunks": len(uploadedChunks),
				"file_size":       subTask.FileSize,
				"status":          subTask.Status,
				"created_at":      subTask.CreatedAt,
				"updated_at":      subTask.UpdatedAt,
				"completion_rate": completionRate,
				"retry_count":     subTask.RetryCount,
			})
		}

		c.JSON(200, gin.H{
			"task_id":         task.FileID,
			"task_type":       task.TaskType,
			"folder_name":     task.FolderName,
			"filename":        task.FileName,
			"total_files":     summary.TotalFiles,
			"completed_files": summary.CompletedFiles,
			"failed_files":    summary.FailedFiles,
			"file_size":       task.FileSize,
			"uploaded_size":   summary.UploadedSize,
			"status":          summary.Status,
			"created_at":      task.CreatedAt,
			"updated_at":      task.UpdatedAt,
			"completion_rate": summary.CompletionRate,
			"retry_count":     task.RetryCount,
			"sub_tasks":       subTaskDetails,
		})
	} else {
		// 单文件任务详情
		uploadedChunks := utils.Storage.GetUploadedChunks(fileID)
		completionRate := float64(0)
		if task.TotalChunks > 0 {
			completionRate = float64(len(uploadedChunks)) / float64(task.TotalChunks) * 100
		}

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
			"task_id":         task.FileID,
			"task_type":       task.TaskType,
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
			"parent_task_id":  task.ParentTaskID,
			"is_sub_task":     task.IsSubTask,
		})
	}
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
	task, exists := utils.Storage.GetTask(fileID)
	if !exists {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	// 删除任务
	if err := utils.Storage.DeleteTask(fileID); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("删除任务失败: %v", err)})
		return
	}

	message := "任务删除成功"
	if task.TaskType == "folder" {
		message = fmt.Sprintf("文件夹任务 '%s' 及其所有子任务删除成功", task.FolderName)
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": message,
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
		// 根据条件清理 - 只清理主任务
		tasks := utils.Storage.GetMainTasks()
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
	
	// 如果是文件夹任务，暂停所有子任务
	if task.TaskType == "folder" {
		for _, subTaskID := range task.SubTasks {
			if subTask, exists := utils.Storage.GetTask(subTaskID); exists && subTask.Status == "uploading" {
				subTask.Status = "paused"
				utils.Storage.SaveTask(subTask)
			}
		}
	}
	
	if err := utils.Storage.SaveTask(task); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("暂停任务失败: %v", err)})
		return
	}

	message := "任务已暂停"
	if task.TaskType == "folder" {
		message = fmt.Sprintf("文件夹任务 '%s' 及其所有子任务已暂停", task.FolderName)
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": message,
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
	
	// 如果是文件夹任务，恢复所有暂停的子任务
	if task.TaskType == "folder" {
		for _, subTaskID := range task.SubTasks {
			if subTask, exists := utils.Storage.GetTask(subTaskID); exists && (subTask.Status == "paused" || subTask.Status == "failed") {
				subTask.Status = "uploading"
				subTask.RetryCount++
				utils.Storage.SaveTask(subTask)
			}
		}
	}
	
	if err := utils.Storage.SaveTask(task); err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("恢复任务失败: %v", err)})
		return
	}

	message := "任务已恢复"
	if task.TaskType == "folder" {
		message = fmt.Sprintf("文件夹任务 '%s' 及其所有子任务已恢复", task.FolderName)
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": message,
	})
} 