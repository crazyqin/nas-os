// Package contacts 提供联系人管理核心业务逻辑
package contacts

import (
	"encoding/csv"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 联系人管理器.
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	contacts map[string]*Contact
	groups   map[string]*ContactGroup
	shares   map[string]*ShareInfo
	config   *ContactsConfig
}

// ContactsConfig 联系人配置.
type ContactsConfig struct {
	MaxContacts    int     `json:"max_contacts"`
	MaxGroups      int     `json:"max_groups"`
	DefaultCountry string  `json:"default_country"`
	DedupThreshold float64 `json:"dedup_threshold"` // 去重阈值 0-1
}

// DefaultContactsConfig 默认配置.
func DefaultContactsConfig() *ContactsConfig {
	return &ContactsConfig{
		MaxContacts:    10000,
		MaxGroups:      100,
		DefaultCountry: "CN",
		DedupThreshold: 0.8,
	}
}

// NewManager 创建联系人管理器.
func NewManager(logger *zap.Logger, config *ContactsConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultContactsConfig()
	}

	return &Manager{
		logger:   logger,
		config:   config,
		contacts: make(map[string]*Contact),
		groups:   make(map[string]*ContactGroup),
		shares:   make(map[string]*ShareInfo),
	}
}

// generateID 生成唯一 ID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ========== 联系人 CRUD ==========

// CreateContact 创建联系人.
func (m *Manager) CreateContact(req *ContactCreateRequest) (*Contact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.contacts) >= m.config.MaxContacts {
		return nil, fmt.Errorf("maximum contacts limit reached: %d", m.config.MaxContacts)
	}

	contact := &Contact{
		ID:        generateID(),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		NickName:  req.NickName,
		Company:   req.Company,
		JobTitle:  req.JobTitle,
		Phones:    req.Phones,
		Emails:    req.Emails,
		Addresses: req.Addresses,
		Groups:    req.Groups,
		Avatar:    req.Avatar,
		Notes:     req.Notes,
		Birthday:  req.Birthday,
		Website:   req.Website,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.contacts[contact.ID] = contact

	// 自动加入分组
	for _, groupID := range req.Groups {
		if group, ok := m.groups[groupID]; ok {
			group.ContactIDs = append(group.ContactIDs, contact.ID)
			group.UpdatedAt = time.Now()
		}
	}

	m.logger.Info("contact created",
		zap.String("id", contact.ID),
		zap.String("name", contact.FirstName+" "+contact.LastName))

	return contact, nil
}

// GetContact 获取联系人.
func (m *Manager) GetContact(id string) (*Contact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contact, ok := m.contacts[id]
	if !ok {
		return nil, fmt.Errorf("contact not found: %s", id)
	}
	return contact, nil
}

// UpdateContact 更新联系人.
func (m *Manager) UpdateContact(id string, req *ContactUpdateRequest) (*Contact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	contact, ok := m.contacts[id]
	if !ok {
		return nil, fmt.Errorf("contact not found: %s", id)
	}

	// 移除旧分组关联
	for _, groupID := range contact.Groups {
		if group, ok := m.groups[groupID]; ok {
			for i, cid := range group.ContactIDs {
				if cid == id {
					group.ContactIDs = append(group.ContactIDs[:i], group.ContactIDs[i+1:]...)
					break
				}
			}
		}
	}

	contact.FirstName = req.FirstName
	contact.LastName = req.LastName
	contact.NickName = req.NickName
	contact.Company = req.Company
	contact.JobTitle = req.JobTitle
	contact.Phones = req.Phones
	contact.Emails = req.Emails
	contact.Addresses = req.Addresses
	contact.Groups = req.Groups
	contact.Avatar = req.Avatar
	contact.Notes = req.Notes
	contact.Birthday = req.Birthday
	contact.Website = req.Website
	contact.UpdatedAt = time.Now()

	// 加入新分组
	for _, groupID := range req.Groups {
		if group, ok := m.groups[groupID]; ok {
			group.ContactIDs = append(group.ContactIDs, contact.ID)
			group.UpdatedAt = time.Now()
		}
	}

	return contact, nil
}

