//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package smartdedup

import (
	"os"
	"syscall"
)

// getInode 获取文件的 inode 号。
func getInode(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino
	}
	return 0
}

// getNlink 获取文件的硬链接数。
func getNlink(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Nlink)
	}
	return 1
}
