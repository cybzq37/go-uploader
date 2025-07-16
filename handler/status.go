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
	dir := filepath.Join(utils.Config.UploadDir, fileID)

	files, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(200, gin.H{"uploaded_chunks": []int{}})
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
	})
}
