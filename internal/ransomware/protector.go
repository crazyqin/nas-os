// Package ransomshield - 防护引擎
// 蜜罐文件、自动快照、实时阻断、恢复机制
package ransomware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 防护引擎
// ============================================================

// Protector 勒索软件防护引擎.
type Protector struct {
	mu sync.RWMutex

	// detector 检测器引用
	detector *Detector

	// honeypots 蜜罐文件列表
	honeypots map[string]*HoneypotFile

	// recoveryPoints 恢复点列表
	recoveryPoints map[string]*RecoveryPoint

	// blockedProcesses 已阻断的进程
	blockedProcesses map[string]time.Time

	// honeypotConfig 蜜罐配置
	honeypotConfig HoneypotConfig

	// snapshotCallback 快照创建回调
	snapshotCallback func(path string) (string, error)

	// processBlockCallback 进程阻断回调
	processBlockCallback func(pid int, name string) error

	// stats 统计
	blockedCount    int64
	snapshotCount   int64
	rollbackCount   int64
	quarantineCount int64

	// running 运行状态
	running  bool
	stopChan chan struct{}
}

// NewProtector 创建防护引擎.
func NewProtector(detector *Detector, honeypotConfig HoneypotConfig) *Protector {
	return &Protector{
		detector:         detector,
		honeypots:        make(map[string]*HoneypotFile),
		recoveryPoints:   make(map[string]*RecoveryPoint),
		blockedProcesses: make(map[string]time.Time),
		honeypotConfig:   honeypotConfig,
		stopChan:         make(chan struct{}),
	}
}

// SetSnapshotCallback 设置快照创建回调.
func (p *Protector) SetSnapshotCallback(fn func(path string) (string, error)) {
	p.mu.Lock()
	p.snapshotCallback = fn
	p.mu.Unlock()
}

// SetProcessBlockCallback 设置进程阻断回调.
func (p *Protector) SetProcessBlockCallback(fn func(pid int, name string) error) {
	p.mu.Lock()
	p.processBlockCallback = fn
	p.mu.Unlock()
}

// Start 启动防护引擎.
func (p *Protector) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = true
	p.mu.Unlock()

	// 部署蜜罐
	if p.honeypotConfig.Enabled {
		if err := p.deployHoneypots(); err != nil {
			log.Printf("[RansomShield] 蜜罐部署失败: %v", err)
		}
	}

	// 启动防护循环
	go p.protectionLoop(ctx)

	// 启动蜜罐刷新
	go p.honeypotRefreshLoop(ctx)

	// 启动恢复点清理
	go p.recoveryCleanupLoop(ctx)

	log.Println("[RansomShield] 防护引擎已启动")
	return nil
}

// Stop 停止防护引擎.
func (p *Protector) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	close(p.stopChan)
	p.running = false
	log.Println("[RansomShield] 防护引擎已停止")
}

// ============================================================
// 防护循环
// ============================================================

// protectionLoop 主防护循环.
func (p *Protector) protectionLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.checkThreats()
		}
	}
}

// checkThreats 检查并处理威胁.
func (p *Protector) checkThreats() {
	events, _ := p.detector.GetThreatEvents(1, 10)
	if len(events) == 0 {
		return
	}

	for _, event := range events {
		if event.Resolved {
			continue
		}

		// 根据威胁级别执行不同动作
		switch event.Level {
		case ThreatLevelCritical:
			p.handleCriticalThreat(event)
		case ThreatLevelHigh:
			p.handleHighThreat(event)
		case ThreatLevelMedium:
			p.handleMediumThreat(event)
		case ThreatLevelLow:
			p.handleLowThreat(event)
		}
	}
}

// handleCriticalThreat 处理严重威胁.
func (p *Protector) handleCriticalThreat(event ThreatEvent) {
	log.Printf("[RansomShield] 处理严重威胁: %s", event.ID)

	// 1. 立即创建快照
	if p.snapshotCallback != nil {
		p.createAutoSnapshot(event.SourcePath, event.ID, ThreatLevelCritical)
	}

	// 2. 阻断进程
	if event.ProcessID > 0 {
		p.blockProcess(event.ProcessID, event.ProcessName)
	}

	// 3. 隔离文件
	p.quarantineFile(event.SourcePath)
}

