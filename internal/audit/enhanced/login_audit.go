package enhanced

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LoginAuditor 登录审计器.
type LoginAuditor struct {
	entries    []*LoginAuditEntry
	sessions   map[string]*LoginSession
	patterns   map[string]*LoginPattern
	config     LoginAuditConfig
	mu         sync.RWMutex
	storageDir string
	stopCh     chan struct{}
}

// LoginAuditConfig 登录审计配置.
type LoginAuditConfig struct {
	Enabled             bool          `json:"enabled"`
	MaxEntries          int           `json:"max_entries"`
	MaxSessions         int           `json:"max_sessions"`
	SessionTimeout      time.Duration `json:"session_timeout"`
	TrackDevice         bool          `json:"track_device"`
	TrackLocation       bool          `json:"track_location"`
	GeoIPDatabasePath   string        `json:"geoip_database_path"`
	AnomalyThreshold    int           `json:"anomaly_threshold"` // 异常分数阈值
	StoreIPHistory      bool          `json:"store_ip_history"`
	MaxIPHistoryPerUser int           `json:"max_ip_history_per_user"`
	AlertOnAnomaly      bool          `json:"alert_on_anomaly"`
	AlertOnNewDevice    bool          `json:"alert_on_new_device"`
	AlertOnNewLocation  bool          `json:"alert_on_new_location"`
}

// DefaultLoginAuditConfig 默认登录审计配置.
func DefaultLoginAuditConfig() LoginAuditConfig {
	return LoginAuditConfig{
		Enabled:             true,
		MaxEntries:          100000,
		MaxSessions:         10000,
		SessionTimeout:      time.Hour * 24,
		TrackDevice:         true,
		TrackLocation:       true,
		AnomalyThreshold:    70,
		StoreIPHistory:      true,
		MaxIPHistoryPerUser: 100,
		AlertOnAnomaly:      true,
		AlertOnNewDevice:    true,
		AlertOnNewLocation:  true,
	}
}

// NewLoginAuditor 创建登录审计器.
func NewLoginAuditor(config LoginAuditConfig) *LoginAuditor {
	storageDir := "/var/log/nas-os/audit/login"
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		// 如果无法创建存储目录，继续运行但不持久化
		storageDir = ""
	}

	return &LoginAuditor{
		entries:    make([]*LoginAuditEntry, 0),
		sessions:   make(map[string]*LoginSession),
		patterns:   make(map[string]*LoginPattern),
		config:     config,
		storageDir: storageDir,
		stopCh:     make(chan struct{}),
	}
}

// Stop 停止审计器.
func (la *LoginAuditor) Stop() {
	close(la.stopCh)
	la.save()
}

// ========== 登录事件记录 ==========

// RecordLogin 记录登录事件.
func (la *LoginAuditor) RecordLogin(
	userID, username, ip, userAgent string,
	authMethod AuthMethod,
	status string,
	failureReason string,
	deviceID, deviceName string,
) *LoginAuditEntry {
	la.mu.Lock()
	defer la.mu.Unlock()

	if !la.config.Enabled {
		return nil
	}

	now := time.Now()
	entry := &LoginAuditEntry{
		ID:            uuid.New().String(),
		Timestamp:     now,
		EventType:     LoginEventSuccess,
		UserID:        userID,
		Username:      username,
		IP:            ip,
		UserAgent:     userAgent,
		AuthMethod:    authMethod,
		DeviceID:      deviceID,
		DeviceName:    deviceName,
		Status:        status,
		FailureReason: failureReason,
		RiskFactors:   make([]string, 0),
		Metadata:      make(map[string]interface{}),
	}

	// 设置事件类型
	switch status {
	case "failure":
		entry.EventType = LoginEventFailure
	case "success":
		entry.EventType = LoginEventSuccess
	}

	// 获取地理位置
	if la.config.TrackLocation {
		entry.Location = la.getGeoLocation(ip)
	}

	// 获取历史信息
	pattern := la.getOrCreatePattern(userID, username)
	if pattern != nil {
		entry.PreviousIP = pattern.LastLoginIP
		entry.PreviousLogin = &pattern.LastLoginTime
	}

	// 计算风险分数
	entry.RiskScore = la.calculateRiskScore(entry, pattern)
	if entry.RiskScore >= la.config.AnomalyThreshold {
		entry.RiskFactors = append(entry.RiskFactors, "high_risk_score")
	}

	// 检测异常
	la.detectAnomalies(entry, pattern)

	// 保存条目
	la.entries = append(la.entries, entry)
	if len(la.entries) > la.config.MaxEntries {
		la.entries = la.entries[len(la.entries)-la.config.MaxEntries:]
	}

	// 更新模式
	la.updatePattern(entry, pattern)

	// 创建会话（仅成功登录）
	if status == "success" {
		la.createSession(entry)
	}

	return entry
}

