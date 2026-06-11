// Package ransomhoneypot - 蜜罐管理器
// 负责诱饵文件的自动部署、生命周期管理和行为监控
package ransomhoneypot

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 蜜罐管理器
// ============================================================

// HoneypotManager 蜜罐管理器 — 核心协调组件
type HoneypotManager struct {
	mu            sync.RWMutex
	config        *HoneypotConfig
	decoys        map[string]*DecoyFile      // decoyID -> 诱饵文件
	targets       map[string]*MonitorTarget   // targetID -> 监控目标
	detections    []*ThreatDetection          // 威胁检测结果
	events        []*FileChangeEvent          // 文件变更事件
	detector      *AIBehaviorDetector         // AI 行为检测器
	stats         *DetectionStats             // 检测统计
	isMonitoring  bool                        // 是否正在监控
	startTime     time.Time                   // 启动时间
	eventCounter  int64                       // 事件计数器
	detectCounter int64                       // 检测计数器
	stopCh        chan struct{}                // 停止信号
}

// NewHoneypotManager 创建蜜罐管理器
func NewHoneypotManager(config *HoneypotConfig) *HoneypotManager {
	if config == nil {
		config = DefaultConfig()
	}

	m := &HoneypotManager{
		config:     config,
		decoys:     make(map[string]*DecoyFile),
		targets:    make(map[string]*MonitorTarget),
		detections: make([]*ThreatDetection, 0),
		events:     make([]*FileChangeEvent, 0),
		stats:      &DetectionStats{},
		stopCh:     make(chan struct{}),
	}

	// 初始化 AI 行为检测器
	m.detector = NewAIBehaviorDetector(config)

	return m
}

// DefaultConfig 返回默认配置
func DefaultConfig() *HoneypotConfig {
	return &HoneypotConfig{
		Enabled:             true,
		DecoyCountPerDir:    DefaultDecoyCount,
		MonitorIntervalSec:  DefaultMonitorIntervalSec,
		EntropyThreshold:    DefaultEntropyThreshold,
		BatchThreshold:      DefaultBatchThreshold,
		MaxEvents:           DefaultMaxEvents,
		AutoResponse:        true,
		DefaultAction:       ResponseActionIsolate,
		QuarantinePath:      DefaultIsolationQuarantine,
		ProtectedExtensions: []string{".docx", ".xlsx", ".pptx", ".pdf", ".jpg", ".png", ".raw", ".mp4", ".zip", ".7z", ".sql", ".db", ".py", ".go", ".js"},
		ExemptUsers:         []string{},
		ExemptIPs:           []string{},
		BackupOnThreat:      true,
	}
}

// ============================================================
// 诱饵文件管理
// ============================================================

// DeployDecoys 在指定监控目录部署诱饵文件
// 自动生成多种类型的诱饵文件，模拟真实 NAS 文件结构
func (m *HoneypotManager) DeployDecoys(target *MonitorTarget) ([]*DecoyFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if target.Path == "" {
		return nil, ErrInvalidPath
	}

	// 确保目标目录存在
	if err := os.MkdirAll(target.Path, 0755); err != nil {
		return nil, fmt.Errorf("创建监控目录失败: %w", err)
	}

	if target.ID == "" {
		target.ID = generateID()
	}
	target.CreatedAt = time.Now()

	count := target.DecoyCount
	if count <= 0 {
		count = m.config.DecoyCountPerDir
	}

	// 确定要部署的诱饵类型
	types := target.WatchTypes
	if len(types) == 0 {
		types = []string{
			DecoyTypeDocument, DecoyTypePhoto, DecoyTypeDatabase,
			DecoyTypeCode, DecoyTypeBackup, DecoyTypeConfig,
		}
	}

	deployed := make([]*DecoyFile, 0, count)
	for i := 0; i < count; i++ {
		decoyType := types[i%len(types)]
		decoy, err := m.createDecoyFile(target.Path, target.ShareName, decoyType)
		if err != nil {
			continue // 跳过部署失败的诱饵
		}

		m.decoys[decoy.ID] = decoy
		deployed = append(deployed, decoy)
		m.stats.TotalDecoys++
		m.stats.ActiveDecoys++
	}

	m.targets[target.ID] = target
	return deployed, nil
}

