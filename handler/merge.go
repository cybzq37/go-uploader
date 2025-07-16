package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-uploader/utils"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func MergeChunks(c *gin.Context) {
	fileID := c.PostForm("file_id")
	filename := c.PostForm("filename")
	totalChunksStr := c.PostForm("total_chunks")
	relativePath := c.PostForm("relative_path") // 新增：文件相对路径

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		c.String(400, "Invalid total_chunks")
		return
	}

	srcDir := filepath.Join(utils.Config.UploadDir, fileID)
	
	// 如果提供了相对路径，使用相对路径；否则使用文件名
	var dstPath string
	if relativePath != "" {
		// 清理路径，防止目录遍历攻击
		cleanPath := filepath.Clean(relativePath)
		if strings.Contains(cleanPath, "..") {
			c.String(400, "Invalid relative path")
			return
		}
		dstPath = filepath.Join(utils.Config.MergedDir, cleanPath)
	} else {
		dstPath = filepath.Join(utils.Config.MergedDir, filename)
	}
	
	// 确保目标目录存在
	dstDir := filepath.Dir(dstPath)
	if err := utils.EnsureDirectory(dstDir); err != nil {
		c.String(500, "创建目标目录失败: %v", err)
		return
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		c.String(500, "Create output file error: %v", err)
		return
	}
	defer dstFile.Close()

	// 排序分片名
	chunks := make([]string, 0)
	for i := 0; i < totalChunks; i++ {
		chunkName := fmt.Sprintf("%06d.part", i)
		chunks = append(chunks, filepath.Join(srcDir, chunkName))
	}
	sort.Strings(chunks)

	for _, chunkPath := range chunks {
		srcFile, err := os.Open(chunkPath)
		if err != nil {
			c.String(500, "Read chunk error: %v", err)
			return
		}
		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		if err != nil {
			c.String(500, "Merge failed on chunk: %v", err)
			return
		}
	}

	// 计算合并后文件的MD5
	md5Hash, err := utils.FileMD5(dstPath)
	if err != nil {
		c.String(500, "Calculate MD5 error: %v", err)
		return
	}

	// 清理临时分片文件
	os.RemoveAll(srcDir)

	c.JSON(200, gin.H{
		"status":        "ok",
		"filePath":      dstPath,
		"md5":           md5Hash,
		"relative_path": relativePath,
		"size":          getFileSize(dstPath),
	})
}

func getFileSize(filePath string) int64 {
	if info, err := os.Stat(filePath); err == nil {
		return info.Size()
	}
	return 0
}
