// Package familydashboard 家庭仪表板 - 家庭成员共享空间
package familydashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// MemberRole 成员角色
type MemberRole string

const (
	RoleAdmin  MemberRole = "admin"
	RoleParent MemberRole = "parent"
	RoleChild  MemberRole = "child"
	RoleGuest  MemberRole = "guest"
)

// MemberStatus 成员状态
type MemberStatus string

const (
	MemberOnline  MemberStatus = "online"
	MemberOffline MemberStatus = "offline"
	MemberAway    MemberStatus = "away"
)

// FamilyMember 家庭成员
type FamilyMember struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Avatar       string       `json:"avatar,omitempty"`
	Role         MemberRole   `json:"role"`
	Status       MemberStatus `json:"status"`
	Email        string       `json:"email,omitempty"`
	Birthday     string       `json:"birthday,omitempty"`
	StorageQuota int64        `json:"storage_quota"` // bytes
	StorageUsed  int64        `json:"storage_used"`
	CreatedAt    time.Time    `json:"created_at"`
	LastSeen     time.Time    `json:"last_seen"`
}

// Chore 家务任务
type Chore struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	AssigneeID  string     `json:"assignee_id"`
	Points      int        `json:"points"`
	DueDate     string     `json:"due_date,omitempty"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Allowance 零花钱记录
type Allowance struct {
	ID       string    `json:"id"`
	MemberID string    `json:"member_id"`
	Amount   float64   `json:"amount"`
	Reason   string    `json:"reason"`
	Type     string    `json:"type"` // "earn" or "spend"
	Date     time.Time `json:"date"`
}

// FamilyNote 家庭便签
type FamilyNote struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Color     string    `json:"color"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ScreenTime 屏幕时间
type ScreenTime struct {
	MemberID string         `json:"member_id"`
	Date     string         `json:"date"`
	Minutes  int            `json:"minutes"`
	Limit    int            `json:"limit"` // 分钟
	AppUsage map[string]int `json:"app_usage,omitempty"`
}

// Config 配置
type Config struct {
	FamilyName        string `json:"family_name"`
	MaxMembers        int    `json:"max_members"`
	AllowanceCurrency string `json:"allowance_currency"`
}

// Manager 管理器
type Manager struct {
	mu         sync.RWMutex
	members    map[string]*FamilyMember
	chores     map[string]*Chore
	allowances []*Allowance
	notes      map[string]*FamilyNote
	screenTime map[string]*ScreenTime
	config     *Config
	dataFile   string
}

var (
	ErrMemberNotFound  = errors.New("member not found")
	ErrChoreNotFound   = errors.New("chore not found")
	ErrNoteNotFound    = errors.New("note not found")
	ErrMaxMembers      = errors.New("max members reached")
	ErrDuplicateMember = errors.New("member already exists")
)

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		members:    make(map[string]*FamilyMember),
		chores:     make(map[string]*Chore),
		notes:      make(map[string]*FamilyNote),
		screenTime: make(map[string]*ScreenTime),
		config: &Config{
			FamilyName:        "我的家庭",
			MaxMembers:        20,
			AllowanceCurrency: "CNY",
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error { return m.load() }

// AddMember 添加成员
func (m *Manager) AddMember(member *FamilyMember) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.members[member.ID]; exists {
		return ErrDuplicateMember
	}
	if len(m.members) >= m.config.MaxMembers {
		return ErrMaxMembers
	}
	member.Status = MemberOffline
	member.CreatedAt = time.Now()
	m.members[member.ID] = member
	return m.save()
}

// UpdateMember 更新成员
func (m *Manager) UpdateMember(member *FamilyMember) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.members[member.ID]; !ok {
		return ErrMemberNotFound
	}
	m.members[member.ID] = member
	return m.save()
}

// RemoveMember 移除成员
func (m *Manager) RemoveMember(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.members[id]; !ok {
		return ErrMemberNotFound
	}
	delete(m.members, id)
	return m.save()
}

// GetMember 获取成员
func (m *Manager) GetMember(id string) (*FamilyMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	member, ok := m.members[id]
	if !ok {
		return nil, ErrMemberNotFound
	}
	return member, nil
}

// ListMembers 列出成员
func (m *Manager) ListMembers() []*FamilyMember {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*FamilyMember
	for _, m := range m.members {
		result = append(result, m)
	}
	return result
}

// CreateChore 创建家务
func (m *Manager) CreateChore(chore *Chore) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	chore.CreatedAt = time.Now()
	m.chores[chore.ID] = chore
	return m.save()
}

// CompleteChore 完成家务
func (m *Manager) CompleteChore(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	chore, ok := m.chores[id]
	if !ok {
		return ErrChoreNotFound
	}
	now := time.Now()
	chore.Completed = true
	chore.CompletedAt = &now
	return m.save()
}

// ListChores 列出家务
func (m *Manager) ListChores(assigneeID string, pending bool) []*Chore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Chore
	for _, c := range m.chores {
		if assigneeID != "" && c.AssigneeID != assigneeID {
			continue
		}
		if pending && c.Completed {
			continue
		}
		result = append(result, c)
	}
	return result
}

// AddAllowance 添加零花钱
func (m *Manager) AddAllowance(a *Allowance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a.Date = time.Now()
	m.allowances = append(m.allowances, a)
	return m.save()
}

// GetAllowance 获取零花钱记录
func (m *Manager) GetAllowance(memberID string) []*Allowance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Allowance
	for _, a := range m.allowances {
		if memberID == "" || a.MemberID == memberID {
			result = append(result, a)
		}
	}
	return result
}

// GetBalance 获取余额
func (m *Manager) GetBalance(memberID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var balance float64
	for _, a := range m.allowances {
		if a.MemberID == memberID {
			if a.Type == "earn" {
				balance += a.Amount
			} else {
				balance -= a.Amount
			}
		}
	}
	return balance
}

// CreateNote 创建便签
func (m *Manager) CreateNote(note *FamilyNote) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()
	m.notes[note.ID] = note
	return m.save()
}

// UpdateNote 更新便签
func (m *Manager) UpdateNote(note *FamilyNote) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.notes[note.ID]; !ok {
		return ErrNoteNotFound
	}
	note.UpdatedAt = time.Now()
	m.notes[note.ID] = note
	return m.save()
}

// DeleteNote 删除便签
func (m *Manager) DeleteNote(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.notes[id]; !ok {
		return ErrNoteNotFound
	}
	delete(m.notes, id)
	return m.save()
}

// ListNotes 列出便签
func (m *Manager) ListNotes(pinned bool) []*FamilyNote {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*FamilyNote
	for _, n := range m.notes {
		if pinned && !n.Pinned {
			continue
		}
		result = append(result, n)
	}
	return result
}

// SetScreenTime 设置屏幕时间
func (m *Manager) SetScreenTime(st *ScreenTime) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s-%s", st.MemberID, st.Date)
	m.screenTime[key] = st
	return m.save()
}

// GetScreenTime 获取屏幕时间
func (m *Manager) GetScreenTime(memberID, date string) *ScreenTime {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s-%s", memberID, date)
	return m.screenTime[key]
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pending := 0
	for _, c := range m.chores {
		if !c.Completed {
			pending++
		}
	}
	return map[string]interface{}{
		"total_members":    len(m.members),
		"pending_chores":   pending,
		"total_chores":     len(m.chores),
		"total_notes":      len(m.notes),
		"total_allowances": len(m.allowances),
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
