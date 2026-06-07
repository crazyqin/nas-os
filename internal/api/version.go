// Package api 提供 API 版本管理功能
// 对标 TrueNAS SCALE 25.04 API 版本化设计
// 支持多版本共存、版本发现、废弃通知

package api

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ========== 版本定义 ==========

// APIVersion API 版本结构
type APIVersion struct {
	Version      string     `json:"version"`                // 版本号，如 "v1", "v2"
	Status       string     `json:"status"`                 // 状态: stable, deprecated, beta, alpha
	Deprecated   bool       `json:"deprecated"`             // 是否已废弃
	SunsetDate   *time.Time `json:"sunsetDate,omitempty"`   // 废弃截止日期
	ReleaseDate  time.Time  `json:"releaseDate"`            // 发布日期
	Description  string     `json:"description"`            // 版本描述
	MigrationURL string     `json:"migrationUrl,omitempty"` // 迁移指南 URL
}

// VersionStatus 版本状态常量
const (
	VersionStatusStable     = "stable"
	VersionStatusDeprecated = "deprecated"
	VersionStatusBeta       = "beta"
	VersionStatusAlpha      = "alpha"
)

// ========== 版本管理器 ==========

// VersionManager API 版本管理器
type VersionManager struct {
	mu                sync.RWMutex
	versions          map[string]*APIVersion
	current           string
	minSupported      string
	deprecatedHeaders map[string]string // 废弃版本响应头模板
}

// NewVersionManager 创建版本管理器
func NewVersionManager() *VersionManager {
	vm := &VersionManager{
		versions:          make(map[string]*APIVersion),
		deprecatedHeaders: make(map[string]string),
	}

	// 初始化默认版本
	vm.InitializeDefaultVersions()
	return vm
}

// InitializeDefaultVersions 初始化默认版本配置
func (vm *VersionManager) InitializeDefaultVersions() {
	now := time.Now()

	// v1 - 当前稳定版本
	vm.RegisterVersion(&APIVersion{
		Version:     "v1",
		Status:      VersionStatusStable,
		Deprecated:  false,
		ReleaseDate: now.AddDate(-1, 0, 0),
		Description: "NAS-OS API v1 - 当前稳定版本，推荐使用",
	})

	// v2 - Beta 版本（开发中）
	vm.RegisterVersion(&APIVersion{
		Version:     "v2",
		Status:      VersionStatusBeta,
		Deprecated:  false,
		ReleaseDate: now,
		Description: "NAS-OS API v2 - Beta 版本，支持 JSON-RPC 2.0 WebSocket",
	})

	// v0 - 已废弃版本
	sunset := now.AddDate(0, 3, 0) // 3个月后废弃
	vm.RegisterVersion(&APIVersion{
		Version:      "v0",
		Status:       VersionStatusDeprecated,
		Deprecated:   true,
		SunsetDate:   &sunset,
		ReleaseDate:  now.AddDate(-2, 0, 0),
		Description:  "NAS-OS API v0 - 已废弃，请迁移到 v1",
		MigrationURL: "/docs/api/migration/v0-to-v1",
	})

	vm.SetCurrent("v1")
	vm.SetMinSupported("v0")
}

// RegisterVersion 注册 API 版本
func (vm *VersionManager) RegisterVersion(v *APIVersion) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.versions[v.Version] = v
}

// SetCurrent 设置当前版本
func (vm *VersionManager) SetCurrent(version string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.current = version
}

// SetMinSupported 设置最低支持版本
func (vm *VersionManager) SetMinSupported(version string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.minSupported = version
}

// GetCurrent 获取当前版本
func (vm *VersionManager) GetCurrent() string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.current
}

// GetVersion 获取指定版本信息
func (vm *VersionManager) GetVersion(version string) (*APIVersion, bool) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	v, ok := vm.versions[version]
	return v, ok
}

// GetAllVersions 获取所有版本列表
func (vm *VersionManager) GetAllVersions() []*APIVersion {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := make([]*APIVersion, 0, len(vm.versions))
	for _, v := range vm.versions {
		versions = append(versions, v)
	}
	return versions
}

// IsVersionSupported 检查版本是否支持
func (vm *VersionManager) IsVersionSupported(version string) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	v, ok := vm.versions[version]
	if !ok {
		return false
	}
	// 已完全废弃且超过 sunset 日期的版本不再支持
	if v.Deprecated && v.SunsetDate != nil && time.Now().After(*v.SunsetDate) {
		return false
	}
	return true
}

// IsVersionDeprecated 检查版本是否已废弃
func (vm *VersionManager) IsVersionDeprecated(version string) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	v, ok := vm.versions[version]
	if !ok {
		return false
	}
	return v.Deprecated
}

