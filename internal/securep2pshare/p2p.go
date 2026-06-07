// Package securep2pshare 提供端到端加密的P2P文件共享功能
// 学习 Resilio Sync 与 Syncthing 的 P2P 架构
// 支持 E2E 加密、NAT 穿透、断点续传、多设备同步

package securep2pshare

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// ShareStatus 共享状态
type ShareStatus string

const (
	ShareStatusPending   ShareStatus = "pending"
	ShareStatusActive    ShareStatus = "active"
	ShareStatusSyncing   ShareStatus = "syncing"
	ShareStatusPaused    ShareStatus = "paused"
	ShareStatusCompleted ShareStatus = "completed"
	ShareStatusError     ShareStatus = "error"
)

// PeerRole 对等节点角色
type PeerRole string

const (
	PeerRoleOwner  PeerRole = "owner"
	PeerRoleEditor PeerRole = "editor"
	PeerRoleViewer PeerRole = "viewer"
)

// ConnectionType 连接类型
type ConnectionType string

const (
	ConnectionDirect ConnectionType = "direct"
	ConnectionRelay  ConnectionType = "relay"
	ConnectionLAN    ConnectionType = "lan"
	ConnectionWAN    ConnectionType = "wan"
)

// Share 共享定义
type Share struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Path           string            `json:"path"`
	Status         ShareStatus       `json:"status"`
	EncryptionKey  string            `json:"-"` // 不序列化
	KeyFingerprint string            `json:"key_fingerprint"`
	Owner          string            `json:"owner"`
	Peers          []Peer            `json:"peers"`
	Permissions    SharePermissions  `json:"permissions"`
	Size           int64             `json:"size"`
	FileCount      int               `json:"file_count"`
	SyncProgress   float64           `json:"sync_progress"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	ExpiresAt      *time.Time        `json:"expires_at,omitempty"`
	MaxDownloads   int               `json:"maxDownloads"`
	DownloadCount  int               `json:"download_count"`
	Metadata       map[string]string `json:"metadata"`
}

// Peer 对等节点
type Peer struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Role           PeerRole       `json:"role"`
	Address        string         `json:"address"`
	ConnectionType ConnectionType `json:"connection_type"`
	Latency        int            `json:"latency"`
	Bandwidth      int64          `json:"bandwidth"`
	LastSeen       time.Time      `json:"last_seen"`
	IsOnline       bool           `json:"is_online"`
	SyncStatus     string         `json:"sync_status"`
}

// SharePermissions 共享权限
type SharePermissions struct {
	AllowRead     bool     `json:"allow_read"`
	AllowWrite    bool     `json:"allow_write"`
	AllowDelete   bool     `json:"allow_delete"`
	AllowShare    bool     `json:"allow_share"`
	AllowDownload bool     `json:"allow_download"`
	MaxFileSize   int64    `json:"max_file_size"`
	AllowedTypes  []string `json:"allowed_types"`
}

// Transfer 传输记录
type Transfer struct {
	ID          string        `json:"id"`
	ShareID     string        `json:"share_id"`
	FileName    string        `json:"file_name"`
	FileSize    int64         `json:"file_size"`
	Transferred int64         `json:"transferred"`
	Speed       int64         `json:"speed"`
	Direction   string        `json:"direction"` // upload/download
	Status      string        `json:"status"`
	PeerID      string        `json:"peer_id"`
	StartedAt   time.Time     `json:"started_at"`
	ETA         time.Duration `json:"eta"`
	Progress    float64       `json:"progress"`
}

// SyncEvent 同步事件
type SyncEvent struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	Type      string    `json:"type"`
	FileName  string    `json:"file_name"`
	PeerID    string    `json:"peer_id"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details"`
}

// Manager P2P共享管理器
type Manager struct {
	mu           sync.RWMutex
	shares       map[string]*Share
	transfers    map[string]*Transfer
	events       []SyncEvent
	nodeID       string
	nodeName     string
	listenPort   int
	relayServers []string
	maxPeers     int
	encryption   bool
	compression  bool
	dedup        bool
}

