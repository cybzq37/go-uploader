package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig 存储应用程序配置
type AppConfig struct {
	UploadDir string `json:"upload_dir"` // 上传临时目录
	MergedDir string `json:"merged_dir"` // 合并后文件存储目录
	Port      string `json:"port"`       // 服务器监听端口
}

// Config 全局配置实例
var Config = AppConfig{
	UploadDir: "./upload",
	MergedDir: "./merged",
	Port:      "9876",
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