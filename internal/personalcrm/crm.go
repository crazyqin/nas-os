// Package personalcrm 个人关系管理（Personal CRM）
package personalcrm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager CRM管理器
type Manager struct {
	mu            sync.RWMutex
	contacts      map[string]*Contact
	interactions  map[string][]*Interaction // contactID -> interactions
	relationships map[string]*Relationship
	anniversaries map[string]*Anniversary
	reminders     []*Reminder
	logger        Logger
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建CRM管理器
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		contacts:      make(map[string]*Contact),
		interactions:  make(map[string][]*Interaction),
		relationships: make(map[string]*Relationship),
		anniversaries: make(map[string]*Anniversary),
		reminders:     make([]*Reminder, 0),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}

	// 启动提醒检查器
	m.wg.Add(1)
	go m.reminderCheckLoop()

	return m
}

// ========== 联系人管理 ==========

// CreateContact 创建联系人
func (m *Manager) CreateContact(contact *Contact) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if contact.Name == "" {
		return fmt.Errorf("联系人姓名不能为空")
	}

	if contact.ID == "" {
		contact.ID = generateID("contact")
	}
	contact.CreatedAt = time.Now()
	contact.UpdatedAt = time.Now()

	if contact.Tags == nil {
		contact.Tags = []string{}
	}
	if contact.Groups == nil {
		contact.Groups = []ContactGroup{}
	}

	m.contacts[contact.ID] = contact
	m.logger.Info("联系人创建成功: %s (%s)", contact.Name, contact.ID)
	return nil
}

// UpdateContact 更新联系人
func (m *Manager) UpdateContact(contact *Contact) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.contacts[contact.ID]
	if !ok {
		return fmt.Errorf("联系人不存在: %s", contact.ID)
	}

	contact.CreatedAt = existing.CreatedAt
	contact.UpdatedAt = time.Now()
	m.contacts[contact.ID] = contact
	m.logger.Info("联系人更新成功: %s (%s)", contact.Name, contact.ID)
	return nil
}

// DeleteContact 删除联系人
func (m *Manager) DeleteContact(contactID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[contactID]; !ok {
		return fmt.Errorf("联系人不存在: %s", contactID)
	}

	// 删除相关的互动记录
	delete(m.interactions, contactID)

	// 删除相关的关系
	for id, rel := range m.relationships {
		if rel.ContactID1 == contactID || rel.ContactID2 == contactID {
			delete(m.relationships, id)
		}
	}

	// 删除相关的纪念日
	for id, ann := range m.anniversaries {
		if ann.ContactID == contactID {
			delete(m.anniversaries, id)
		}
	}

	delete(m.contacts, contactID)
	m.logger.Info("联系人删除成功: %s", contactID)
	return nil
}

// GetContact 获取联系人
func (m *Manager) GetContact(contactID string) (*Contact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contact, ok := m.contacts[contactID]
	if !ok {
		return nil, fmt.Errorf("联系人不存在: %s", contactID)
	}
	return contact, nil
}

// ListContacts 列出所有联系人
func (m *Manager) ListContacts(group ContactGroup, tag string) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contacts := make([]*Contact, 0)
	for _, contact := range m.contacts {
		// 按分组过滤
		if group != "" {
			found := false
			for _, g := range contact.Groups {
				if g == group {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 按标签过滤
		if tag != "" {
			found := false
			for _, t := range contact.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		contacts = append(contacts, contact)
	}

	// 按姓名排序
	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].Name < contacts[j].Name
	})

	return contacts
}

// SearchContacts 搜索联系人
func (m *Manager) SearchContacts(keyword string) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyword = strings.ToLower(keyword)
	contacts := make([]*Contact, 0)

	for _, contact := range m.contacts {
		if strings.Contains(strings.ToLower(contact.Name), keyword) ||
			strings.Contains(strings.ToLower(contact.Phone), keyword) ||
			strings.Contains(strings.ToLower(contact.Email), keyword) ||
			strings.Contains(strings.ToLower(contact.Company), keyword) {
			contacts = append(contacts, contact)
		}
	}

	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].Name < contacts[j].Name
	})

	return contacts
}

