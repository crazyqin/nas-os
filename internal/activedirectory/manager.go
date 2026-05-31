package activedirectory

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager Active Directory 集成管理器.
type Manager struct {
	mu          sync.RWMutex
	config      ADConfig
	domains     map[string]*Domain
	users       map[string]*ADUser
	groups      map[string]*ADGroup
	syncJobs    map[string]*SyncJob
	running     bool
	stopCh      chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg ADConfig) *Manager {
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = 30 * time.Minute
	}
	if cfg.ConnectionTimeout == 0 {
		cfg.ConnectionTimeout = 30 * time.Second
	}
	if cfg.MaxConnections == 0 {
		cfg.MaxConnections = 10
	}
	if cfg.SearchLimit == 0 {
		cfg.SearchLimit = 1000
	}
	return &Manager{
		config:   cfg,
		domains:  make(map[string]*Domain),
		users:    make(map[string]*ADUser),
		groups:   make(map[string]*ADGroup),
		syncJobs: make(map[string]*SyncJob),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.syncLoop()
	log.Println("[ActiveDirectory] Active Directory 集成已启动")
	return nil
}

// Stop 停止.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Println("[ActiveDirectory] Active Directory 集成已停止")
}

// ========== Domain 管理 ==========

// AddDomain 添加域.
func (m *Manager) AddDomain(domain *Domain) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if domain.Name == "" {
		return fmt.Errorf("域名不能为空")
	}
	if _, exists := m.domains[domain.Name]; exists {
		return fmt.Errorf("域 %s 已存在", domain.Name)
	}
	domain.Status = DomainStatusConnected
	domain.LastSync = time.Now()
	m.domains[domain.Name] = domain
	log.Printf("[ActiveDirectory] 域已添加: %s (Server: %s)", domain.Name, domain.Server)
	return nil
}

// RemoveDomain 移除域.
func (m *Manager) RemoveDomain(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.domains[name]; !exists {
		return fmt.Errorf("域 %s 不存在", name)
	}
	delete(m.domains, name)
	log.Printf("[ActiveDirectory] 域已移除: %s", name)
	return nil
}

// GetDomain 获取域.
func (m *Manager) GetDomain(name string) (*Domain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	domain, exists := m.domains[name]
	if !exists {
		return nil, fmt.Errorf("域 %s 不存在", name)
	}
	return domain, nil
}

// ListDomains 列出所有域.
func (m *Manager) ListDomains() []*Domain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	domains := make([]*Domain, 0, len(m.domains))
	for _, d := range m.domains {
		domains = append(domains, d)
	}
	return domains
}

// ========== 用户同步 ==========

// SyncUsers 同步用户.
func (m *Manager) SyncUsers(domainName string) (*SyncResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	domain, exists := m.domains[domainName]
	if !exists {
		return nil, fmt.Errorf("域 %s 不存在", domainName)
	}
	job := &SyncJob{
		ID:        fmt.Sprintf("sync-%d", time.Now().UnixNano()),
		Domain:    domainName,
		Type:      SyncTypeUsers,
		Status:    SyncStatusRunning,
		StartedAt: time.Now(),
	}
	m.syncJobs[job.ID] = job
	// 模拟同步
	synced := 0
	for i := 0; i < 10; i++ {
		userID := fmt.Sprintf("%s-user-%d", domainName, i)
		m.users[userID] = &ADUser{
			ID:       userID,
			Username: fmt.Sprintf("user%d", i),
			Domain:   domainName,
			Email:    fmt.Sprintf("user%d@%s", i, domain.Name),
			Enabled:  true,
		}
		synced++
	}
	domain.LastSync = time.Now()
	job.Status = SyncStatusCompleted
	job.CompletedAt = time.Now()
	job.RecordsSynced = synced
	log.Printf("[ActiveDirectory] 用户同步完成: %s (%d 条记录)", domainName, synced)
	return &SyncResult{
		JobID:       job.ID,
		Domain:      domainName,
		RecordsSynced: synced,
		Duration:    time.Since(job.StartedAt),
	}, nil
}

// GetADUser 获取 AD 用户.
func (m *Manager) GetADUser(id string) (*ADUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, exists := m.users[id]
	if !exists {
		return nil, fmt.Errorf("用户 %s 不存在", id)
	}
	return user, nil
}

// ListADUsers 列出所有 AD 用户.
func (m *Manager) ListADUsers(domainName string) []*ADUser {
	m.mu.RLock()
	defer m.mu.RUnlock()
	users := make([]*ADUser, 0)
	for _, u := range m.users {
		if domainName == "" || u.Domain == domainName {
			users = append(users, u)
		}
	}
	return users
}

// ========== 组同步 ==========

// SyncGroups 同步组.
func (m *Manager) SyncGroups(domainName string) (*SyncResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.domains[domainName]; !exists {
		return nil, fmt.Errorf("域 %s 不存在", domainName)
	}
	job := &SyncJob{
		ID:        fmt.Sprintf("sync-%d", time.Now().UnixNano()),
		Domain:    domainName,
		Type:      SyncTypeGroups,
		Status:    SyncStatusRunning,
		StartedAt: time.Now(),
	}
	m.syncJobs[job.ID] = job
	synced := 0
	groupNames := []string{"Domain Admins", "Domain Users", "Enterprise Admins", "Schema Admins"}
	for _, name := range groupNames {
		groupID := fmt.Sprintf("%s-group-%s", domainName, name)
		m.groups[groupID] = &ADGroup{
			ID:      groupID,
			Name:    name,
			Domain:  domainName,
			Members: []string{},
		}
		synced++
	}
	job.Status = SyncStatusCompleted
	job.CompletedAt = time.Now()
	job.RecordsSynced = synced
	log.Printf("[ActiveDirectory] 组同步完成: %s (%d 条记录)", domainName, synced)
	return &SyncResult{
		JobID:       job.ID,
		Domain:      domainName,
		RecordsSynced: synced,
		Duration:    time.Since(job.StartedAt),
	}, nil
}

// ========== 统计 ==========

// GetStats 获取统计信息.
func (m *Manager) GetStats() *ADStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	enabledUsers := 0
	for _, u := range m.users {
		if u.Enabled {
			enabledUsers++
		}
	}
	return &ADStats{
		TotalDomains:  len(m.domains),
		TotalUsers:    len(m.users),
		EnabledUsers:  enabledUsers,
		TotalGroups:   len(m.groups),
		TotalSyncJobs: len(m.syncJobs),
	}
}

// ========== 内部方法 ==========

// syncLoop 定期同步循环.
func (m *Manager) syncLoop() {
	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.autoSync()
		}
	}
}

// autoSync 自动同步所有域.
func (m *Manager) autoSync() {
	m.mu.RLock()
	domains := make([]string, 0, len(m.domains))
	for name := range m.domains {
		domains = append(domains, name)
	}
	m.mu.RUnlock()
	for _, name := range domains {
		m.SyncUsers(name)
		m.SyncGroups(name)
	}
}
