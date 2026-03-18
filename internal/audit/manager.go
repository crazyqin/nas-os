package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 审计日志管理器
type Manager struct {
	config     Config
	entries    []*Entry
	mu         sync.RWMutex
	signingKey []byte // 签名密钥
	stopCh     chan struct{}
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		LogPath:           "/var/log/nas-os/audit",
		MaxEntries:        100000,
		MaxAgeDays:        365,
		AutoSave:          true,
		SaveInterval:      time.Minute * 5,
		EnableSignatures:  true,
		EnableCompression: false,
		CompressionType:   "gzip",
		RetentionPolicies: []RetentionPolicy{
			{Category: CategoryAuth, MaxAge: 365, MaxCount: 50000, Compress: true},
			{Category: CategorySecurity, MaxAge: 365, MaxCount: 50000, Compress: true},
			{Category: CategoryAccess, MaxAge: 180, MaxCount: 30000, Compress: true},
			{Category: CategoryData, MaxAge: 90, MaxCount: 20000, Compress: false},
		},
	}
}

// NewManager 创建审计日志管理器
func NewManager(config Config) *Manager {
	m := &Manager{
		config:     config,
		entries:    make([]*Entry, 0),
		signingKey: []byte(uuid.New().String()), // 随机生成签名密钥
		stopCh:     make(chan struct{}),
	}

	// 确保日志目录存在
	if err := os.MkdirAll(config.LogPath, 0750); err != nil {
		// 记录错误但继续运行
		fmt.Printf("[WARN] 无法创建审计日志目录: %v\n", err)
	}

	// 启动自动保存
	if config.AutoSave {
		go m.autoSaveLoop()
	}

	return m
}

// Stop 停止管理器
func (m *Manager) Stop() {
	close(m.stopCh)
}

// SetConfig 设置配置
func (m *Manager) SetConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ========== 日志记录 ==========

// Log 记录审计日志
func (m *Manager) Log(entry *Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil
	}

	// 设置默认值
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Level == "" {
		entry.Level = LevelInfo
	}

	// 生成签名
	if m.config.EnableSignatures {
		entry.Signature = m.generateSignature(entry)
	}

	// 添加到内存
	m.entries = append(m.entries, entry)

	// 限制内存中的日志数量
	if len(m.entries) > m.config.MaxEntries {
		// 保留最新的日志
		m.entries = m.entries[len(m.entries)-m.config.MaxEntries:]
	}

	return nil
}

// LogWithDetails 记录带详细信息的审计日志
func (m *Manager) LogWithDetails(level Level, category Category, event, userID, username, ip, resource, action string, status Status, message string, details map[string]interface{}) error {
	entry := &Entry{
		Level:    level,
		Category: category,
		Event:    event,
		UserID:   userID,
		Username: username,
		IP:       ip,
		Resource: resource,
		Action:   action,
		Status:   status,
		Message:  message,
		Details:  details,
	}
	return m.Log(entry)
}

// ========== 便捷日志方法 ==========

// LogAuth 记录认证事件
func (m *Manager) LogAuth(event, userID, username, ip, userAgent string, status Status, message string, details map[string]interface{}) error {
	entry := &Entry{
		Level:     LevelInfo,
		Category:  CategoryAuth,
		Event:     event,
		UserID:    userID,
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Status:    status,
		Message:   message,
		Details:   details,
	}

	// 认证失败提高日志级别
	if status == StatusFailure {
		entry.Level = LevelWarning
	}

	return m.Log(entry)
}

// LogAccess 记录访问控制事件
func (m *Manager) LogAccess(userID, username, ip, resource, action string, status Status, details map[string]interface{}) error {
	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryAccess,
		Event:    "access_" + action,
		UserID:   userID,
		Username: username,
		IP:       ip,
		Resource: resource,
		Action:   action,
		Status:   status,
		Details:  details,
	}

	if status == StatusFailure {
		entry.Level = LevelWarning
	}

	return m.Log(entry)
}

// LogSecurity 记录安全事件
func (m *Manager) LogSecurity(event, userID, username, ip string, level Level, message string, details map[string]interface{}) error {
	entry := &Entry{
		Level:    level,
		Category: CategorySecurity,
		Event:    event,
		UserID:   userID,
		Username: username,
		IP:       ip,
		Status:   StatusSuccess,
		Message:  message,
		Details:  details,
	}
	return m.Log(entry)
}

// LogDataOperation 记录数据操作
func (m *Manager) LogDataOperation(userID, username, ip, resource, action string, status Status, details map[string]interface{}) error {
	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryData,
		Event:    "data_" + action,
		UserID:   userID,
		Username: username,
		IP:       ip,
		Resource: resource,
		Action:   action,
		Status:   status,
		Details:  details,
	}
	return m.Log(entry)
}

