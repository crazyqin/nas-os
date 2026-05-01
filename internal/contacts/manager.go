// Package contacts 提供联系人管理功能，对标群晖 Contacts
package contacts

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Contact 联系人.
type Contact struct {
	ID        string    `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	NickName  string    `json:"nick_name,omitempty"`
	Emails    []string  `json:"emails,omitempty"`
	Phones    []string  `json:"phones,omitempty"`
	Company   string    `json:"company,omitempty"`
	Title     string    `json:"title,omitempty"`
	Birthday  string    `json:"birthday,omitempty"`
	Address   string    `json:"address,omitempty"`
	Avatar    string    `json:"avatar,omitempty"`
	Groups    []string  `json:"groups,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Group 联系人分组.
type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	ContactIDs  []string `json:"contact_ids,omitempty"`
}

// CreateContactRequest 创建联系人请求.
type CreateContactRequest struct {
	FirstName string   `json:"first_name" binding:"required"`
	LastName  string   `json:"last_name"`
	NickName  string   `json:"nick_name,omitempty"`
	Emails    []string `json:"emails,omitempty"`
	Phones    []string `json:"phones,omitempty"`
	Company   string   `json:"company,omitempty"`
	Title     string   `json:"title,omitempty"`
	Birthday  string   `json:"birthday,omitempty"`
	Address   string   `json:"address,omitempty"`
	Groups    []string `json:"groups,omitempty"`
	Notes     string   `json:"notes,omitempty"`
}

// UpdateContactRequest 更新联系人请求.
type UpdateContactRequest struct {
	FirstName *string  `json:"first_name,omitempty"`
	LastName  *string  `json:"last_name,omitempty"`
	NickName  *string  `json:"nick_name,omitempty"`
	Emails    []string `json:"emails,omitempty"`
	Phones    []string `json:"phones,omitempty"`
	Company   *string  `json:"company,omitempty"`
	Title     *string  `json:"title,omitempty"`
	Birthday  *string  `json:"birthday,omitempty"`
	Address   *string  `json:"address,omitempty"`
	Groups    []string `json:"groups,omitempty"`
	Notes     *string  `json:"notes,omitempty"`
}

// CreateGroupRequest 创建分组请求.
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
}

// Manager 联系人管理器.
type Manager struct {
	mu       sync.RWMutex
	contacts map[string]*Contact
	groups   map[string]*Group
	counter  int
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		contacts: make(map[string]*Contact),
		groups:   make(map[string]*Group),
	}
}

// CreateContact 创建联系人.
func (m *Manager) CreateContact(req CreateContactRequest) (*Contact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter++
	now := time.Now()
	contact := &Contact{
		ID:        fmt.Sprintf("contact-%d", m.counter),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		NickName:  req.NickName,
		Emails:    req.Emails,
		Phones:    req.Phones,
		Company:   req.Company,
		Title:     req.Title,
		Birthday:  req.Birthday,
		Address:   req.Address,
		Groups:    req.Groups,
		Notes:     req.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.contacts[contact.ID] = contact

	// 添加到分组
	for _, gid := range req.Groups {
		if g, ok := m.groups[gid]; ok {
			g.ContactIDs = append(g.ContactIDs, contact.ID)
		}
	}

	return contact, nil
}

// GetContact 获取联系人.
func (m *Manager) GetContact(id string) (*Contact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.contacts[id]
	if !ok {
		return nil, fmt.Errorf("联系人 %s 不存在", id)
	}
	return c, nil
}

// UpdateContact 更新联系人.
func (m *Manager) UpdateContact(id string, req UpdateContactRequest) (*Contact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contacts[id]
	if !ok {
		return nil, fmt.Errorf("联系人 %s 不存在", id)
	}

	if req.FirstName != nil {
		c.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		c.LastName = *req.LastName
	}
	if req.NickName != nil {
		c.NickName = *req.NickName
	}
	if req.Emails != nil {
		c.Emails = req.Emails
	}
	if req.Phones != nil {
		c.Phones = req.Phones
	}
	if req.Company != nil {
		c.Company = *req.Company
	}
	if req.Title != nil {
		c.Title = *req.Title
	}
	if req.Birthday != nil {
		c.Birthday = *req.Birthday
	}
	if req.Address != nil {
		c.Address = *req.Address
	}
	if req.Groups != nil {
		c.Groups = req.Groups
	}
	if req.Notes != nil {
		c.Notes = *req.Notes
	}
	c.UpdatedAt = time.Now()
	return c, nil
}

// DeleteContact 删除联系人.
func (m *Manager) DeleteContact(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[id]; !ok {
		return fmt.Errorf("联系人 %s 不存在", id)
	}
	delete(m.contacts, id)

	// 从所有分组移除
	for _, g := range m.groups {
		var filtered []string
		for _, cid := range g.ContactIDs {
			if cid != id {
				filtered = append(filtered, cid)
			}
		}
		g.ContactIDs = filtered
	}
	return nil
}

// ListContacts 列出联系人.
func (m *Manager) ListContacts(groupID string) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ids map[string]bool
	if groupID != "" {
		if g, ok := m.groups[groupID]; ok {
			ids = make(map[string]bool)
			for _, cid := range g.ContactIDs {
				ids[cid] = true
			}
		}
	}

	var result []*Contact
	for _, c := range m.contacts {
		if ids == nil || ids[c.ID] {
			result = append(result, c)
		}
	}
	return result
}