// createDecoyFile 创建单个诱饵文件
func (m *HoneypotManager) createDecoyFile(dir, shareName, decoyType string) (*DecoyFile, error) {
	// 生成诱饵文件名和内容
	fileName, content := generateDecoyContent(decoyType)
	filePath := filepath.Join(dir, fileName)

	// 写入诱饵文件
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return nil, fmt.Errorf("写入诱饵文件失败 %s: %w", filePath, err)
	}

	// 计算内容哈希
	hash := sha256.Sum256(content)
	entropy := AnalyzeEntropy(content)

	decoy := &DecoyFile{
		ID:            generateID(),
		Path:          filePath,
		Type:          decoyType,
		FileName:      fileName,
		FileSize:      int64(len(content)),
		ContentHash:   hex.EncodeToString(hash[:]),
		Entropy:       entropy,
		Enabled:       true,
		MonitorDir:    dir,
		Tags:          []string{"auto-deployed", decoyType},
		CreatedAt:     time.Now(),
		LastCheckedAt: time.Now(),
	}

	return decoy, nil
}

// RemoveDecoy 移除单个诱饵文件
func (m *HoneypotManager) RemoveDecoy(decoyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	decoy, exists := m.decoys[decoyID]
	if !exists {
		return ErrDecoyNotFound
	}

	// 删除物理文件（忽略不存在的情况）
	os.Remove(decoy.Path)

	if decoy.Enabled {
		m.stats.ActiveDecoys--
	}
	m.stats.TotalDecoys--
	delete(m.decoys, decoyID)
	return nil
}

// RemoveTarget 移除监控目标及其所有诱饵
func (m *HoneypotManager) RemoveTarget(targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[targetID]
	if !exists {
		return ErrDecoyNotFound
	}

	// 删除该目标下的所有诱饵
	for id, decoy := range m.decoys {
		if decoy.MonitorDir == target.Path {
			os.Remove(decoy.Path)
			if decoy.Enabled {
				m.stats.ActiveDecoys--
			}
			m.stats.TotalDecoys--
			delete(m.decoys, id)
		}
	}

	delete(m.targets, targetID)
	return nil
}

// RefreshDecoys 刷新指定目录的诱饵文件
// 删除被篡改的诱饵并重新部署
func (m *HoneypotManager) RefreshDecoys(targetID string) (int, error) {
	m.mu.Lock()
	target, exists := m.targets[targetID]
	if !exists {
		m.mu.Unlock()
		return 0, ErrDecoyNotFound
	}
	dir := target.Path
	m.mu.Unlock()

	// 检查哪些诱饵文件已被篡改
	corrupted := m.CheckDecoyIntegrity(dir)

	m.mu.Lock()
	defer m.mu.Unlock()

	refreshed := 0
	for _, decoy := range corrupted {
		// 删除被篡改的诱饵
		os.Remove(decoy.Path)
		delete(m.decoys, decoy.ID)
		m.stats.TotalDecoys--
		if decoy.Enabled {
			m.stats.ActiveDecoys--
		}

		// 重新部署
		newDecoy, err := m.createDecoyFile(dir, decoy.MonitorDir, decoy.Type)
		if err != nil {
			continue
		}
		m.decoys[newDecoy.ID] = newDecoy
		m.stats.TotalDecoys++
		m.stats.ActiveDecoys++
		refreshed++
	}

	return refreshed, nil
}

// CheckDecoyIntegrity 检查诱饵文件完整性
// 返回被篡改的诱饵列表
func (m *HoneypotManager) CheckDecoyIntegrity(dir string) []*DecoyFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	corrupted := make([]*DecoyFile, 0)
	for _, decoy := range m.decoys {
		if !decoy.Enabled || (dir != "" && decoy.MonitorDir != dir) {
			continue
		}

		// 检查文件是否存在
		data, err := os.ReadFile(decoy.Path)
		if err != nil {
			// 文件被删除 — 可疑！
			decoyCopy := *decoy
			corrupted = append(corrupted, &decoyCopy)
			continue
		}

		// 验证内容哈希
		hash := sha256.Sum256(data)
		currentHash := hex.EncodeToString(hash[:])
		if currentHash != decoy.ContentHash {
			decoyCopy := *decoy
			corrupted = append(corrupted, &decoyCopy)
		}

		decoy.LastCheckedAt = time.Now()
	}

	return corrupted
}

// ============================================================
// 文件变更事件处理
// ============================================================

// ReportFileChange 上报文件变更事件
func (m *HoneypotManager) ReportFileChange(event *FileChangeEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.eventCounter++
	event.ID = fmt.Sprintf("fch-%d-%s", m.eventCounter, generateShortID())
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 检查是否命中诱饵文件
	if decoy := m.findDecoyByPath(event.FilePath); decoy != nil {
		event.IsDecoy = true
		event.DecoyID = decoy.ID
		decoy.TriggerCount++
		m.stats.TriggeredDecoys++
	}

	// 保留事件到最大限制
	m.events = append(m.events, event)
	if len(m.events) > m.config.MaxEvents {
		m.events = m.events[len(m.events)-m.config.MaxEvents:]
	}

	m.stats.TotalEvents++
}

