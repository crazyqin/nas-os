//go:build windows

package storage_efficiency

// syscallStat 平台相关的文件状态信息 (Windows).
type syscallStat struct {
	uid int
}

// getFileInfo 获取文件的系统级信息 (Windows 不支持).
func getFileInfo(sysInfo interface{}) *syscallStat {
	return nil
}
