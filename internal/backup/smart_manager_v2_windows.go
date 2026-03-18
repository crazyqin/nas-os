//go:build windows

package backup

import (
	"errors"
)

// getFreeSpace 获取路径的可用空间（Windows stub）
// Windows 上的磁盘空间检查需要使用不同的 API
func getFreeSpace(path string) (int64, error) {
	return 0, errors.New("disk space check not implemented on Windows")
}