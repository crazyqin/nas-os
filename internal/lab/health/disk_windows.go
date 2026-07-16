//go:build windows

package health

import (
	"errors"
)

// getDiskStats 获取磁盘统计信息（Windows stub）
func getDiskStats(path string) (totalBytes, freeBytes uint64, err error) {
	return 0, 0, errors.New("disk stats not implemented on Windows")
}
