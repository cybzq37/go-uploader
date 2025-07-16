package utils

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries    int           // 最大重试次数
	InitialDelay  time.Duration // 初始延迟
	MaxDelay      time.Duration // 最大延迟
	BackoffFactor float64       // 退避因子
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxRetries:    3,
	InitialDelay:  1 * time.Second,
	MaxDelay:      30 * time.Second,
	BackoffFactor: 2.0,
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// 网络相关错误
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network is unreachable",
		"temporary failure",
		"server error",
		"service unavailable",
		"timeout",
		"deadline exceeded",
		"i/o timeout",
		"broken pipe",
		"no route to host",
		"operation timed out",
	}
	
	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}
	
	return false
}

// RetryWithBackoff 带退避的重试机制
func RetryWithBackoff(ctx context.Context, operation func() error, config RetryConfig) error {
	var lastErr error
	
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// 执行操作
		if err := operation(); err != nil {
			lastErr = err
			
			// 检查是否可重试
			if !IsRetryableError(err) {
				return fmt.Errorf("不可重试的错误: %v", err)
			}
			
			// 如果是最后一次尝试，直接返回错误
			if attempt == config.MaxRetries {
				break
			}
			
			// 计算延迟时间
			delay := calculateDelay(attempt, config)
			log.Printf("操作失败，第 %d 次重试，%v 后重试: %v", attempt+1, delay, err)
			
			// 等待或检查取消
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		} else {
			// 成功
			if attempt > 0 {
				log.Printf("操作在第 %d 次重试后成功", attempt+1)
			}
			return nil
		}
	}
	
	return fmt.Errorf("操作在 %d 次重试后仍然失败: %v", config.MaxRetries, lastErr)
}

// calculateDelay 计算延迟时间（指数退避）
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	delay := config.InitialDelay
	
	// 指数退避
	if attempt > 0 {
		multiplier := math.Pow(config.BackoffFactor, float64(attempt))
		delay = time.Duration(float64(config.InitialDelay) * multiplier)
	}
	
	// 限制最大延迟
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}
	
	return delay
}

// contains 检查字符串是否包含子字符串（忽略大小写）
func contains(str, substr string) bool {
	return len(str) >= len(substr) && 
		   (str == substr || len(str) > len(substr) && 
		   containsIgnoreCase(str, substr))
}

// containsIgnoreCase 忽略大小写的包含检查
func containsIgnoreCase(str, substr string) bool {
	str = toLower(str)
	substr = toLower(substr)
	
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// toLower 简单的小写转换
func toLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	failures     int
	lastFailTime time.Time
	state        string // "closed", "open", "half-open"
}

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        "closed",
	}
}

// Execute 执行操作（带熔断保护）
func (cb *CircuitBreaker) Execute(operation func() error) error {
	// 检查熔断器状态
	if cb.state == "open" {
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = "half-open"
		} else {
			return fmt.Errorf("熔断器开启，拒绝执行")
		}
	}
	
	// 执行操作
	err := operation()
	
	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()
		
		if cb.failures >= cb.maxFailures {
			cb.state = "open"
		}
		
		return err
	}
	
	// 成功时重置
	cb.failures = 0
	cb.state = "closed"
	return nil
} 