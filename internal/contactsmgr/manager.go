// Package contactsmgr 提供通讯录管理核心业务逻辑
package contactsmgr

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 通讯录管理器.
type Manager struct {
	contacts map[string]*Contact
	groups   map[string]*ContactGroup
	mu       sync.RWMutex
}

// NewManager 创建通讯录管理器.
func NewManager() *Manager {
	return &Manager{
		contacts: make(map[string]*Contact),
		groups:   make(map[string]*ContactGroup),
	}
}

// ========== 联系人增删改查 ==========

// CreateContact 创建联系人.
func (m *Manager) CreateContact(req CreateContactRequest) *Contact {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	fullName := buildFullName(req.FirstName, req.LastName)

	contact := &Contact{
		ID:         uuid.New().String(),
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		FullName:   fullName,
		Nickname:   req.Nickname,
		Emails:     req.Emails,
		Phones:     req.Phones,
		Addresses:  req.Addresses,
		Company:    req.Company,
		Title:      req.Title,
		Department: req.Department,
		Birthday:   req.Birthday,
		Notes:      req.Notes,
		Tags:       req.Tags,
		Groups:     req.GroupIDs,
		IsFavorite: false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if contact.Emails == nil {
		contact.Emails = []Email{}
	}
	if contact.Phones == nil {
		contact.Phones = []Phone{}
	}
	if contact.Addresses == nil {
		contact.Addresses = []Address{}
	}
	if contact.Tags == nil {
		contact.Tags = []string{}
	}
	if contact.Groups == nil {
		contact.Groups = []string{}
	}

	m.contacts[contact.ID] = contact

	// 更新组的联系人计数
	for _, gid := range req.GroupIDs {
		if g, ok := m.groups[gid]; ok {
			g.ContactCount++
		}
	}

	log.Printf("[contactsmgr] 创建联系人: %s (%s)", contact.FullName, contact.ID)
	return contact
}

// GetContact 获取联系人.
func (m *Manager) GetContact(id string) (*Contact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contact, ok := m.contacts[id]
	if !ok {
		return nil, fmt.Errorf("contact %q not found", id)
	}
	return contact, nil
}

// ListContacts 列出所有联系人.
func (m *Manager) ListContacts() []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contacts := make([]*Contact, 0, len(m.contacts))
	for _, c := range m.contacts {
		contacts = append(contacts, c)
	}

	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].FullName < contacts[j].FullName
	})

	return contacts
}

// UpdateContact 更新联系人.
func (m *Manager) UpdateContact(id string, req UpdateContactRequest) (*Contact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	contact, ok := m.contacts[id]
	if !ok {
		return nil, fmt.Errorf("contact %q not found", id)
	}

	if req.FirstName != nil {
		contact.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		contact.LastName = *req.LastName
	}
	contact.FullName = buildFullName(contact.FirstName, contact.LastName)

	if req.Nickname != nil {
		contact.Nickname = *req.Nickname
	}
	if req.Emails != nil {
		contact.Emails = req.Emails
	}
	if req.Phones != nil {
		contact.Phones = req.Phones
	}
	if req.Addresses != nil {
		contact.Addresses = req.Addresses
	}
	if req.Company != nil {
		contact.Company = *req.Company
	}
	if req.Title != nil {
		contact.Title = *req.Title
	}
	if req.Department != nil {
		contact.Department = *req.Department
	}
	if req.Birthday != nil {
		contact.Birthday = req.Birthday
	}
	if req.Notes != nil {
		contact.Notes = *req.Notes
	}
	if req.Tags != nil {
		contact.Tags = req.Tags
	}
	if req.GroupIDs != nil {
		// 更新组计数
		for _, oldGID := range contact.Groups {
			if g, ok := m.groups[oldGID]; ok {
				g.ContactCount--
			}
		}
		for _, newGID := range req.GroupIDs {
			if g, ok := m.groups[newGID]; ok {
				g.ContactCount++
			}
		}
		contact.Groups = req.GroupIDs
	}
	if req.IsFavorite != nil {
		contact.IsFavorite = *req.IsFavorite
	}

	contact.UpdatedAt = time.Now()

	log.Printf("[contactsmgr] 更新联系人: %s", contact.FullName)
	return contact, nil
}

// DeleteContact 删除联系人.
func (m *Manager) DeleteContact(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	contact, ok := m.contacts[id]
	if !ok {
		return fmt.Errorf("contact %q not found", id)
	}

	// 更新组的联系人计数
	for _, gid := range contact.Groups {
		if g, ok := m.groups[gid]; ok {
			g.ContactCount--
		}
	}

	delete(m.contacts, id)
	log.Printf("[contactsmgr] 删除联系人: %s", contact.FullName)
	return nil
}

// ========== 联系人组管理 ==========