// handleHighThreat 处理高威胁.
func (p *Protector) handleHighThreat(event ThreatEvent) {
	log.Printf("[RansomShield] 处理高威胁: %s", event.ID)

	// 创建快照
	p.createAutoSnapshot(event.SourcePath, event.ID, ThreatLevelHigh)

	// 阻断可疑进程
	if event.ProcessID > 0 {
		p.blockProcess(event.ProcessID, event.ProcessName)
	}
}

// handleMediumThreat 处理中等威胁.
func (p *Protector) handleMediumThreat(event ThreatEvent) {
	log.Printf("[RansomShield] 处理中等威胁: %s", event.ID)

	// 创建预防性快照
	p.createAutoSnapshot(event.SourcePath, event.ID, ThreatLevelMedium)
}

// handleLowThreat 处理低威胁.
func (p *Protector) handleLowThreat(event ThreatEvent) {
	log.Printf("[RansomShield] 低威胁告警: %s", event.ID)
	// 仅记录，不执行阻断
}

// ============================================================
// 快照管理
// ============================================================

// createAutoSnapshot 创建自动快照.
func (p *Protector) createAutoSnapshot(triggerPath, triggerEventID string, level ThreatLevel) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.snapshotCallback == nil {
		log.Println("[RansomShield] 快照回调未设置，跳过快照创建")
		return
	}

	// 确定快照目录
	dir := filepath.Dir(triggerPath)

	snapshotID, err := p.snapshotCallback(dir)
	if err != nil {
		log.Printf("[RansomShield] 快照创建失败: %v", err)
		return
	}

	now := time.Now()
	rp := &RecoveryPoint{
		ID:           uuid.New().String(),
		Name:         fmt.Sprintf("auto-snapshot-%s", now.Format("20060102-150405")),
		Description:  fmt.Sprintf("威胁触发的自动快照 (事件: %s, 级别: %s)", triggerEventID, level.String()),
		Type:         RecoveryTypeAuto,
		Path:         snapshotID,
		TriggerEvent: triggerEventID,
		ThreatLevel:  level,
		Status:       RecoveryStatusReady,
		CreatedAt:    now,
	}

	p.recoveryPoints[rp.ID] = rp
	p.snapshotCount++

	log.Printf("[RansomShield] 自动快照已创建: %s (路径: %s)", rp.ID, dir)
}

// CreateRecoveryPoint 手动创建恢复点.
func (p *Protector) CreateRecoveryPoint(name, path, description string) (*RecoveryPoint, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.snapshotCallback == nil {
		return nil, fmt.Errorf("快照回调未设置")
	}

	snapshotID, err := p.snapshotCallback(path)
	if err != nil {
		return nil, fmt.Errorf("快照创建失败: %w", err)
	}

	now := time.Now()
	rp := &RecoveryPoint{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Type:        RecoveryTypeManual,
		Path:        snapshotID,
		Status:      RecoveryStatusReady,
		CreatedAt:   now,
	}

	p.recoveryPoints[rp.ID] = rp
	p.snapshotCount++

	return rp, nil
}

// GetRecoveryPoints 获取所有恢复点.
func (p *Protector) GetRecoveryPoints() []RecoveryPoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	points := make([]RecoveryPoint, 0, len(p.recoveryPoints))
	for _, rp := range p.recoveryPoints {
		points = append(points, *rp)
	}
	return points
}

// RollbackToRecoveryPoint 回滚到指定恢复点.
func (p *Protector) RollbackToRecoveryPoint(recoveryPointID, targetPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	rp, ok := p.recoveryPoints[recoveryPointID]
	if !ok {
		return fmt.Errorf("恢复点不存在: %s", recoveryPointID)
	}

	if rp.Status != RecoveryStatusReady {
		return fmt.Errorf("恢复点状态不可用: %s", rp.Status)
	}

	// 标记为回滚中
	rp.Status = RecoveryStatusRollback

	log.Printf("[RansomShield] 开始回滚: 恢复点=%s, 目标=%s", recoveryPointID, targetPath)
	if rp.Path != "" && targetPath != "" {
		if err := copyPath(rp.Path, targetPath); err != nil {
			rp.Status = RecoveryStatusReady
			return fmt.Errorf("恢复文件失败: %w", err)
		}
	}

	rp.Status = RecoveryStatusReady
	p.rollbackCount++

	log.Printf("[RansomShield] 回滚完成: %s", recoveryPointID)
	return nil
}

// ============================================================
// 进程阻断
// ============================================================