// findDecoyByPath 通过文件路径查找诱饵（需持锁）
func (m *HoneypotManager) findDecoyByPath(path string) *DecoyFile {
	for _, decoy := range m.decoys {
		if decoy.Path == path && decoy.Enabled {
			return decoy
		}
	}
	return nil
}

// ============================================================
// 监控生命周期
// ============================================================

// StartMonitoring 启动行为监控
func (m *HoneypotManager) StartMonitoring() error {
	m.mu.Lock()
	if m.isMonitoring {
		m.mu.Unlock()
		return ErrAlreadyMonitoring
	}
	m.isMonitoring = true
	m.startTime = time.Now()
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go m.monitorLoop()
	return nil
}

// StopMonitoring 停止行为监控
func (m *HoneypotManager) StopMonitoring() error {
	m.mu.Lock()
	if !m.isMonitoring {
		m.mu.Unlock()
		return ErrNotMonitoring
	}
	m.isMonitoring = false
	close(m.stopCh)
	m.mu.Unlock()
	return nil
}

// monitorLoop 监控主循环
func (m *HoneypotManager) monitorLoop() {
	ticker := time.NewTicker(time.Duration(m.config.MonitorIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runDetectionCycle()
		}
	}
}

// runDetectionCycle 执行一次检测周期
func (m *HoneypotManager) runDetectionCycle() {
	// 1. 检查诱饵文件完整性
	m.mu.RLock()
	targets := make([]*MonitorTarget, 0, len(m.targets))
	for _, t := range m.targets {
		if t.Enabled {
			targets = append(targets, t)
		}
	}
	m.mu.RUnlock()

	for _, target := range targets {
		corrupted := m.CheckDecoyIntegrity(target.Path)
		if len(corrupted) > 0 {
			// 发现被篡改的诱饵 — 触发 AI 分析
			m.handleDecoyTampered(corrupted)
		}
	}

	// 2. AI 分析近期事件
	m.mu.RLock()
	recentEvents := m.getRecentEvents(60) // 最近 60 秒
	m.mu.RUnlock()

	if len(recentEvents) > 0 {
		result := m.detector.Analyze(recentEvents, m.stats)
		if result.ThreatLevel >= ThreatLevelMedium {
			m.handleThreatDetected(result, recentEvents)
		}
	}

	// 3. 更新统计
	m.mu.Lock()
	m.stats.UptimeSeconds = int64(time.Since(m.startTime).Seconds())
	m.mu.Unlock()
}

// handleDecoyTampered 处理诱饵文件被篡改
func (m *HoneypotManager) handleDecoyTampered(corrupted []*DecoyFile) {
	affectedPaths := make([]string, 0, len(corrupted))
	decoyIDs := make([]string, 0, len(corrupted))
	for _, c := range corrupted {
		affectedPaths = append(affectedPaths, c.Path)
		decoyIDs = append(decoyIDs, c.ID)
	}

	detection := &ThreatDetection{
		ID:              fmt.Sprintf("det-%s", generateShortID()),
		Timestamp:       time.Now(),
		ThreatLevel:     ThreatLevelHigh,
		ConfidenceScore: 0.85,
		Description:     fmt.Sprintf("检测到 %d 个诱饵文件被篡改，疑似勒索软件活动", len(corrupted)),
		TriggeredDecoys: decoyIDs,
		AffectedFiles:   affectedPaths,
	}

	// 自动响应
	if m.config.AutoResponse {
		m.executeResponse(detection, affectedPaths)
	}

	m.mu.Lock()
	m.detections = append(m.detections, detection)
	m.stats.ThreatsDetected++
	if detection.Isolated {
		m.stats.ThreatsBlocked++
	}
	now := time.Now()
	m.stats.LastDetection = &now
	m.detectCounter++
	m.mu.Unlock()
}