// ========== 互动记录管理 ==========

// AddInteraction 添加互动记录
func (m *Manager) AddInteraction(interaction *Interaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[interaction.ContactID]; !ok {
		return fmt.Errorf("联系人不存在: %s", interaction.ContactID)
	}

	if interaction.ID == "" {
		interaction.ID = generateID("interaction")
	}
	interaction.CreatedAt = time.Now()

	m.interactions[interaction.ContactID] = append(m.interactions[interaction.ContactID], interaction)
	m.logger.Info("互动记录添加成功: %s - %s", interaction.ContactID, interaction.Type)
	return nil
}

// GetInteractions 获取联系人的互动记录
func (m *Manager) GetInteractions(contactID string, limit int) []*Interaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	interactions := m.interactions[contactID]
	if limit > 0 && limit < len(interactions) {
		// 返回最近的记录
		start := len(interactions) - limit
		return interactions[start:]
	}
	return interactions
}

// ========== 关系管理 ==========

// AddRelationship 添加关系
func (m *Manager) AddRelationship(rel *Relationship) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[rel.ContactID1]; !ok {
		return fmt.Errorf("联系人不存在: %s", rel.ContactID1)
	}
	if _, ok := m.contacts[rel.ContactID2]; !ok {
		return fmt.Errorf("联系人不存在: %s", rel.ContactID2)
	}

	if rel.ID == "" {
		rel.ID = generateID("rel")
	}
	rel.CreatedAt = time.Now()

	m.relationships[rel.ID] = rel
	m.logger.Info("关系添加成功: %s - %s (%s)", rel.ContactID1, rel.ContactID2, rel.Type)
	return nil
}

// DeleteRelationship 删除关系
func (m *Manager) DeleteRelationship(relID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.relationships[relID]; !ok {
		return fmt.Errorf("关系不存在: %s", relID)
	}

	delete(m.relationships, relID)
	m.logger.Info("关系删除成功: %s", relID)
	return nil
}

// GetRelationships 获取联系人的所有关系
func (m *Manager) GetRelationships(contactID string) []*Relationship {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rels := make([]*Relationship, 0)
	for _, rel := range m.relationships {
		if rel.ContactID1 == contactID || rel.ContactID2 == contactID {
			rels = append(rels, rel)
		}
	}
	return rels
}

// ========== 纪念日管理 ==========

// AddAnniversary 添加纪念日
func (m *Manager) AddAnniversary(ann *Anniversary) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[ann.ContactID]; !ok {
		return fmt.Errorf("联系人不存在: %s", ann.ContactID)
	}

	if ann.ID == "" {
		ann.ID = generateID("ann")
	}
	ann.CreatedAt = time.Now()

	m.anniversaries[ann.ID] = ann

	// 创建提醒
	m.createReminder(ann.ContactID, "anniversary", ann.Name, ann.Date, ann.RemindDays)

	m.logger.Info("纪念日添加成功: %s - %s", ann.ContactID, ann.Name)
	return nil
}

// GetAnniversaries 获取联系人的纪念日
func (m *Manager) GetAnniversaries(contactID string) []*Anniversary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	anns := make([]*Anniversary, 0)
	for _, ann := range m.anniversaries {
		if ann.ContactID == contactID {
			anns = append(anns, ann)
		}
	}
	return anns
}

// ========== 提醒管理 ==========

// createReminder 创建提醒（内部方法，需在锁内调用）
func (m *Manager) createReminder(contactID, reminderType, title string, eventDate time.Time, remindDays int) {
	now := time.Now()
	remindAt := eventDate.AddDate(0, 0, -remindDays)

	// 如果提醒时间已过，跳过
	if remindAt.Before(now) {
		return
	}

	reminder := &Reminder{
		ID:        generateID("reminder"),
		ContactID: contactID,
		Type:      reminderType,
		Title:     title,
		DueDate:   eventDate,
		RemindAt:  remindAt,
		IsSent:    false,
		CreatedAt: now,
	}
	m.reminders = append(m.reminders, reminder)
}

