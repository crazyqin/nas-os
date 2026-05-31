package virtualdesktop

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DesktopStatus 虚拟桌面状态
type DesktopStatus string

const (
	StatusRunning  DesktopStatus = "running"
	StatusStopped  DesktopStatus = "stopped"
	StatusPaused   DesktopStatus = "paused"
	StatusCreating DesktopStatus = "creating"
	StatusError    DesktopStatus = "error"
)

// DesktopType 桌面类型
type DesktopType string

const (
	TypeWindows DesktopType = "windows"
	TypeLinux   DesktopType = "linux"
	TypeMacOS   DesktopType = "macos"
	TypeCustom  DesktopType = "custom"
)

// VirtualDesktop 虚拟桌面
type VirtualDesktop struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Type        DesktopType   `json:"type"`
	Status      DesktopStatus `json:"status"`
	Owner       string        `json:"owner"`
	CPU         int           `json:"cpu"`         // vCPU数量
	Memory      int64         `json:"memory"`       // 内存MB
	Disk        int64         `json:"disk"`         // 磁盘GB
	Resolution  string        `json:"resolution"`   // 分辨率
	OSImage     string        `json:"os_image"`     // OS镜像
	IP          string        `json:"ip,omitempty"`
	Port        int           `json:"port,omitempty"`
	Protocol    string        `json:"protocol"`     // rdp, vnc, spice
	Tags        []string      `json:"tags,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
}

// DesktopTemplate 桌面模板
type DesktopTemplate struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        DesktopType `json:"type"`
	OSImage     string      `json:"os_image"`
	DefaultCPU  int         `json:"default_cpu"`
	DefaultMem  int64       `json:"default_memory"`
	DefaultDisk int64       `json:"default_disk"`
	Icon        string      `json:"icon,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
}

