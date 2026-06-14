// Package ransomware_defense 提供勒索软件防护模块
// responder.go - 响应器（自动响应机制：IP阻止、共享禁用、只读、快照保护、恢复）
package ransomware_defense

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// =============================================================================
// Responder 响应器
// =============================================================================

// Responder 自动响应器
type Responder struct {
	mu             sync.RWMutex
	config         DefenseConfig
	shareMgr       ShareManager
	snapshotMgr    SnapshotManager
	firewallMgr    FirewallManager
	blockedIPs     map[string]*BlockedIP
	shareProtections map[string]*ShareProtection
	snapshotProtections map[string]*SnapshotProtection
	stopCh         chan struct{}
	running        bool
	alertHandler   func(ThreatEvent)
}

// NewResponder 创建新的响应器
func NewResponder(config DefenseConfig, shareMgr ShareManager, snapshotMgr SnapshotManager, firewallMgr FirewallManager) *Responder {
	return &Responder{
		config:              config,
		shareMgr:            shareMgr,
		snapshotMgr:         snapshotMgr,
		firewallMgr:         firewallMgr,
		blockedIPs:          make(map[string]*BlockedIP),
		shareProtections:    make(map[string]*ShareProtection),
		snapshotProtections: make(map[string]*SnapshotProtection),
		stopCh:              make(chan struct{}),
	}
}

// SetAlertHandler 设置告警处理函数
func (r *Responder) SetAlertHandler(handler func(ThreatEvent)) {
	r.mu.Lock()
	r.alertHandler = handler
	r.mu.Unlock()
}

// Start 启动响应器
func (r *Responder) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = true
	r.mu.Unlock()

	// 启动定期清理过期阻止
	go r.cleanupLoop()

	log.Println("✅ 勒索防护响应器已启动")
	return nil
}

// Stop 停止响应器
func (r *Responder) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	close(r.stopCh)
	r.mu.Unlock()

	log.Println("勒索防护响应器已停止")
}

// =============================================================================
// 响应处理入口
// =============================================================================

// HandleThreat 处理威胁事件（根据配置自动执行响应动作）
func (r *Responder) HandleThreat(threat ThreatEvent) {
	r.mu.RLock()
	handler := r.alertHandler
	r.mu.RUnlock()

	log.Printf("🚨 勒索防护响应: ID=%s, 级别=%s, 动作=%s, 方法=%s",
		threat.ID, threat.ThreatLevel.String(), string(threat.Action), threat.DetectionMethod)

	// 根据威胁级别和配置执行响应
	actions := r.determineActions(threat)

	for _, action := range actions {
		if err := r.executeAction(action, threat); err != nil {
			log.Printf("执行响应动作 %s 失败: %v", string(action), err)
		}
	}

	// 自动保护快照
	if r.config.AutoSnapshotProtect {
		r.protectRecentSnapshots(threat)
	}

	// 自动恢复（仅在严重威胁时触发）
	if r.config.AutoRestore && threat.ThreatLevel >= ThreatLevelCritical {
		r.autoRestoreFromSnapshot(threat)
	}

	// 通知告警
	if handler != nil {
		handler(threat)
	}
}

// determineActions 根据威胁级别和配置确定要执行的动作列表
func (r *Responder) determineActions(threat ThreatEvent) []ResponseAction {
	var actions []ResponseAction

	switch threat.ThreatLevel {
	case ThreatLevelCritical:
		// 严重威胁：全面响应
		if r.config.AutoDisableShare && threat.AffectedShare != "" {
			actions = append(actions, ActionDisableShare)
		}
		if r.config.AutoBlockIP && threat.SourceIP != "" {
			actions = append(actions, ActionBlockIP)
		}
		actions = append(actions, ActionPauseSnapshotDelete)

	case ThreatLevelHigh:
		// 高威胁：限制+只读
		if r.config.AutoReadOnly && threat.AffectedShare != "" {
			actions = append(actions, ActionReadOnly)
		}
		if r.config.AutoBlockIP && threat.SourceIP != "" {
			actions = append(actions, ActionBlockIP)
		}

	case ThreatLevelMedium:
		// 中威胁：限制访问
		if r.config.AutoBlockIP && threat.SourceIP != "" {
			actions = append(actions, ActionRestrictAccess)
		}

	default:
		// 低威胁/无威胁：仅告警
		actions = append(actions, ActionAlert)
	}

	return actions
}

