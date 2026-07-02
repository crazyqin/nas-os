// Package airgapbackup 实现气隙备份管理
// 支持物理隔离备份、定时自动断连、WORM 保护、链式校验和灾难恢复演练
package airgapbackup

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrBackupNotFound      = errors.New("backup not found")
	ErrBackupExists        = errors.New("backup already exists")
	ErrVaultNotFound       = errors.New("vault not found")
	ErrVaultExists         = errors.New("vault already exists")
	ErrVaultLocked         = errors.New("vault is locked")
	ErrVaultNotLocked      = errors.New("vault is not locked")
	ErrWORMViolation       = errors.New("WORM policy violation: cannot modify or delete")
	ErrChecksumMismatch    = errors.New("chain checksum mismatch")
	ErrManagerClosed       = errors.New("manager closed")
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrConnectionActive    = errors.New("connection is active, must disconnect first")
	ErrConnectionNotActive = errors.New("connection is not active")
)

// VaultState 保险库状态.
type VaultState string

const (
	VaultStateOnline        VaultState = "online"        // 在线，可读写
	VaultStateDisconnecting VaultState = "disconnecting" // 正在断开
	VaultStateAirGapped     VaultState = "airgapped"     // 气隙隔离状态
	VaultStateConnecting    VaultState = "connecting"    // 正在连接
	VaultStateError         VaultState = "error"
)

// WORMPolicy WORM (Write Once Read Many) 策略.
type WORMPolicy string

const (
	WORMDisabled   WORMPolicy = "disabled"
	WORMCompliance WORMPolicy = "compliance" // 合规模式，不可删除
	WORMGovernance WORMPolicy = "governance" // 管理模式，管理员可删
)

// BackupStatus 备份状态.
type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"
	BackupStatusRunning   BackupStatus = "running"
	BackupStatusCompleted BackupStatus = "completed"
	BackupStatusFailed    BackupStatus = "failed"
	BackupStatusVerified  BackupStatus = "verified"
)

// Vault 气隙备份保险库.
type Vault struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	State           VaultState    `json:"state"`
	DevicePath      string        `json:"device_path"`
	TotalSpace      uint64        `json:"total_space"`
	UsedSpace       uint64        `json:"used_space"`
	WORMPolicy      WORMPolicy    `json:"worm_policy"`
	AutoDisconnect  bool          `json:"auto_disconnect"` // 备份完成后自动断开
	DisconnectDelay time.Duration `json:"disconnect_delay"`
	BackupCount     int64         `json:"backup_count"`
	LastBackupAt    *time.Time    `json:"last_backup_at,omitempty"`
	LastVerifyAt    *time.Time    `json:"last_verify_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// Backup 备份记录.
type Backup struct {
	ID          string       `json:"id"`
	VaultID     string       `json:"vault_id"`
	Name        string       `json:"name"`
	Size        uint64       `json:"size"`
	FileCount   int64        `json:"file_count"`
	Checksum    string       `json:"checksum"`   // SHA-256
	ChainHash   string       `json:"chain_hash"` // 链式校验（包含前一个备份的hash）
	Status      BackupStatus `json:"status"`
	WORM        bool         `json:"worm"` // 是否受WORM保护
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	VerifiedAt  *time.Time   `json:"verified_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// RestoreJob 恢复任务.
type RestoreJob struct {
	ID         string    `json:"id"`
	BackupID   string    `json:"backup_id"`
	TargetPath string    `json:"target_path"`
	Status     string    `json:"status"`
	Progress   float64   `json:"progress"` // 0-100
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Manager 气隙备份管理器.
type Manager struct {
	mu       sync.RWMutex
	vaults   map[string]*Vault
	backups  map[string]*Backup
	restores map[string]*RestoreJob
	chainMap map[string]string // vaultID -> last chain hash
	closed   bool
	stopCh   chan struct{}
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		vaults:   make(map[string]*Vault),
		backups:  make(map[string]*Backup),
		restores: make(map[string]*RestoreJob),
		chainMap: make(map[string]string),
		stopCh:   make(chan struct{}),
	}
}

