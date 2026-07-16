package nids

import (
	"fmt"
	"log"
	"net"
	"sort"
	"time"
)

// AddRule 添加检测规则.
func (m *Manager) AddRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.rules) >= m.config.MaxRules {
		return ErrMaxRulesReached
	}

	if err := ValidateRule(rule); err != nil {
		return err
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	m.updateStatsLocked()
	log.Printf("[NIDS] 添加规则: %s (%s)", rule.ID, rule.Name)
	return nil
}

// UpdateRule 更新规则.
func (m *Manager) UpdateRule(id string, update *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.rules[id]
	if !ok {
		return ErrRuleNotFound
	}

	if err := ValidateRule(update); err != nil {
		return err
	}

	update.ID = id
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()
	update.HitCount = existing.HitCount
	m.rules[id] = update
	return nil
}

// DeleteRule 删除规则.
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return ErrRuleNotFound
	}
	delete(m.rules, id)
	m.updateStatsLocked()
	return nil
}

// GetRule 获取规则.
func (m *Manager) GetRule(id string) (*Rule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules 列出所有规则.
func (m *Manager) ListRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return rules
}

// EnableRule 启用规则.
func (m *Manager) EnableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return ErrRuleNotFound
	}
	rule.Enabled = true
	rule.UpdatedAt = time.Now()
	return nil
}

// DisableRule 禁用规则.
func (m *Manager) DisableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return ErrRuleNotFound
	}
	rule.Enabled = false
	rule.UpdatedAt = time.Now()
	return nil
}

// GetAlert 获取告警.
func (m *Manager) GetAlert(id string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[id]
	if !ok {
		return nil, ErrAlertNotFound
	}
	return alert, nil
}

// ListAlerts 列出告警.
func (m *Manager) ListAlerts(status AlertStatus, limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Alert, 0)
	for _, a := range m.alerts {
		if status != "" && a.Status != status {
			continue
		}
		result = append(result, a)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].LastSeen.After(result[j].LastSeen)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return ErrAlertNotFound
	}
	now := time.Now()
	alert.Status = AlertAcked
	alert.AckedAt = &now
	return nil
}

// ResolveAlert 解决告警.
func (m *Manager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return ErrAlertNotFound
	}
	now := time.Now()
	alert.Status = AlertResolved
	alert.ResolvedAt = &now
	return nil
}

// MarkFalsePositive 标记误报.
func (m *Manager) MarkFalsePositive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return ErrAlertNotFound
	}
	alert.Status = AlertFalsePos
	return nil
}

// AddToBlacklist 添加 IP 到黑名单.
func (m *Manager) AddToBlacklist(ip, reason string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return ErrInvalidCIDR
	}

	entry := &IPEntry{
		IP:      ip,
		Reason:  reason,
		AddedAt: time.Now(),
	}
	if ttl > 0 {
		exp := time.Now().Add(ttl)
		entry.ExpiresAt = &exp
	}
	m.blacklist[ip] = entry
	log.Printf("[NIDS] IP %s 已加入黑名单: %s", ip, reason)
	return nil
}

// RemoveFromBlacklist 从黑名单移除.
func (m *Manager) RemoveFromBlacklist(ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.blacklist[ip]; !ok {
		return fmt.Errorf("nids: IP %s not in blacklist", ip)
	}
	delete(m.blacklist, ip)
	return nil
}

// ListBlacklist 列出黑名单.
func (m *Manager) ListBlacklist() []*IPEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*IPEntry, 0, len(m.blacklist))
	for _, e := range m.blacklist {
		result = append(result, e)
	}
	return result
}

// AddToWhitelist 添加 IP 到白名单.
func (m *Manager) AddToWhitelist(ip, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return ErrInvalidCIDR
	}

	m.whitelist[ip] = &IPEntry{
		IP:      ip,
		Reason:  reason,
		AddedAt: time.Now(),
	}
	log.Printf("[NIDS] IP %s 已加入白名单: %s", ip, reason)
	return nil
}

// RemoveFromWhitelist 从白名单移除.
func (m *Manager) RemoveFromWhitelist(ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.whitelist[ip]; !ok {
		return fmt.Errorf("nids: IP %s not in whitelist", ip)
	}
	delete(m.whitelist, ip)
	return nil
}

// ListWhitelist 列出白名单.
func (m *Manager) ListWhitelist() []*IPEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*IPEntry, 0, len(m.whitelist))
	for _, e := range m.whitelist {
		result = append(result, e)
	}
	return result
}

// CreateForensic 创建取证记录.
func (m *Manager) CreateForensic(alertID string, packets []PacketInfo, notes string) (*ForensicRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return nil, ErrAlertNotFound
	}

	m.forensicID++
	record := &ForensicRecord{
		ID:        fmt.Sprintf("fr_%d", m.forensicID),
		AlertID:   alertID,
		SrcIP:     alert.SrcIP,
		DstIP:     alert.DstIP,
		Protocol:  alert.Protocol,
		Packets:   packets,
		StartTime: alert.FirstSeen,
		EndTime:   time.Now(),
		Notes:     notes,
	}
	m.forensics[record.ID] = record
	return record, nil
}

