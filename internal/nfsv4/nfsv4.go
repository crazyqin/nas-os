// Package nfsv4 NFSv4.2 协议支持
// 对标TrueNAS NFS v4.2支持
package nfsv4

import (
	"errors"
	"sync"
	"time"
)

// NFSVersion NFS版本
type NFSVersion string

const (
	NFSVersion40 NFSVersion = "4.0"
	NFSVersion41 NFSVersion = "4.1"
	NFSVersion42 NFSVersion = "4.2"
)

// ExportState 导出状态
type ExportState string

const (
	ExportStateActive   ExportState = "active"
	ExportStateInactive ExportState = "inactive"
	ExportStateError    ExportState = "error"
)

// SecurityType 安全类型
type SecurityType string

const (
	SecuritySys    SecurityType = "sys"
	SecurityKrb5   SecurityType = "krb5"
	SecurityKrb5i  SecurityType = "krb5i"
	SecurityKrb5p  SecurityType = "krb5p"
)

// NFSExport NFS导出配置
type NFSExport struct {
	ID              string            `json:"id"`
	Path            string            `json:"path"`
	Alias           string            `json:"alias,omitempty"`
	AllowedHosts    []string          `json:"allowed_hosts"`
	DeniedHosts     []string          `json:"denied_hosts"`
	Options         ExportOptions     `json:"options"`
	Security        []SecurityType    `json:"security"`
	State           ExportState       `json:"state"`
	NFSVersion      NFSVersion        `json:"nfs_version"`
	Protocol        []string          `json:"protocol"` // tcp, udp
	Squash          SquashType        `json:"squash"`
	AnonymousUID    int               `json:"anonymous_uid"`
	AnonymousGID    int               `json:"anonymous_gid"`
	ReadOnly        bool              `json:"read_only"`
	Enabled         bool              `json:"enabled"`
	Stats           *ExportStats      `json:"stats,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// ExportOptions 导出选项
type ExportOptions struct {
	// NFSv4.2 特性
	SupportsLayouts      bool `json:"supports_layouts"`       // pNFS layout support
	SupportsCopy         bool `json:"supports_copy"`          // Server-side copy
	SupportsClone        bool `json:"supports_clone"`         // File cloning
	SupportsSeek         bool `json:"supports_seek"`          // SEEK_HOLE/SEEK_DATA
	SupportsAllocate     bool `json:"supports_allocate"`      // fallocate
	SupportsDeallocate   bool `json:"supports_deallocate"`    // ftruncate/punch hole
	SupportsXattr        bool `json:"supports_xattr"`         // Extended attributes
	SupportsACL          bool `json:"supports_acl"`           // NFSv4 ACLs
	SupportsFlock        bool `json:"supports_flock"`         // BSD file locks
	SupportsPNFS         bool `json:"supports_pnfs"`          // Parallel NFS
	
	// 性能选项
	ReadAhead            int  `json:"read_ahead,omitempty"`   // KB
	WriteBehind          bool `json:"write_behind"`
	AsyncWrites          bool `json:"async_writes"`
	CacheTimeout         int  `json:"cache_timeout,omitempty"` // 秒
}

// SquashType squash类型
type SquashType string

const (
	SquashAll    SquashType = "all"
	SquashRoot   SquashType = "root"
	SquashNone   SquashType = "none"
	SquashNoAll  SquashType = "no_all"
)

// ExportStats 导出统计
type ExportStats struct {
	ReadBytes      int64     `json:"read_bytes"`
	WriteBytes     int64     `json:"write_bytes"`
	ReadOps        int64     `json:"read_ops"`
	WriteOps       int64     `json:"write_ops"`
	ConnectedClients int    `json:"connected_clients"`
	LastAccessAt   time.Time `json:"last_access_at"`
}

// NFSv4Server NFSv4服务器
type NFSv4Server struct {
	mu          sync.RWMutex
	exports     map[string]*NFSExport
	clients     map[string]*NFSClient
	config      *NFSv4Config
	running     bool
}

// NFSv4Config NFSv4配置
type NFSv4Config struct {
	DefaultVersion    NFSVersion     `json:"default_version"`
	SupportedVersions []NFSVersion   `json:"supported_versions"`
	DefaultSecurity   []SecurityType `json:"default_security"`
	GracePeriod       int            `json:"grace_period"`       // 秒
	LeaseTime         int            `json:"lease_time"`         // 秒
	MaxClients        int            `json:"max_clients"`
	EnablePNFS        bool           `json:"enable_pnfs"`
	EnableCopy        bool           `json:"enable_copy"`
	EnableClone       bool           `json:"enable_clone"`
	Domain            string         `json:"domain"`
}

// DefaultNFSv4Config 默认配置
func DefaultNFSv4Config() *NFSv4Config {
	return &NFSv4Config{
		DefaultVersion:    NFSVersion42,
		SupportedVersions: []NFSVersion{NFSVersion40, NFSVersion41, NFSVersion42},
		DefaultSecurity:   []SecurityType{SecuritySys},
		GracePeriod:       90,
		LeaseTime:         60,
		MaxClients:        1000,
		EnablePNFS:        true,
		EnableCopy:        true,
		EnableClone:       true,
		Domain:            "localdomain",
	}
}

// NFSClient NFS客户端
type NFSClient struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	Hostname    string    `json:"hostname,omitempty"`
	Version     NFSVersion `json:"version"`
	Security    SecurityType `json:"security"`
	ConnectedAt time.Time `json:"connected_at"`
	LastActive  time.Time `json:"last_active"`
	State       string    `json:"state"`
}

// NewNFSv4Server 创建NFSv4服务器
func NewNFSv4Server(config *NFSv4Config) *NFSv4Server {
	if config == nil {
		config = DefaultNFSv4Config()
	}

	return &NFSv4Server{
		exports: make(map[string]*NFSExport),
		clients: make(map[string]*NFSClient),
		config:  config,
	}
}

// AddExport 添加导出
func (s *NFSv4Server) AddExport(export *NFSExport) error {
	if export == nil {
		return errors.New("export is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查路径是否已存在
	for _, existing := range s.exports {
		if existing.Path == export.Path {
			return errors.New("export path already exists: " + export.Path)
		}
	}

	// 设置默认值
	now := time.Now()
	if export.CreatedAt.IsZero() {
		export.CreatedAt = now
	}
	export.UpdatedAt = now

	if export.State == "" {
		export.State = ExportStateActive
	}

	if export.NFSVersion == "" {
		export.NFSVersion = s.config.DefaultVersion
	}

	if len(export.Security) == 0 {
		export.Security = s.config.DefaultSecurity
	}

	if len(export.Protocol) == 0 {
		export.Protocol = []string{"tcp"}
	}

	// 启用NFSv4.2特性
	if export.NFSVersion == NFSVersion42 {
		export.Options.SupportsLayouts = s.config.EnablePNFS
		export.Options.SupportsCopy = s.config.EnableCopy
		export.Options.SupportsClone = s.config.EnableClone
		export.Options.SupportsSeek = true
		export.Options.SupportsAllocate = true
		export.Options.SupportsDeallocate = true
		export.Options.SupportsXattr = true
		export.Options.SupportsACL = true
	}

	s.exports[export.ID] = export

	return nil
}

// GetExport 获取导出
func (s *NFSv4Server) GetExport(id string) (*NFSExport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	export, exists := s.exports[id]
	return export, exists
}

// UpdateExport 更新导出
func (s *NFSv4Server) UpdateExport(id string, update func(*NFSExport)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	export, exists := s.exports[id]
	if !exists {
		return errors.New("export not found: " + id)
	}

	update(export)
	export.UpdatedAt = time.Now()

	return nil
}

// DeleteExport 删除导出
func (s *NFSv4Server) DeleteExport(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.exports[id]; !exists {
		return errors.New("export not found: " + id)
	}

	delete(s.exports, id)
	return nil
}

// ListExports 列出导出
func (s *NFSv4Server) ListExports() []*NFSExport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exports := make([]*NFSExport, 0, len(s.exports))
	for _, export := range s.exports {
		exports = append(exports, export)
	}

	return exports
}

// GetExportByPath 通过路径获取导出
func (s *NFSv4Server) GetExportByPath(path string) (*NFSExport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, export := range s.exports {
		if export.Path == path {
			return export, true
		}
	}

	return nil, false
}

// EnableExport 启用导出
func (s *NFSv4Server) EnableExport(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	export, exists := s.exports[id]
	if !exists {
		return errors.New("export not found: " + id)
	}

	export.Enabled = true
	export.State = ExportStateActive
	export.UpdatedAt = time.Now()

	return nil
}

// DisableExport 禁用导出
func (s *NFSv4Server) DisableExport(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	export, exists := s.exports[id]
	if !exists {
		return errors.New("export not found: " + id)
	}

	export.Enabled = false
	export.State = ExportStateInactive
	export.UpdatedAt = time.Now()

	return nil
}

// AddAllowedHost 添加允许的主机
func (s *NFSv4Server) AddAllowedHost(exportID, host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	export, exists := s.exports[exportID]
	if !exists {
		return errors.New("export not found: " + exportID)
	}

	// 检查是否已存在
	for _, h := range export.AllowedHosts {
		if h == host {
			return nil // 已存在
		}
	}

	export.AllowedHosts = append(export.AllowedHosts, host)
	export.UpdatedAt = time.Now()

	return nil
}

// RemoveAllowedHost 移除允许的主机
func (s *NFSv4Server) RemoveAllowedHost(exportID, host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	export, exists := s.exports[exportID]
	if !exists {
		return errors.New("export not found: " + exportID)
	}

	for i, h := range export.AllowedHosts {
		if h == host {
			export.AllowedHosts = append(export.AllowedHosts[:i], export.AllowedHosts[i+1:]...)
			export.UpdatedAt = time.Now()
			return nil
		}
	}

	return nil
}

// AddDeniedHost 添加拒绝的主机
func (s *NFSv4Server) AddDeniedHost(exportID, host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	export, exists := s.exports[exportID]
	if !exists {
		return errors.New("export not found: " + exportID)
	}

	// 检查是否已存在
	for _, h := range export.DeniedHosts {
		if h == host {
			return nil // 已存在
		}
	}

	export.DeniedHosts = append(export.DeniedHosts, host)
	export.UpdatedAt = time.Now()

	return nil
}

// GetClients 获取客户端列表
func (s *NFSv4Server) GetClients() []*NFSClient {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*NFSClient, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	return clients
}

// GetClient 获取客户端
func (s *NFSv4Server) GetClient(id string) (*NFSClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, exists := s.clients[id]
	return client, exists
}

// DisconnectClient 断开客户端
func (s *NFSv4Server) DisconnectClient(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[id]; !exists {
		return errors.New("client not found: " + id)
	}

	delete(s.clients, id)
	return nil
}

// GetStats 获取统计信息
func (s *NFSv4Server) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total_exports":  len(s.exports),
		"total_clients":  len(s.clients),
		"active_exports": 0,
		"by_version":     make(map[NFSVersion]int),
	}

	byVersion := stats["by_version"].(map[NFSVersion]int)

	for _, export := range s.exports {
		if export.State == ExportStateActive {
			stats["active_exports"] = stats["active_exports"].(int) + 1
		}
		byVersion[export.NFSVersion]++
	}

	return stats
}

// GetExportStats 获取导出统计
func (s *NFSv4Server) GetExportStats(id string) (*ExportStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	export, exists := s.exports[id]
	if !exists {
		return nil, errors.New("export not found: " + id)
	}

	if export.Stats == nil {
		return &ExportStats{}, nil
	}

	return export.Stats, nil
}

// GetConfig 获取配置
func (s *NFSv4Server) GetConfig() *NFSv4Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config
}

// UpdateConfig 更新配置
func (s *NFSv4Server) UpdateConfig(config *NFSv4Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
}

// GetSupportedVersions 获取支持的版本
func (s *NFSv4Server) GetSupportedVersions() []NFSVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.SupportedVersions
}

// GetDefaultVersion 获取默认版本
func (s *NFSv4Server) GetDefaultVersion() NFSVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.DefaultVersion
}