// executeAction 执行单个响应动作
func (r *Responder) executeAction(action ResponseAction, threat ThreatEvent) error {
	switch action {
	case ActionAlert:
		return r.executeAlert(threat)
	case ActionBlockIP:
		return r.executeBlockIP(threat)
	case ActionDisableShare:
		return r.executeDisableShare(threat)
	case ActionReadOnly:
		return r.executeReadOnly(threat)
	case ActionRestrictAccess:
		return r.executeRestrictAccess(threat)
	case ActionPauseSnapshotDelete:
		return r.executePauseSnapshotDelete(threat)
	case ActionIsolate:
		return r.executeIsolate(threat)
	default:
		return fmt.Errorf("未知的响应动作: %s", string(action))
	}
}

// =============================================================================
// 具体响应动作实现
// =============================================================================

// executeAlert 执行告警动作
func (r *Responder) executeAlert(threat ThreatEvent) error {
	log.Printf("📢 勒索防护告警: [%s] %s (来源: %s/%s)",
		threat.ThreatLevel.String(), threat.Description, threat.SourceIP, threat.SourceUser)
	return nil
}

// executeBlockIP 执行IP阻止动作
func (r *Responder) executeBlockIP(threat ThreatEvent) error {
	if threat.SourceIP == "" {
		return fmt.Errorf("无来源IP，跳过IP阻止")
	}

	// 检查是否已被阻止
	r.mu.RLock()
	_, exists := r.blockedIPs[threat.SourceIP]
	r.mu.RUnlock()
	if exists {
		return nil
	}

	// 阻止IP
	duration := r.getBlockDuration(threat.ThreatLevel)
	var expiresAt *time.Time
	permanent := false

	if duration == 0 {
		permanent = true
	} else {
		t := time.Now().Add(duration)
		expiresAt = &t
	}

	// 调用防火墙管理器
	if r.firewallMgr != nil {
		if err := r.firewallMgr.BlockIP(threat.SourceIP, duration, threat.Description); err != nil {
			return fmt.Errorf("调用防火墙阻止IP失败: %w", err)
		}
	}

	record := &BlockedIP{
		IP:             threat.SourceIP,
		Reason:         fmt.Sprintf("勒索防护: %s", threat.Description),
		ThreatEventID:  threat.ID,
		ThreatScore:    0,
		BlockedAt:      time.Now(),
		ExpiresAt:      expiresAt,
		Permanent:      permanent,
	}

	if threat.Score != nil {
		record.ThreatScore = threat.Score.OverallScore
	}

	r.mu.Lock()
	r.blockedIPs[threat.SourceIP] = record
	r.mu.Unlock()

	log.Printf("🚫 IP已阻止: %s (原因: %s, 永久: %v)", threat.SourceIP, threat.Description, permanent)
	return nil
}

// executeDisableShare 执行禁用共享动作
func (r *Responder) executeDisableShare(threat ThreatEvent) error {
	if threat.AffectedShare == "" {
		return fmt.Errorf("无受影响共享，跳过禁用")
	}

	// 检查是否已被保护
	r.mu.RLock()
	_, exists := r.shareProtections[threat.AffectedShare]
	r.mu.RUnlock()
	if exists {
		return nil
	}

	// 调用共享管理器
	if r.shareMgr != nil {
		if err := r.shareMgr.DisableShare(threat.AffectedShare, threat.Protocol); err != nil {
			return fmt.Errorf("禁用共享失败: %w", err)
		}
	}

	record := &ShareProtection{
		ShareName:      threat.AffectedShare,
		Protocol:       threat.Protocol,
		OriginalMode:   "read-write",
		CurrentMode:    "disabled",
		ProtectionType: ActionDisableShare,
		Reason:         fmt.Sprintf("勒索防护: %s", threat.Description),
		ThreatEventID:  threat.ID,
		AppliedAt:      time.Now(),
	}

	r.mu.Lock()
	r.shareProtections[threat.AffectedShare] = record
	r.mu.Unlock()

	log.Printf("🔒 共享已禁用: %s (协议: %s, 原因: %s)", threat.AffectedShare, string(threat.Protocol), threat.Description)
	return nil
}