// Snapshot 快照
type Snapshot struct {
	ID          string    `json:"id"`
	DesktopID   string    `json:"desktop_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}

// Session 会话
type Session struct {
	ID        string    `json:"id"`
	DesktopID string    `json:"desktop_id"`
	User      string    `json:"user"`
	Protocol  string    `json:"protocol"`
	IP        string    `json:"ip"`
	StartTime time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Active    bool      `json:"active"`
}

// VirtualDesktopManager 虚拟桌面管理器
type VirtualDesktopManager struct {
	mu        sync.RWMutex
	desktops  map[string]*VirtualDesktop
	templates map[string]*DesktopTemplate
	snapshots map[string][]*Snapshot
	sessions  map[string][]*Session
}

// NewVirtualDesktopManager 创建管理器
func NewVirtualDesktopManager() *VirtualDesktopManager {
	m := &VirtualDesktopManager{
		desktops:  make(map[string]*VirtualDesktop),
		templates: make(map[string]*DesktopTemplate),
		snapshots: make(map[string][]*Snapshot),
		sessions:  make(map[string][]*Session),
	}
	
	// 初始化默认模板
	m.initDefaultTemplates()
	return m
}

func (m *VirtualDesktopManager) initDefaultTemplates() {
	templates := []*DesktopTemplate{
		{
			ID:          "tpl-win11",
			Name:        "Windows 11",
			Description: "Windows 11 专业版",
			Type:        TypeWindows,
			OSImage:     "windows-11-pro",
			DefaultCPU:  4,
			DefaultMem:  8192,
			DefaultDisk: 64,
		},
		{
			ID:          "tpl-ubuntu",
			Name:        "Ubuntu 24.04",
			Description: "Ubuntu 24.04 LTS",
			Type:        TypeLinux,
			OSImage:     "ubuntu-24.04",
			DefaultCPU:  2,
			DefaultMem:  4096,
			DefaultDisk: 32,
		},
		{
			ID:          "tpl-debian",
			Name:        "Debian 12",
			Description: "Debian 12 稳定版",
			Type:        TypeLinux,
			OSImage:     "debian-12",
			DefaultCPU:  2,
			DefaultMem:  4096,
			DefaultDisk: 32,
		},
	}
	
	for _, tpl := range templates {
		m.templates[tpl.ID] = tpl
	}
}

// CreateDesktop 创建虚拟桌面
func (m *VirtualDesktopManager) CreateDesktop(ctx context.Context, desktop *VirtualDesktop) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if desktop.ID == "" {
		return fmt.Errorf("desktop ID required")
	}
	
	desktop.Status = StatusCreating
	desktop.CreatedAt = time.Now()
	desktop.UpdatedAt = time.Now()
	if desktop.Protocol == "" {
		desktop.Protocol = "vnc"
	}
	
	m.desktops[desktop.ID] = desktop
	
	// 模拟创建过程
	go func() {
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		if d, ok := m.desktops[desktop.ID]; ok {
			d.Status = StatusStopped
			d.UpdatedAt = time.Now()
		}
	}()
	
	return nil
}

// StartDesktop 启动桌面
func (m *VirtualDesktopManager) StartDesktop(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	desktop, ok := m.desktops[id]
	if !ok {
		return fmt.Errorf("desktop %s not found", id)
	}
	
	if desktop.Status == StatusRunning {
		return fmt.Errorf("desktop already running")
	}
	
	desktop.Status = StatusRunning
	now := time.Now()
	desktop.StartedAt = &now
	desktop.UpdatedAt = now
	
	return nil
}

// StopDesktop 停止桌面
func (m *VirtualDesktopManager) StopDesktop(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	desktop, ok := m.desktops[id]
	if !ok {
		return fmt.Errorf("desktop %s not found", id)
	}
	
	desktop.Status = StatusStopped
	desktop.StartedAt = nil
	desktop.UpdatedAt = time.Now()
	
	return nil
}

// GetDesktop 获取桌面
func (m *VirtualDesktopManager) GetDesktop(ctx context.Context, id string) (*VirtualDesktop, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	desktop, ok := m.desktops[id]
	if !ok {
		return nil, fmt.Errorf("desktop %s not found", id)
	}
	return desktop, nil
}

// ListDesktops 列出桌面
func (m *VirtualDesktopManager) ListDesktops(ctx context.Context, owner string) []*VirtualDesktop {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var result []*VirtualDesktop
	for _, d := range m.desktops {
		if owner != "" && d.Owner != owner {
			continue
		}
		result = append(result, d)
	}
	return result
}

// DeleteDesktop 删除桌面
func (m *VirtualDesktopManager) DeleteDesktop(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.desktops[id]; !ok {
		return fmt.Errorf("desktop %s not found", id)
	}
	
	delete(m.desktops, id)
	delete(m.snapshots, id)
	delete(m.sessions, id)
	
	return nil
}

// CreateSnapshot 创建快照
func (m *VirtualDesktopManager) CreateSnapshot(ctx context.Context, desktopID, name string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.desktops[desktopID]; !ok {
		return nil, fmt.Errorf("desktop %s not found", desktopID)
	}
	
	snapshot := &Snapshot{
		ID:        fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		DesktopID: desktopID,
		Name:      name,
		CreatedAt: time.Now(),
	}
	
	m.snapshots[desktopID] = append(m.snapshots[desktopID], snapshot)
	return snapshot, nil
}

// ListSnapshots 列出快照
func (m *VirtualDesktopManager) ListSnapshots(ctx context.Context, desktopID string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.snapshots[desktopID]
}

// ListTemplates 列出模板
func (m *VirtualDesktopManager) ListTemplates(ctx context.Context) []*DesktopTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var result []*DesktopTemplate
	for _, tpl := range m.templates {
		result = append(result, tpl)
	}
	return result
}

// CreateSession 创建会话
func (m *VirtualDesktopManager) CreateSession(ctx context.Context, desktopID, user, protocol, ip string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	desktop, ok := m.desktops[desktopID]
	if !ok {
		return nil, fmt.Errorf("desktop %s not found", desktopID)
	}
	
	if desktop.Status != StatusRunning {
		return nil, fmt.Errorf("desktop not running")
	}
	
	session := &Session{
		ID:        fmt.Sprintf("sess_%d", time.Now().UnixNano()),
		DesktopID: desktopID,
		User:      user,
		Protocol:  protocol,
		IP:        ip,
		StartTime: time.Now(),
		Active:    true,
	}
	
	m.sessions[desktopID] = append(m.sessions[desktopID], session)
	return session, nil
}
