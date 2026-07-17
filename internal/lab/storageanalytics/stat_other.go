//go:build !linux

package storageanalytics

import (
	"os"
	"time"
)

// StatInfo 访问时间信息.
type StatInfo struct {
	Atime time.Time
}

// getStat 从 FileInfo 中提取 Atime（非Linux平台回退）.
func getStat(info os.FileInfo) *StatInfo {
	// 非Linux平台使用修改时间作为近似
	return &StatInfo{
		Atime: info.ModTime(),
	}
}
