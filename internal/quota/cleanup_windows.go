//go:build windows

package quota

import (
	"time"
)

// getFileAccessTime 获取文件访问时间 (Windows - 不支持)
func getFileAccessTime(info interface{}) (time.Time, bool) {
	// Windows 不支持访问时间，返回 false
	return time.Time{}, false
}