// blockProcess 阻断可疑进程.
func (p *Protector) blockProcess(pid int, name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	processKey := fmt.Sprintf("%s:%d", name, pid)

	// 检查是否已阻断
	if _, blocked := p.blockedProcesses[processKey]; blocked {
		return
	}

	if p.processBlockCallback != nil {
		if err := p.processBlockCallback(pid, name); err != nil {
			log.Printf("[RansomShield] 进程阻断失败: PID=%d, Name=%s, Error=%v", pid, name, err)
			return
		}
	}

	p.blockedProcesses[processKey] = time.Now()
	p.blockedCount++

	log.Printf("[RansomShield] 进程已阻断: PID=%d, Name=%s", pid, name)
}

// UnblockProcess 解除进程阻断.
func (p *Protector) UnblockProcess(pid int, name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	processKey := fmt.Sprintf("%s:%d", name, pid)
	delete(p.blockedProcesses, processKey)

	log.Printf("[RansomShield] 进程已解除阻断: PID=%d, Name=%s", pid, name)
}

// GetBlockedProcesses 获取已阻断的进程.
func (p *Protector) GetBlockedProcesses() map[string]time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]time.Time)
	for k, v := range p.blockedProcesses {
		result[k] = v
	}
	return result
}

// ============================================================
// 文件隔离
// ============================================================

// quarantineFile 隔离可疑文件.
func (p *Protector) quarantineFile(path string) {
	quarantineDir := filepath.Join(filepath.Dir(path), ".ransomshield-quarantine")
	if err := os.MkdirAll(quarantineDir, 0700); err != nil {
		log.Printf("[RansomShield] 隔离目录创建失败: %v", err)
		return
	}
	dst := filepath.Join(quarantineDir, filepath.Base(path)+"."+time.Now().Format("20060102150405"))
	if err := os.Rename(path, dst); err != nil {
		log.Printf("[RansomShield] 文件隔离失败: %v", err)
		return
	}
	_ = os.Chmod(dst, 0400)
	p.mu.Lock()
	p.quarantineCount++
	p.mu.Unlock()
	log.Printf("[RansomShield] 文件已隔离: %s -> %s", path, dst)
}

// ============================================================
// 蜜罐管理
// ============================================================

// deployHoneypots 部署蜜罐文件.
func (p *Protector) deployHoneypots() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, basePath := range p.honeypotConfig.BasePaths {
		// 创建蜜罐目录
		honeypotDir := filepath.Join(basePath, ".honeypot")
		if err := os.MkdirAll(honeypotDir, 0755); err != nil {
			return fmt.Errorf("创建蜜罐目录失败 %s: %w", honeypotDir, err)
		}

		// 生成蜜罐文件
		for i := 0; i < p.honeypotConfig.FileCount; i++ {
			if err := p.createHoneypotFile(honeypotDir, i); err != nil {
				log.Printf("[RansomShield] 创建蜜罐文件失败: %v", err)
			}
		}
	}

	log.Printf("[RansomShield] 蜜罐部署完成: %d 个路径, 每路径 %d 个文件",
		len(p.honeypotConfig.BasePaths), p.honeypotConfig.FileCount)
	return nil
}

// createHoneypotFile 创建单个蜜罐文件.
func (p *Protector) createHoneypotFile(dir string, index int) error {
	// 随机选择扩展名
	ext := p.honeypotConfig.FileExtensions[index%len(p.honeypotConfig.FileExtensions)]

	// 生成诱饵文件名
	names := []string{
		"财务报表", "工资明细", "合同模板", "客户资料", "项目文档",
		"密码备份", "银行账号", "税务记录", "身份证扫描", "营业执照",
	}
	name := names[index%len(names)]
	filename := fmt.Sprintf("%s_%d%s", name, time.Now().UnixNano()%10000, ext)
	filePath := filepath.Join(dir, filename)

	// 生成随机内容（1-10KB）
	size := 1024 + index*512
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	// 计算哈希
	hash := sha256.Sum256(data)

	honeypot := &HoneypotFile{
		ID:        uuid.New().String(),
		Path:      filePath,
		Name:      filename,
		SizeBytes: int64(size),
		Extension: ext,
		Hash:      hex.EncodeToString(hash[:]),
		CreatedAt: time.Now(),
	}

	p.honeypots[honeypot.ID] = honeypot
	return nil
}

// honeypotRefreshLoop 蜜罐刷新循环.
func (p *Protector) honeypotRefreshLoop(ctx context.Context) {
	interval := time.Duration(p.honeypotConfig.RefreshIntervalMin) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.refreshHoneypots()
		}
	}
}

