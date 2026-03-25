//go:build !windows

package quota

import (
	"time"

	"golang.org/x/sys/unix"
)

// getFileAccessTime 获取文件访问时间 (Unix)
func getFileAccessTime(info interface{}) (time.Time, bool) {
	if stat, ok := info.(*unix.Stat_t); ok {
		return time.Unix(stat.Atim.Sec, stat.Atim.Nsec), true
	}
	return time.Time{}, false
}