// executeReadOnly 执行只读动作
func (r *Responder) executeReadOnly(threat ThreatEvent) error {
	if threat.AffectedShare == "" {
		return fmt.Errorf("无受影响共享，跳过只读设置")
	}

	// 调用共享管理器
	if r.shareMgr != nil {
		if err := r.shareMgr.SetReadOnly(threat.AffectedShare, threat.Protocol); err != nil {
			return fmt.Errorf("设置只读失败: %w", err)
		}
	}

	record := &ShareProtection{
		ShareName:      threat.AffectedShare,
		Protocol:       threat.Protocol,
		OriginalMode:   "read-write",
		CurrentMode:    "read-only",
		ProtectionType: ActionReadOnly,
		Reason:         fmt.Sprintf("勒索防护: %s", threat.Description),
		ThreatEventID:  threat.ID,
		AppliedAt:      time.Now(),
	}

	r.mu.Lock()
	r.shareProtections[threat.AffectedShare] = record
	r.mu.Unlock()

	log.Printf("📖 共享已设为只读: %s (协议: %s)", threat.AffectedShare, string(threat.Protocol))
	return nil
}

// executeRestrictAccess 执行限制访问动作
func (r *Responder) executeRestrictAccess(threat ThreatEvent) error {
	if threat.SourceIP == "" || threat.AffectedShare == "" {
		return fmt.Errorf("缺少信息，跳过访问限制")
	}

	// 临时阻止IP访问
	duration := r.getBlockDuration(threat.ThreatLevel)
	if r.firewallMgr != nil {
		if err := r.firewallMgr.BlockIP(threat.SourceIP, duration, "勒索防护: 访问限制"); err != nil {
			return fmt.Errorf("限制访问失败: %w", err)
		}
	}

	t := time.Now().Add(duration)
	record := &ShareProtection{
		ShareName:      threat.AffectedShare,
		Protocol:       threat.Protocol,
		CurrentMode:    "restricted",
		ProtectionType: ActionRestrictAccess,
		Reason:         fmt.Sprintf("勒索防护: 限制IP %s 访问共享 %s", threat.SourceIP, threat.AffectedShare),
		ThreatEventID:  threat.ID,
		AppliedAt:      time.Now(),
		AutoReleaseAt:  &t,
	}

	r.mu.Lock()
	r.shareProtections[threat.AffectedShare+"|"+threat.SourceIP] = record
	r.mu.Unlock()

	log.Printf("⚠️ 访问已限制: IP %s -> 共享 %s (时长: %v)", threat.SourceIP, threat.AffectedShare, duration)
	return nil
}

// executePauseSnapshotDelete 执行暂停快照删除动作
func (r *Responder) executePauseSnapshotDelete(threat ThreatEvent) error {
	if r.snapshotMgr == nil {
		log.Println("快照管理器未配置，跳过快照保护")
		return nil
	}

	// 获取最近的快照并保护它们
	dataset := threat.AffectedShare
	if dataset == "" {
		// 从受影响文件推断数据集
		if len(threat.AffectedFiles) > 0 {
			dataset = extractDataset(threat.AffectedFiles[0])
		}
	}

	if dataset == "" {
		return fmt.Errorf("无法确定数据集，跳过快照保护")
	}

	snapshots, err := r.snapshotMgr.ListSnapshots(dataset)
	if err != nil {
		return fmt.Errorf("获取快照列表失败: %w", err)
	}

	protected := 0
	for _, snap := range snapshots {
		if snap.Protected {
			continue
		}

		if err := r.snapshotMgr.ProtectSnapshot(snap.ID); err != nil {
			log.Printf("保护快照失败 %s: %v", snap.ID, err)
			continue
		}

		record := &SnapshotProtection{
			SnapshotID:      snap.ID,
			Dataset:         dataset,
			Reason:          fmt.Sprintf("勒索防护: %s", threat.Description),
			ThreatEventID:   threat.ID,
			ProtectedAt:     time.Now(),
			DeleteProtected: true,
		}

		r.mu.Lock()
		r.snapshotProtections[snap.ID] = record
		r.mu.Unlock()
		protected++
	}

	log.Printf("🛡️ 已保护 %d 个快照 (数据集: %s)", protected, dataset)
	return nil
}

// executeIsolate 执行隔离动作
func (r *Responder) executeIsolate(threat ThreatEvent) error {
	// 隔离 = 禁用共享 + 阻止IP
	if threat.AffectedShare != "" {
		_ = r.executeDisableShare(threat)
	}
	if threat.SourceIP != "" {
		_ = r.executeBlockIP(threat)
	}
	return nil
}

// =============================================================================
// 快照保护与恢复
// =============================================================================

