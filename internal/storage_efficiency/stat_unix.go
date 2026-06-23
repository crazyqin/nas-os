//go:build !windows

package storage_efficiency

import "syscall"

// syscallStat 平台相关的文件状态信息.
type syscallStat struct {
	uid int
}

// getFileInfo 获取文件的系统级信息.
func getFileInfo(sysInfo interface{}) *syscallStat {
	if stat, ok := sysInfo.(*syscall.Stat_t); ok {
		return &syscallStat{uid: int(stat.Uid)}
	}
	return nil
}
