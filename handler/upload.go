package handler

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func UploadChunk(c *gin.Context) {
	// 创建超时上下文
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	fileID := c.PostForm("file_id")
	chunkIndex := c.PostForm("chunk_index")
	chunkMD5 := c.PostForm("md5") // 可选
	relativePath := c.PostForm("relative_path") // 新增：文件相对路径
	totalChunks := c.PostForm("total_chunks")
	fileSize := c.PostForm("file_size")

	// 验证必要参数
	if fileID == "" || chunkIndex == "" {
		c.JSON(400, gin.H{"error": "缺少必要参数: file_id 或 chunk_index"})
		return
	}

	file, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("上传文件错误: %v", err)})
		return
	}

	// 验证分片大小
	if file.Size > utils.Config.MaxChunkSize {
		c.JSON(400, gin.H{"error": fmt.Sprintf("分片大小超出限制: %d > %d", file.Size, utils.Config.MaxChunkSize)})
		return
	}

	index, err := strconv.Atoi(chunkIndex)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的分片索引"})
		return
	}

	// 创建文件锁防止并发冲突
	lockPath := filepath.Join(utils.Config.UploadDir, fileID+".lock")
	lock := utils.NewLockFile(lockPath)
	if err := lock.Acquire(); err != nil {
		log.Printf("获取文件锁失败: %v", err)
		// 继续执行，但要小心处理
	} else {
		defer lock.Release()
	}

	// 检查或创建任务记录
	task, exists := utils.Storage.GetTask(fileID)
	if !exists {
		// 创建新任务
		totalChunksInt, _ := strconv.Atoi(totalChunks)
		fileSizeInt, _ := strconv.ParseInt(fileSize, 10, 64)
		
		task = &utils.UploadTask{
			FileID:       fileID,
			FileName:     file.Filename,
			RelativePath: relativePath,
			TotalChunks:  totalChunksInt,
			FileSize:     fileSizeInt,
			Status:       "uploading",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Chunks:       make(map[int]utils.ChunkInfo),
		}
		
		if err := utils.Storage.SaveTask(task); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("保存任务失败: %v", err)})
			return
		}
	}

	// 执行上传操作（带重试机制）
	err = utils.RetryWithBackoff(ctx, func() error {
		return uploadChunkWithAtomicOperation(fileID, index, file, chunkMD5, relativePath)
	}, utils.DefaultRetryConfig)

	if err != nil {
		// 更新分片状态为失败
		chunkInfo := utils.ChunkInfo{
			Index:  index,
			Size:   file.Size,
			Status: "failed",
		}
		utils.Storage.UpdateChunk(fileID, index, chunkInfo)
		
		c.JSON(500, gin.H{"error": fmt.Sprintf("上传分片失败: %v", err)})
		return
	}

	// 更新分片状态为成功
	chunkInfo := utils.ChunkInfo{
		Index:  index,
		Size:   file.Size,
		MD5:    chunkMD5,
		Status: "completed",
	}
	
	if err := utils.Storage.UpdateChunk(fileID, index, chunkInfo); err != nil {
		log.Printf("更新分片状态失败: %v", err)
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"chunk_index":   index,
		"md5_checked":   chunkMD5 != "",
		"relative_path": relativePath,
		"size":          file.Size,
	})
}

// uploadChunkWithAtomicOperation 使用原子操作上传分片
func uploadChunkWithAtomicOperation(fileID string, index int, file *multipart.FileHeader, chunkMD5, relativePath string) error {
	saveDir := filepath.Join(utils.Config.UploadDir, fileID)
	if err := utils.EnsureDirectory(saveDir); err != nil {
		return fmt.Errorf("创建上传目录失败: %v", err)
	}

	chunkName := fmt.Sprintf("%06d.part", index)
	savePath := filepath.Join(saveDir, chunkName)

	// 检查分片是否已存在且完整
	if info, err := os.Stat(savePath); err == nil && info.Size() == file.Size {
		// 分片已存在，验证MD5
		if chunkMD5 != "" {
			existingMD5, err := utils.FileMD5(savePath)
			if err == nil && existingMD5 == chunkMD5 {
				return nil // 分片已存在且正确
			}
		} else {
			return nil // 没有MD5校验，认为已存在
		}
	}

	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("打开上传文件失败: %v", err)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("读取分片数据失败: %v", err)
	}

	// 校验 MD5（如果提供）
	if chunkMD5 != "" && utils.Config.EnableIntegrityCheck {
		calculated := utils.BytesMD5(data)
		if calculated != chunkMD5 {
			return fmt.Errorf("MD5校验失败: 期望=%s, 实际=%s", chunkMD5, calculated)
		}
	}

	// 使用原子操作写入文件
	if utils.Config.EnableAtomicOperations {
		writer, err := utils.NewAtomicWriter(savePath)
		if err != nil {
			return fmt.Errorf("创建原子写入器失败: %v", err)
		}

		if _, err := writer.Write(data); err != nil {
			writer.Rollback()
			return fmt.Errorf("写入分片数据失败: %v", err)
		}

		if err := writer.Commit(); err != nil {
			return fmt.Errorf("提交原子操作失败: %v", err)
		}
	} else {
		// 普通文件写入
		if err := os.WriteFile(savePath, data, 0644); err != nil {
			return fmt.Errorf("写入分片文件失败: %v", err)
		}
	}

	return nil
}