// handleThreatDetected 处理 AI 检测到的威胁
func (m *HoneypotManager) handleThreatDetected(result *AIAnalysisResult, events []*FileChangeEvent) {
	affectedPaths := make([]string, 0)
	eventIDs := make([]string, 0)
	sourceIP := ""
	sourceUser := ""

	for _, e := range events {
		affectedPaths = appendUnique(affectedPaths, e.FilePath)
		eventIDs = append(eventIDs, e.ID)
		if e.SourceIP != "" && sourceIP == "" {
			sourceIP = e.SourceIP
		}
		if e.SourceUser != "" && sourceUser == "" {
			sourceUser = e.SourceUser
		}
	}

	detection := &ThreatDetection{
		ID:              fmt.Sprintf("det-%s", generateShortID()),
		Timestamp:       time.Now(),
		ThreatLevel:     result.ThreatLevel,
		ConfidenceScore: result.OverallScore,
		Description:     fmt.Sprintf("AI 行为分析检测到威胁（综合得分: %.2f），级别: %d", result.OverallScore, result.ThreatLevel),
		TriggeredEvents: eventIDs,
		AffectedFiles:   affectedPaths,
		SourceIP:        sourceIP,
		SourceUser:      sourceUser,
	}

	// 构建详细分析
	var details strings.Builder
	details.WriteString(fmt.Sprintf("熵值异常得分: %.2f\n", result.EntropyScore))
	details.WriteString(fmt.Sprintf("批量重命名得分: %.2f\n", result.BatchRenameScore))
	details.WriteString(fmt.Sprintf("文件变更速率得分: %.2f\n", result.FileChangeScore))
	details.WriteString(fmt.Sprintf("蜜罐触发得分: %.2f\n", result.DecoyTriggerScore))
	for _, ind := range result.Indicators {
		if ind.Exceeded {
			details.WriteString(fmt.Sprintf("[!] %s: %v (阈值: %v)\n", ind.Name, ind.Value, ind.Threshold))
		}
	}
	detection.Details = details.String()

	// 自动响应
	if m.config.AutoResponse {
		m.executeResponse(detection, affectedPaths)
	}

	m.mu.Lock()
	m.detections = append(m.detections, detection)
	m.stats.ThreatsDetected++
	if detection.Isolated {
		m.stats.ThreatsBlocked++
		m.stats.FilesIsolated += int64(len(affectedPaths))
	}
	now := time.Now()
	m.stats.LastDetection = &now
	m.updateThreatSource(detection)
	m.detectCounter++
	m.mu.Unlock()
}

// executeResponse 执行自动响应动作
func (m *HoneypotManager) executeResponse(detection *ThreatDetection, paths []string) {
	action := m.config.DefaultAction
	detection.ResponseAction = action

	switch action {
	case ResponseActionIsolate:
		if err := m.isolateFiles(paths); err == nil {
			detection.Isolated = true
			detection.IsolationPath = m.config.QuarantinePath
		}
	case ResponseActionSnapshot:
		detection.AutoResponded = true
		// 实际环境中调用 ZFS/Btrfs 快照接口
	case ResponseActionLockShare:
		detection.AutoResponded = true
		// 实际环境中锁定相关共享目录
	default:
		detection.AutoResponded = true
	}
}

// isolateFiles 将受影响文件隔离到安全目录
func (m *HoneypotManager) isolateFiles(paths []string) error {
	quarantineDir := m.config.QuarantinePath
	if quarantineDir == "" {
		quarantineDir = DefaultIsolationQuarantine
	}

	// 创建隔离目录（带时间戳子目录）
	isolationDir := filepath.Join(quarantineDir, time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(isolationDir, 0700); err != nil {
		return fmt.Errorf("%w: %v", ErrIsolationFailed, err)
	}

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		// 移动到隔离目录
		destPath := filepath.Join(isolationDir, filepath.Base(path))
		if err := os.Rename(path, destPath); err != nil {
			// 如果跨文件系统无法 rename，尝试复制后删除
			if err := copyFile(path, destPath); err != nil {
				continue
			}
			os.Remove(path)
		}
	}

	return nil
}

// ============================================================
// 查询接口
// ============================================================

// GetConfig 获取当前配置
func (m *HoneypotManager) GetConfig() *HoneypotConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *HoneypotManager) UpdateConfig(config *HoneypotConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	// 同步更新检测器配置
	m.detector = NewAIBehaviorDetector(config)
}

// GetStats 获取检测统计
func (m *HoneypotManager) GetStats() *DetectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := *m.stats
	stats.UptimeSeconds = int64(time.Since(m.startTime).Seconds())
	return &stats
}

// GetDetections 获取威胁检测结果列表
func (m *HoneypotManager) GetDetections(limit int, level int) []*ThreatDetection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ThreatDetection, 0)
	for i := len(m.detections) - 1; i >= 0; i-- {
		if level > 0 && m.detections[i].ThreatLevel != level {
			continue
		}
		result = append(result, m.detections[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// GetEvents 获取文件变更事件
func (m *HoneypotManager) GetEvents(limit int, eventType string) []*FileChangeEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FileChangeEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if eventType != "" && m.events[i].EventType != eventType {
			continue
		}
		result = append(result, m.events[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ListDecoys 列出所有诱饵文件
func (m *HoneypotManager) ListDecoys() []*DecoyFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*DecoyFile, 0, len(m.decoys))
	for _, d := range m.decoys {
		list = append(list, d)
	}
	return list
}

// ListTargets 列出所有监控目标
func (m *HoneypotManager) ListTargets() []*MonitorTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*MonitorTarget, 0, len(m.targets))
	for _, t := range m.targets {
		list = append(list, t)
	}
	return list
}

// IsMonitoring 返回监控状态
func (m *HoneypotManager) IsMonitoring() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isMonitoring
}

