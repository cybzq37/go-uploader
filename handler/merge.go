package handler

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func MergeChunks(c *gin.Context) {
	// 创建超时上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	
	fileID := c.PostForm("file_id")
	filename := c.PostForm("filename")
	totalChunksStr := c.PostForm("total_chunks")
	relativePath := c.PostForm("relative_path") // 新增：文件相对路径
	expectedMD5 := c.PostForm("expected_md5")   // 可选：期望的文件MD5

	// 验证必要参数
	if fileID == "" || filename == "" || totalChunksStr == "" {
		c.JSON(400, gin.H{"error": "缺少必要参数"})
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的分片总数"})
		return
	}

	// 获取任务信息
	task, exists := utils.Storage.GetTask(fileID)
	if !exists {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	// 验证所有分片是否已上传
	uploadedChunks := utils.Storage.GetUploadedChunks(fileID)
	if len(uploadedChunks) != totalChunks {
		c.JSON(400, gin.H{
			"error":          "分片未完全上传",
			"uploaded":       len(uploadedChunks),
			"total_required": totalChunks,
		})
		return
	}

	// 创建文件锁 - 使用安全的文件名
	safeFileID := utils.SanitizeFileID(fileID)
	lockPath := filepath.Join(utils.Config.UploadDir, safeFileID+".merge.lock")
	lock := utils.NewLockFile(lockPath)
	if err := lock.Acquire(); err != nil {
		c.JSON(409, gin.H{"error": "合并操作正在进行中"})
		return
	}
	defer lock.Release()

	// 执行合并操作（带重试机制）
	var result *MergeResult
	err = utils.RetryWithBackoff(ctx, func() error {
		var mergeErr error
		result, mergeErr = mergeChunksWithIntegrityCheck(fileID, filename, relativePath, totalChunks, expectedMD5, task)
		return mergeErr
	}, utils.DefaultRetryConfig)

	if err != nil {
		// 更新任务状态为失败
		task.Status = "failed"
		utils.Storage.SaveTask(task)
		
		c.JSON(500, gin.H{"error": fmt.Sprintf("合并文件失败: %v", err)})
		return
	}

	// 更新任务状态为完成
	task.Status = "completed"
	task.FileMD5 = result.MD5
	if err := utils.Storage.SaveTask(task); err != nil {
		log.Printf("更新任务状态失败: %v", err)
	}

	// 清理临时分片文件（异步执行）
	go func() {
		srcDir := filepath.Join(utils.Config.UploadDir, fileID)
		if err := os.RemoveAll(srcDir); err != nil {
			log.Printf("清理临时文件失败: %v", err)
		}
	}()

	c.JSON(200, gin.H{
		"status":        "ok",
		"filePath":      result.FilePath,
		"md5":           result.MD5,
		"relative_path": relativePath,
		"size":          result.Size,
		"merge_time":    result.MergeTime,
	})
}

// MergeResult 合并结果
type MergeResult struct {
	FilePath  string
	MD5       string
	Size      int64
	MergeTime time.Duration
}

// mergeChunksWithIntegrityCheck 带完整性检查的分片合并
func mergeChunksWithIntegrityCheck(fileID, filename, relativePath string, totalChunks int, expectedMD5 string, task *utils.UploadTask) (*MergeResult, error) {
	startTime := time.Now()
	
	// 使用安全的文件ID作为目录名，实现扁平化存储
	safeFileID := utils.SanitizeFileID(fileID)
	srcDir := filepath.Join(utils.Config.UploadDir, safeFileID)
	
	// 确定目标路径
	var dstPath string
	if relativePath != "" {
		// 清理路径，防止目录遍历攻击
		cleanPath := filepath.Clean(relativePath)
		if strings.Contains(cleanPath, "..") {
			return nil, fmt.Errorf("无效的相对路径")
		}
		dstPath = filepath.Join(utils.Config.MergedDir, cleanPath)
	} else {
		dstPath = filepath.Join(utils.Config.MergedDir, filename)
	}
	
	// 确保目标目录存在
	dstDir := filepath.Dir(dstPath)
	if err := utils.EnsureDirectory(dstDir); err != nil {
		return nil, fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 验证所有分片文件是否存在
	chunkPaths := make([]string, totalChunks)
	for i := 0; i < totalChunks; i++ {
		chunkName := fmt.Sprintf("%06d.part", i)
		chunkPath := filepath.Join(srcDir, chunkName)
		
		if _, err := os.Stat(chunkPath); err != nil {
			return nil, fmt.Errorf("分片文件缺失: %s", chunkName)
		}
		
		chunkPaths[i] = chunkPath
	}

	// 使用原子操作合并文件
	if utils.Config.EnableAtomicOperations {
		writer, err := utils.NewAtomicWriter(dstPath)
		if err != nil {
			return nil, fmt.Errorf("创建原子写入器失败: %v", err)
		}

		// 按顺序合并分片
		for i, chunkPath := range chunkPaths {
			chunkFile, err := os.Open(chunkPath)
			if err != nil {
				writer.Rollback()
				return nil, fmt.Errorf("打开分片 %d 失败: %v", i, err)
			}

			_, err = io.Copy(writer, chunkFile)
			chunkFile.Close()
			
			if err != nil {
				writer.Rollback()
				return nil, fmt.Errorf("复制分片 %d 失败: %v", i, err)
			}
		}

		// 提交原子操作
		if err := writer.Commit(); err != nil {
			return nil, fmt.Errorf("提交合并操作失败: %v", err)
		}
		
		calculatedMD5 := writer.GetMD5()
		fileSize := writer.GetSize()
		
		// 验证文件完整性
		if expectedMD5 != "" && utils.Config.EnableIntegrityCheck {
			if calculatedMD5 != expectedMD5 {
				os.Remove(dstPath)
				return nil, fmt.Errorf("文件完整性验证失败: 期望=%s, 实际=%s", expectedMD5, calculatedMD5)
			}
		}

		// 合并成功后，异步清理分片文件和锁文件
		go func() {
			// 清理分片目录
			if err := os.RemoveAll(srcDir); err != nil {
				log.Printf("清理分片目录失败 [%s]: %v", safeFileID, err)
			} else {
				log.Printf("成功清理分片目录: %s", srcDir)
			}
			
			// 清理锁文件
			lockPath := filepath.Join(utils.Config.UploadDir, safeFileID+".lock")
			if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
				log.Printf("清理上传锁文件失败 [%s]: %v", safeFileID, err)
			}
			
			mergeLockPath := filepath.Join(utils.Config.UploadDir, safeFileID+".merge.lock")
			if err := os.Remove(mergeLockPath); err != nil && !os.IsNotExist(err) {
				log.Printf("清理合并锁文件失败 [%s]: %v", safeFileID, err)
			}
		}()

		return &MergeResult{
			FilePath:  dstPath,
			MD5:       calculatedMD5,
			Size:      fileSize,
			MergeTime: time.Since(startTime),
		}, nil

	} else {
		// 传统合并方式
		dstFile, err := os.Create(dstPath)
		if err != nil {
			return nil, fmt.Errorf("创建目标文件失败: %v", err)
		}
		defer dstFile.Close()

		// 按顺序合并分片
		for i, chunkPath := range chunkPaths {
			srcFile, err := os.Open(chunkPath)
			if err != nil {
				return nil, fmt.Errorf("打开分片 %d 失败: %v", i, err)
			}

			_, err = io.Copy(dstFile, srcFile)
			srcFile.Close()
			
			if err != nil {
				return nil, fmt.Errorf("复制分片 %d 失败: %v", i, err)
			}
		}

		// 确保数据写入磁盘
		if err := dstFile.Sync(); err != nil {
			return nil, fmt.Errorf("同步文件失败: %v", err)
		}

		// 计算MD5
		md5Hash, err := utils.FileMD5(dstPath)
		if err != nil {
			return nil, fmt.Errorf("计算MD5失败: %v", err)
		}

		// 验证文件完整性
		if expectedMD5 != "" && utils.Config.EnableIntegrityCheck {
			if md5Hash != expectedMD5 {
				os.Remove(dstPath)
				return nil, fmt.Errorf("文件完整性验证失败: 期望=%s, 实际=%s", expectedMD5, md5Hash)
			}
		}

		fileInfo, _ := os.Stat(dstPath)
		
		// 合并成功后，异步清理分片文件和锁文件
		go func() {
			// 清理分片目录
			if err := os.RemoveAll(srcDir); err != nil {
				log.Printf("清理分片目录失败 [%s]: %v", safeFileID, err)
			} else {
				log.Printf("成功清理分片目录: %s", srcDir)
			}
			
			// 清理锁文件
			lockPath := filepath.Join(utils.Config.UploadDir, safeFileID+".lock")
			if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
				log.Printf("清理上传锁文件失败 [%s]: %v", safeFileID, err)
			}
			
			mergeLockPath := filepath.Join(utils.Config.UploadDir, safeFileID+".merge.lock")
			if err := os.Remove(mergeLockPath); err != nil && !os.IsNotExist(err) {
				log.Printf("清理合并锁文件失败 [%s]: %v", safeFileID, err)
			}
		}()
		
		return &MergeResult{
			FilePath:  dstPath,
			MD5:       md5Hash,
			Size:      fileInfo.Size(),
			MergeTime: time.Since(startTime),
		}, nil
	}
}

func getFileSize(filePath string) int64 {
	if info, err := os.Stat(filePath); err == nil {
		return info.Size()
	}
	return 0
}