// RecordLogout 记录登出事件.
func (la *LoginAuditor) RecordLogout(userID, sessionID, ip string) *LoginAuditEntry {
	la.mu.Lock()
	defer la.mu.Unlock()

	// 更新会话状态
	if session, exists := la.sessions[sessionID]; exists {
		session.IsActive = false
		session.LastActivity = time.Now()
	}

	entry := &LoginAuditEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		EventType: LoginEventLogout,
		UserID:    userID,
		IP:        ip,
		Status:    "success",
	}

	la.entries = append(la.entries, entry)
	return entry
}

// RecordSessionExpired 记录会话过期.
func (la *LoginAuditor) RecordSessionExpired(userID, sessionID string) *LoginAuditEntry {
	la.mu.Lock()
	defer la.mu.Unlock()

	if session, exists := la.sessions[sessionID]; exists {
		session.IsActive = false
	}

	entry := &LoginAuditEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		EventType: LoginEventSessionExpired,
		UserID:    userID,
		SessionID: sessionID,
		Status:    "success",
	}

	la.entries = append(la.entries, entry)
	return entry
}

// RecordPasswordChange 记录密码修改.
func (la *LoginAuditor) RecordPasswordChange(userID, username, ip string, success bool) *LoginAuditEntry {
	la.mu.Lock()
	defer la.mu.Unlock()

	entry := &LoginAuditEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		EventType: LoginEventPasswordChange,
		UserID:    userID,
		Username:  username,
		IP:        ip,
		Status:    "success",
	}

	if !success {
		entry.Status = "failure"
	}

	la.entries = append(la.entries, entry)
	return entry
}

// RecordMFAChange 记录MFA状态变更.
func (la *LoginAuditor) RecordMFAChange(userID, username, ip string, enabled bool) *LoginAuditEntry {
	la.mu.Lock()
	defer la.mu.Unlock()

	eventType := LoginEventMFAEnabled
	if !enabled {
		eventType = LoginEventMFADisabled
	}

	entry := &LoginAuditEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		EventType: eventType,
		UserID:    userID,
		Username:  username,
		IP:        ip,
		Status:    "success",
	}

	la.entries = append(la.entries, entry)
	return entry
}

// RecordAccountLock 记录账户锁定.
func (la *LoginAuditor) RecordAccountLock(userID, username, ip, reason string) *LoginAuditEntry {
	la.mu.Lock()
	defer la.mu.Unlock()

	entry := &LoginAuditEntry{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		EventType:     LoginEventAccountLocked,
		UserID:        userID,
		Username:      username,
		IP:            ip,
		Status:        "success",
		FailureReason: reason,
		RiskFactors:   []string{"account_locked"},
		RiskScore:     80,
	}

	la.entries = append(la.entries, entry)
	return entry
}

// RecordAccountUnlock 记录账户解锁.
func (la *LoginAuditor) RecordAccountUnlock(userID, username, operatorID string) *LoginAuditEntry {
	la.mu.Lock()
	defer la.mu.Unlock()

	entry := &LoginAuditEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		EventType: LoginEventAccountUnlocked,
		UserID:    userID,
		Username:  username,
		Status:    "success",
		Metadata: map[string]interface{}{
			"operator_id": operatorID,
		},
	}

	la.entries = append(la.entries, entry)
	return entry
}

// ========== 会话管理 ==========

// createSession 创建会话.
func (la *LoginAuditor) createSession(entry *LoginAuditEntry) *LoginSession {
	sessionID := uuid.New().String()
	session := &LoginSession{
		SessionID:    sessionID,
		UserID:       entry.UserID,
		Username:     entry.Username,
		IP:           entry.IP,
		UserAgent:    entry.UserAgent,
		DeviceID:     entry.DeviceID,
		DeviceName:   entry.DeviceName,
		AuthMethod:   entry.AuthMethod,
		LoginTime:    entry.Timestamp,
		LastActivity: entry.Timestamp,
		ExpiresAt:    entry.Timestamp.Add(la.config.SessionTimeout),
		IsActive:     true,
		Location:     entry.Location,
		RiskScore:    entry.RiskScore,
		RiskFactors:  entry.RiskFactors,
	}

	la.sessions[sessionID] = session
	entry.SessionID = sessionID

	// 限制会话数量
	if len(la.sessions) > la.config.MaxSessions {
		la.cleanupExpiredSessions()
	}

	return session
}

