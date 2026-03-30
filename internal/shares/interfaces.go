// Package shares 提供共享管理接口定义
package shares

import (
	"nas-os/internal/nfs"
	"nas-os/internal/smb"
)

// SMBManager 定义 SMB 管理器接口.
type SMBManager interface {
	ListShares() ([]*smb.Share, error)
	GetShare(name string) (*smb.Share, error)
	CreateShare(share *smb.Share) error
	CreateShareFromInput(input smb.ShareInput) (*smb.Share, error)
	UpdateShare(name string, share *smb.Share) error
	UpdateShareFromInput(name string, input smb.ShareInput) (*smb.Share, error)
	DeleteShare(name string) error
	Reload() error
	Status() (*smb.ServiceStatus, error)
	Connections() ([]*smb.Connection, error)
	GetStatus() (bool, error)
	Start() error
	Stop() error
	Restart() error
	TestConfig() (bool, string, error)
	ApplyConfig() error
	GetConfig() *smb.Config
	UpdateConfig(config *smb.Config) error
	SetSharePermission(shareName, username string, readWrite bool) error
	RemoveSharePermission(shareName, username string) error
	GetUserShares(username string) []*smb.Share
	CloseShare(name string) error
	OpenShare(name string) error
	GetSharePath(name string) string
}

// NFSManager 定义 NFS 管理器接口.
type NFSManager interface {
	ListExports() ([]*nfs.Export, error)
	GetExport(path string) (*nfs.Export, error)
	CreateExport(export *nfs.Export) error
	UpdateExport(path string, export *nfs.Export) error
	DeleteExport(path string) error
	Reload() error
	Status() (*nfs.ServiceStatus, error)
	Start() error
	Stop() error
	Restart() error
	GetConfig() *nfs.Config
	UpdateConfig(config *nfs.Config) error
	GetClients() ([]map[string]string, error)
	ValidateExport(export *nfs.Export) error
}