// GetDeprecationHeaders 获取废弃版本响应头
func (vm *VersionManager) GetDeprecationHeaders(version string) map[string]string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	v, ok := vm.versions[version]
	if !ok || !v.Deprecated {
		return nil
	}

	headers := make(map[string]string)
	headers["X-API-Deprecated"] = "true"
	headers["X-API-Deprecation-Reason"] = fmt.Sprintf("API %s 已废弃，请迁移到 %s", version, vm.current)

	if v.SunsetDate != nil {
		headers["X-API-Removal-Date"] = v.SunsetDate.Format("2006-01-02")
	}

	if v.MigrationURL != "" {
		headers["X-API-Migration-Url"] = v.MigrationURL
	}

	headers["X-API-Alternatives"] = fmt.Sprintf("/api/%s/", vm.current)

	return headers
}

// ========== 版本发现响应 ==========

// VersionDiscovery 版本发现响应结构
type VersionDiscovery struct {
	Current      string       `json:"current"`
	MinSupported string       `json:"minSupported"`
	Versions     []APIVersion `json:"versions"`
}

// GetVersionDiscovery 获取版本发现信息
func (vm *VersionManager) GetVersionDiscovery() *VersionDiscovery {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := make([]APIVersion, 0, len(vm.versions))
	for _, v := range vm.versions {
		versions = append(versions, *v)
	}

	return &VersionDiscovery{
		Current:      vm.current,
		MinSupported: vm.minSupported,
		Versions:     versions,
	}
}

// ========== 版本解析辅助函数 ==========

// ParseVersion 从路径解析版本号
// 例如: /api/v1/volumes -> "v1"
func ParseVersion(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "api" && i+1 < len(parts) {
			next := parts[i+1]
			if strings.HasPrefix(next, "v") {
				return next
			}
		}
	}
	return ""
}

// NormalizeVersion 规范化版本号
// 支持 "1", "v1", "V1" -> "v1"
func NormalizeVersion(version string) string {
	if version == "" {
		return ""
	}

	// 移除前导斜杠
	version = strings.TrimPrefix(version, "/")

	// 如果没有 v 前缀，添加它
	if !strings.HasPrefix(strings.ToLower(version), "v") {
		return "v" + version
	}

	return strings.ToLower(version)
}

// ========== 版本路由辅助 ==========

// VersionedRoute 版本化路由信息
type VersionedRoute struct {
	Version string
	Path    string
	Method  string
}

// VersionedRoutes 版本化路由集合
type VersionedRoutes struct {
	mu     sync.RWMutex
	routes map[string][]VersionedRoute // version -> routes
}

// NewVersionedRoutes 创建版本化路由集合
func NewVersionedRoutes() *VersionedRoutes {
	return &VersionedRoutes{
		routes: make(map[string][]VersionedRoute),
	}
}

// AddRoute 添加版本化路由
func (vr *VersionedRoutes) AddRoute(version, path, method string) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.routes[version] = append(vr.routes[version], VersionedRoute{
		Version: version,
		Path:    path,
		Method:  method,
	})
}

// GetRoutes 获取指定版本的路由
func (vr *VersionedRoutes) GetRoutes(version string) []VersionedRoute {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	return vr.routes[version]
}

// ========== 全局版本管理器实例 ==========

var (
	globalVersionManager     *VersionManager
	globalVersionManagerOnce sync.Once
)

// GetGlobalVersionManager 获取全局版本管理器实例
func GetGlobalVersionManager() *VersionManager {
	globalVersionManagerOnce.Do(func() {
		globalVersionManager = NewVersionManager()
	})
	return globalVersionManager
}

// ========== 版本错误 ==========

// VersionError 版本相关错误
type VersionError struct {
	Code      int
	Message   string
	Requested string
	Supported []string
	Current   string
}

func (e *VersionError) Error() string {
	return e.Message
}

// NewVersionNotFoundError 创建版本不存在错误
func NewVersionNotFoundError(requested string) *VersionError {
	vm := GetGlobalVersionManager()
	return &VersionError{
		Code:      400,
		Message:   fmt.Sprintf("不支持 API 版本: %s", requested),
		Requested: requested,
		Supported: func() []string {
			var s []string
			for _, v := range vm.GetAllVersions() {
				if vm.IsVersionSupported(v.Version) {
					s = append(s, v.Version)
				}
			}
			return s
		}(),
		Current: vm.GetCurrent(),
	}
}

// NewVersionDeprecatedError 创建版本已废弃错误
func NewVersionDeprecatedError(requested string) *VersionError {
	vm := GetGlobalVersionManager()
	v, _ := vm.GetVersion(requested)

	msg := fmt.Sprintf("API %s 已废弃", requested)
	if v != nil && v.SunsetDate != nil {
		msg += fmt.Sprintf("，将于 %s 移除", v.SunsetDate.Format("2006-01-02"))
	}
	msg += fmt.Sprintf("，请迁移到 %s", vm.GetCurrent())

	return &VersionError{
		Code:      410, // Gone
		Message:   msg,
		Requested: requested,
		Supported: []string{vm.GetCurrent()},
		Current:   vm.GetCurrent(),
	}
}
