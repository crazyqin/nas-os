//go:build !windows

package health

import (
	"syscall"
)

// getDiskStats 获取磁盘统计信息（Unix/Linux/Darwin）.
func getDiskStats(path string) (totalBytes, freeBytes uint64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	return stat.Blocks * uint64(stat.Bsize), stat.Bfree * uint64(stat.Bsize), nil
}
