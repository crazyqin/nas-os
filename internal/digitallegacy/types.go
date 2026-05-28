// Package digitallegacy 数字遗产管理 - 紧急联系人/数据传承
package digitallegacy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// BeneficiaryStatus 受益人状态
type BeneficiaryStatus string

const (
	BeneficiaryPending  BeneficiaryStatus = "pending"
	BeneficiaryVerified BeneficiaryStatus = "verified"
	BeneficiaryActive   BeneficiaryStatus = "active"
	BeneficiaryRevoked  BeneficiaryStatus = "revoked"
)

// AssetType 资产类型
type AssetType string

const (
	AssetFiles    AssetType = "files"
	AssetPhotos   AssetType = "photos"
	AssetDocs     AssetType = "documents"
	AssetPasswords AssetType = "passwords"
	AssetAll      AssetType = "all"
)

// AccessLevel 访问级别
type AccessLevel string

const (
	AccessRead      AccessLevel = "read"
	AccessDownload  AccessLevel = "download"
	AccessFull      AccessLevel = "full"
)

// Beneficiary 受益人
type Beneficiary struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Email       string            `json:"email"`
	Phone       string            `json:"phone,omitempty"`
	Relation    string            `json:"relation"`
	Status      BeneficiaryStatus `json:"status"`
	AccessLevel AccessLevel       `json:"access_level"`
	Assets      []AssetGrant      `json:"assets"`
	CreatedAt   time.Time         `json:"created_at"`
	VerifiedAt  *time.Time        `json:"verified_at,omitempty"`
}

// AssetGrant 资产授权
type AssetGrant struct {
	Type   AssetType `json:"type"`
	Paths  []string  `json:"paths,omitempty"`
	Note   string    `json:"note,omitempty"`
}

// DeadmanConfig 遗嘱触发配置
type DeadmanConfig struct {
	Enabled         bool      `json:"enabled"`
	InactivityDays  int       `json:"inactivity_days"`  // 不活跃天数触发
	WarningDays     int       `json:"warning_days"`     // 提前提醒天数
	NotifyEmail     string    `json:"notify_email"`
	LastActive      time.Time `json:"last_active"`
	TriggeredAt     *time.Time `json:"triggered_at,omitempty"`
}

// LegacyPlan 遗产计划
type LegacyPlan struct {
	ID           string          `json:"id"`
	OwnerID      string          `json:"owner_id"`
	Beneficiaries []string       `json:"beneficiaries"`
	Deadman      *DeadmanConfig  `json:"deadman"`
	Instructions string          `json:"instructions"`
	IsSealed     bool            `json:"is_sealed"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// AccessLog 访问日志
type AccessLog struct {
	ID           string    `json:"id"`
	BeneficiaryID string   `json:"beneficiary_id"`
	Action       string    `json:"action"`
	Resource     string    `json:"resource"`
	IP           string    `json:"ip,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// Manager 管理器
type Manager struct {
	mu            sync.RWMutex
	beneficiaries map[string]*Beneficiary
	plans         map[string]*LegacyPlan
	accessLogs    []*AccessLog
	dataFile      string
}

var (
	ErrBeneficiaryNotFound = errors.New("beneficiary not found")
	ErrPlanNotFound        = errors.New("legacy plan not found")
	ErrAlreadySealed       = errors.New("plan is already sealed")
)

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		beneficiaries: make(map[string]*Beneficiary),
		plans:         make(map[string]*LegacyPlan),
		dataFile:      dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error { return m.load() }

// AddBeneficiary 添加受益人
func (m *Manager) AddBeneficiary(b *Beneficiary) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b.Status = BeneficiaryPending
	b.CreatedAt = time.Now()
	m.beneficiaries[b.ID] = b
	return m.save()
}

// VerifyBeneficiary 验证受益人
func (m *Manager) VerifyBeneficiary(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.beneficiaries[id]
	if !ok {
		return ErrBeneficiaryNotFound
	}
	b.Status = BeneficiaryVerified
	now := time.Now()
	b.VerifiedAt = &now
	return m.save()
}

// RemoveBeneficiary 移除受益人
func (m *Manager) RemoveBeneficiary(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.beneficiaries[id]; !ok {
		return ErrBeneficiaryNotFound
	}
	delete(m.beneficiaries, id)
	return m.save()
}

// GetBeneficiary 获取受益人
func (m *Manager) GetBeneficiary(id string) (*Beneficiary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.beneficiaries[id]
	if !ok {
		return nil, ErrBeneficiaryNotFound
	}
	return b, nil
}

// ListBeneficiaries 列出受益人
func (m *Manager) ListBeneficiaries() []*Beneficiary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Beneficiary
	for _, b := range m.beneficiaries {
		result = append(result, b)
	}
	return result
}

// CreatePlan 创建遗产计划
func (m *Manager) CreatePlan(plan *LegacyPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()
	m.plans[plan.ID] = plan
	return m.save()
}

// SealPlan 封印计划
func (m *Manager) SealPlan(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	plan, ok := m.plans[id]
	if !ok {
		return ErrPlanNotFound
	}
	if plan.IsSealed {
		return ErrAlreadySealed
	}
	plan.IsSealed = true
	plan.UpdatedAt = time.Now()
	return m.save()
}

// GetPlan 获取计划
func (m *Manager) GetPlan(id string) (*LegacyPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	plan, ok := m.plans[id]
	if !ok {
		return nil, ErrPlanNotFound
	}
	return plan, nil
}

// CheckDeadman 检查遗嘱触发
func (m *Manager) CheckDeadman() []*LegacyPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var triggered []*LegacyPlan
	for _, plan := range m.plans {
		if plan.Deadman != nil && plan.Deadman.Enabled {
			days := int(time.Since(plan.Deadman.LastActive).Hours() / 24)
			if days >= plan.Deadman.InactivityDays {
				triggered = append(triggered, plan)
			}
		}
	}
	return triggered
}

// LogAccess 记录访问
func (m *Manager) LogAccess(log *AccessLog) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Timestamp = time.Now()
	m.accessLogs = append(m.accessLogs, log)
}

// GetAccessLogs 获取访问日志
func (m *Manager) GetAccessLogs(beneficiaryID string) []*AccessLog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*AccessLog
	for _, l := range m.accessLogs {
		if beneficiaryID == "" || l.BeneficiaryID == beneficiaryID {
			result = append(result, l)
		}
	}
	return result
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	verified := 0
	for _, b := range m.beneficiaries {
		if b.Status == BeneficiaryVerified {
			verified++
		}
	}
	return map[string]interface{}{
		"total_beneficiaries": len(m.beneficiaries),
		"verified":            verified,
		"total_plans":         len(m.plans),
		"total_access_logs":   len(m.accessLogs),
	}
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m)
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}

func fmt_Sprintf(f string, a ...interface{}) string { return fmt.Sprintf(f, a...) }
