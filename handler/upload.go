package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func UploadChunk(c *gin.Context) {
	fileID := c.PostForm("file_id")
	chunkIndex := c.PostForm("chunk_index")
	chunkMD5 := c.PostForm("md5") // 可选
	relativePath := c.PostForm("relative_path") // 新增：文件相对路径

	file, err := c.FormFile("chunk")
	if err != nil {
		c.String(400, "Missing file chunk: %v", err)
		return
	}

	index, err := strconv.Atoi(chunkIndex)
	if err != nil {
		c.String(400, "Invalid chunk index")
		return
	}

	saveDir := filepath.Join(utils.Config.UploadDir, fileID)
	if err := utils.EnsureDirectory(saveDir); err != nil {
		c.String(500, "创建上传目录失败: %v", err)
		return
	}

	chunkName := fmt.Sprintf("%06d.part", index)
	savePath := filepath.Join(saveDir, chunkName)

	src, err := file.Open()
	if err != nil {
		c.String(500, "Open uploaded chunk error: %v", err)
		return
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		c.String(500, "Read chunk data error: %v", err)
		return
	}

	// 校验 MD5（如果提供）
	if chunkMD5 != "" {
		calculated := utils.BytesMD5(data)
		if calculated != chunkMD5 {
			c.String(400, "MD5 mismatch: expect %s, got %s", chunkMD5, calculated)
			return
		}
	}

	dst, err := os.Create(savePath)
	if err != nil {
		c.String(500, "Create chunk file error: %v", err)
		return
	}
	defer dst.Close()

	_, err = dst.Write(data)
	if err != nil {
		c.String(500, "Write chunk error: %v", err)
		return
	}

	c.JSON(200, gin.H{
		"status":        "ok",
		"chunk_index":   index,
		"md5_checked":   chunkMD5 != "",
		"relative_path": relativePath,
	})
}
