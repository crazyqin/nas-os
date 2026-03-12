// Package quota 提供存储配额管理功能
package quota

import (
	"nas-os/internal/storage"
	"nas-os/internal/users"
)

// StorageAdapter 存储适配器
type StorageAdapter struct {
	mgr *storage.Manager
}

// NewStorageAdapter 创建存储适配器
func NewStorageAdapter(mgr *storage.Manager) *StorageAdapter {
	return &StorageAdapter{mgr: mgr}
}

// GetVolume 获取卷信息
func (a *StorageAdapter) GetVolume(name string) *VolumeInfo {
	if a.mgr == nil {
		return nil
	}
	vol := a.mgr.GetVolume(name)
	if vol == nil {
		return nil
	}
	return &VolumeInfo{
		Name:       vol.Name,
		MountPoint: vol.MountPoint,
		Size:       vol.Size,
		Used:       vol.Used,
		Free:       vol.Free,
	}
}

// GetUsage 获取卷使用情况
func (a *StorageAdapter) GetUsage(volumeName string) (total, used, free uint64, err error) {
	return a.mgr.GetUsage(volumeName)
}

// UserAdapter 用户适配器
type UserAdapter struct {
	mgr *users.Manager
}

// NewUserAdapter 创建用户适配器
func NewUserAdapter(mgr *users.Manager) *UserAdapter {
	return &UserAdapter{mgr: mgr}
}

// UserExists 检查用户是否存在
func (a *UserAdapter) UserExists(username string) bool {
	if a.mgr == nil {
		return false
	}
	_, err := a.mgr.GetUser(username)
	return err == nil
}

// GroupExists 检查用户组是否存在
func (a *UserAdapter) GroupExists(groupName string) bool {
	if a.mgr == nil {
		return false
	}
	_, err := a.mgr.GetGroup(groupName)
	return err == nil
}

// GetUserHomeDir 获取用户主目录
func (a *UserAdapter) GetUserHomeDir(username string) string {
	if a.mgr == nil {
		return ""
	}
	user, err := a.mgr.GetUser(username)
	if err != nil {
		return ""
	}
	return user.HomeDir
}