// CreateGroup 创建联系人组.
func (m *Manager) CreateGroup(req CreateGroupRequest) *ContactGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	group := &ContactGroup{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		Color:        req.Color,
		ContactCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.groups[group.ID] = group
	log.Printf("[contactsmgr] 创建联系人组: %s", group.Name)
	return group
}

// GetGroup 获取联系人组.
func (m *Manager) GetGroup(id string) (*ContactGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, fmt.Errorf("group %q not found", id)
	}
	return group, nil
}

// ListGroups 列出所有联系人组.
func (m *Manager) ListGroups() []*ContactGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*ContactGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}

// UpdateGroup 更新联系人组.
func (m *Manager) UpdateGroup(id string, req UpdateGroupRequest) (*ContactGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, fmt.Errorf("group %q not found", id)
	}

	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Description != nil {
		group.Description = *req.Description
	}
	if req.Color != nil {
		group.Color = *req.Color
	}

	group.UpdatedAt = time.Now()

	log.Printf("[contactsmgr] 更新联系人组: %s", group.Name)
	return group, nil
}

// DeleteGroup 删除联系人组.
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[id]; !ok {
		return fmt.Errorf("group %q not found", id)
	}

	// 将组中的联系人移除该组
	for _, contact := range m.contacts {
		var newGroups []string
		for _, gid := range contact.Groups {
			if gid != id {
				newGroups = append(newGroups, gid)
			}
		}
		contact.Groups = newGroups
	}

	delete(m.groups, id)
	log.Printf("[contactsmgr] 删除联系人组: %s", id)
	return nil
}

// AddContactsToGroup 添加联系人到组.
func (m *Manager) AddContactsToGroup(groupID string, contactIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group %q not found", groupID)
	}

	for _, cid := range contactIDs {
		contact, ok := m.contacts[cid]
		if !ok {
			continue
		}

		// 检查是否已在组中
		alreadyInGroup := false
		for _, gid := range contact.Groups {
			if gid == groupID {
				alreadyInGroup = true
				break
			}
		}

		if !alreadyInGroup {
			contact.Groups = append(contact.Groups, groupID)
			group.ContactCount++
		}
	}

	return nil
}

// RemoveContactsFromGroup 从组中移除联系人.
func (m *Manager) RemoveContactsFromGroup(groupID string, contactIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group %q not found", groupID)
	}

	contactIDSet := make(map[string]bool)
	for _, cid := range contactIDs {
		contactIDSet[cid] = true
	}

	for _, contact := range m.contacts {
		var newGroups []string
		removed := false
		for _, gid := range contact.Groups {
			if gid == groupID && contactIDSet[contact.ID] {
				removed = true
			} else {
				newGroups = append(newGroups, gid)
			}
		}
		if removed {
			contact.Groups = newGroups
			group.ContactCount--
		}
	}

	return nil
}

// ListContactsByGroup 列出组中的联系人.
func (m *Manager) ListContactsByGroup(groupID string) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var contacts []*Contact
	for _, c := range m.contacts {
		for _, gid := range c.Groups {
			if gid == groupID {
				contacts = append(contacts, c)
				break
			}
		}
	}

	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].FullName < contacts[j].FullName
	})

	return contacts
}

// ========== vCard 导入导出 ==========