// refreshHoneypots 刷新蜜罐文件.
func (p *Protector) refreshHoneypots() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 删除旧蜜罐
	for id, hp := range p.honeypots {
		if err := os.Remove(hp.Path); err != nil && !os.IsNotExist(err) {
			log.Printf("[RansomShield] 删除旧蜜罐失败: %s, %v", hp.Path, err)
		}
		delete(p.honeypots, id)
	}

	// 重新部署
	for _, basePath := range p.honeypotConfig.BasePaths {
		honeypotDir := filepath.Join(basePath, ".honeypot")
		for i := 0; i < p.honeypotConfig.FileCount; i++ {
			if err := p.createHoneypotFile(honeypotDir, i); err != nil {
				log.Printf("[RansomShield] 刷新蜜罐文件失败: %v", err)
			}
		}
	}

	log.Println("[RansomShield] 蜜罐文件已刷新")
}

// GetHoneypots 获取所有蜜罐文件.
func (p *Protector) GetHoneypots() []HoneypotFile {
	p.mu.RLock()
	defer p.mu.RUnlock()

	hps := make([]HoneypotFile, 0, len(p.honeypots))
	for _, hp := range p.honeypots {
		hps = append(hps, *hp)
	}
	return hps
}

// CheckHoneypot 检查文件是否为蜜罐.
func (p *Protector) CheckHoneypot(path string) *HoneypotFile {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, hp := range p.honeypots {
		if hp.Path == path || strings.HasPrefix(path, filepath.Dir(hp.Path)) {
			return hp
		}
	}
	return nil
}

// TriggerHoneypot 触发蜜罐.
func (p *Protector) TriggerHoneypot(honeypotID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	hp, ok := p.honeypots[honeypotID]
	if !ok {
		return
	}

	now := time.Now()
	hp.Triggered = true
	hp.TriggerCount++
	hp.LastTrigger = &now

	log.Printf("[RansomShield] 蜜罐已触发: %s (路径: %s, 触发次数: %d)",
		honeypotID, hp.Path, hp.TriggerCount)
}

// ============================================================
// 恢复点清理
// ============================================================

// recoveryCleanupLoop 恢复点清理循环.
func (p *Protector) recoveryCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.cleanupExpiredRecoveryPoints()
		}
	}
}

// cleanupExpiredRecoveryPoints 清理过期的恢复点.
func (p *Protector) cleanupExpiredRecoveryPoints() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for id, rp := range p.recoveryPoints {
		if rp.ExpiresAt != nil && now.After(*rp.ExpiresAt) {
			rp.Status = RecoveryStatusExpired
			log.Printf("[RansomShield] 恢复点已过期: %s", id)
		}
	}
}

// ============================================================
// 统计信息
// ============================================================

// GetProtectorStats 获取防护引擎统计.
func (p *Protector) GetProtectorStats() ProtectorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProtectorStats{
		HoneypotsDeployed:  len(p.honeypots),
		HoneypotsTriggered: p.countTriggeredHoneypots(),
		RecoveryPoints:     len(p.recoveryPoints),
		BlockedProcesses:   len(p.blockedProcesses),
		SnapshotsCreated:   p.snapshotCount,
		RollbacksDone:      p.rollbackCount,
		QuarantinesDone:    p.quarantineCount,
		BlocksTriggered:    p.blockedCount,
	}
}

// ProtectorStats 防护引擎统计.
type ProtectorStats struct {
	HoneypotsDeployed  int   `json:"honeypots_deployed"`
	HoneypotsTriggered int   `json:"honeypots_triggered"`
	RecoveryPoints     int   `json:"recovery_points"`
	BlockedProcesses   int   `json:"blocked_processes"`
	SnapshotsCreated   int64 `json:"snapshots_created"`
	RollbacksDone      int64 `json:"rollbacks_done"`
	QuarantinesDone    int64 `json:"quarantines_done"`
	BlocksTriggered    int64 `json:"blocks_triggered"`
}

// countTriggeredHoneypots 统计已触发的蜜罐数.
func (p *Protector) countTriggeredHoneypots() int {
	count := 0
	for _, hp := range p.honeypots {
		if hp.Triggered {
			count++
		}
	}
	return count
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, path)
			target := filepath.Join(dst, rel)
			if info.IsDir() {
				return os.MkdirAll(target, info.Mode())
			}
			return copyFile(path, target, info.Mode())
		})
	}
	return copyFile(src, dst, info.Mode())
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(in)
	return err
}