// Search 搜索联系人.
func (m *Manager) Search(query string) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(query)
	var result []*Contact
	for _, c := range m.contacts {
		if strings.Contains(strings.ToLower(c.FirstName), q) ||
			strings.Contains(strings.ToLower(c.LastName), q) ||
			strings.Contains(strings.ToLower(c.Company), q) ||
			strings.Contains(strings.ToLower(c.NickName), q) {
			result = append(result, c)
			continue
		}
		for _, e := range c.Emails {
			if strings.Contains(strings.ToLower(e), q) {
				result = append(result, c)
				break
			}
		}
		for _, p := range c.Phones {
			if strings.Contains(p, query) {
				result = append(result, c)
				break
			}
		}
	}
	return result
}

// CreateGroup 创建分组.
func (m *Manager) CreateGroup(req CreateGroupRequest) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter++
	group := &Group{
		ID:          fmt.Sprintf("group-%d", m.counter),
		Name:        req.Name,
		Description: req.Description,
	}
	m.groups[group.ID] = group
	return group, nil
}

// DeleteGroup 删除分组.
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[id]; !ok {
		return fmt.Errorf("分组 %s 不存在", id)
	}
	delete(m.groups, id)
	return nil
}

// ListGroups 列出所有分组.
func (m *Manager) ListGroups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Group
	for _, g := range m.groups {
		result = append(result, g)
	}
	return result
}

// AddToGroup 添加联系人到分组.
func (m *Manager) AddToGroup(contactID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[contactID]; !ok {
		return fmt.Errorf("联系人 %s 不存在", contactID)
	}
	g, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("分组 %s 不存在", groupID)
	}
	for _, cid := range g.ContactIDs {
		if cid == contactID {
			return nil
		}
	}
	g.ContactIDs = append(g.ContactIDs, contactID)
	return nil
}

// RemoveFromGroup 从分组移除联系人.
func (m *Manager) RemoveFromGroup(contactID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("分组 %s 不存在", groupID)
	}
	var filtered []string
	for _, cid := range g.ContactIDs {
		if cid != contactID {
			filtered = append(filtered, cid)
		}
	}
	g.ContactIDs = filtered
	return nil
}

// ExportVCard 导出为 vCard 格式.
func (m *Manager) ExportVCard(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.contacts[id]
	if !ok {
		return "", fmt.Errorf("联系人 %s 不存在", id)
	}

	var vcf strings.Builder
	vcf.WriteString("BEGIN:VCARD\r\n")
	vcf.WriteString("VERSION:3.0\r\n")
	vcf.WriteString(fmt.Sprintf("FN:%s %s\r\n", c.FirstName, c.LastName))
	vcf.WriteString(fmt.Sprintf("N:%s;%s;;;\r\n", c.LastName, c.FirstName))
	if c.Company != "" {
		vcf.WriteString(fmt.Sprintf("ORG:%s\r\n", c.Company))
	}
	if c.Title != "" {
		vcf.WriteString(fmt.Sprintf("TITLE:%s\r\n", c.Title))
	}
	for _, e := range c.Emails {
		vcf.WriteString(fmt.Sprintf("EMAIL:%s\r\n", e))
	}
	for _, p := range c.Phones {
		vcf.WriteString(fmt.Sprintf("TEL:%s\r\n", p))
	}
	if c.Address != "" {
		vcf.WriteString(fmt.Sprintf("ADR:;;%s;;;;\r\n", c.Address))
	}
	if c.Birthday != "" {
		vcf.WriteString(fmt.Sprintf("BDAY:%s\r\n", c.Birthday))
	}
	vcf.WriteString("END:VCARD\r\n")
	return vcf.String(), nil
}

// DetectDuplicates 检测重复联系人.
func (m *Manager) DetectDuplicates() [][]*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type key struct {
		first string
		last  string
	}
	groups := make(map[key][]*Contact)
	for _, c := range m.contacts {
		k := key{strings.ToLower(c.FirstName), strings.ToLower(c.LastName)}
		groups[k] = append(groups[k], c)
	}

	var duplicates [][]*Contact
	for _, g := range groups {
		if len(g) > 1 {
			duplicates = append(duplicates, g)
		}
	}
	return duplicates
}

// ImportVCard 导入 vCard 数据.
func (m *Manager) ImportVCard(data string) ([]*Contact, error) {
	var contacts []*Contact
	blocks := strings.Split(data, "BEGIN:VCARD")
	for _, block := range blocks {
		if !strings.Contains(block, "END:VCARD") {
			continue
		}
		lines := strings.Split(block, "\r\n")
		var fn, org, title string
		var emails, phones []string
		for _, line := range lines {
			if strings.HasPrefix(line, "FN:") {
				fn = strings.TrimPrefix(line, "FN:")
			}
			if strings.HasPrefix(line, "ORG:") {
				org = strings.TrimPrefix(line, "ORG:")
			}
			if strings.HasPrefix(line, "TITLE:") {
				title = strings.TrimPrefix(line, "TITLE:")
			}
			if strings.HasPrefix(line, "EMAIL:") {
				emails = append(emails, strings.TrimPrefix(line, "EMAIL:"))
			}
			if strings.HasPrefix(line, "TEL:") {
				phones = append(phones, strings.TrimPrefix(line, "TEL:"))
			}
		}
		if fn != "" {
			parts := strings.SplitN(fn, " ", 2)
			firstName := parts[0]
			lastName := ""
			if len(parts) > 1 {
				lastName = parts[1]
			}
			c, err := m.CreateContact(CreateContactRequest{
				FirstName: firstName,
				LastName:  lastName,
				Emails:    emails,
				Phones:    phones,
				Company:   org,
				Title:     title,
			})
			if err == nil {
				contacts = append(contacts, c)
			}
		}
	}
	return contacts, nil
}

// BatchDelete 批量删除联系人.
func (m *Manager) BatchDelete(ids []string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, id := range ids {
		if _, ok := m.contacts[id]; ok {
			delete(m.contacts, id)
			count++
		}
	}
	return count
}
