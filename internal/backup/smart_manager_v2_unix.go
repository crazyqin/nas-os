//go:build !windows

package backup

import (
	"syscall"
)

// getFreeSpace 获取路径的可用空间（Unix/Linux/Darwin）
func getFreeSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail * uint64(stat.Bsize)), nil
}