// GetForensic 获取取证记录.
func (m *Manager) GetForensic(id string) (*ForensicRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.forensics[id]
	if !ok {
		return nil, ErrForensicNotFound
	}
	return record, nil
}

// ListForensics 列出取证记录.
func (m *Manager) ListForensics() []*ForensicRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ForensicRecord, 0, len(m.forensics))
	for _, f := range m.forensics {
		result = append(result, f)
	}
	return result
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *NIDSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *NIDSConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// GetStats 获取统计.
func (m *Manager) GetStats() NIDSStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.buildStatsLocked()
}

// GetDetector 获取检测引擎.
func (m *Manager) GetDetector() *Detector {
	return m.detector
}

// AnalyzePacket 分析数据包.
func (m *Manager) AnalyzePacket(pkt *PacketInfo) []*Alert {
	if !m.config.Enabled {
		return nil
	}
	return m.detector.AnalyzePacket(pkt)
}

// SyncToFirewall 将高危告警同步到防火墙（联动接口）.
func (m *Manager) SyncToFirewall() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.FirewallSync {
		return nil
	}

	var blockedIPs []string
	for _, alert := range m.alerts {
		if alert.Status == AlertOpen && (alert.Severity == SeverityCritical || alert.Severity == SeverityHigh) {
			if alert.Action == ActionBlock || alert.Action == ActionDrop {
				ip := alert.SrcIP.String()
				if !m.isIPBlacklisted(ip) {
					m.blacklist[ip] = &IPEntry{
						IP:      ip,
						Reason:  fmt.Sprintf("Auto-blocked by NIDS: %s", alert.RuleName),
						AddedAt: time.Now(),
					}
					blockedIPs = append(blockedIPs, ip)
				}
			}
		}
	}
	return blockedIPs
}

// processAlert 处理告警（内部方法）.
func (m *Manager) processAlert(alert *Alert) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 聚合同源告警
	key := alert.RuleID + ":" + alert.SrcIP.String()
	if existing, ok := m.alerts[key]; ok {
		existing.Count++
		existing.LastSeen = time.Now()
		return
	}

	alert.ID = key
	m.alerts[key] = alert
	m.alertLog = append(m.alertLog, alert)

	// 限制告警日志大小
	if len(m.alertLog) > m.maxAlertLog {
		m.alertLog = m.alertLog[len(m.alertLog)-m.maxAlertLog:]
	}

	// 自动封锁
	if m.config.FirewallSync && alert.Action == ActionBlock {
		if alert.Count >= m.config.BlockThreshold {
			m.blacklist[alert.SrcIP.String()] = &IPEntry{
				IP:      alert.SrcIP.String(),
				Reason:  fmt.Sprintf("Auto-blocked: %s (threshold %d)", alert.RuleName, m.config.BlockThreshold),
				AddedAt: time.Now(),
			}
			log.Printf("[NIDS] 自动封锁 IP: %s (规则: %s)", alert.SrcIP, alert.RuleName)
		}
	}

	m.updateStatsLocked()
}

// isIPBlacklisted 检查 IP 是否在黑名单.
func (m *Manager) isIPBlacklisted(ip string) bool {
	_, ok := m.blacklist[ip]
	return ok
}

// isIPWhitelisted 检查 IP 是否在白名单.
func (m *Manager) isIPWhitelisted(ip string) bool {
	_, ok := m.whitelist[ip]
	return ok
}

// nextAlertID 生成下一个告警 ID.
func (m *Manager) nextAlertID() int64 {
	m.alertCounter++
	return m.alertCounter
}

// updateStatsLocked 更新统计（需持有锁）.
func (m *Manager) updateStatsLocked() {
	m.trafficStats.TotalRules = int64(len(m.rules))
	enabled := 0
	for _, r := range m.rules {
		if r.Enabled {
			enabled++
		}
	}
	m.trafficStats.LastUpdate = time.Now()
	_ = enabled
}

// buildStatsLocked 构建统计（需持有读锁）.
func (m *Manager) buildStatsLocked() NIDSStats {
	openAlerts := 0
	for _, a := range m.alerts {
		if a.Status == AlertOpen {
			openAlerts++
		}
	}

	enabled := 0
	for _, r := range m.rules {
		if r.Enabled {
			enabled++
		}
	}

	packets, attacks := m.detector.GetStats()

	return NIDSStats{
		TotalRules:      len(m.rules),
		EnabledRules:    enabled,
		TotalAlerts:     len(m.alerts),
		OpenAlerts:      openAlerts,
		BlockedIPs:      len(m.blacklist),
		WhitelistedIPs:  len(m.whitelist),
		BlacklistedIPs:  len(m.blacklist),
		PacketsAnalyzed: packets,
		AttacksDetected: attacks,
		LastUpdate:      time.Now(),
	}
}
