package utils

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SanitizeFileID 将包含路径的fileID转换为安全的文件名
// 使用MD5哈希确保唯一性，避免文件名冲突
func SanitizeFileID(fileID string) string {
	// 使用MD5哈希生成唯一标识符
	hasher := md5.New()
	hasher.Write([]byte(fileID))
	hash := hex.EncodeToString(hasher.Sum(nil))
	
	// 保留原始文件名的可读部分（但去除路径分隔符）
	readablePart := strings.ReplaceAll(fileID, "/", "_")
	readablePart = strings.ReplaceAll(readablePart, "\\", "_")
	readablePart = strings.ReplaceAll(readablePart, "..", "_")
	
	// 限制可读部分长度，避免文件名过长
	if len(readablePart) > 50 {
		readablePart = readablePart[:50]
	}
	
	// 组合可读部分和哈希值，确保唯一性
	return fmt.Sprintf("%s_%s", readablePart, hash[:8])
}

// sanitizeFileID 内部使用的版本，保持向后兼容
func sanitizeFileID(fileID string) string {
	return SanitizeFileID(fileID)
}

// UploadTask 上传任务结构 - 支持文件夹和单文件任务
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
	
	// 新增字段 - 支持文件夹任务
	TaskType     string            `json:"task_type"`      // "file" 或 "folder"
	ParentTaskID string            `json:"parent_task_id"` // 父任务ID（用于子文件）
	FolderName   string            `json:"folder_name"`    // 文件夹名称
	SubTasks     []string          `json:"sub_tasks"`      // 子任务ID列表（文件夹任务使用）
	IsSubTask    bool              `json:"is_sub_task"`    // 是否为子任务
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

// FolderTaskSummary 文件夹任务摘要信息
type FolderTaskSummary struct {
	TotalFiles      int     `json:"total_files"`
	CompletedFiles  int     `json:"completed_files"`
	FailedFiles     int     `json:"failed_files"`
	TotalSize       int64   `json:"total_size"`
	UploadedSize    int64   `json:"uploaded_size"`
	CompletionRate  float64 `json:"completion_rate"`
	Status          string  `json:"status"` // uploading, completed, failed, paused
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

// CreateFolderTask 创建文件夹任务
func (s *TaskStorage) CreateFolderTask(folderName string, files []FileInfo) (*UploadTask, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 创建文件夹任务ID
	folderTaskID := fmt.Sprintf("folder_%s_%d", folderName, time.Now().UnixNano())
	
	// 计算总大小
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	// 创建主文件夹任务
	folderTask := &UploadTask{
		FileID:       folderTaskID,
		FileName:     folderName,
		FolderName:   folderName,
		TaskType:     "folder",
		Status:       "uploading",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		FileSize:     totalSize,
		SubTasks:     make([]string, 0, len(files)),
		IsSubTask:    false,
	}

	// 创建子文件任务
	for _, file := range files {
		subTaskID := fmt.Sprintf("%s_%s_%d", folderTaskID, file.RelativePath, time.Now().UnixNano())
		
		subTask := &UploadTask{
			FileID:       subTaskID,
			FileName:     file.Name,
			RelativePath: file.RelativePath,
			TotalChunks:  file.TotalChunks,
			FileSize:     file.Size,
			TaskType:     "file",
			Status:       "pending",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Chunks:       make(map[int]ChunkInfo),
			ParentTaskID: folderTaskID,
			IsSubTask:    true,
		}

		// 保存子任务
		s.tasks[subTaskID] = subTask
		folderTask.SubTasks = append(folderTask.SubTasks, subTaskID)
		
		// 保存子任务到磁盘
		if err := s.saveTaskFile(subTask); err != nil {
			return nil, fmt.Errorf("保存子任务失败: %v", err)
		}
	}

	// 保存主任务
	s.tasks[folderTaskID] = folderTask
	if err := s.saveTaskFile(folderTask); err != nil {
		return nil, fmt.Errorf("保存文件夹任务失败: %v", err)
	}

	return folderTask, nil
}

// FileInfo 文件信息结构
type FileInfo struct {
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	Size         int64  `json:"size"`
	TotalChunks  int    `json:"total_chunks"`
}

// GetFolderTaskSummary 获取文件夹任务摘要
func (s *TaskStorage) GetFolderTaskSummary(folderTaskID string) (*FolderTaskSummary, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	folderTask, exists := s.tasks[folderTaskID]
	if !exists || folderTask.TaskType != "folder" {
		return nil, fmt.Errorf("文件夹任务不存在")
	}

	summary := &FolderTaskSummary{
		TotalFiles: len(folderTask.SubTasks),
		TotalSize:  folderTask.FileSize,
	}

	// 统计子任务状态
	for _, subTaskID := range folderTask.SubTasks {
		subTask, exists := s.tasks[subTaskID]
		if !exists {
			continue
		}

		switch subTask.Status {
		case "completed":
			summary.CompletedFiles++
			summary.UploadedSize += subTask.FileSize
		case "failed":
			summary.FailedFiles++
		default:
			// 计算部分上传的大小
			uploadedChunks := s.getUploadedChunksInternal(subTaskID)
			if len(uploadedChunks) > 0 && subTask.TotalChunks > 0 {
				chunkSize := subTask.FileSize / int64(subTask.TotalChunks)
				summary.UploadedSize += int64(len(uploadedChunks)) * chunkSize
			}
		}
	}

	// 计算完成率
	if summary.TotalSize > 0 {
		summary.CompletionRate = float64(summary.UploadedSize) / float64(summary.TotalSize) * 100
	}

	// 确定文件夹任务状态
	if summary.CompletedFiles == summary.TotalFiles {
		summary.Status = "completed"
		folderTask.Status = "completed"
		s.saveTaskFile(folderTask)
	} else if summary.FailedFiles > 0 && summary.CompletedFiles+summary.FailedFiles == summary.TotalFiles {
		summary.Status = "failed"
	} else {
		summary.Status = "uploading"
	}

	return summary, nil
}

// GetSubTasks 获取文件夹的所有子任务
func (s *TaskStorage) GetSubTasks(folderTaskID string) ([]*UploadTask, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	folderTask, exists := s.tasks[folderTaskID]
	if !exists || folderTask.TaskType != "folder" {
		return nil, fmt.Errorf("文件夹任务不存在")
	}

	subTasks := make([]*UploadTask, 0, len(folderTask.SubTasks))
	for _, subTaskID := range folderTask.SubTasks {
		if subTask, exists := s.tasks[subTaskID]; exists {
			subTasks = append(subTasks, subTask)
		}
	}

	return subTasks, nil
}

// SaveTask 保存任务信息
func (s *TaskStorage) SaveTask(task *UploadTask) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	task.UpdatedAt = time.Now()
	s.tasks[task.FileID] = task

	return s.saveTaskFile(task)
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
		
		// 如果是子任务，检查父任务是否完成
		if task.IsSubTask && task.ParentTaskID != "" {
			s.checkAndUpdateParentTask(task.ParentTaskID)
		}
	}

	return s.saveTaskFile(task)
}

