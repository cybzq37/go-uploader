package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UploadTask 上传任务结构
type UploadTask struct {
	FileID       string            `json:"file_id"`
	FileName     string            `json:"filename"`
	RelativePath string            `json:"relative_path"`
	TotalChunks  int               `json:"total_chunks"`
	FileSize     int64             `json:"file_size"`
	FileMD5      string            `json:"file_md5"`
	Status       string            `json:"status"` // uploading, completed, failed, paused
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Chunks       map[int]ChunkInfo `json:"chunks"`
	RetryCount   int               `json:"retry_count"`
}

// ChunkInfo 分片信息
type ChunkInfo struct {
	Index     int       `json:"index"`
	Size      int64     `json:"size"`
	MD5       string    `json:"md5"`
	Status    string    `json:"status"` // pending, uploading, completed, failed
	UploadedAt time.Time `json:"uploaded_at"`
	RetryCount int       `json:"retry_count"`
}

// TaskStorage 任务存储管理器
type TaskStorage struct {
	storageDir string
	mutex      sync.RWMutex
	tasks      map[string]*UploadTask
}

var Storage *TaskStorage

// InitStorage 初始化存储管理器
func InitStorage() error {
	storageDir := filepath.Join(Config.UploadDir, ".metadata")
	if err := EnsureDirectory(storageDir); err != nil {
		return fmt.Errorf("创建元数据目录失败: %v", err)
	}

	Storage = &TaskStorage{
		storageDir: storageDir,
		tasks:      make(map[string]*UploadTask),
	}

	// 加载已存在的任务
	return Storage.loadTasks()
}

// SaveTask 保存任务信息
func (s *TaskStorage) SaveTask(task *UploadTask) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	task.UpdatedAt = time.Now()
	s.tasks[task.FileID] = task

	taskFile := filepath.Join(s.storageDir, fmt.Sprintf("%s.json", task.FileID))
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(taskFile, data, 0644)
}

// GetTask 获取任务信息
func (s *TaskStorage) GetTask(fileID string) (*UploadTask, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	task, exists := s.tasks[fileID]
	return task, exists
}

// UpdateChunk 更新分片状态
func (s *TaskStorage) UpdateChunk(fileID string, chunkIndex int, chunkInfo ChunkInfo) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	task, exists := s.tasks[fileID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", fileID)
	}

	if task.Chunks == nil {
		task.Chunks = make(map[int]ChunkInfo)
	}

	chunkInfo.UploadedAt = time.Now()
	task.Chunks[chunkIndex] = chunkInfo
	task.UpdatedAt = time.Now()

	// 检查是否所有分片都完成
	completedChunks := 0
	for _, chunk := range task.Chunks {
		if chunk.Status == "completed" {
			completedChunks++
		}
	}

	if completedChunks == task.TotalChunks {
		task.Status = "completed"
	}

	return s.saveTaskFile(task)
}

// GetUploadedChunks 获取已上传的分片列表
func (s *TaskStorage) GetUploadedChunks(fileID string) []int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	task, exists := s.tasks[fileID]
	if !exists {
		return []int{}
	}

	var uploaded []int
	for index, chunk := range task.Chunks {
		if chunk.Status == "completed" {
			uploaded = append(uploaded, index)
		}
	}

	return uploaded
}

// CleanupExpiredTasks 清理过期任务（超过7天的失败任务）
func (s *TaskStorage) CleanupExpiredTasks() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	expiredTime := time.Now().AddDate(0, 0, -7) // 7天前

	for fileID, task := range s.tasks {
		if (task.Status == "failed" || task.Status == "paused") && task.UpdatedAt.Before(expiredTime) {
			// 删除相关文件
			taskDir := filepath.Join(Config.UploadDir, fileID)
			os.RemoveAll(taskDir)

			// 删除元数据文件
			taskFile := filepath.Join(s.storageDir, fmt.Sprintf("%s.json", fileID))
			os.Remove(taskFile)

			delete(s.tasks, fileID)
		}
	}

	return nil
}

// loadTasks 加载所有已存在的任务
func (s *TaskStorage) loadTasks() error {
	files, err := os.ReadDir(s.storageDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			taskFile := filepath.Join(s.storageDir, file.Name())
			data, err := os.ReadFile(taskFile)
			if err != nil {
				continue
			}

			var task UploadTask
			if err := json.Unmarshal(data, &task); err != nil {
				continue
			}

			s.tasks[task.FileID] = &task
		}
	}

	return nil
}

// saveTaskFile 保存单个任务文件
func (s *TaskStorage) saveTaskFile(task *UploadTask) error {
	taskFile := filepath.Join(s.storageDir, fmt.Sprintf("%s.json", task.FileID))
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(taskFile, data, 0644)
}

// GetAllTasks 获取所有任务
func (s *TaskStorage) GetAllTasks() map[string]*UploadTask {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	tasks := make(map[string]*UploadTask)
	for k, v := range s.tasks {
		tasks[k] = v
	}
	return tasks
}

// DeleteTask 删除任务
func (s *TaskStorage) DeleteTask(fileID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 删除相关文件
	taskDir := filepath.Join(Config.UploadDir, fileID)
	os.RemoveAll(taskDir)

	// 删除元数据文件
	taskFile := filepath.Join(s.storageDir, fmt.Sprintf("%s.json", fileID))
	os.Remove(taskFile)

	delete(s.tasks, fileID)
	return nil
} 