// DeleteContact 删除联系人.
func (m *Manager) DeleteContact(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	contact, ok := m.contacts[id]
	if !ok {
		return fmt.Errorf("contact not found: %s", id)
	}

	// 从分组中移除
	for _, groupID := range contact.Groups {
		if group, ok := m.groups[groupID]; ok {
			for i, cid := range group.ContactIDs {
				if cid == id {
					group.ContactIDs = append(group.ContactIDs[:i], group.ContactIDs[i+1:]...)
					break
				}
			}
		}
	}

	delete(m.contacts, id)
	m.logger.Info("contact deleted", zap.String("id", id))
	return nil
}

// ListContacts 列出联系人.
func (m *Manager) ListContacts(limit, offset int) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contacts := make([]*Contact, 0, len(m.contacts))
	for _, c := range m.contacts {
		contacts = append(contacts, c)
	}

	// 按创建时间排序
	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].CreatedAt.After(contacts[j].CreatedAt)
	})

	if offset >= len(contacts) {
		return []*Contact{}
	}

	end := offset + limit
	if end > len(contacts) {
		end = len(contacts)
	}

	return contacts[offset:end]
}

// ========== 分组管理 ==========

// CreateGroup 创建分组.
func (m *Manager) CreateGroup(req *ContactGroupCreateRequest) (*ContactGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.groups) >= m.config.MaxGroups {
		return nil, fmt.Errorf("maximum groups limit reached: %d", m.config.MaxGroups)
	}

	group := &ContactGroup{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		ContactIDs:  make([]string, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.groups[group.ID] = group
	return group, nil
}

// GetGroup 获取分组.
func (m *Manager) GetGroup(id string) (*ContactGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", id)
	}
	return group, nil
}

// UpdateGroup 更新分组.
func (m *Manager) UpdateGroup(id string, req *ContactGroupUpdateRequest) (*ContactGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", id)
	}

	group.Name = req.Name
	group.Description = req.Description
	group.Color = req.Color
	group.UpdatedAt = time.Now()

	return group, nil
}

// DeleteGroup 删除分组.
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return fmt.Errorf("group not found: %s", id)
	}

	// 清除联系人的分组引用
	for _, contactID := range group.ContactIDs {
		if contact, ok := m.contacts[contactID]; ok {
			for i, gid := range contact.Groups {
				if gid == id {
					contact.Groups = append(contact.Groups[:i], contact.Groups[i+1:]...)
					break
				}
			}
		}
	}

	delete(m.groups, id)
	return nil
}

// ListGroups 列出所有分组.
func (m *Manager) ListGroups() []*ContactGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*ContactGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// AddContactsToGroup 批量添加联系人到分组.
func (m *Manager) AddContactsToGroup(groupID string, contactIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found: %s", groupID)
	}

	for _, contactID := range contactIDs {
		contact, ok := m.contacts[contactID]
		if !ok {
			continue
		}

		// 检查是否已在分组中
		found := false
		for _, cid := range group.ContactIDs {
			if cid == contactID {
				found = true
				break
			}
		}

		if !found {
			group.ContactIDs = append(group.ContactIDs, contactID)
			contact.Groups = append(contact.Groups, groupID)
		}
	}

	group.UpdatedAt = time.Now()
	return nil
}

// RemoveContactsFromGroup 批量移除联系人.
func (m *Manager) RemoveContactsFromGroup(groupID string, contactIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found: %s", groupID)
	}

	removeSet := make(map[string]bool)
	for _, id := range contactIDs {
		removeSet[id] = true
	}

	newContactIDs := make([]string, 0, len(group.ContactIDs))
	for _, cid := range group.ContactIDs {
		if !removeSet[cid] {
			newContactIDs = append(newContactIDs, cid)
		} else if contact, ok := m.contacts[cid]; ok {
			// 从联系人的分组列表中移除
			for i, gid := range contact.Groups {
				if gid == groupID {
					contact.Groups = append(contact.Groups[:i], contact.Groups[i+1:]...)
					break
				}
			}
		}
	}

	group.ContactIDs = newContactIDs
	group.UpdatedAt = time.Now()
	return nil
}

// ========== 搜索功能 ==========

// SearchContacts 搜索联系人.
func (m *Manager) SearchContacts(req *SearchRequest) []*Contact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*Contact, 0)

	for _, contact := range m.contacts {
		if m.matchesSearch(contact, req) {
			results = append(results, contact)
		}
	}

	// 按创建时间排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// 分页
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := req.Offset

	if offset >= len(results) {
		return []*Contact{}
	}

	end := offset + limit
	if end > len(results) {
		end = len(results)
	}

	return results[offset:end]
}