// protectRecentSnapshots 保护最近的快照
func (r *Responder) protectRecentSnapshots(threat ThreatEvent) {
	if r.snapshotMgr == nil {
		return
	}

	dataset := threat.AffectedShare
	if dataset == "" && len(threat.AffectedFiles) > 0 {
		dataset = extractDataset(threat.AffectedFiles[0])
	}
	if dataset == "" {
		return
	}

	snapshots, err := r.snapshotMgr.ListSnapshots(dataset)
	if err != nil {
		log.Printf("获取快照失败: %v", err)
		return
	}

	// 保护最近24小时内的快照
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, snap := range snapshots {
		if snap.CreatedAt.After(cutoff) && !snap.Protected {
			if err := r.snapshotMgr.ProtectSnapshot(snap.ID); err != nil {
				log.Printf("保护快照失败 %s: %v", snap.ID, err)
				continue
			}

			r.mu.Lock()
			r.snapshotProtections[snap.ID] = &SnapshotProtection{
				SnapshotID:      snap.ID,
				Dataset:         dataset,
				Reason:          "勒索防护自动保护",
				ThreatEventID:   threat.ID,
				ProtectedAt:     time.Now(),
				DeleteProtected: true,
			}
			r.mu.Unlock()
		}
	}
}

// autoRestoreFromSnapshot 从快照自动恢复
func (r *Responder) autoRestoreFromSnapshot(threat ThreatEvent) {
	if r.snapshotMgr == nil {
		log.Println("快照管理器未配置，跳过自动恢复")
		return
	}

	dataset := threat.AffectedShare
	if dataset == "" && len(threat.AffectedFiles) > 0 {
		dataset = extractDataset(threat.AffectedFiles[0])
	}
	if dataset == "" {
		log.Println("无法确定数据集，跳过自动恢复")
		return
	}

	snapshots, err := r.snapshotMgr.ListSnapshots(dataset)
	if err != nil {
		log.Printf("获取快照列表失败: %v", err)
		return
	}

	// 选择攻击前最近的快照
	maxAge := time.Duration(r.config.AutoRestoreSnapshotAge) * time.Hour
	cutoff := time.Now().Add(-maxAge)

	var bestSnapshot *SnapshotInfo
	for i := len(snapshots) - 1; i >= 0; i-- {
		snap := &snapshots[i]
		if snap.CreatedAt.Before(cutoff) {
			continue
		}
		// 选择攻击时间窗口内最早的快照
		bestSnapshot = snap
	}

	if bestSnapshot == nil {
		log.Printf("未找到合适的数据集 %s 快照用于恢复", dataset)
		return
	}

	log.Printf("🔄 自动恢复: 数据集=%s, 快照=%s (创建时间: %s)",
		dataset, bestSnapshot.ID, bestSnapshot.CreatedAt.Format(time.RFC3339))

	if err := r.snapshotMgr.RestoreSnapshot(bestSnapshot.ID); err != nil {
		log.Printf("自动恢复失败: %v", err)
		return
	}

	log.Printf("✅ 自动恢复成功: 数据集=%s, 快照=%s", dataset, bestSnapshot.ID)
}

// =============================================================================
// 恢复与清理
// =============================================================================

// RestoreShare 恢复共享到原始状态
func (r *Responder) RestoreShare(shareName string) error {
	r.mu.Lock()
	protection, exists := r.shareProtections[shareName]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("共享保护记录不存在: %s", shareName)
	}
	delete(r.shareProtections, shareName)
	r.mu.Unlock()

	if r.shareMgr == nil {
		return fmt.Errorf("共享管理器未配置")
	}

	// 根据原始模式恢复
	switch protection.ProtectionType {
	case ActionDisableShare:
		if err := r.shareMgr.EnableShare(shareName, protection.Protocol); err != nil {
			return fmt.Errorf("启用共享失败: %w", err)
		}
	case ActionReadOnly:
		if err := r.shareMgr.SetReadWrite(shareName, protection.Protocol); err != nil {
			return fmt.Errorf("恢复读写失败: %w", err)
		}
	}

	log.Printf("✅ 共享已恢复: %s (协议: %s)", shareName, string(protection.Protocol))
	return nil
}

// UnblockIP 解除IP阻止
func (r *Responder) UnblockIP(ip string) error {
	r.mu.Lock()
	_, exists := r.blockedIPs[ip]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("IP未被阻止: %s", ip)
	}
	delete(r.blockedIPs, ip)
	r.mu.Unlock()

	if r.firewallMgr != nil {
		if err := r.firewallMgr.UnblockIP(ip); err != nil {
			return fmt.Errorf("解除IP阻止失败: %w", err)
		}
	}

	log.Printf("✅ IP已解除阻止: %s", ip)
	return nil
}

