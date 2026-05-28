// Package wanoptimize 提供 WAN 传输加速与优化
package wanoptimize

import (
	"fmt"
	"sync"
	"time"
)

// Manager WAN加速管理器.
type Manager struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
	stats   map[string][]*TransferStats
}

// NewManager 创建WAN加速管理器.
func NewManager() *Manager {
	return &Manager{
		tunnels: make(map[string]*Tunnel),
		stats:   make(map[string][]*TransferStats),
	}
}

// CreateTunnel 创建隧道.
func (m *Manager) CreateTunnel(req CreateTunnelRequest) (*Tunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	compress := req.Compress
	if compress == "" {
		compress = CompressLZ4
	}

	id := fmt.Sprintf("wan-%d", time.Now().UnixNano())
	tunnel := &Tunnel{
		ID:         id,
		Name:       req.Name,
		LocalAddr:  req.LocalAddr,
		RemoteAddr: req.RemoteAddr,
		Port:       req.Port,
		Compress:   compress,
		Encrypt:    req.Encrypt,
		Bandwidth:  req.Bandwidth,
		Status:     TunnelStatusInactive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.tunnels[id] = tunnel
	return tunnel, nil
}

// GetTunnel 获取隧道.
func (m *Manager) GetTunnel(id string) (*Tunnel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tunnels[id]
	if !ok {
		return nil, ErrTunnelNotFound
	}
	return t, nil
}

// ListTunnels 列出所有隧道.
func (m *Manager) ListTunnels() []*Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnels := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	return tunnels
}

// DeleteTunnel 删除隧道.
func (m *Manager) DeleteTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tunnels[id]
	if !ok {
		return ErrTunnelNotFound
	}
	if t.Status == TunnelStatusActive {
		return ErrTunnelActive
	}

	delete(m.tunnels, id)
	return nil
}

// ConnectTunnel 连接隧道.
func (m *Manager) ConnectTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tunnels[id]
	if !ok {
		return ErrTunnelNotFound
	}

	t.Status = TunnelStatusActive
	t.UpdatedAt = time.Now()
	return nil
}

// DisconnectTunnel 断开隧道.
func (m *Manager) DisconnectTunnel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tunnels[id]
	if !ok {
		return ErrTunnelNotFound
	}

	t.Status = TunnelStatusInactive
	t.UpdatedAt = time.Now()
	return nil
}

// GetStats 获取全局统计.
func (m *Manager) GetStats() WANStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := WANStats{
		TotalTunnels: int64(len(m.tunnels)),
	}

	for _, t := range m.tunnels {
		stats.TotalSent += t.BytesSent
		stats.TotalRecv += t.BytesRecv
		if t.Status == TunnelStatusActive {
			stats.ActiveTunnels++
			stats.AvgLatency += t.Latency
		}
	}

	if stats.ActiveTunnels > 0 {
		stats.AvgLatency /= time.Duration(stats.ActiveTunnels)
	}

	return stats
}