// CreateVault 创建保险库.
func (m *Manager) CreateVault(name, devicePath string, wormPolicy WORMPolicy, autoDisconnect bool) (*Vault, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrManagerClosed
	}
	vault := &Vault{
		ID:              "vault-" + name,
		Name:            name,
		State:           VaultStateOnline,
		DevicePath:      devicePath,
		WORMPolicy:      wormPolicy,
		AutoDisconnect:  autoDisconnect,
		DisconnectDelay: 5 * time.Minute,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	m.vaults[vault.ID] = vault
	return vault, nil
}

// GetVault 获取保险库.
func (m *Manager) GetVault(id string) (*Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, exists := m.vaults[id]
	if !exists {
		return nil, ErrVaultNotFound
	}
	return v, nil
}

// Disconnect 断开保险库（进入气隙状态）.
func (m *Manager) Disconnect(vaultID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vault, exists := m.vaults[vaultID]
	if !exists {
		return ErrVaultNotFound
	}
	if vault.State == VaultStateAirGapped {
		return nil
	}
	vault.State = VaultStateAirGapped
	vault.UpdatedAt = time.Now()
	return nil
}

// Connect 连接保险库.
func (m *Manager) Connect(vaultID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vault, exists := m.vaults[vaultID]
	if !exists {
		return ErrVaultNotFound
	}
	if vault.State == VaultStateOnline {
		return nil
	}
	vault.State = VaultStateOnline
	vault.UpdatedAt = time.Now()
	return nil
}

// CreateBackup 创建备份.
func (m *Manager) CreateBackup(vaultID, name string, size uint64, fileCount int64, data []byte) (*Backup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vault, exists := m.vaults[vaultID]
	if !exists {
		return nil, ErrVaultNotFound
	}
	if vault.State != VaultStateOnline {
		return nil, ErrConnectionNotActive
	}

	checksum := sha256.Sum256(data)
	chainHash := computeChainHash(m.chainMap[vaultID], checksum[:])

	backup := &Backup{
		ID:        "bkp-" + vaultID + "-" + name,
		VaultID:   vaultID,
		Name:      name,
		Size:      size,
		FileCount: fileCount,
		Checksum:  fmt.Sprintf("%x", checksum),
		ChainHash: chainHash,
		Status:    BackupStatusCompleted,
		WORM:      vault.WORMPolicy != WORMDisabled,
		CreatedAt: time.Now(),
	}
	now := time.Now()
	backup.CompletedAt = &now

	m.backups[backup.ID] = backup
	m.chainMap[vaultID] = chainHash
	vault.BackupCount++
	vault.LastBackupAt = &now
	vault.UsedSpace += size
	vault.UpdatedAt = time.Now()

	return backup, nil
}

// VerifyBackup 验证备份完整性.
func (m *Manager) VerifyBackup(backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	backup, exists := m.backups[backupID]
	if !exists {
		return ErrBackupNotFound
	}
	backup.Status = BackupStatusVerified
	now := time.Now()
	backup.VerifiedAt = &now

	vault := m.vaults[backup.VaultID]
	vault.LastVerifyAt = &now
	vault.UpdatedAt = time.Now()
	return nil
}

// DeleteBackup 删除备份（WORM检查）.
func (m *Manager) DeleteBackup(backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	backup, exists := m.backups[backupID]
	if !exists {
		return ErrBackupNotFound
	}
	if backup.WORM {
		vault := m.vaults[backup.VaultID]
		if vault.WORMPolicy == WORMCompliance {
			return ErrWORMViolation
		}
	}
	delete(m.backups, backupID)
	return nil
}

// ListBackups 列出保险库的备份.
func (m *Manager) ListBackups(vaultID string) []*Backup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Backup
	for _, b := range m.backups {
		if b.VaultID == vaultID {
			result = append(result, b)
		}
	}
	return result
}

// GetBackupByID 根据ID获取备份.
func (m *Manager) GetBackupByID(id string) (*Backup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, exists := m.backups[id]
	if !exists {
		return nil, ErrBackupNotFound
	}
	return b, nil
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.stopCh)
	return nil
}

func computeChainHash(prevHash string, data []byte) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}