// GetUpcomingReminders 获取即将到来的提醒
func (m *Manager) GetUpcomingReminders(days int) []*Reminder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	endDate := now.AddDate(0, 0, days)

	reminders := make([]*Reminder, 0)
	for _, r := range m.reminders {
		if !r.IsSent && r.RemindAt.After(now) && r.RemindAt.Before(endDate) {
			reminders = append(reminders, r)
		}
	}

	sort.Slice(reminders, func(i, j int) bool {
		return reminders[i].RemindAt.Before(reminders[j].RemindAt)
	})

	return reminders
}

// reminderCheckLoop 提醒检查循环
func (m *Manager) reminderCheckLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkReminders()
		}
	}
}

// checkReminders 检查提醒
func (m *Manager) checkReminders() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, r := range m.reminders {
		if !r.IsSent && r.RemindAt.Before(now) {
			r.IsSent = true
			m.logger.Info("提醒触发: %s - %s", r.ContactID, r.Title)
		}
	}
}

// ========== 统计分析 ==========

// GetContactStats 获取联系人统计
func (m *Manager) GetContactStats(contactID string) (*ContactStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.contacts[contactID]; !ok {
		return nil, fmt.Errorf("联系人不存在: %s", contactID)
	}

	interactions := m.interactions[contactID]
	stats := &ContactStats{
		ContactID:         contactID,
		TotalInteractions: len(interactions),
	}

	if len(interactions) > 0 {
		// 最近一次互动
		last := interactions[len(interactions)-1]
		stats.LastInteraction = &last.Time

		// 计算互动频率（次/月）
		if len(interactions) >= 2 {
			first := interactions[0]
			months := last.Time.Sub(first.Time).Hours() / 24 / 30
			if months > 0 {
				stats.InteractionFreq = float64(len(interactions)) / months
			}
		}

		// 距上次见面天数
		stats.DaysSinceLastMeet = int(time.Since(last.Time).Hours() / 24)
	}

	// 计算亲密度评分
	stats.ClosenessScore = m.calculateClosenessScore(contactID, interactions)

	return stats, nil
}

// calculateClosenessScore 计算亲密度评分
func (m *Manager) calculateClosenessScore(contactID string, interactions []*Interaction) float64 {
	if len(interactions) == 0 {
		return 0
	}

	score := 0.0

	// 互动次数得分（最多40分）
	interactionScore := float64(len(interactions)) * 2
	if interactionScore > 40 {
		interactionScore = 40
	}
	score += interactionScore

	// 最近互动时间得分（最多30分）
	if len(interactions) > 0 {
		last := interactions[len(interactions)-1]
		daysSince := time.Since(last.Time).Hours() / 24
		if daysSince <= 7 {
			score += 30
		} else if daysSince <= 30 {
			score += 20
		} else if daysSince <= 90 {
			score += 10
		}
	}

	// 互动频率得分（最多20分）
	if len(interactions) >= 2 {
		first := interactions[0]
		last := interactions[len(interactions)-1]
		months := last.Time.Sub(first.Time).Hours() / 24 / 30
		if months > 0 {
			freq := float64(len(interactions)) / months
			freqScore := freq * 5
			if freqScore > 20 {
				freqScore = 20
			}
			score += freqScore
		}
	}

	// 关系类型得分（最多10分）
	for _, rel := range m.relationships {
		if rel.ContactID1 == contactID || rel.ContactID2 == contactID {
			switch rel.Type {
			case RelationSpouse, RelationParent, RelationChild:
				score += 10
			case RelationSibling, RelationFriend:
				score += 5
			}
			break
		}
	}

	if score > 100 {
		score = 100
	}

	return score
}

