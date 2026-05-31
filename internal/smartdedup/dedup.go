// Package smartdedup 提供内容感知的智能文件去重功能
//
// dedup.go 包含去重引擎的补充工具函数。
// 核心扫描和去重逻辑在 manager.go 中实现。
package smartdedup

import (
	"fmt"
	"os"
)

// FileExists 检查文件是否存在.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FormatSize 格式化文件大小为人类可读格式.
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