// LogConfigChange 记录配置变更
func (m *Manager) LogConfigChange(userID, username, ip, resource, action string, oldVal, newVal interface{}) error {
	entry := &Entry{
		Level:    LevelWarning, // 配置变更默认为警告级别
		Category: CategorySystem,
		Event:    "config_change",
		UserID:   userID,
		Username: username,
		IP:       ip,
		Resource: resource,
		Action:   action,
		Status:   StatusSuccess,
		Details: map[string]interface{}{
			"old_value": oldVal,
			"new_value": newVal,
		},
	}
	return m.Log(entry)
}

// LogUserOperation 记录用户操作
func (m *Manager) LogUserOperation(operatorID, operatorName, ip, targetUser, action string, status Status, details map[string]interface{}) error {
	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryUser,
		Event:    "user_" + action,
		UserID:   operatorID,
		Username: operatorName,
		IP:       ip,
		Resource: targetUser,
		Action:   action,
		Status:   status,
		Details:  details,
	}
	return m.Log(entry)
}

// ========== 查询功能 ==========

// Query 查询审计日志
func (m *Manager) Query(opts QueryOptions) (*QueryResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled {
		return nil, errors.New(ErrAuditDisabled)
	}

	// 筛选日志
	filtered := make([]*Entry, 0)
	for _, entry := range m.entries {
		if !m.matchesFilters(entry, opts) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// 按时间倒序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// 计算总数
	total := len(filtered)

	// 应用分页
	start := opts.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + opts.Limit
	if opts.Limit <= 0 {
		opts.Limit = 50 // 默认50条
	}
	if end > total {
		end = total
	}

	return &QueryResult{
		Total:   total,
		Entries: filtered[start:end],
	}, nil
}

// GetByID 根据ID获取日志
func (m *Manager) GetByID(id string) (*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, entry := range m.entries {
		if entry.ID == id {
			return entry, nil
		}
	}

	return nil, errors.New(ErrEntryNotFound)
}

// matchesFilters 检查日志是否匹配筛选条件
func (m *Manager) matchesFilters(entry *Entry, opts QueryOptions) bool {
	// 时间范围
	if opts.StartTime != nil && entry.Timestamp.Before(*opts.StartTime) {
		return false
	}
	if opts.EndTime != nil && entry.Timestamp.After(*opts.EndTime) {
		return false
	}

	// 日志级别
	if opts.Level != "" && entry.Level != opts.Level {
		return false
	}

	// 分类
	if opts.Category != "" && entry.Category != opts.Category {
		return false
	}

	// 用户ID
	if opts.UserID != "" && entry.UserID != opts.UserID {
		return false
	}

	// 用户名
	if opts.Username != "" && !strings.Contains(strings.ToLower(entry.Username), strings.ToLower(opts.Username)) {
		return false
	}

	// IP地址
	if opts.IP != "" && entry.IP != opts.IP {
		return false
	}

	// 状态
	if opts.Status != "" && entry.Status != opts.Status {
		return false
	}

	// 事件类型
	if opts.Event != "" && entry.Event != opts.Event {
		return false
	}

	// 资源
	if opts.Resource != "" && !strings.Contains(entry.Resource, opts.Resource) {
		return false
	}

	// 关键词搜索
	if opts.Keyword != "" {
		keyword := strings.ToLower(opts.Keyword)
		if !strings.Contains(strings.ToLower(entry.Message), keyword) &&
			!strings.Contains(strings.ToLower(entry.Event), keyword) &&
			!strings.Contains(strings.ToLower(entry.Resource), keyword) &&
			!strings.Contains(strings.ToLower(entry.Username), keyword) {
			return false
		}
	}

	return true
}

// ========== 统计功能 ==========

// GetStatistics 获取审计统计
func (m *Manager) GetStatistics() *Statistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stats := &Statistics{
		TotalEntries:     len(m.entries),
		EventsByCategory: make(map[string]int),
		EventsByLevel:    make(map[string]int),
		TopUsers:         make([]UserActivity, 0),
		TopIPs:           make([]IPActivity, 0),
	}

	userCounts := make(map[string]*UserActivity)
	ipCounts := make(map[string]*IPActivity)

	for _, entry := range m.entries {
		// 分类统计
		stats.EventsByCategory[string(entry.Category)]++

		// 级别统计
		stats.EventsByLevel[string(entry.Level)]++

		// 今日统计
		if entry.Timestamp.After(todayStart) || entry.Timestamp.Equal(todayStart) {
			stats.TodayEntries++

			if entry.Category == CategoryAuth {
				if entry.Status == StatusFailure {
					stats.FailedAuthToday++
				} else if entry.Status == StatusSuccess && entry.Event == "login" {
					stats.SuccessAuthToday++
				}
			}
		}

		// 用户统计
		if entry.UserID != "" {
			if ua, exists := userCounts[entry.UserID]; exists {
				ua.Count++
			} else {
				userCounts[entry.UserID] = &UserActivity{
					UserID:   entry.UserID,
					Username: entry.Username,
					Count:    1,
				}
			}
		}

		// IP统计
		if entry.IP != "" {
			if ia, exists := ipCounts[entry.IP]; exists {
				ia.Count++
			} else {
				ipCounts[entry.IP] = &IPActivity{IP: entry.IP, Count: 1}
			}
		}

		// 时间范围
		if stats.OldestEntry == nil || entry.Timestamp.Before(*stats.OldestEntry) {
			stats.OldestEntry = &entry.Timestamp
		}
		if stats.NewestEntry == nil || entry.Timestamp.After(*stats.NewestEntry) {
			stats.NewestEntry = &entry.Timestamp
		}
	}

	// 转换用户统计为切片并排序
	for _, ua := range userCounts {
		stats.TopUsers = append(stats.TopUsers, *ua)
	}
	sort.Slice(stats.TopUsers, func(i, j int) bool {
		return stats.TopUsers[i].Count > stats.TopUsers[j].Count
	})
	if len(stats.TopUsers) > 10 {
		stats.TopUsers = stats.TopUsers[:10]
	}

	// 转换IP统计为切片并排序
	for _, ia := range ipCounts {
		stats.TopIPs = append(stats.TopIPs, *ia)
	}
	sort.Slice(stats.TopIPs, func(i, j int) bool {
		return stats.TopIPs[i].Count > stats.TopIPs[j].Count
	})
	if len(stats.TopIPs) > 10 {
		stats.TopIPs = stats.TopIPs[:10]
	}

	return stats
}

