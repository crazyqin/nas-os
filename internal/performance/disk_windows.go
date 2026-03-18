//go:build windows

package performance

import (
	"errors"
)

// getDiskUsageStat 获取磁盘使用统计（Windows stub）
func getDiskUsageStat(path string) (total, free, used uint64, inodeTotal, inodeUsed uint64, err error) {
	return 0, 0, 0, 0, 0, errors.New("disk stats not implemented on Windows")
}