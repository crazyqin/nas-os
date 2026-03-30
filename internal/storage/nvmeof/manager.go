package nvmeof

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Transport 传输类型.
type Transport string

const (
	// TransportTCP TCP 传输.
	TransportTCP Transport = "tcp"
	// TransportRDMA RDMA 传输.
	TransportRDMA Transport = "rdma"
)

// ServiceStatus 服务状态.
type ServiceStatus string

const (
	// StatusStopped 已停止.
	StatusStopped ServiceStatus = "stopped"
	// StatusRunning 运行中.
	StatusRunning ServiceStatus = "running"
)

var (
	// ErrTargetNotFound Target 不存在.
	ErrTargetNotFound = errors.New("nvmeof target not found")
	// ErrTargetExists Target 已存在.
	ErrTargetExists = errors.New("nvmeof target already exists")
	// ErrInitiatorNotFound Initiator 不存在.
	ErrInitiatorNotFound = errors.New("nvmeof initiator not found")
	// ErrInitiatorExists Initiator 已存在.
	ErrInitiatorExists = errors.New("nvmeof initiator already exists")
)

// Namespace NVMe namespace.
type Namespace struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	DevicePath string    `json:"devicePath"`
	Size       int64     `json:"size"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Target NVMe-oF target.
type Target struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	NQN         string      `json:"nqn"`
	Transport   Transport   `json:"transport"`
	Address     string      `json:"address"`
	Port        int         `json:"port"`
	Namespaces  []Namespace `json:"namespaces"`
	Enabled     bool        `json:"enabled"`
	Description string      `json:"description,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

// Initiator 远端连接配置.
type Initiator struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Transport  Transport `json:"transport"`
	TargetNQN  string    `json:"targetNqn"`
	Address    string    `json:"address"`
	Port       int       `json:"port"`
	HostNQN    string    `json:"hostNqn,omitempty"`
	Connected  bool      `json:"connected"`
	DevicePath string    `json:"devicePath,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Config 持久化配置.
type Config struct {
	Targets    []*Target    `json:"targets"`
	Initiators []*Initiator `json:"initiators"`
}

// Manager NVMe-oF 管理器（当前为安全的内存/文件配置层，后续可接内核 nvmet/nvme-cli）.
type Manager struct {
	mu         sync.RWMutex
	configPath string
	status     ServiceStatus
	targets    map[string]*Target
	initiators map[string]*Initiator
}

// NewManager 创建管理器.
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{
		configPath: configPath,
		status:     StatusStopped,
		targets:    make(map[string]*Target),
		initiators: make(map[string]*Initiator),
	}

	if err := m.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	for _, t := range cfg.Targets {
		cp := *t
		m.targets[t.ID] = &cp
	}
	for _, i := range cfg.Initiators {
		cp := *i
		m.initiators[i.ID] = &cp
	}

	return nil
}

func (m *Manager) save() error {
	targets := make([]*Target, 0, len(m.targets))
	for _, t := range m.targets {
		cp := *t
		targets = append(targets, &cp)
	}
	initiators := make([]*Initiator, 0, len(m.initiators))
	for _, i := range m.initiators {
		cp := *i
		initiators = append(initiators, &cp)
	}

	cfg := Config{
		Targets:    targets,
		Initiators: initiators,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0o600)
}

// Start 启动服务（当前为逻辑状态，后续对接 nvmet）.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = StatusRunning
	return nil
}

// Stop 停止服务.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = StatusStopped
	return nil
}

// GetStatus 获取服务状态.
func (m *Manager) GetStatus() ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// ListTargets 列出 Targets.
func (m *Manager) ListTargets() []*Target {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*Target, 0, len(m.targets))
	for _, t := range m.targets {
		cp := *t
		items = append(items, &cp)
	}
	return items
}

// CreateTarget 创建 Target.
func (m *Manager) CreateTarget(input Target) (*Target, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.targets {
		if t.Name == input.Name || t.NQN == input.NQN {
			return nil, ErrTargetExists
		}
	}

	now := time.Now()
	item := &Target{
		ID:          uuid.NewString(),
		Name:        input.Name,
		NQN:         input.NQN,
		Transport:   input.Transport,
		Address:     input.Address,
		Port:        input.Port,
		Namespaces:  input.Namespaces,
		Enabled:     input.Enabled,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if item.NQN == "" {
		item.NQN = "nqn.2026-03.io.nas-os:" + item.Name
	}
	for idx := range item.Namespaces {
		if item.Namespaces[idx].ID == "" {
			item.Namespaces[idx].ID = uuid.NewString()
		}
		if item.Namespaces[idx].CreatedAt.IsZero() {
			item.Namespaces[idx].CreatedAt = now
		}
	}
	m.targets[item.ID] = item
	return item, m.save()
}

// DeleteTarget 删除 Target.
func (m *Manager) DeleteTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.targets[id]; !ok {
		return ErrTargetNotFound
	}
	delete(m.targets, id)
	return m.save()
}

// GetTarget 获取 Target.
func (m *Manager) GetTarget(id string) (*Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.targets[id]
	if !ok {
		return nil, ErrTargetNotFound
	}
	cp := *item
	return &cp, nil
}

// ListInitiators 列出 Initiators.
func (m *Manager) ListInitiators() []*Initiator {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*Initiator, 0, len(m.initiators))
	for _, i := range m.initiators {
		cp := *i
		items = append(items, &cp)
	}
	return items
}

// CreateInitiator 创建 Initiator.
func (m *Manager) CreateInitiator(input Initiator) (*Initiator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, i := range m.initiators {
		if i.Name == input.Name {
			return nil, ErrInitiatorExists
		}
	}

	now := time.Now()
	item := &Initiator{
		ID:        uuid.NewString(),
		Name:      input.Name,
		Transport: input.Transport,
		TargetNQN: input.TargetNQN,
		Address:   input.Address,
		Port:      input.Port,
		HostNQN:   input.HostNQN,
		Connected: false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.initiators[item.ID] = item
	return item, m.save()
}

// DeleteInitiator 删除 Initiator.
func (m *Manager) DeleteInitiator(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.initiators[id]; !ok {
		return ErrInitiatorNotFound
	}
	delete(m.initiators, id)
	return m.save()
}

// ConnectInitiator 连接 Initiator（当前为模拟连接状态）.
func (m *Manager) ConnectInitiator(id string) (*Initiator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.initiators[id]
	if !ok {
		return nil, ErrInitiatorNotFound
	}
	item.Connected = true
	item.DevicePath = "/dev/nvme-fabrics/" + item.Name
	item.UpdatedAt = time.Now()
	return item, m.save()
}

// DisconnectInitiator 断开 Initiator.
func (m *Manager) DisconnectInitiator(id string) (*Initiator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.initiators[id]
	if !ok {
		return nil, ErrInitiatorNotFound
	}
	item.Connected = false
	item.DevicePath = ""
	item.UpdatedAt = time.Now()
	return item, m.save()
}
