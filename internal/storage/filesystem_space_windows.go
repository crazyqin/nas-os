package storage

import "golang.org/x/sys/windows"

func filesystemSpace(path string) (total, available int64, err error) {
	directory, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var totalBytes, availableBytes uint64
	if err = windows.GetDiskFreeSpaceEx(directory, &availableBytes, &totalBytes, nil); err != nil {
		return 0, 0, err
	}
	return int64(totalBytes), int64(availableBytes), nil
}
