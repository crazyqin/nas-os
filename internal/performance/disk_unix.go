//go:build !windows

package performance

import (
	"syscall"
)

// getDiskUsageStat 获取磁盘使用统计（Unix/Linux/Darwin）
func getDiskUsageStat(path string) (total, free, used uint64, inodeTotal, inodeUsed uint64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, 0, 0, err
	}

	total = stat.Blocks * uint64(stat.Bsize)
	free = stat.Bavail * uint64(stat.Bsize)
	used = total - free
	inodeTotal = stat.Files
	inodeUsed = inodeTotal - stat.Ffree

	return total, free, used, inodeTotal, inodeUsed, nil
}