// ========== 完整性验证 ==========

// generateSignature 生成签名
func (m *Manager) generateSignature(entry *Entry) string {
	// 创建待签名数据（不包含签名本身）
	signData := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.Level,
		entry.Category,
		entry.Event,
		entry.UserID,
		entry.Resource,
		entry.Status,
	)

	h := hmac.New(sha256.New, m.signingKey)
	h.Write([]byte(signData))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyIntegrity 验证日志完整性
func (m *Manager) VerifyIntegrity() *IntegrityReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &IntegrityReport{
		GeneratedAt:     time.Now(),
		TotalEntries:    len(m.entries),
		Verified:        0,
		Tampered:        0,
		Missing:         0,
		TamperedEntries: make([]TamperedEntry, 0),
	}

	if !m.config.EnableSignatures {
		report.Valid = true
		return report
	}

	for _, entry := range m.entries {
		if entry.Signature == "" {
			report.Missing++
			continue
		}

		expectedSig := m.generateSignature(entry)
		if !hmac.Equal([]byte(entry.Signature), []byte(expectedSig)) {
			report.Tampered++
			report.TamperedEntries = append(report.TamperedEntries, TamperedEntry{
				EntryID:     entry.ID,
				Timestamp:   entry.Timestamp,
				Reason:      "签名不匹配",
				OriginalSig: entry.Signature,
				ComputedSig: expectedSig,
			})
		} else {
			report.Verified++
		}
	}

	report.Valid = report.Tampered == 0 && report.Missing == 0
	return report
}

// ========== 日志清理 ==========

// Cleanup 清理过期日志
func (m *Manager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.MaxAgeDays)
	cleaned := 0

	// 清理内存中的过期日志
	newEntries := make([]*Entry, 0)
	for _, entry := range m.entries {
		if entry.Timestamp.After(cutoff) {
			newEntries = append(newEntries, entry)
		} else {
			cleaned++
		}
	}
	m.entries = newEntries

	// 清理磁盘上的过期日志文件
	if m.config.LogPath != "" {
		// 这里可以添加清理磁盘日志文件的逻辑
		m.cleanupDiskLogs(cutoff)
	}

	return cleaned
}

// cleanupDiskLogs 清理磁盘上的过期日志文件
func (m *Manager) cleanupDiskLogs(cutoff time.Time) {
	entries, err := os.ReadDir(m.config.LogPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			// 清理过期日志文件，忽略错误（文件可能已被删除或权限问题）
			_ = os.Remove(filepath.Join(m.config.LogPath, entry.Name()))
		}
	}
}