// ImportVCard 导入 vCard.
func (m *Manager) ImportVCard(vcard VCard, groupID string) *Contact {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	fullName := vcard.FullName
	if fullName == "" {
		fullName = buildFullName(vcard.FirstName, vcard.LastName)
	}

	var groups []string
	if groupID != "" {
		groups = []string{groupID}
		if g, ok := m.groups[groupID]; ok {
			g.ContactCount++
		}
	}

	contact := &Contact{
		ID:         uuid.New().String(),
		FirstName:  vcard.FirstName,
		LastName:   vcard.LastName,
		FullName:   fullName,
		Emails:     vcard.Emails,
		Phones:     vcard.Phones,
		Addresses:  vcard.Addresses,
		Company:    vcard.Organization,
		Title:      vcard.Title,
		Birthday:   vcard.Birthday,
		Notes:      vcard.Notes,
		Groups:     groups,
		IsFavorite: false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if contact.Emails == nil {
		contact.Emails = []Email{}
	}
	if contact.Phones == nil {
		contact.Phones = []Phone{}
	}
	if contact.Addresses == nil {
		contact.Addresses = []Address{}
	}
	if contact.Groups == nil {
		contact.Groups = []string{}
	}

	m.contacts[contact.ID] = contact

	log.Printf("[contactsmgr] 导入 vCard 联系人: %s", contact.FullName)
	return contact
}

// ExportVCard 导出 vCard.
func (m *Manager) ExportVCard(contactID string) (*VCard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contact, ok := m.contacts[contactID]
	if !ok {
		return nil, fmt.Errorf("contact %q not found", contactID)
	}

	vcard := &VCard{
		Version:      "3.0",
		FirstName:    contact.FirstName,
		LastName:     contact.LastName,
		FullName:     contact.FullName,
		Emails:       contact.Emails,
		Phones:       contact.Phones,
		Addresses:    contact.Addresses,
		Organization: contact.Company,
		Title:        contact.Title,
		Birthday:     contact.Birthday,
		Notes:        contact.Notes,
	}

	return vcard, nil
}

// ExportMultipleVCard 导出多个 vCard.
func (m *Manager) ExportMultipleVCard(contactIDs []string) ([]*VCard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var vcards []*VCard
	for _, cid := range contactIDs {
		contact, ok := m.contacts[cid]
		if !ok {
			continue
		}

		vcard := &VCard{
			Version:      "3.0",
			FirstName:    contact.FirstName,
			LastName:     contact.LastName,
			FullName:     contact.FullName,
			Emails:       contact.Emails,
			Phones:       contact.Phones,
			Addresses:    contact.Addresses,
			Organization: contact.Company,
			Title:        contact.Title,
			Birthday:     contact.Birthday,
			Notes:        contact.Notes,
		}
		vcards = append(vcards, vcard)
	}

	return vcards, nil
}

// ExportAllVCard 导出所有 vCard.
func (m *Manager) ExportAllVCard() []*VCard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var vcards []*VCard
	for _, contact := range m.contacts {
		vcard := &VCard{
			Version:      "3.0",
			FirstName:    contact.FirstName,
			LastName:     contact.LastName,
			FullName:     contact.FullName,
			Emails:       contact.Emails,
			Phones:       contact.Phones,
			Addresses:    contact.Addresses,
			Organization: contact.Company,
			Title:        contact.Title,
			Birthday:     contact.Birthday,
			Notes:        contact.Notes,
		}
		vcards = append(vcards, vcard)
	}

	return vcards
}

// ========== 联系人搜索 ==========

// SearchContacts 搜索联系人.
func (m *Manager) SearchContacts(query string) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []*Contact

	for _, c := range m.contacts {
		if matchContact(c, queryLower) {
			results = append(results, c)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FullName < results[j].FullName
	})

	return results
}

// SearchContactsInGroup 在指定组中搜索联系人.
func (m *Manager) SearchContactsInGroup(query, groupID string) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []*Contact

	for _, c := range m.contacts {
		// 检查是否在指定组中
		inGroup := false
		for _, gid := range c.Groups {
			if gid == groupID {
				inGroup = true
				break
			}
		}
		if !inGroup {
			continue
		}

		if matchContact(c, queryLower) {
			results = append(results, c)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FullName < results[j].FullName
	})

	return results
}

// matchContact 匹配联系人.
func matchContact(c *Contact, queryLower string) bool {
	// 姓名匹配
	if strings.Contains(strings.ToLower(c.FullName), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(c.FirstName), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(c.LastName), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(c.Nickname), queryLower) {
		return true
	}

	// 公司匹配
	if strings.Contains(strings.ToLower(c.Company), queryLower) {
		return true
	}

	// 邮箱匹配
	for _, e := range c.Emails {
		if strings.Contains(strings.ToLower(e.Address), queryLower) {
			return true
		}
	}

	// 电话匹配
	for _, p := range c.Phones {
		if strings.Contains(p.Number, queryLower) {
			return true
		}
	}

	// 标签匹配
	for _, t := range c.Tags {
		if strings.Contains(strings.ToLower(t), queryLower) {
			return true
		}
	}

	return false
}

// ========== 联系人去重 ==========

// FindDuplicates 查找重复联系人.
func (m *Manager) FindDuplicates(field string) []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var duplicates []*DuplicateGroup

	switch field {
	case "name":
		duplicates = m.findDuplicatesByName()
	case "email":
		duplicates = m.findDuplicatesByEmail()
	case "phone":
		duplicates = m.findDuplicatesByPhone()
	default:
		// auto: 按所有字段查找
		duplicates = m.findDuplicatesByName()
		emailDups := m.findDuplicatesByEmail()
		phoneDups := m.findDuplicatesByPhone()
		duplicates = append(duplicates, emailDups...)
		duplicates = append(duplicates, phoneDups...)
	}

	return duplicates
}

// findDuplicatesByName 按姓名查找重复.
func (m *Manager) findDuplicatesByName() []*DuplicateGroup {
	nameMap := make(map[string][]*Contact)
	for _, c := range m.contacts {
		key := strings.ToLower(strings.TrimSpace(c.FullName))
		if key != "" {
			nameMap[key] = append(nameMap[key], c)
		}
	}

	var duplicates []*DuplicateGroup
	for _, contacts := range nameMap {
		if len(contacts) > 1 {
			duplicates = append(duplicates, &DuplicateGroup{
				Contacts: contacts,
				Field:    "name",
			})
		}
	}

	return duplicates
}