// matchesSearch 检查联系人是否匹配搜索条件.
func (m *Manager) matchesSearch(contact *Contact, req *SearchRequest) bool {
	query := strings.ToLower(req.Query)

	// 通用查询
	if query != "" {
		name := strings.ToLower(contact.FirstName + " " + contact.LastName)
		nick := strings.ToLower(contact.NickName)
		company := strings.ToLower(contact.Company)

		if !strings.Contains(name, query) &&
			!strings.Contains(nick, query) &&
			!strings.Contains(company, query) {
			// 检查电话和邮箱
			phoneMatch := false
			for _, p := range contact.Phones {
				if strings.Contains(p.Number, query) {
					phoneMatch = true
					break
				}
			}
			emailMatch := false
			for _, e := range contact.Emails {
				if strings.Contains(strings.ToLower(e.Email), query) {
					emailMatch = true
					break
				}
			}
			if !phoneMatch && !emailMatch {
				return false
			}
		}
	}

	// 姓名过滤
	if req.Name != "" {
		name := strings.ToLower(contact.FirstName + " " + contact.LastName)
		if !strings.Contains(name, strings.ToLower(req.Name)) {
			return false
		}
	}

	// 电话过滤
	if req.Phone != "" {
		found := false
		for _, p := range contact.Phones {
			if strings.Contains(p.Number, req.Phone) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 邮箱过滤
	if req.Email != "" {
		found := false
		for _, e := range contact.Emails {
			if strings.Contains(strings.ToLower(e.Email), strings.ToLower(req.Email)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 公司过滤
	if req.Company != "" {
		if !strings.Contains(strings.ToLower(contact.Company), strings.ToLower(req.Company)) {
			return false
		}
	}

	// 分组过滤
	if req.GroupID != "" {
		found := false
		for _, gid := range contact.Groups {
			if gid == req.GroupID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// ========== vCard 支持 ==========

// ExportVCard 导出单个联系人为 vCard.
func (m *Manager) ExportVCard(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contact, ok := m.contacts[id]
	if !ok {
		return "", fmt.Errorf("contact not found: %s", id)
	}

	return m.contactToVCard(contact), nil
}

// ExportVCardBatch 批量导出 vCard.
func (m *Manager) ExportVCardBatch(ids []string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	for _, id := range ids {
		contact, ok := m.contacts[id]
		if !ok {
			continue
		}
		sb.WriteString(m.contactToVCard(contact))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// contactToVCard 联系人转 vCard 格式.
func (m *Manager) contactToVCard(contact *Contact) string {
	var sb strings.Builder

	sb.WriteString("BEGIN:VCARD\n")
	sb.WriteString("VERSION:3.0\n")

	// N: LastName;FirstName;MiddleName;Prefix;Suffix
	fmt.Fprintf(&sb, "N:%s;%s;;;\n", contact.LastName, contact.FirstName)
	fmt.Fprintf(&sb, "FN:%s %s\n", contact.FirstName, contact.LastName)

	if contact.NickName != "" {
		fmt.Fprintf(&sb, "NICKNAME:%s\n", contact.NickName)
	}
	if contact.Company != "" {
		fmt.Fprintf(&sb, "ORG:%s\n", contact.Company)
	}
	if contact.JobTitle != "" {
		fmt.Fprintf(&sb, "TITLE:%s\n", contact.JobTitle)
	}

	// 电话
	for _, phone := range contact.Phones {
		fmt.Fprintf(&sb, "TEL;TYPE=%s:%s\n", phone.Type, phone.Number)
	}

	// 邮箱
	for _, email := range contact.Emails {
		fmt.Fprintf(&sb, "EMAIL;TYPE=%s:%s\n", email.Type, email.Email)
	}

	// 地址
	for _, addr := range contact.Addresses {
		fmt.Fprintf(&sb, "ADR;TYPE=%s:;;%s;%s;%s;%s;%s\n",
			addr.Type, addr.Street, addr.City, addr.State, addr.PostalCode, addr.Country)
	}

	if contact.Birthday != "" {
		fmt.Fprintf(&sb, "BDAY:%s\n", contact.Birthday)
	}
	if contact.Website != "" {
		fmt.Fprintf(&sb, "URL:%s\n", contact.Website)
	}
	if contact.Notes != "" {
		fmt.Fprintf(&sb, "NOTE:%s\n", contact.Notes)
	}

	sb.WriteString("END:VCARD\n")
	return sb.String()
}

// ImportVCard 导入 vCard 数据.
func (m *Manager) ImportVCard(content string, groupID string) (*ImportResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &ImportResult{}
	vcards := parseVCard(content)
	result.Total = len(vcards)

	for _, vc := range vcards {
		contact := m.vcardToContact(vc)

		if groupID != "" {
			contact.Groups = []string{groupID}
		}

		m.contacts[contact.ID] = contact
		result.Imported++

		// 加入分组
		if groupID != "" {
			if group, ok := m.groups[groupID]; ok {
				group.ContactIDs = append(group.ContactIDs, contact.ID)
			}
		}
	}

	return result, nil
}

// parseVCard 解析 vCard 格式.
func parseVCard(content string) []*VCard {
	vcards := make([]*VCard, 0)
	lines := strings.Split(content, "\n")

	var current *VCard

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "BEGIN:VCARD" {
			current = &VCard{}
			continue
		}

		if line == "END:VCARD" {
			if current != nil {
				vcards = append(vcards, current)
			}
			current = nil
			continue
		}

		if current == nil {
			continue
		}

		// 解析字段
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToUpper(parts[0])
		value := parts[1]

		switch {
		case key == "VERSION":
			current.Version = value
		case key == "FN":
			current.FullName = value
		case strings.HasPrefix(key, "N"):
			names := strings.Split(value, ";")
			if len(names) >= 2 {
				current.LastName = names[0]
				current.FirstName = names[1]
			}
		case strings.HasPrefix(key, "TEL"):
			current.Phones = append(current.Phones, Phone{
				Type:   extractType(key),
				Number: value,
			})
		case strings.HasPrefix(key, "EMAIL"):
			current.Emails = append(current.Emails, Email{
				Type:  extractType(key),
				Email: value,
			})
		case key == "ORG":
			current.Org = value
		case key == "TITLE":
			current.Title = value
		case key == "NOTE":
			current.Note = value
		case key == "BDAY":
			current.Birthday = value
		case key == "URL":
			current.URL = value
		}
	}

	return vcards
}

// extractType 从 vCard 字段中提取类型.
func extractType(key string) string {
	re := regexp.MustCompile(`TYPE=(\w+)`)
	matches := re.FindStringSubmatch(key)
	if len(matches) > 1 {
		return strings.ToLower(matches[1])
	}
	return "other"
}

// vcardToContact vCard 转联系人.
func (m *Manager) vcardToContact(vc *VCard) *Contact {
	firstName := vc.FirstName
	if firstName == "" && vc.FullName != "" {
		parts := strings.SplitN(vc.FullName, " ", 2)
		if len(parts) > 0 {
			firstName = parts[0]
		}
	}

	lastName := vc.LastName
	if lastName == "" && vc.FullName != "" {
		parts := strings.SplitN(vc.FullName, " ", 2)
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}

	return &Contact{
		ID:        generateID(),
		FirstName: firstName,
		LastName:  lastName,
		Company:   vc.Org,
		JobTitle:  vc.Title,
		Phones:    vc.Phones,
		Emails:    vc.Emails,
		Addresses: vc.Addresses,
		Notes:     vc.Note,
		Birthday:  vc.Birthday,
		Website:   vc.URL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ========== CSV 导入 ==========

// ImportCSV 导入 CSV 格式.
func (m *Manager) ImportCSV(content string, groupID string) (*ImportResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &ImportResult{}
	reader := csv.NewReader(strings.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return result, nil
	}

	// 跳过表头
	result.Total = len(records) - 1

	for i, record := range records {
		if i == 0 {
			continue // 跳过表头
		}

		if len(record) < 2 {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: insufficient columns", i+1))
			continue
		}

		contact := &Contact{
			ID:        generateID(),
			FirstName: record[0],
			LastName:  record[1],
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if len(record) > 2 {
			contact.Company = record[2]
		}
		if len(record) > 3 {
			contact.Phones = []Phone{{Type: "mobile", Number: record[3]}}
		}
		if len(record) > 4 {
			contact.Emails = []Email{{Type: "work", Email: record[4]}}
		}

		if groupID != "" {
			contact.Groups = []string{groupID}
		}

		m.contacts[contact.ID] = contact
		result.Imported++

		if groupID != "" {
			if group, ok := m.groups[groupID]; ok {
				group.ContactIDs = append(group.ContactIDs, contact.ID)
			}
		}
	}

	return result, nil
}

// ========== 去重功能 ==========

// FindDuplicates 查找重复联系人.
func (m *Manager) FindDuplicates() []*DuplicateContact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	duplicates := make([]*DuplicateContact, 0)
	contacts := make([]*Contact, 0)
	for _, c := range m.contacts {
		contacts = append(contacts, c)
	}

	for i := 0; i < len(contacts); i++ {
		for j := i + 1; j < len(contacts); j++ {
			score, reasons := m.calculateSimilarity(contacts[i], contacts[j])
			if score >= m.config.DedupThreshold {
				duplicates = append(duplicates, &DuplicateContact{
					Contact1: contacts[i],
					Contact2: contacts[j],
					Score:    score,
					Reasons:  reasons,
				})
			}
		}
	}

	sort.Slice(duplicates, func(i, j int) bool {
		return duplicates[i].Score > duplicates[j].Score
	})

	return duplicates
}

// calculateSimilarity 计算两个联系人的相似度.
func (m *Manager) calculateSimilarity(c1, c2 *Contact) (float64, []string) {
	score := 0.0
	maxScore := 0.0
	reasons := make([]string, 0)

	// 姓名相似度 (权重: 0.3)
	nameScore := 0.0
	name1 := strings.ToLower(c1.FirstName + c1.LastName)
	name2 := strings.ToLower(c2.FirstName + c2.LastName)
	if name1 == name2 {
		nameScore = 1.0
		reasons = append(reasons, "姓名完全相同")
	} else if strings.Contains(name1, name2) || strings.Contains(name2, name1) {
		nameScore = 0.7
		reasons = append(reasons, "姓名部分匹配")
	}
	score += nameScore * 0.3
	maxScore += 0.3

	// 电话匹配 (权重: 0.35)
	phoneScore := 0.0
	for _, p1 := range c1.Phones {
		for _, p2 := range c2.Phones {
			if normalizePhone(p1.Number) == normalizePhone(p2.Number) {
				phoneScore = 1.0
				reasons = append(reasons, "电话号码相同")
				break
			}
		}
		if phoneScore > 0 {
			break
		}
	}
	score += phoneScore * 0.35
	maxScore += 0.35

	// 邮箱匹配 (权重: 0.25)
	emailScore := 0.0
	for _, e1 := range c1.Emails {
		for _, e2 := range c2.Emails {
			if strings.EqualFold(e1.Email, e2.Email) {
				emailScore = 1.0
				reasons = append(reasons, "邮箱相同")
				break
			}
		}
		if emailScore > 0 {
			break
		}
	}
	score += emailScore * 0.25
	maxScore += 0.25

	// 公司匹配 (权重: 0.1)
	companyScore := 0.0
	if c1.Company != "" && c2.Company != "" &&
		strings.EqualFold(c1.Company, c2.Company) {
		companyScore = 1.0
		reasons = append(reasons, "公司相同")
	}
	score += companyScore * 0.1
	maxScore += 0.1

	if maxScore == 0 {
		return 0, reasons
	}

	return score / maxScore, reasons
}

// normalizePhone 标准化电话号码.
func normalizePhone(phone string) string {
	re := regexp.MustCompile(`[^\d+]`)
	cleaned := re.ReplaceAllString(phone, "")
	// 去掉国际区号
	if strings.HasPrefix(cleaned, "+86") {
		cleaned = cleaned[3:]
	} else if strings.HasPrefix(cleaned, "86") {
		cleaned = cleaned[2:]
	}
	return cleaned
}

// MergeContacts 合并联系人.
func (m *Manager) MergeContacts(keepID string, mergeIDs []string) (*MergeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keep, ok := m.contacts[keepID]
	if !ok {
		return nil, fmt.Errorf("contact not found: %s", keepID)
	}

	merged := make([]*Contact, 0)
	fieldMap := make(map[string]string)

	for _, mergeID := range mergeIDs {
		if mergeID == keepID {
			continue
		}

		merge, ok := m.contacts[mergeID]
		if !ok {
			continue
		}

		// 合并字段（保留 keep 的，补充 merge 的）
		if keep.Company == "" && merge.Company != "" {
			keep.Company = merge.Company
			fieldMap["company"] = merge.Company
		}
		if keep.JobTitle == "" && merge.JobTitle != "" {
			keep.JobTitle = merge.JobTitle
			fieldMap["job_title"] = merge.JobTitle
		}
		if keep.NickName == "" && merge.NickName != "" {
			keep.NickName = merge.NickName
			fieldMap["nick_name"] = merge.NickName
		}
		if keep.Birthday == "" && merge.Birthday != "" {
			keep.Birthday = merge.Birthday
			fieldMap["birthday"] = merge.Birthday
		}
		if keep.Website == "" && merge.Website != "" {
			keep.Website = merge.Website
			fieldMap["website"] = merge.Website
		}
		if keep.Avatar == "" && merge.Avatar != "" {
			keep.Avatar = merge.Avatar
			fieldMap["avatar"] = merge.Avatar
		}

		// 合并电话（去重）
		phoneSet := make(map[string]bool)
		for _, p := range keep.Phones {
			phoneSet[normalizePhone(p.Number)] = true
		}
		for _, p := range merge.Phones {
			if !phoneSet[normalizePhone(p.Number)] {
				keep.Phones = append(keep.Phones, p)
			}
		}

		// 合并邮箱（去重）
		emailSet := make(map[string]bool)
		for _, e := range keep.Emails {
			emailSet[strings.ToLower(e.Email)] = true
		}
		for _, e := range merge.Emails {
			if !emailSet[strings.ToLower(e.Email)] {
				keep.Emails = append(keep.Emails, e)
			}
		}

		// 合并地址
		keep.Addresses = append(keep.Addresses, merge.Addresses...)

		// 合并分组
		groupSet := make(map[string]bool)
		for _, g := range keep.Groups {
			groupSet[g] = true
		}
		for _, g := range merge.Groups {
			if !groupSet[g] {
				keep.Groups = append(keep.Groups, g)
			}
		}

		// 合并备注
		if keep.Notes == "" {
			keep.Notes = merge.Notes
		} else if merge.Notes != "" {
			keep.Notes += "\n" + merge.Notes
		}

		merged = append(merged, merge)
		delete(m.contacts, mergeID)

		// 从分组中移除被合并的联系人
		for _, groupID := range merge.Groups {
			if group, ok := m.groups[groupID]; ok {
				for i, cid := range group.ContactIDs {
					if cid == mergeID {
						group.ContactIDs = append(group.ContactIDs[:i], group.ContactIDs[i+1:]...)
						break
					}
				}
			}
		}
	}

	keep.UpdatedAt = time.Now()

	return &MergeResult{
		Kept:     keep,
		Merged:   merged,
		FieldMap: fieldMap,
	}, nil
}

// ========== 分享功能 ==========

// ShareGroup 分享联系人组.
func (m *Manager) ShareGroup(req *ShareRequest) (*ShareInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[req.GroupID]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", req.GroupID)
	}

	permission := req.Permission
	if permission == "" {
		permission = "read"
	}

	share := &ShareInfo{
		ID:         generateID(),
		GroupID:    req.GroupID,
		GroupName:  group.Name,
		SharedBy:   "system", // 可以后续扩展为实际用户
		SharedWith: req.TargetUser,
		Permission: permission,
		CreatedAt:  time.Now(),
	}

	m.shares[share.ID] = share

	m.logger.Info("group shared",
		zap.String("group_id", req.GroupID),
		zap.Strings("shared_with", req.TargetUser))

	return share, nil
}

// GetShares 获取分组的分享信息.
func (m *Manager) GetShares(groupID string) []*ShareInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shares := make([]*ShareInfo, 0)
	for _, s := range m.shares {
		if s.GroupID == groupID {
			shares = append(shares, s)
		}
	}
	return shares
}

// RevokeShare 撤销分享.
func (m *Manager) RevokeShare(shareID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.shares[shareID]; !ok {
		return fmt.Errorf("share not found: %s", shareID)
	}

	delete(m.shares, shareID)
	return nil
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_contacts": len(m.contacts),
		"total_groups":   len(m.groups),
		"total_shares":   len(m.shares),
	}
}
