// Package smb3unixext 提供 SMB3 Unix 扩展支持。
// 服务层：管理扩展配置、客户端能力协商、状态查询。
package smb3unixext

import (
	"fmt"
	"sync"
	"time"
)

// Service SMB3 Unix 扩展管理服务
type Service struct {
	mu      sync.RWMutex
	configs map[string]*UnixExtensionConfig
}

// NewService 创建 SMB3 Unix 扩展管理服务
func NewService() *Service {
	return &Service{
		configs: make(map[string]*UnixExtensionConfig),
	}
}

// SetExtension 设置共享的 Unix 扩展配置
func (s *Service) SetExtension(req *SetExtensionRequest) (*UnixExtensionConfig, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	if req.ShareName == "" {
		return nil, fmt.Errorf("共享名称不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 如果配置已存在，保留 CreatedAt
	var createdAt time.Time
	if existing, ok := s.configs[req.ShareName]; ok {
		createdAt = existing.CreatedAt
	} else {
		createdAt = now
	}

	cfg := &UnixExtensionConfig{
		ShareName:       req.ShareName,
		Enabled:         req.Enabled,
		Protocol:        ProtocolMulti,
		IsMultiProtocol: req.Enabled, // 启用时自动标记为多协议
		Capabilities:    DefaultCapabilities,
		UpdatedAt:       now,
		CreatedAt:       createdAt,
	}
	s.configs[req.ShareName] = cfg

	return cfg, nil
}

// GetExtension 获取共享的 Unix 扩展配置
func (s *Service) GetExtension(shareName string) (*UnixExtensionConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, ok := s.configs[shareName]
	if !ok {
		return nil, fmt.Errorf("共享 %s 的扩展配置不存在", shareName)
	}
	return cfg, nil
}

// GetExtensionStatus 获取扩展状态响应
func (s *Service) GetExtensionStatus(shareName string) (*ExtensionStatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, ok := s.configs[shareName]
	if !ok {
		return nil, fmt.Errorf("共享 %s 的扩展配置不存在", shareName)
	}

	status := ExtensionStatusDisabled
	if cfg.Enabled {
		status = ExtensionStatusEnabled
	}

	return &ExtensionStatusResponse{
		ShareName:             cfg.ShareName,
		Enabled:               cfg.Enabled,
		Protocol:              cfg.Protocol,
		IsMultiProtocol:       cfg.IsMultiProtocol,
		Status:                status,
		Capabilities:          cfg.Capabilities,
		ClientNegotiated:      cfg.ClientNegotiated,
		NegotiatedCapabilities: cfg.NegotiatedCapabilities,
	}, nil
}

// ListExtensions 列出所有扩展配置
func (s *Service) ListExtensions() []*UnixExtensionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*UnixExtensionConfig, 0, len(s.configs))
	for _, cfg := range s.configs {
		result = append(result, cfg)
	}
	return result
}

// RemoveExtension 移除共享的扩展配置
func (s *Service) RemoveExtension(shareName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[shareName]; !ok {
		return fmt.Errorf("共享 %s 的扩展配置不存在", shareName)
	}
	delete(s.configs, shareName)
	return nil
}

// IsMultiProtocol 检查共享是否为多协议模式
func (s *Service) IsMultiProtocol(shareName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, ok := s.configs[shareName]
	if !ok {
		return false
	}
	return cfg.IsMultiProtocol
}

// CanEnableUnixExtensions 检查是否可以启用 Unix 扩展
// 只有启用的共享才支持
func (s *Service) CanEnableUnixExtensions(shareName string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, ok := s.configs[shareName]
	if !ok {
		return false, fmt.Errorf("共享 %s 的扩展配置不存在", shareName)
	}
	return cfg.Enabled && cfg.IsMultiProtocol, nil
}

// NegotiateClientCapabilities 客户端能力协商
// 检测客户端能力，自动协商支持的 Unix 扩展
func (s *Service) NegotiateClientCapabilities(req *ClientCapabilityRequest) (*ExtensionStatusResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	if req.ShareName == "" {
		return nil, fmt.Errorf("共享名称不能为空")
	}
	if len(req.ClientCapabilities) == 0 {
		return nil, fmt.Errorf("客户端能力列表不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, ok := s.configs[req.ShareName]
	if !ok {
		return nil, fmt.Errorf("共享 %s 的扩展配置不存在", req.ShareName)
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("共享 %s 未启用 Unix 扩展", req.ShareName)
	}

	// 计算交集：服务端能力 ∩ 客户端能力
	serverCapSet := make(map[ClientCapability]bool)
	for _, cap := range cfg.Capabilities {
		serverCapSet[cap] = true
	}

	negotiated := make([]ClientCapability, 0)
	for _, cap := range req.ClientCapabilities {
		if serverCapSet[cap] {
			negotiated = append(negotiated, cap)
		}
	}

	cfg.ClientNegotiated = true
	cfg.NegotiatedCapabilities = negotiated
	cfg.UpdatedAt = time.Now()

	status := ExtensionStatusDisabled
	if cfg.Enabled {
		status = ExtensionStatusEnabled
	}

	return &ExtensionStatusResponse{
		ShareName:             cfg.ShareName,
		Enabled:               cfg.Enabled,
		Protocol:              cfg.Protocol,
		IsMultiProtocol:       cfg.IsMultiProtocol,
		Status:                status,
		Capabilities:          cfg.Capabilities,
		ClientNegotiated:      cfg.ClientNegotiated,
		NegotiatedCapabilities: cfg.NegotiatedCapabilities,
	}, nil
}

// GetSupportStatus 获取全局支持状态
func (s *Service) GetSupportStatus() *SupportStatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabledCount := 0
	for _, cfg := range s.configs {
		if cfg.Enabled {
			enabledCount++
		}
	}

	return &SupportStatusResponse{
		Supported:           true,
		MinSMBVersion:       "3.1.1",
		EnabledShares:       enabledCount,
		TotalShares:         len(s.configs),
		DefaultCapabilities: DefaultCapabilities,
	}
}

// EnableAll 为所有已配置的共享启用 Unix 扩展
func (s *Service) EnableAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()
	for _, cfg := range s.configs {
		if !cfg.Enabled {
			cfg.Enabled = true
			cfg.IsMultiProtocol = true
			cfg.UpdatedAt = now
			count++
		}
	}
	return count
}

// DisableAll 为所有已配置的共享禁用 Unix 扩展
func (s *Service) DisableAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()
	for _, cfg := range s.configs {
		if cfg.Enabled {
			cfg.Enabled = false
			cfg.IsMultiProtocol = false
			cfg.ClientNegotiated = false
			cfg.NegotiatedCapabilities = nil
			cfg.UpdatedAt = now
			count++
		}
	}
	return count
}