// findDuplicatesByEmail 按邮箱查找重复.
func (m *Manager) findDuplicatesByEmail() []*DuplicateGroup {
	emailMap := make(map[string][]*Contact)
	for _, c := range m.contacts {
		for _, e := range c.Emails {
			key := strings.ToLower(strings.TrimSpace(e.Address))
			if key != "" {
				emailMap[key] = append(emailMap[key], c)
			}
		}
	}

	var duplicates []*DuplicateGroup
	seen := make(map[string]bool)
	for _, contacts := range emailMap {
		if len(contacts) > 1 {
			// 去重（同一组联系人可能因多个邮箱重复）
			key := ""
			for _, c := range contacts {
				key += c.ID + ","
			}
			if !seen[key] {
				seen[key] = true
				duplicates = append(duplicates, &DuplicateGroup{
					Contacts: contacts,
					Field:    "email",
				})
			}
		}
	}

	return duplicates
}

// findDuplicatesByPhone 按电话查找重复.
func (m *Manager) findDuplicatesByPhone() []*DuplicateGroup {
	phoneMap := make(map[string][]*Contact)
	for _, c := range m.contacts {
		for _, p := range c.Phones {
			key := strings.TrimSpace(p.Number)
			if key != "" {
				phoneMap[key] = append(phoneMap[key], c)
			}
		}
	}

	var duplicates []*DuplicateGroup
	seen := make(map[string]bool)
	for _, contacts := range phoneMap {
		if len(contacts) > 1 {
			key := ""
			for _, c := range contacts {
				key += c.ID + ","
			}
			if !seen[key] {
				seen[key] = true
				duplicates = append(duplicates, &DuplicateGroup{
					Contacts: contacts,
					Field:    "phone",
				})
			}
		}
	}

	return duplicates
}

// MergeContacts 合并重复联系人.
func (m *Manager) MergeContacts(primaryID string, mergeIDs []string) (*Contact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	primary, ok := m.contacts[primaryID]
	if !ok {
		return nil, fmt.Errorf("primary contact %q not found", primaryID)
	}

	for _, mid := range mergeIDs {
		merge, ok := m.contacts[mid]
		if !ok {
			continue
		}

		// 合并邮箱
		for _, e := range merge.Emails {
			if !emailExists(primary.Emails, e.Address) {
				primary.Emails = append(primary.Emails, e)
			}
		}

		// 合并电话
		for _, p := range merge.Phones {
			if !phoneExists(primary.Phones, p.Number) {
				primary.Phones = append(primary.Phones, p)
			}
		}

		// 合并地址
		for _, a := range merge.Addresses {
			if !addressExists(primary.Addresses, a) {
				primary.Addresses = append(primary.Addresses, a)
			}
		}

		// 合并标签
		for _, t := range merge.Tags {
			if !tagExists(primary.Tags, t) {
				primary.Tags = append(primary.Tags, t)
			}
		}

		// 合并组
		for _, gid := range merge.Groups {
			if !groupExists(primary.Groups, gid) {
				primary.Groups = append(primary.Groups, gid)
			}
		}

		// 删除被合并的联系人
		delete(m.contacts, mid)
	}

	primary.UpdatedAt = time.Now()
	return primary, nil
}

// ========== 辅助函数 ==========

// buildFullName 构建全名.
func buildFullName(firstName, lastName string) string {
	if lastName == "" {
		return firstName
	}
	if firstName == "" {
		return lastName
	}
	return firstName + " " + lastName
}

func emailExists(emails []Email, address string) bool {
	for _, e := range emails {
		if strings.EqualFold(e.Address, address) {
			return true
		}
	}
	return false
}

func phoneExists(phones []Phone, number string) bool {
	for _, p := range phones {
		if p.Number == number {
			return true
		}
	}
	return false
}

func addressExists(addresses []Address, addr Address) bool {
	for _, a := range addresses {
		if a.Street == addr.Street && a.City == addr.City && a.Country == addr.Country {
			return true
		}
	}
	return false
}

func tagExists(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func groupExists(groups []string, groupID string) bool {
	for _, g := range groups {
		if g == groupID {
			return true
		}
	}
	return false
}

// GetStats 获取通讯录统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	favoriteCount := 0
	totalEmails := 0
	totalPhones := 0

	for _, c := range m.contacts {
		if c.IsFavorite {
			favoriteCount++
		}
		totalEmails += len(c.Emails)
		totalPhones += len(c.Phones)
	}

	return map[string]interface{}{
		"total_contacts": len(m.contacts),
		"total_groups":   len(m.groups),
		"favorites":      favoriteCount,
		"total_emails":   totalEmails,
		"total_phones":   totalPhones,
	}
}