// ========== 持久化 ==========

// autoSaveLoop 自动保存循环
func (m *Manager) autoSaveLoop() {
	ticker := time.NewTicker(m.config.SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.save()
		case <-m.stopCh:
			m.save() // 停止前保存一次
			return
		}
	}
}

// save 保存日志到磁盘
func (m *Manager) save() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.AutoSave || len(m.entries) == 0 {
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(m.config.LogPath, 0750); err != nil {
		return
	}

	// 按日期保存
	today := time.Now().Format("2006-01-02")
	filename := filepath.Join(m.config.LogPath, fmt.Sprintf("audit-%s.log", today))

	// 序列化日志
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return
	}

	// 写入文件（覆盖模式，因为内存中已经是完整数据）
	// 保存失败时忽略错误（下次自动保存会重试）
	_ = os.WriteFile(filename, data, 0640)
}

// Load 加载日志
func (m *Manager) Load(date string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filename := filepath.Join(m.config.LogPath, fmt.Sprintf("audit-%s.log", date))
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var entries []*Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	m.entries = append(m.entries, entries...)
	return nil
}

// ========== 导出功能 ==========

// Export 导出日志
func (m *Manager) Export(opts ExportOptions) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 筛选时间范围内的日志
	filtered := make([]*Entry, 0)
	for _, entry := range m.entries {
		if entry.Timestamp.Before(opts.StartTime) || entry.Timestamp.After(opts.EndTime) {
			continue
		}

		// 分类筛选
		if len(opts.Categories) > 0 {
			found := false
			for _, cat := range opts.Categories {
				if entry.Category == cat {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 如果不包含签名，清除签名字段
		exportEntry := entry
		if !opts.IncludeSignatures {
			// 创建副本以避免修改原始数据
			copy := *entry
			copy.Signature = ""
			exportEntry = &copy
		}

		filtered = append(filtered, exportEntry)
	}

	switch opts.Format {
	case ExportJSON:
		return json.MarshalIndent(filtered, "", "  ")
	case ExportCSV:
		return m.exportToCSV(filtered)
	case ExportXML:
		return m.exportToXML(filtered)
	default:
		return json.MarshalIndent(filtered, "", "  ")
	}
}

// exportToCSV 导出为CSV
func (m *Manager) exportToCSV(entries []*Entry) ([]byte, error) {
	var csv strings.Builder
	csv.WriteString("ID,Timestamp,Level,Category,Event,UserID,Username,IP,Resource,Action,Status,Message\n")

	for _, e := range entries {
		fmt.Fprintf(&csv, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			e.ID,
			e.Timestamp.Format(time.RFC3339),
			string(e.Level),
			string(e.Category),
			e.Event,
			e.UserID,
			e.Username,
			e.IP,
			e.Resource,
			e.Action,
			string(e.Status),
			strings.ReplaceAll(e.Message, ",", ";"),
		)
	}

	return []byte(csv.String()), nil
}

// exportToXML 导出为XML
func (m *Manager) exportToXML(entries []*Entry) ([]byte, error) {
	var xml strings.Builder
	xml.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	xml.WriteString("<audit_logs>")

	for _, e := range entries {
		xml.WriteString("<entry>")
		fmt.Fprintf(&xml, "<id>%s</id>", e.ID)
		fmt.Fprintf(&xml, "<timestamp>%s</timestamp>", e.Timestamp.Format(time.RFC3339))
		fmt.Fprintf(&xml, "<level>%s</level>", e.Level)
		fmt.Fprintf(&xml, "<category>%s</category>", e.Category)
		fmt.Fprintf(&xml, "<event>%s</event>", e.Event)
		if e.UserID != "" {
			fmt.Fprintf(&xml, "<user_id>%s</user_id>", e.UserID)
		}
		if e.Username != "" {
			fmt.Fprintf(&xml, "<username>%s</username>", e.Username)
		}
		if e.IP != "" {
			fmt.Fprintf(&xml, "<ip>%s</ip>", e.IP)
		}
		if e.Resource != "" {
			fmt.Fprintf(&xml, "<resource>%s</resource>", e.Resource)
		}
		if e.Action != "" {
			fmt.Fprintf(&xml, "<action>%s</action>", e.Action)
		}
		fmt.Fprintf(&xml, "<status>%s</status>", e.Status)
		if e.Message != "" {
			fmt.Fprintf(&xml, "<message>%s</message>", e.Message)
		}
		xml.WriteString("</entry>")
	}

	xml.WriteString("</audit_logs>")
	return []byte(xml.String()), nil
}
