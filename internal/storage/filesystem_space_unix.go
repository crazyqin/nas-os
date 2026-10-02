//go:build !windows

package storage

import "syscall"

func filesystemSpace(path string) (total, available int64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	return int64(stat.Blocks) * int64(stat.Bsize), int64(stat.Bavail) * int64(stat.Bsize), nil //nolint:unconvert // Statfs_t.Bsize is int64 on Linux and uint32 on Darwin.
}
