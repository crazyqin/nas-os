//go:build linux

package storageanalytics

import (
	"os"
	"syscall"
	"time"
)

// StatInfo 访问时间信息.
type StatInfo struct {
	Atime time.Time
}

// getStat 从 FileInfo 中提取 Atime（Linux）.
func getStat(info os.FileInfo) *StatInfo {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return &StatInfo{
		Atime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
	}
}
