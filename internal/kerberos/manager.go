package kerberos

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager Kerberos 认证管理器.
type Manager struct {
	mu          sync.RWMutex
	config      KerberosConfig
	principals  map[string]*Principal
	keytabs     map[string]*Keytab
	realm       *Realm
	tickets     map[string]*Ticket
	running     bool
	stopCh      chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg KerberosConfig) *Manager {
	if cfg.TicketLifetime == 0 {
		cfg.TicketLifetime = 10 * time.Hour
	}
	if cfg.RenewLifetime == 0 {
		cfg.RenewLifetime = 7 * 24 * time.Hour
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.CacheSize == 0 {
		cfg.CacheSize = 1000
	}
	return &Manager{
		config:     cfg,
		principals: make(map[string]*Principal),
		keytabs:    make(map[string]*Keytab),
		tickets:    make(map[string]*Ticket),
		stopCh:     make(chan struct{}),
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
	go m.ticketCleanupLoop()
	log.Printf("[Kerberos] Kerberos 认证已启动 (Realm: %s)", m.config.Realm)
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
	log.Println("[Kerberos] Kerberos 认证已停止")
}

// ========== Realm 管理 ==========

// ConfigureRealm 配置 Kerberos Realm.
func (m *Manager) ConfigureRealm(realm *Realm) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if realm.Name == "" {
		return fmt.Errorf("realm 名称不能为空")
	}
	m.realm = realm
	log.Printf("[Kerberos] Realm 已配置: %s (KDC: %s)", realm.Name, realm.KDCHost)
	return nil
}

// GetRealm 获取 Realm 信息.
func (m *Manager) GetRealm() *Realm {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.realm
}

// ========== Principal 管理 ==========

// CreatePrincipal 创建 Principal.
func (m *Manager) CreatePrincipal(principal *Principal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if principal.Name == "" {
		return fmt.Errorf("principal 名称不能为空")
	}
	if _, exists := m.principals[principal.ID]; exists {
		return fmt.Errorf("principal %s 已存在", principal.ID)
	}
	principal.CreatedAt = time.Now()
	principal.UpdatedAt = time.Now()
	if principal.ExpiresAt.IsZero() {
		principal.ExpiresAt = time.Now().Add(365 * 24 * time.Hour)
	}
	m.principals[principal.ID] = principal
	log.Printf("[Kerberos] Principal 已创建: %s@%s", principal.Name, m.config.Realm)
	return nil
}

// GetPrincipal 获取 Principal.
func (m *Manager) GetPrincipal(id string) (*Principal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	principal, exists := m.principals[id]
	if !exists {
		return nil, fmt.Errorf("principal %s 不存在", id)
	}
	return principal, nil
}

// ListPrincipals 列出所有 Principal.
func (m *Manager) ListPrincipals() []*Principal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	principals := make([]*Principal, 0, len(m.principals))
	for _, p := range m.principals {
		principals = append(principals, p)
	}
	return principals
}

// DeletePrincipal 删除 Principal.
func (m *Manager) DeletePrincipal(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.principals[id]; !exists {
		return fmt.Errorf("principal %s 不存在", id)
	}
	delete(m.principals, id)
	log.Printf("[Kerberos] Principal 已删除: %s", id)
	return nil
}

// ========== Keytab 管理 ==========

// CreateKeytab 创建 Keytab.
func (m *Manager) CreateKeytab(keytab *Keytab) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if keytab.PrincipalID == "" {
		return fmt.Errorf("principal ID 不能为空")
	}
	if _, exists := m.principals[keytab.PrincipalID]; !exists {
		return fmt.Errorf("principal %s 不存在", keytab.PrincipalID)
	}
	keytab.CreatedAt = time.Now()
	m.keytabs[keytab.ID] = keytab
	log.Printf("[Kerberos] Keytab 已创建: %s (Principal: %s)", keytab.ID, keytab.PrincipalID)
	return nil
}

// GetKeytab 获取 Keytab.
func (m *Manager) GetKeytab(id string) (*Keytab, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keytab, exists := m.keytabs[id]
	if !exists {
		return nil, fmt.Errorf("keytab %s 不存在", id)
	}
	return keytab, nil
}

// ========== Ticket 管理 ==========

// RequestTicket 请求票据.
func (m *Manager) RequestTicket(principalID string) (*Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	principal, exists := m.principals[principalID]
	if !exists {
		return nil, fmt.Errorf("principal %s 不存在", principalID)
	}
	if principal.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("principal %s 已过期", principalID)
	}
	ticket := &Ticket{
		ID:          fmt.Sprintf("tkt-%d", time.Now().UnixNano()),
		PrincipalID: principalID,
		Service:     fmt.Sprintf("krbtgt/%s@%s", m.config.Realm, m.config.Realm),
		IssuedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(m.config.TicketLifetime),
		Renewable:   m.config.RenewLifetime > 0,
	}
	m.tickets[ticket.ID] = ticket
	log.Printf("[Kerberos] 票据已签发: %s (Principal: %s)", ticket.ID, principal.Name)
	return ticket, nil
}

// ValidateTicket 验证票据.
func (m *Manager) ValidateTicket(ticketID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ticket, exists := m.tickets[ticketID]
	if !exists {
		return false, fmt.Errorf("票据 %s 不存在", ticketID)
	}
	if ticket.ExpiresAt.Before(time.Now()) {
		return false, fmt.Errorf("票据 %s 已过期", ticketID)
	}
	return true, nil
}

// RevokeTicket 吊销票据.
func (m *Manager) RevokeTicket(ticketID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.tickets[ticketID]; !exists {
		return fmt.Errorf("票据 %s 不存在", ticketID)
	}
	delete(m.tickets, ticketID)
	log.Printf("[Kerberos] 票据已吊销: %s", ticketID)
	return nil
}

// ========== 统计 ==========

// GetStats 获取统计信息.
func (m *Manager) GetStats() *KerberosStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	activeTickets := 0
	expiredTickets := 0
	now := time.Now()
	for _, t := range m.tickets {
		if t.ExpiresAt.After(now) {
			activeTickets++
		} else {
			expiredTickets++
		}
	}
	return &KerberosStats{
		Realm:          m.config.Realm,
		TotalPrincipals: len(m.principals),
		TotalKeytabs:   len(m.keytabs),
		TotalTickets:   len(m.tickets),
		ActiveTickets:  activeTickets,
		ExpiredTickets: expiredTickets,
	}
}

// ========== 内部方法 ==========

// ticketCleanupLoop 定期清理过期票据.
func (m *Manager) ticketCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupExpiredTickets()
		}
	}
}

// cleanupExpiredTickets 清理过期票据.
func (m *Manager) cleanupExpiredTickets() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, ticket := range m.tickets {
		if ticket.ExpiresAt.Before(now) {
			delete(m.tickets, id)
		}
	}
}
