//go:build windows

package smartdedup

import (
	"os"
)

// getInode 获取文件的 inode 号（Windows 不支持，返回 0）。
func getInode(info os.FileInfo) uint64 {
	return 0
}

// getNlink 获取文件的硬链接数（Windows 不支持，返回 1）。
func getNlink(info os.FileInfo) uint64 {
	return 1
}
