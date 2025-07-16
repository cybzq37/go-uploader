package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig 存储应用程序配置
type AppConfig struct {
	UploadDir              string `json:"upload_dir"`               // 上传临时目录
	MergedDir              string `json:"merged_dir"`               // 合并后文件存储目录
	Port                   string `json:"port"`                     // 服务器监听端口
	MaxFileSize            int64  `json:"max_file_size"`            // 最大文件大小（字节）
	MaxChunkSize           int64  `json:"max_chunk_size"`           // 最大分片大小（字节）
	CleanupInterval        int64  `json:"cleanup_interval"`         // 清理间隔（秒）
	RetryMaxAttempts       int    `json:"retry_max_attempts"`       // 最大重试次数
	RetryInitialDelay      int64  `json:"retry_initial_delay"`      // 初始重试延迟（毫秒）
	ConcurrentUploads      int    `json:"concurrent_uploads"`       // 并发上传数
	EnableIntegrityCheck   bool   `json:"enable_integrity_check"`   // 启用完整性检查
	EnableAtomicOperations bool   `json:"enable_atomic_operations"` // 启用原子操作
	LogLevel               string `json:"log_level"`                // 日志级别
}

// Config 全局配置实例
var Config = AppConfig{
	UploadDir:              "./upload",
	MergedDir:              "./merged",
	Port:                   "9876",
	MaxFileSize:            10 * 1024 * 1024 * 1024, // 10GB
	MaxChunkSize:           100 * 1024 * 1024,       // 100MB
	CleanupInterval:        3600,                    // 1小时
	RetryMaxAttempts:       3,
	RetryInitialDelay:      1000, // 1秒
	ConcurrentUploads:      5,
	EnableIntegrityCheck:   true,
	EnableAtomicOperations: true,
	LogLevel:               "info",
}

// LoadConfig 从配置文件加载配置
func LoadConfig(configPath string) error {
	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 配置文件不存在，创建默认配置文件
		defaultConfig, err := json.MarshalIndent(Config, "", "  ")
		if err != nil {
			return err
		}
		
		// 确保配置文件目录存在
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, os.ModePerm); err != nil {
			return err
		}
		
		// 写入默认配置
		if err := os.WriteFile(configPath, defaultConfig, 0644); err != nil {
			return err
		}
	} else {
		// 配置文件存在，读取配置
		configData, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		
		// 解析配置
		if err := json.Unmarshal(configData, &Config); err != nil {
			return err
		}
	}
	
	return nil
}

// InitDirectories 初始化所有配置的目录
func InitDirectories() error {
	dirs := []string{
		Config.UploadDir,
		Config.MergedDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

// EnsureDirectory 确保指定目录存在
func EnsureDirectory(path string) error {
	return os.MkdirAll(path, os.ModePerm)
} 