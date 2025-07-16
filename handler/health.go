package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// HealthCheck 健康检查
func HealthCheck(c *gin.Context) {
	status := "healthy"
	checks := make(map[string]interface{})
	
	// 检查存储管理器
	if utils.Storage == nil {
		status = "unhealthy"
		checks["storage"] = "未初始化"
	} else {
		checks["storage"] = "正常"
	}
	
	// 检查上传目录
	if _, err := os.Stat(utils.Config.UploadDir); os.IsNotExist(err) {
		status = "unhealthy"
		checks["upload_dir"] = "目录不存在"
	} else {
		checks["upload_dir"] = "正常"
	}
	
	// 检查合并目录
	if _, err := os.Stat(utils.Config.MergedDir); os.IsNotExist(err) {
		status = "unhealthy"
		checks["merged_dir"] = "目录不存在"
	} else {
		checks["merged_dir"] = "正常"
	}
	
	// 检查磁盘空间
	diskUsage, err := getDiskUsage(utils.Config.UploadDir)
	if err != nil {
		checks["disk_space"] = fmt.Sprintf("检查失败: %v", err)
	} else {
		checks["disk_space"] = diskUsage
		// 如果磁盘使用率超过95%，标记为不健康
		if diskUsage["usage_percent"].(float64) > 95 {
			status = "warning"
		}
	}
	
	httpStatus := 200
	if status == "unhealthy" {
		httpStatus = 503
	} else if status == "warning" {
		httpStatus = 200 // 警告状态仍返回200，但在响应中标明
	}
	
	c.JSON(httpStatus, gin.H{
		"status":    status,
		"timestamp": time.Now(),
		"checks":    checks,
	})
}

// SystemInfo 系统信息
func SystemInfo(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// 获取任务统计
	var taskStats map[string]int
	if utils.Storage != nil {
		taskStats = getTaskStatistics()
	} else {
		taskStats = map[string]int{"error": 0}
	}
	
	// 获取磁盘使用情况
	diskUsage, err := getDiskUsage(utils.Config.UploadDir)
	if err != nil {
		diskUsage = map[string]interface{}{
			"error": err.Error(),
		}
	}
	
	c.JSON(200, gin.H{
		"system": gin.H{
			"go_version":      runtime.Version(),
			"goroutines":      runtime.NumGoroutine(),
			"memory_alloc":    bToMb(m.Alloc),
			"memory_total":    bToMb(m.TotalAlloc),
			"memory_sys":      bToMb(m.Sys),
			"gc_runs":         m.NumGC,
		},
		"config": gin.H{
			"upload_dir":               utils.Config.UploadDir,
			"merged_dir":               utils.Config.MergedDir,
			"max_file_size":            utils.Config.MaxFileSize,
			"max_chunk_size":           utils.Config.MaxChunkSize,
			"concurrent_uploads":       utils.Config.ConcurrentUploads,
			"enable_integrity_check":   utils.Config.EnableIntegrityCheck,
			"enable_atomic_operations": utils.Config.EnableAtomicOperations,
		},
		"tasks":      taskStats,
		"disk_usage": diskUsage,
		"timestamp":  time.Now(),
	})
}

// getTaskStatistics 获取任务统计信息
func getTaskStatistics() map[string]int {
	stats := map[string]int{
		"total":     0,
		"uploading": 0,
		"completed": 0,
		"failed":    0,
		"paused":    0,
	}
	
	tasks := utils.Storage.GetAllTasks()
	for _, task := range tasks {
		stats["total"]++
		stats[task.Status]++
	}
	
	return stats
}

// getDiskUsage 获取磁盘使用情况
func getDiskUsage(path string) (map[string]interface{}, error) {
	// 获取文件系统信息
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	
	// 在Windows和Unix系统上，这个实现会有所不同
	// 这里提供一个简化的版本
	uploadDirSize, err := getDirSize(utils.Config.UploadDir)
	if err != nil {
		uploadDirSize = 0
	}
	
	mergedDirSize, err := getDirSize(utils.Config.MergedDir)
	if err != nil {
		mergedDirSize = 0
	}
	
	// 简化的磁盘使用率计算
	// 实际应用中应该使用系统调用获取真实的磁盘空间信息
	totalUsed := uploadDirSize + mergedDirSize
	
	return map[string]interface{}{
		"upload_dir_size":  uploadDirSize,
		"merged_dir_size":  mergedDirSize,
		"total_used":       totalUsed,
		"usage_percent":    float64(totalUsed) / float64(utils.Config.MaxFileSize) * 100, // 简化计算
		"last_checked":     time.Now(),
	}, nil
}

// getDirSize 计算目录大小
func getDirSize(path string) (int64, error) {
	var size int64
	
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	
	return size, err
}

// bToMb 字节转MB
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// GetMetrics 获取性能指标
func GetMetrics(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// 计算活跃任务数
	activeTasks := 0
	if utils.Storage != nil {
		tasks := utils.Storage.GetAllTasks()
		for _, task := range tasks {
			if task.Status == "uploading" {
				activeTasks++
			}
		}
	}
	
	c.JSON(200, gin.H{
		"timestamp": time.Now().Unix(),
		"metrics": gin.H{
			"goroutines":     runtime.NumGoroutine(),
			"memory_mb":      bToMb(m.Alloc),
			"gc_runs":        m.NumGC,
			"active_tasks":   activeTasks,
		},
	})
} 