// GetSystemStats 获取系统统计
func (m *Manager) GetSystemStats() *SystemStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SystemStats{
		TotalContacts:      len(m.contacts),
		TotalRelationships: len(m.relationships),
	}

	// 统计总互动次数
	for _, interactions := range m.interactions {
		stats.TotalInteractions += len(interactions)
	}

	// 计算平均亲密度
	totalScore := 0.0
	count := 0
	for contactID := range m.contacts {
		interactions := m.interactions[contactID]
		score := m.calculateClosenessScore(contactID, interactions)
		totalScore += score
		count++
	}
	if count > 0 {
		stats.AvgClosenessScore = totalScore / float64(count)
	}

	// 统计即将到来的提醒
	now := time.Now()
	endDate := now.AddDate(0, 0, 7)
	for _, r := range m.reminders {
		if !r.IsSent && r.RemindAt.After(now) && r.RemindAt.Before(endDate) {
			stats.UpcomingReminders++
		}
	}

	return stats
}

// ========== HTTP API ==========

// RegisterRoutes 注册HTTP路由
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	// 联系人API
	mux.HandleFunc("/api/crm/contacts", m.handleContacts)
	mux.HandleFunc("/api/crm/contacts/search", m.handleSearchContacts)
	mux.HandleFunc("/api/crm/contacts/stats", m.handleContactStats)

	// 互动记录API
	mux.HandleFunc("/api/crm/interactions", m.handleInteractions)

	// 关系API
	mux.HandleFunc("/api/crm/relationships", m.handleRelationships)

	// 纪念日API
	mux.HandleFunc("/api/crm/anniversaries", m.handleAnniversaries)

	// 提醒API
	mux.HandleFunc("/api/crm/reminders", m.handleReminders)

	// 统计API
	mux.HandleFunc("/api/crm/stats", m.handleSystemStats)
}

func (m *Manager) handleContacts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		group := ContactGroup(r.URL.Query().Get("group"))
		tag := r.URL.Query().Get("tag")
		contacts := m.ListContacts(group, tag)
		writeJSON(w, contacts)

	case http.MethodPost:
		var contact Contact
		if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateContact(&contact); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, contact)

	case http.MethodPut:
		var contact Contact
		if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.UpdateContact(&contact); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, contact)

	case http.MethodDelete:
		contactID := r.URL.Query().Get("id")
		if contactID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		if err := m.DeleteContact(contactID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleSearchContacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keyword := r.URL.Query().Get("q")
	if keyword == "" {
		http.Error(w, "q is required", http.StatusBadRequest)
		return
	}

	contacts := m.SearchContacts(keyword)
	writeJSON(w, contacts)
}

func (m *Manager) handleContactStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contactID := r.URL.Query().Get("id")
	if contactID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	stats, err := m.GetContactStats(contactID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func (m *Manager) handleInteractions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		contactID := r.URL.Query().Get("contact_id")
		if contactID == "" {
			http.Error(w, "contact_id is required", http.StatusBadRequest)
			return
		}
		limit := 50 // 默认返回50条
		interactions := m.GetInteractions(contactID, limit)
		writeJSON(w, interactions)

	case http.MethodPost:
		var interaction Interaction
		if err := json.NewDecoder(r.Body).Decode(&interaction); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddInteraction(&interaction); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, interaction)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleRelationships(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		contactID := r.URL.Query().Get("contact_id")
		if contactID == "" {
			http.Error(w, "contact_id is required", http.StatusBadRequest)
			return
		}
		rels := m.GetRelationships(contactID)
		writeJSON(w, rels)

	case http.MethodPost:
		var rel Relationship
		if err := json.NewDecoder(r.Body).Decode(&rel); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddRelationship(&rel); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, rel)

	case http.MethodDelete:
		relID := r.URL.Query().Get("id")
		if relID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		if err := m.DeleteRelationship(relID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleAnniversaries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		contactID := r.URL.Query().Get("contact_id")
		if contactID == "" {
			http.Error(w, "contact_id is required", http.StatusBadRequest)
			return
		}
		anns := m.GetAnniversaries(contactID)
		writeJSON(w, anns)

	case http.MethodPost:
		var ann Anniversary
		if err := json.NewDecoder(r.Body).Decode(&ann); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddAnniversary(&ann); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, ann)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleReminders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	days := 7 // 默认7天
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	reminders := m.GetUpcomingReminders(days)
	writeJSON(w, reminders)
}

func (m *Manager) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := m.GetSystemStats()
	writeJSON(w, stats)
}

// ========== 工具函数 ==========

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
