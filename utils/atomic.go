package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// AtomicWriter 原子写入器
type AtomicWriter struct {
	targetPath string
	tempPath   string
	file       *os.File
	hash       io.Writer
	size       int64
}

// NewAtomicWriter 创建原子写入器
func NewAtomicWriter(targetPath string) (*AtomicWriter, error) {
	// 创建临时文件路径
	tempPath := targetPath + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
	
	// 确保目标目录存在
	if err := EnsureDirectory(filepath.Dir(targetPath)); err != nil {
		return nil, fmt.Errorf("创建目标目录失败: %v", err)
	}
	
	// 创建临时文件
	file, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %v", err)
	}
	
	hasher := md5.New()
	
	return &AtomicWriter{
		targetPath: targetPath,
		tempPath:   tempPath,
		file:       file,
		hash:       hasher,
	}, nil
}

// Write 写入数据
func (aw *AtomicWriter) Write(data []byte) (int, error) {
	n, err := aw.file.Write(data)
	if err != nil {
		return n, err
	}
	
	// 更新哈希和大小
	aw.hash.Write(data[:n])
	aw.size += int64(n)
	
	return n, nil
}

// Commit 提交更改（原子操作）
func (aw *AtomicWriter) Commit() error {
	// 确保数据写入磁盘
	if err := aw.file.Sync(); err != nil {
		aw.file.Close()
		os.Remove(aw.tempPath)
		return fmt.Errorf("同步文件失败: %v", err)
	}
	
	// 关闭文件
	if err := aw.file.Close(); err != nil {
		os.Remove(aw.tempPath)
		return fmt.Errorf("关闭文件失败: %v", err)
	}
	
	// 原子性重命名
	if err := os.Rename(aw.tempPath, aw.targetPath); err != nil {
		os.Remove(aw.tempPath)
		return fmt.Errorf("原子重命名失败: %v", err)
	}
	
	return nil
}

// Rollback 回滚更改
func (aw *AtomicWriter) Rollback() error {
	if aw.file != nil {
		aw.file.Close()
	}
	return os.Remove(aw.tempPath)
}

// GetMD5 获取当前内容的MD5
func (aw *AtomicWriter) GetMD5() string {
	if hasher, ok := aw.hash.(interface{ Sum([]byte) []byte }); ok {
		return hex.EncodeToString(hasher.Sum(nil))
	}
	return ""
}

// GetSize 获取当前大小
func (aw *AtomicWriter) GetSize() int64 {
	return aw.size
}

// SafeFileOperation 安全的文件操作包装器
type SafeFileOperation struct {
	backupPath string
	targetPath string
}

// NewSafeFileOperation 创建安全文件操作
func NewSafeFileOperation(targetPath string) *SafeFileOperation {
	return &SafeFileOperation{
		targetPath: targetPath,
		backupPath: targetPath + ".backup." + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}

// Execute 执行安全的文件操作
func (sfo *SafeFileOperation) Execute(operation func(string) error) error {
	// 如果目标文件存在，先备份
	if _, err := os.Stat(sfo.targetPath); err == nil {
		if err := copyFile(sfo.targetPath, sfo.backupPath); err != nil {
			return fmt.Errorf("创建备份失败: %v", err)
		}
		
		// 确保在操作失败时能恢复
		defer func() {
			if r := recover(); r != nil {
				sfo.Restore()
				panic(r)
			}
		}()
	}
	
	// 执行操作
	if err := operation(sfo.targetPath); err != nil {
		// 操作失败，恢复备份
		if restoreErr := sfo.Restore(); restoreErr != nil {
			return fmt.Errorf("操作失败且恢复失败: 原错误=%v, 恢复错误=%v", err, restoreErr)
		}
		return err
	}
	
	// 操作成功，清理备份
	sfo.CleanupBackup()
	return nil
}

// Restore 恢复备份
func (sfo *SafeFileOperation) Restore() error {
	if _, err := os.Stat(sfo.backupPath); os.IsNotExist(err) {
		return nil // 没有备份文件
	}
	
	return os.Rename(sfo.backupPath, sfo.targetPath)
}

// CleanupBackup 清理备份文件
func (sfo *SafeFileOperation) CleanupBackup() {
	os.Remove(sfo.backupPath)
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}
	
	return destFile.Sync()
}

// VerifyFileIntegrity 验证文件完整性
func VerifyFileIntegrity(filePath string, expectedMD5 string, expectedSize int64) error {
	// 检查文件是否存在
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("文件不存在: %v", err)
	}
	
	// 验证文件大小
	if expectedSize > 0 && fileInfo.Size() != expectedSize {
		return fmt.Errorf("文件大小不匹配: 期望=%d, 实际=%d", expectedSize, fileInfo.Size())
	}
	
	// 验证MD5（如果提供）
	if expectedMD5 != "" {
		actualMD5, err := FileMD5(filePath)
		if err != nil {
			return fmt.Errorf("计算文件MD5失败: %v", err)
		}
		
		if actualMD5 != expectedMD5 {
			return fmt.Errorf("文件MD5不匹配: 期望=%s, 实际=%s", expectedMD5, actualMD5)
		}
	}
	
	return nil
}

// LockFile 文件锁
type LockFile struct {
	path     string
	file     *os.File
	acquired bool
}

// NewLockFile 创建文件锁
func NewLockFile(lockPath string) *LockFile {
	return &LockFile{
		path: lockPath,
	}
}

// Acquire 获取锁
func (lf *LockFile) Acquire() error {
	if lf.acquired {
		return fmt.Errorf("锁已被获取")
	}
	
	// 尝试创建锁文件
	file, err := os.OpenFile(lf.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("锁文件已存在，可能有其他进程正在操作")
		}
		return fmt.Errorf("创建锁文件失败: %v", err)
	}
	
	// 写入进程信息
	fmt.Fprintf(file, "PID: %d\nTime: %s\n", os.Getpid(), time.Now().Format(time.RFC3339))
	
	lf.file = file
	lf.acquired = true
	return nil
}

// Release 释放锁
func (lf *LockFile) Release() error {
	if !lf.acquired {
		return nil
	}
	
	if lf.file != nil {
		lf.file.Close()
	}
	
	err := os.Remove(lf.path)
	lf.acquired = false
	lf.file = nil
	
	return err
}

// IsLocked 检查是否已锁定
func (lf *LockFile) IsLocked() bool {
	_, err := os.Stat(lf.path)
	return err == nil
} 