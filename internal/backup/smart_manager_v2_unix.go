//go:build !windows

package backup

import (
	"syscall"

	"nas-os/pkg/safeguards"
)

// getFreeSpace 获取路径的可用空间（Unix/Linux/Darwin）
func getFreeSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}

	// 使用安全乘法避免溢出
	totalBytes, err := safeguards.SafeMulUint64(stat.Bavail, uint64(stat.Bsize))
	if err != nil {
		// 乘法溢出时返回最大可用空间
		return 0, err
	}

	// 安全转换为 int64
	result, err := safeguards.SafeUint64ToInt64(totalBytes)
	if err != nil {
		// 转换溢出时返回 MaxInt64 作为安全上限
		return 0, err
	}

	return result, nil
}