// GetActiveSession 获取活跃会话.
func (la *LoginAuditor) GetActiveSession(sessionID string) *LoginSession {
	la.mu.RLock()
	defer la.mu.RUnlock()

	session, exists := la.sessions[sessionID]
	if !exists || !session.IsActive {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		session.IsActive = false
		return nil
	}

	return session
}

// GetUserActiveSessions 获取用户的所有活跃会话.
func (la *LoginAuditor) GetUserActiveSessions(userID string) []*LoginSession {
	la.mu.RLock()
	defer la.mu.RUnlock()

	sessions := make([]*LoginSession, 0)
	for _, session := range la.sessions {
		if session.UserID == userID && session.IsActive && time.Now().Before(session.ExpiresAt) {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// UpdateSessionActivity 更新会话活动时间.
func (la *LoginAuditor) UpdateSessionActivity(sessionID string) {
	la.mu.Lock()
	defer la.mu.Unlock()

	if session, exists := la.sessions[sessionID]; exists && session.IsActive {
		session.LastActivity = time.Now()
		session.ExpiresAt = time.Now().Add(la.config.SessionTimeout)
	}
}

// TerminateSession 终止会话.
func (la *LoginAuditor) TerminateSession(sessionID string) bool {
	la.mu.Lock()
	defer la.mu.Unlock()

	if session, exists := la.sessions[sessionID]; exists {
		session.IsActive = false
		return true
	}
	return false
}

// TerminateAllUserSessions 终止用户所有会话.
func (la *LoginAuditor) TerminateAllUserSessions(userID string) int {
	la.mu.Lock()
	defer la.mu.Unlock()

	count := 0
	for _, session := range la.sessions {
		if session.UserID == userID && session.IsActive {
			session.IsActive = false
			count++
		}
	}
	return count
}

// cleanupExpiredSessions 清理过期会话.
func (la *LoginAuditor) cleanupExpiredSessions() {
	now := time.Now()
	for id, session := range la.sessions {
		if !session.IsActive || now.After(session.ExpiresAt) {
			delete(la.sessions, id)
		}
	}
}

// ========== 模式分析 ==========

// getOrCreatePattern 获取或创建用户登录模式.
func (la *LoginAuditor) getOrCreatePattern(userID, username string) *LoginPattern {
	if pattern, exists := la.patterns[userID]; exists {
		return pattern
	}

	pattern := &LoginPattern{
		UserID:   userID,
		Username: username,
	}
	la.patterns[userID] = pattern
	return pattern
}

// updatePattern 更新登录模式.
func (la *LoginAuditor) updatePattern(entry *LoginAuditEntry, pattern *LoginPattern) {
	if pattern == nil {
		return
	}

	pattern.TotalLogins++
	if entry.Status == "success" {
		pattern.SuccessfulLogins++
		pattern.LastLoginTime = entry.Timestamp
		pattern.LastLoginIP = entry.IP
	} else {
		pattern.FailedLogins++
	}

	if entry.RiskScore >= la.config.AnomalyThreshold {
		pattern.AnomalousLogins++
	}
}

// GetLoginPattern 获取用户登录模式.
func (la *LoginAuditor) GetLoginPattern(userID string) *LoginPattern {
	la.mu.RLock()
	defer la.mu.RUnlock()
	return la.patterns[userID]
}

// ========== 风险分析 ==========

// calculateRiskScore 计算风险分数.
func (la *LoginAuditor) calculateRiskScore(entry *LoginAuditEntry, pattern *LoginPattern) int {
	score := 0

	if pattern == nil {
		// 新用户，基础风险
		return 20
	}

	// 失败登录历史
	if pattern.FailedLogins > 5 {
		score += 20
	} else if pattern.FailedLogins > 3 {
		score += 10
	}

	// 新IP检测
	if la.config.StoreIPHistory && pattern.MostUsedIP != "" && entry.IP != pattern.MostUsedIP {
		score += 15
	}

	// 异常时间（假设正常工作时间是8:00-22:00）
	hour := entry.Timestamp.Hour()
	if hour < 6 || hour > 23 {
		score += 10
	}

	// 认证方式风险
	if entry.AuthMethod == AuthMethodPassword {
		// 仅密码认证风险稍高
		if pattern.SuccessfulLogins > 10 {
			// 老用户使用密码，基础风险
			score += 5
		}
	}

	// 地理位置风险
	if entry.Location != nil && pattern.MostUsedLocation != "" {
		if entry.Location.Country != pattern.MostUsedLocation {
			score += 25
		}
	}

	// 新设备检测
	if la.config.TrackDevice && pattern.MostUsedDevice != "" && entry.DeviceID != pattern.MostUsedDevice {
		score += 15
	}

	// 限制最大分数
	if score > 100 {
		score = 100
	}

	return score
}

// detectAnomalies 检测异常.
func (la *LoginAuditor) detectAnomalies(entry *LoginAuditEntry, pattern *LoginPattern) {
	if pattern == nil {
		return
	}

	// 检测暴力破解
	if pattern.FailedLogins > 5 && entry.Status == "failure" {
		entry.RiskFactors = append(entry.RiskFactors, "potential_brute_force")
		entry.RiskScore += 30
	}

	// 检测异地登录
	if entry.Location != nil && pattern.MostUsedLocation != "" {
		if entry.Location.Country != pattern.MostUsedLocation {
			entry.RiskFactors = append(entry.RiskFactors, "unusual_location")
		}
	}

	// 检测新设备
	if la.config.TrackDevice && pattern.MostUsedDevice != "" {
		if entry.DeviceID != "" && entry.DeviceID != pattern.MostUsedDevice {
			entry.RiskFactors = append(entry.RiskFactors, "new_device")
		}
	}

	// 检测快速连续登录失败
	if len(la.entries) > 0 {
		recentFailures := 0
		for i := len(la.entries) - 1; i >= 0 && i >= len(la.entries)-10; i-- {
			e := la.entries[i]
			if e.UserID == entry.UserID && e.Status == "failure" && time.Since(e.Timestamp) < time.Minute*5 {
				recentFailures++
			}
		}
		if recentFailures >= 3 {
			entry.RiskFactors = append(entry.RiskFactors, "rapid_failures")
		}
	}
}

// ========== 查询功能 ==========

// Query 查询登录审计日志.
func (la *LoginAuditor) Query(opts LoginQueryOptions) ([]*LoginAuditEntry, int) {
	la.mu.RLock()
	defer la.mu.RUnlock()

	filtered := make([]*LoginAuditEntry, 0)
	for _, entry := range la.entries {
		if !la.matchesFilter(entry, opts) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// 按时间倒序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	total := len(filtered)

	// 分页
	start := opts.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + opts.Limit
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

// matchesFilter 检查是否匹配筛选条件.
func (la *LoginAuditor) matchesFilter(entry *LoginAuditEntry, opts LoginQueryOptions) bool {
	if opts.StartTime != nil && entry.Timestamp.Before(*opts.StartTime) {
		return false
	}
	if opts.EndTime != nil && entry.Timestamp.After(*opts.EndTime) {
		return false
	}
	if opts.UserID != "" && entry.UserID != opts.UserID {
		return false
	}
	if opts.Username != "" && !strings.Contains(strings.ToLower(entry.Username), strings.ToLower(opts.Username)) {
		return false
	}
	if opts.IP != "" && entry.IP != opts.IP {
		return false
	}
	if opts.EventType != "" && entry.EventType != opts.EventType {
		return false
	}
	if opts.AuthMethod != "" && entry.AuthMethod != opts.AuthMethod {
		return false
	}
	if opts.Status != "" && entry.Status != opts.Status {
		return false
	}
	if opts.MinRiskScore > 0 && entry.RiskScore < opts.MinRiskScore {
		return false
	}
	return true
}

// GetByID 根据ID获取登录审计条目.
func (la *LoginAuditor) GetByID(id string) *LoginAuditEntry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	for _, entry := range la.entries {
		if entry.ID == id {
			return entry
		}
	}
	return nil
}

// GetHighRiskLogins 获取高风险登录.
func (la *LoginAuditor) GetHighRiskLogins(minScore int, limit int) []*LoginAuditEntry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	result := make([]*LoginAuditEntry, 0)
	for _, entry := range la.entries {
		if entry.RiskScore >= minScore {
			result = append(result, entry)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].RiskScore > result[j].RiskScore
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// ========== 统计功能 ==========

// GetLoginStatistics 获取登录统计.
func (la *LoginAuditor) GetLoginStatistics(start, end time.Time) *LoginAnalysis {
	la.mu.RLock()
	defer la.mu.RUnlock()

	analysis := &LoginAnalysis{
		FailedByReason:  make(map[string]int),
		PeakLoginHours:  make([]int, 0),
		TopLocations:    make([]LocationCount, 0),
		TopDevices:      make([]DeviceCount, 0),
		AnomalousLogins: make([]*LoginAuditEntry, 0),
	}

	users := make(map[string]bool)
	ips := make(map[string]bool)
	devices := make(map[string]int)
	locations := make(map[string]int)
	hours := make(map[int]int)
	totalDuration := 0
	sessionCount := 0

	for _, entry := range la.entries {
		if entry.Timestamp.Before(start) || entry.Timestamp.After(end) {
			continue
		}

		analysis.TotalLogins++

		if entry.Status == "success" {
			analysis.SuccessfulLogins++
			users[entry.UserID] = true
			ips[entry.IP] = true

			if entry.DeviceID != "" {
				devices[entry.DeviceName]++
			}

			if entry.Location != nil {
				loc := entry.Location.Country
				if entry.Location.City != "" {
					loc = entry.Location.City + ", " + loc
				}
				locations[loc]++
			}

			hours[entry.Timestamp.Hour()]++
		} else {
			analysis.FailedLogins++
			reason := entry.FailureReason
			if reason == "" {
				reason = "unknown"
			}
			analysis.FailedByReason[reason]++
		}

		if entry.RiskScore >= la.config.AnomalyThreshold {
			analysis.AnomalousLogins = append(analysis.AnomalousLogins, entry)
		}
	}

	analysis.UniqueUsers = len(users)
	analysis.UniqueIPs = len(ips)
	analysis.UniqueDevices = len(devices)

	// 计算平均会话时长
	for _, session := range la.sessions {
		if session.IsActive {
			continue
		}
		duration := session.LastActivity.Sub(session.LoginTime)
		totalDuration += int(duration.Minutes())
		sessionCount++
	}
	if sessionCount > 0 {
		analysis.AvgSessionDuration = totalDuration / sessionCount
	}

	// 计算MFA使用率
	mfaLogins := 0
	for _, entry := range la.entries {
		if entry.Timestamp.Before(start) || entry.Timestamp.After(end) {
			continue
		}
		if entry.Status == "success" && (entry.AuthMethod == AuthMethodTOTP || entry.AuthMethod == AuthMethodOTP || entry.AuthMethod == AuthMethodWebAuthn) {
			mfaLogins++
		}
	}
	if analysis.SuccessfulLogins > 0 {
		analysis.MFAUsageRate = float64(mfaLogins) / float64(analysis.SuccessfulLogins) * 100
	}

	// 处理热门位置
	for loc, count := range locations {
		analysis.TopLocations = append(analysis.TopLocations, LocationCount{
			Location: loc,
			Count:    count,
		})
	}
	sort.Slice(analysis.TopLocations, func(i, j int) bool {
		return analysis.TopLocations[i].Count > analysis.TopLocations[j].Count
	})
	if len(analysis.TopLocations) > 10 {
		analysis.TopLocations = analysis.TopLocations[:10]
	}

	// 处理热门设备
	for dev, count := range devices {
		analysis.TopDevices = append(analysis.TopDevices, DeviceCount{
			Device: dev,
			Count:  count,
		})
	}
	sort.Slice(analysis.TopDevices, func(i, j int) bool {
		return analysis.TopDevices[i].Count > analysis.TopDevices[j].Count
	})
	if len(analysis.TopDevices) > 10 {
		analysis.TopDevices = analysis.TopDevices[:10]
	}

	return analysis
}

// ========== 辅助功能 ==========

// getGeoLocation 获取地理位置（简化实现）.
func (la *LoginAuditor) getGeoLocation(ip string) *GeoLocation {
	// 这里可以集成GeoIP数据库
	// 简化实现，返回基本信息
	return &GeoLocation{
		Country:     "Unknown",
		CountryCode: "XX",
	}
}

// GenerateDeviceID generates a device fingerprint from user agent and IP.
func GenerateDeviceID(userAgent, ip string) string {
	data := userAgent + "|" + ip
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// ========== 持久化 ==========

// save 保存数据.
func (la *LoginAuditor) save() {
	if len(la.entries) == 0 {
		return
	}

	today := time.Now().Format("2006-01-02")
	filename := filepath.Join(la.storageDir, "login-"+today+".log")

	data, err := json.MarshalIndent(la.entries, "", "  ")
	if err != nil {
		return
	}

	// 保存失败时忽略错误（下次自动保存会重试）
	_ = os.WriteFile(filename, data, 0600)
}

// Load 加载数据.
func (la *LoginAuditor) Load(date string) error {
	filename := filepath.Join(la.storageDir, "login-"+date+".log")
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var entries []*LoginAuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	la.mu.Lock()
	la.entries = append(la.entries, entries...)
	la.mu.Unlock()

	return nil
}