// UnprotectSnapshot 取消快照保护
func (r *Responder) UnprotectSnapshot(snapshotID string) error {
	r.mu.Lock()
	_, exists := r.snapshotProtections[snapshotID]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("快照保护记录不存在: %s", snapshotID)
	}
	delete(r.snapshotProtections, snapshotID)
	r.mu.Unlock()

	if r.snapshotMgr != nil {
		if err := r.snapshotMgr.UnprotectSnapshot(snapshotID); err != nil {
			return fmt.Errorf("取消快照保护失败: %w", err)
		}
	}

	log.Printf("✅ 快照保护已取消: %s", snapshotID)
	return nil
}

// =============================================================================
// 状态查询
// =============================================================================

// GetBlockedIPs 获取已阻止的IP列表
func (r *Responder) GetBlockedIPs() []BlockedIP {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]BlockedIP, 0, len(r.blockedIPs))
	for _, ip := range r.blockedIPs {
		result = append(result, *ip)
	}
	return result
}

// GetShareProtections 获取共享保护列表
func (r *Responder) GetShareProtections() []ShareProtection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ShareProtection, 0, len(r.shareProtections))
	for _, sp := range r.shareProtections {
		result = append(result, *sp)
	}
	return result
}

// GetSnapshotProtections 获取快照保护列表
func (r *Responder) GetSnapshotProtections() []SnapshotProtection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]SnapshotProtection, 0, len(r.snapshotProtections))
	for _, sp := range r.snapshotProtections {
		result = append(result, *sp)
	}
	return result
}

// GetStats 获取响应统计
func (r *Responder) GetStats() (blockedIPs int, disabledShares int, protectedSnapshots int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.blockedIPs), len(r.shareProtections), len(r.snapshotProtections)
}

// UpdateConfig 更新响应器配置
func (r *Responder) UpdateConfig(config DefenseConfig) {
	r.mu.Lock()
	r.config = config
	r.mu.Unlock()
	log.Println("响应器配置已更新")
}

// =============================================================================
// 定期清理
// =============================================================================

// cleanupLoop 定期清理过期的阻止记录
func (r *Responder) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.cleanupExpired()
		}
	}
}

// cleanupExpired 清理过期记录
func (r *Responder) cleanupExpired() {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	// 清理过期的IP阻止
	for ip, record := range r.blockedIPs {
		if !record.Permanent && record.ExpiresAt != nil && now.After(*record.ExpiresAt) {
			// 解除防火墙阻止
			if r.firewallMgr != nil {
				_ = r.firewallMgr.UnblockIP(ip)
			}
			delete(r.blockedIPs, ip)
			log.Printf("自动解除过期IP阻止: %s", ip)
		}
	}

	// 清理过期的共享限制
	for key, record := range r.shareProtections {
		if record.AutoReleaseAt != nil && now.After(*record.AutoReleaseAt) {
			// 恢复共享
			if r.shareMgr != nil && record.ProtectionType == ActionReadOnly {
				_ = r.shareMgr.SetReadWrite(record.ShareName, record.Protocol)
			}
			delete(r.shareProtections, key)
			log.Printf("自动释放共享限制: %s", record.ShareName)
		}
	}
}

// =============================================================================
// 辅助函数
// =============================================================================

// getBlockDuration 根据威胁级别返回阻止时长
func (r *Responder) getBlockDuration(level ThreatLevel) time.Duration {
	switch level {
	case ThreatLevelCritical:
		return 0 // 永久阻止，需手动解除
	case ThreatLevelHigh:
		return 4 * time.Hour
	case ThreatLevelMedium:
		return 1 * time.Hour
	case ThreatLevelLow:
		return 15 * time.Minute
	default:
		return 5 * time.Minute
	}
}

// extractDataset 从文件路径提取数据集名
func extractDataset(path string) string {
	// 简单实现：取第一个路径组件作为数据集名
	// 实际应根据ZFS/存储系统配置进行更精确的映射
	if len(path) > 0 && path[0] == '/' {
		parts := splitPath(path)
		if len(parts) >= 2 {
			return parts[1] // 跳过空字符串（根/）
		}
	}
	return ""
}

// splitPath 按'/'分割路径
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}