// getRecentEvents 获取最近 N 秒内的事件（需持锁）
func (m *HoneypotManager) getRecentEvents(seconds int) []*FileChangeEvent {
	cutoff := time.Now().Add(-time.Duration(seconds) * time.Second)
	result := make([]*FileChangeEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if m.events[i].Timestamp.Before(cutoff) {
			break
		}
		result = append(result, m.events[i])
	}
	// 按时间正序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}

// updateThreatSource 更新威胁来源统计（需持锁）
func (m *HoneypotManager) updateThreatSource(detection *ThreatDetection) {
	if detection.SourceIP == "" {
		return
	}
	for i, src := range m.stats.TopThreatSources {
		if src.IP == detection.SourceIP {
			m.stats.TopThreatSources[i].Count++
			m.stats.TopThreatSources[i].LastSeen = detection.Timestamp
			if detection.ThreatLevel > src.MaxLevel {
				m.stats.TopThreatSources[i].MaxLevel = detection.ThreatLevel
			}
			return
		}
	}
	m.stats.TopThreatSources = append(m.stats.TopThreatSources, ThreatSource{
		IP:       detection.SourceIP,
		User:     detection.SourceUser,
		Count:    1,
		LastSeen: detection.Timestamp,
		MaxLevel: detection.ThreatLevel,
	})
}

// ============================================================
// 辅助函数
// ============================================================

// AnalyzeEntropy 计算数据的香农熵值
// 高熵值（>7.0 for 8-bit data）通常表示加密或压缩数据
func AnalyzeEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[byte]int64)
	for _, b := range data {
		freq[b]++
	}

	var entropy float64
	length := float64(len(data))
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// generateDecoyContent 根据类型生成诱饵文件内容
// 生成的内容具有真实文件的特征（大小、熵值），但不含敏感数据
func generateDecoyContent(decoyType string) (string, []byte) {
	id := generateShortID()
	switch decoyType {
	case DecoyTypeDocument:
		name := fmt.Sprintf("财务报表_%s.xlsx", id)
		// 模拟文档内容（低熵，类似真实文本）
		content := []byte(fmt.Sprintf("Q4 Financial Report - Confidential\nRevenue: $1,234,567\nExpenses: $890,123\nID: %s\n%s", id, strings.Repeat("padding", 50)))
		return name, content
	case DecoyTypePhoto:
		name := fmt.Sprintf("IMG_2024%s.jpg", id)
		// 模拟图片头部 + 随机数据
		header := []byte{0xFF, 0xD8, 0xFF, 0xE0}
		body := make([]byte, 2048)
		rand.Read(body)
		return name, append(header, body...)
	case DecoyTypeVideo:
		name := fmt.Sprintf("家庭视频_%s.mp4", id)
		body := make([]byte, 4096)
		rand.Read(body)
		return name, body
	case DecoyTypeDatabase:
		name := fmt.Sprintf("app_data_%s.sqlite", id)
		body := make([]byte, 1024)
		rand.Read(body)
		return name, body
	case DecoyTypeCode:
		name := fmt.Sprintf("project_%s/main.go", id)
		content := []byte(fmt.Sprintf("package main\n\nimport \"fmt\"\n\n// Auto-generated project %s\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n", id))
		return name, content
	case DecoyTypeBackup:
		name := fmt.Sprintf("backup_2024_%s.tar.gz", id)
		body := make([]byte, 8192)
		rand.Read(body)
		return name, body
	case DecoyTypeConfig:
		name := fmt.Sprintf(".env.production.%s", id)
		content := []byte(fmt.Sprintf("# Production Config\nDB_HOST=localhost\nDB_PORT=5432\nSECRET_KEY=placeholder_%s\n", id))
		return name, content
	default:
		name := fmt.Sprintf("decoy_%s.dat", id)
		body := make([]byte, 512)
		rand.Read(body)
		return name, body
	}
}

// generateID 生成 32 字符随机十六进制 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateShortID 生成 8 字符随机十六进制 ID
func generateShortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// copyFile 复制文件（用于跨文件系统隔离）
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// appendUnique 追加去重字符串
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