// NewManager 创建管理器
func NewManager(nodeName string, listenPort int) *Manager {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", nodeName, listenPort)))
	return &Manager{
		shares:       make(map[string]*Share),
		transfers:    make(map[string]*Transfer),
		events:       make([]SyncEvent, 0),
		nodeID:       hex.EncodeToString(hash[:8]),
		nodeName:     nodeName,
		listenPort:   listenPort,
		relayServers: []string{"relay1.nas-os.com", "relay2.nas-os.com"},
		maxPeers:     50,
		encryption:   true,
		compression:  true,
		dedup:        true,
	}
}

// CreateShare 创建共享
func (m *Manager) CreateShare(name, path string, owner string, permissions SharePermissions) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成加密密钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	fingerprint := sha256.Sum256(key)

	share := &Share{
		ID:             fmt.Sprintf("share_%d", time.Now().UnixNano()),
		Name:           name,
		Path:           path,
		Status:         ShareStatusActive,
		EncryptionKey:  hex.EncodeToString(key),
		KeyFingerprint: hex.EncodeToString(fingerprint[:8]),
		Owner:          owner,
		Peers: []Peer{
			{
				ID:       m.nodeID,
				Name:     m.nodeName,
				Role:     PeerRoleOwner,
				IsOnline: true,
				LastSeen: time.Now(),
			},
		},
		Permissions: permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    make(map[string]string),
	}

	m.shares[share.ID] = share
	return share, nil
}

// AddPeer 添加对等节点
func (m *Manager) AddPeer(shareID string, peer Peer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[shareID]
	if !exists {
		return fmt.Errorf("共享不存在: %s", shareID)
	}

	if len(share.Peers) >= m.maxPeers {
		return fmt.Errorf("已达到最大节点数: %d", m.maxPeers)
	}

	peer.LastSeen = time.Now()
	peer.IsOnline = true
	share.Peers = append(share.Peers, peer)
	share.UpdatedAt = time.Now()

	return nil
}

// GetShare 获取共享信息
func (m *Manager) GetShare(shareID string) (*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	share, exists := m.shares[shareID]
	if !exists {
		return nil, fmt.Errorf("共享不存在: %s", shareID)
	}

	return share, nil
}

// ListShares 列出所有共享
func (m *Manager) ListShares(owner string) []*Share {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var shares []*Share
	for _, s := range m.shares {
		if owner == "" || s.Owner == owner {
			shares = append(shares, s)
		}
	}

	return shares
}

// GetTransfers 获取传输列表
func (m *Manager) GetTransfers(shareID string) []*Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var transfers []*Transfer
	for _, t := range m.transfers {
		if shareID == "" || t.ShareID == shareID {
			transfers = append(transfers, t)
		}
	}

	return transfers
}

// GetEvents 获取同步事件
func (m *Manager) GetEvents(shareID string, limit int) []SyncEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []SyncEvent
	for i := len(m.events) - 1; i >= 0 && len(events) < limit; i-- {
		if shareID == "" || m.events[i].ShareID == shareID {
			events = append(events, m.events[i])
		}
	}

	return events
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"node_id":       m.nodeID,
		"node_name":     m.nodeName,
		"total_shares":  len(m.shares),
		"active_shares": 0,
		"total_peers":   0,
		"online_peers":  0,
		"transfers":     len(m.transfers),
		"encryption":    m.encryption,
		"compression":   m.compression,
	}

	for _, s := range m.shares {
		if s.Status == ShareStatusActive {
			stats["active_shares"] = stats["active_shares"].(int) + 1
		}
		stats["total_peers"] = stats["total_peers"].(int) + len(s.Peers)
		for _, p := range s.Peers {
			if p.IsOnline {
				stats["online_peers"] = stats["online_peers"].(int) + 1
			}
		}
	}

	return stats
}

// Close 关闭管理器
func (m *Manager) Close() error {
	return nil
}