// checkAndUpdateParentTask 检查并更新父任务状态
func (s *TaskStorage) checkAndUpdateParentTask(parentTaskID string) {
	parentTask, exists := s.tasks[parentTaskID]
	if !exists || parentTask.TaskType != "folder" {
		return
	}

	allCompleted := true
	anyFailed := false
	
	for _, subTaskID := range parentTask.SubTasks {
		subTask, exists := s.tasks[subTaskID]
		if !exists {
			continue
		}
		
		if subTask.Status != "completed" {
			allCompleted = false
		}
		if subTask.Status == "failed" {
			anyFailed = true
		}
	}

	if allCompleted {
		parentTask.Status = "completed"
	} else if anyFailed {
		parentTask.Status = "uploading" // 保持上传状态，允许重试
	}

	parentTask.UpdatedAt = time.Now()
	s.saveTaskFile(parentTask)
}

// GetUploadedChunks 获取已上传的分片列表
func (s *TaskStorage) GetUploadedChunks(fileID string) []int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.getUploadedChunksInternal(fileID)
}

// getUploadedChunksInternal 内部方法，不加锁
func (s *TaskStorage) getUploadedChunksInternal(fileID string) []int {
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
			// 删除相关文件 - 使用安全的文件ID作为目录名
			safeFileID := sanitizeFileID(fileID)
			taskDir := filepath.Join(Config.UploadDir, safeFileID)
			os.RemoveAll(taskDir)

			// 删除锁文件
			lockPath := filepath.Join(Config.UploadDir, safeFileID+".lock")
			os.Remove(lockPath)
			mergeLockPath := filepath.Join(Config.UploadDir, safeFileID+".merge.lock")
			os.Remove(mergeLockPath)

			// 删除元数据文件
			taskFile := filepath.Join(s.storageDir, fmt.Sprintf("%s.json", safeFileID))
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

			// 向后兼容：为旧任务设置默认值
			if task.TaskType == "" {
				task.TaskType = "file"
			}
			if task.Chunks == nil {
				task.Chunks = make(map[int]ChunkInfo)
			}
			if task.SubTasks == nil {
				task.SubTasks = make([]string, 0)
			}

			s.tasks[task.FileID] = &task
		}
	}

	return nil
}

// saveTaskFile 保存单个任务文件
func (s *TaskStorage) saveTaskFile(task *UploadTask) error {
	// 使用安全的文件名
	safeFileID := sanitizeFileID(task.FileID)
	taskFile := filepath.Join(s.storageDir, fmt.Sprintf("%s.json", safeFileID))
	
	// 确保目标目录存在（处理嵌套目录）
	if err := EnsureDirectory(filepath.Dir(taskFile)); err != nil {
		return fmt.Errorf("创建任务文件目录失败: %v", err)
	}
	
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

// GetMainTasks 获取主任务（非子任务）
func (s *TaskStorage) GetMainTasks() map[string]*UploadTask {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	mainTasks := make(map[string]*UploadTask)
	for k, v := range s.tasks {
		if !v.IsSubTask {
			mainTasks[k] = v
		}
	}
	return mainTasks
}

// DeleteTask 删除任务
func (s *TaskStorage) DeleteTask(fileID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	task, exists := s.tasks[fileID]
	if !exists {
		return fmt.Errorf("任务不存在")
	}

	// 如果是文件夹任务，删除所有子任务
	if task.TaskType == "folder" {
		for _, subTaskID := range task.SubTasks {
			s.deleteTaskInternal(subTaskID)
		}
	}

	return s.deleteTaskInternal(fileID)
}

// deleteTaskInternal 内部删除任务方法
func (s *TaskStorage) deleteTaskInternal(fileID string) error {
	// 删除相关文件 - 使用安全的文件ID作为目录名
	safeFileID := sanitizeFileID(fileID)
	taskDir := filepath.Join(Config.UploadDir, safeFileID)
	os.RemoveAll(taskDir)

	// 删除锁文件
	lockPath := filepath.Join(Config.UploadDir, safeFileID+".lock")
	os.Remove(lockPath)
	mergeLockPath := filepath.Join(Config.UploadDir, safeFileID+".merge.lock")
	os.Remove(mergeLockPath)

	// 删除元数据文件
	taskFile := filepath.Join(s.storageDir, fmt.Sprintf("%s.json", safeFileID))
	os.Remove(taskFile)

	delete(s.tasks, fileID)
	return nil
} 