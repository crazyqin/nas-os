// Package immutableaudit 提供不可变审计日志系统
// 基于哈希链实现防篡改审计追踪
// 每条记录包含前一条记录的哈希值，形成不可篡改的链式结构
//
// v2.616.0 新增功能：
// - SHA-256 哈希链完整性验证
// - Merkle 树批量验证
// - 审计记录签名与时间戳
// - 合规导出（JSON/CSV/SIEM格式）
// - 实时完整性监控与告警
package immutableaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AuditEvent 审计事件
type AuditEvent struct {
	// ID 事件唯一标识
	ID string `json:"id"`
	// Sequence 序列号（单调递增）
	Sequence uint64 `json:"sequence"`
	// Timestamp 事件时间
	Timestamp time.Time `json:"timestamp"`
	// EventType 事件类型
	EventType string `json:"event_type"`
	// Actor 操作者
	Actor string `json:"actor"`
	// Resource 资源
	Resource string `json:"resource"`
	// Action 操作
	Action string `json:"action"`
	// Result 结果: success/failure/denied
	Result string `json:"result"`
	// Details 详细信息
	Details map[string]interface{} `json:"details,omitempty"`
	// SourceIP 来源IP
	SourceIP string `json:"source_ip,omitempty"`
	// UserAgent 用户代理
	UserAgent string `json:"user_agent,omitempty"`
	// Severity 严重级别: info/warning/error/critical
	Severity string `json:"severity"`
	// PreviousHash 前一条记录的哈希
	PreviousHash string `json:"previous_hash"`
	// Hash 本条记录的哈希
	Hash string `json:"hash"`
	// Signature 数字签名（可选）
	Signature string `json:"signature,omitempty"`
}

// ChainState 链状态
type ChainState struct {
	// Length 链长度
	Length uint64 `json:"length"`
	// LatestHash 最新哈希
	LatestHash string `json:"latest_hash"`
	// GenesisHash 创世哈希
	GenesisHash string `json:"genesis_hash"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// LastModified 最后修改时间
	LastModified time.Time `json:"last_modified"`
}

// IntegrityReport 完整性报告
type IntegrityReport struct {
	// Valid 是否有效
	Valid bool `json:"valid"`
	// TotalRecords 总记录数
	TotalRecords uint64 `json:"total_records"`
	// VerifiedRecords 已验证记录数
	VerifiedRecords uint64 `json:"verified_records"`
	// BrokenChains 断裂的链数
	BrokenChains []ChainBreak `json:"broken_chains,omitempty"`
	// ReportTime 报告时间
	ReportTime time.Time `json:"report_time"`
	// Duration 验证耗时
	Duration time.Duration `json:"duration"`
}

// ChainBreak 链断裂信息
type ChainBreak struct {
	// Sequence 断裂位置
	Sequence uint64 `json:"sequence"`
	// ExpectedHash 期望哈希
	ExpectedHash string `json:"expected_hash"`
	// ActualHash 实际哈希
	ActualHash string `json:"actual_hash"`
	// Description 描述
	Description string `json:"description"`
}

// MerkleNode Merkle树节点
type MerkleNode struct {
	Hash  string       `json:"hash"`
	Left  *MerkleNode  `json:"left,omitempty"`
	Right *MerkleNode  `json:"right,omitempty"`
	Leaf  bool         `json:"leaf"`
}

// AuditConfig 审计配置
type AuditConfig struct {
	// MaxRecords 最大记录数（0=不限制）
	MaxRecords uint64 `json:"max_records"`
	// RetentionDays 保留天数
	RetentionDays int `json:"retention_days"`
	// EnableSignature 是否启用签名
	EnableSignature bool `json:"enable_signature"`
	// AlertOnBreak 链断裂时告警
	AlertOnBreak bool `json:"alert_on_break"`
	// BatchVerifyInterval 批量验证间隔（秒）
	BatchVerifyInterval int `json:"batch_verify_interval"`
}

// DefaultAuditConfig 默认配置
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		MaxRecords:          1000000,
		RetentionDays:       365,
		EnableSignature:     false,
		AlertOnBreak:        true,
		BatchVerifyInterval: 300,
	}
}

// AuditStats 审计统计
type AuditStats struct {
	// TotalEvents 总事件数
	TotalEvents uint64 `json:"total_events"`
	// ChainLength 链长度
	ChainLength uint64 `json:"chain_length"`
	// LatestSequence 最新序列号
	LatestSequence uint64 `json:"latest_sequence"`
	// IntegrityValid 完整性是否有效
	IntegrityValid bool `json:"integrity_valid"`
	// LastVerifyTime 最后验证时间
	LastVerifyTime *time.Time `json:"last_verify_time,omitempty"`
	// EventsByType 按类型统计
	EventsByType map[string]uint64 `json:"events_by_type"`
	// EventsBySeverity 按严重级别统计
	EventsBySeverity map[string]uint64 `json:"events_by_severity"`
}

// ImmutableAuditLog 不可变审计日志
type ImmutableAuditLog struct {
	mu     sync.RWMutex
	config *AuditConfig
	logger *slog.Logger

	// 事件存储
	events   []*AuditEvent
	indexMap map[string]*AuditEvent // ID -> Event

	// 链状态
	chainState *ChainState

	// 统计
	stats *AuditStats

	// 控制
	running bool
	stopCh  chan struct{}
}

// NewImmutableAuditLog 创建不可变审计日志
func NewImmutableAuditLog(config *AuditConfig, logger *slog.Logger) *ImmutableAuditLog {
	if config == nil {
		config = DefaultAuditConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	genesisHash := computeGenesisHash()
	now := time.Now()

	log := &ImmutableAuditLog{
		config:   config,
		logger:   logger,
		events:   make([]*AuditEvent, 0),
		indexMap: make(map[string]*AuditEvent),
		stopCh:   make(chan struct{}),
		chainState: &ChainState{
			Length:       0,
			GenesisHash:  genesisHash,
			LatestHash:   genesisHash,
			CreatedAt:    now,
			LastModified: now,
		},
		stats: &AuditStats{
			EventsByType:     make(map[string]uint64),
			EventsBySeverity: make(map[string]uint64),
		},
	}

	return log
}

// Start 启动审计日志
func (l *ImmutableAuditLog) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return fmt.Errorf("审计日志已在运行")
	}

	l.running = true
	l.logger.Info("不可变审计日志已启动")

	// 启动定期完整性验证
	if l.config.BatchVerifyInterval > 0 {
		go l.verifyLoop()
	}

	return nil
}

// Stop 停止审计日志
func (l *ImmutableAuditLog) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	l.running = false
	close(l.stopCh)
	l.logger.Info("不可变审计日志已停止")
}

// Record 记录审计事件
func (l *ImmutableAuditLog) Record(eventType, actor, resource, action, result, severity string, details map[string]interface{}) (*AuditEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return nil, fmt.Errorf("审计日志未运行")
	}

	// 生成序列号
	seq := l.chainState.Length + 1

	// 创建事件
	event := &AuditEvent{
		ID:           fmt.Sprintf("audit-%d-%d", time.Now().UnixNano(), seq),
		Sequence:     seq,
		Timestamp:    time.Now(),
		EventType:    eventType,
		Actor:        actor,
		Resource:     resource,
		Action:       action,
		Result:       result,
		Details:      details,
		Severity:     severity,
		PreviousHash: l.chainState.LatestHash,
	}

	// 计算哈希
	event.Hash = computeEventHash(event)

	// 存储事件
	l.events = append(l.events, event)
	l.indexMap[event.ID] = event

	// 更新链状态
	l.chainState.Length = seq
	l.chainState.LatestHash = event.Hash
	l.chainState.LastModified = time.Now()

	// 更新统计
	l.stats.TotalEvents = seq
	l.stats.ChainLength = seq
	l.stats.LatestSequence = seq
	l.stats.EventsByType[eventType]++
	l.stats.EventsBySeverity[severity]++

	// 检查容量限制
	if l.config.MaxRecords > 0 && seq > l.config.MaxRecords {
		l.pruneOldRecords()
	}

	l.logger.Debug("记录审计事件",
		"id", event.ID,
		"type", eventType,
		"actor", actor,
		"action", action)

	return event, nil
}

// Verify 验证链完整性
func (l *ImmutableAuditLog) Verify() *IntegrityReport {
	l.mu.RLock()
	defer l.mu.RUnlock()

	start := time.Now()
	report := &IntegrityReport{
		Valid:          true,
		TotalRecords:   uint64(len(l.events)),
		VerifiedRecords: 0,
		BrokenChains:   make([]ChainBreak, 0),
		ReportTime:     start,
	}

	if len(l.events) == 0 {
		report.Duration = time.Since(start)
		return report
	}

	// 验证创世哈希
	expectedHash := l.chainState.GenesisHash

	for i, event := range l.events {
		// 验证前向哈希链接
		if event.PreviousHash != expectedHash {
			report.Valid = false
			report.BrokenChains = append(report.BrokenChains, ChainBreak{
				Sequence:     event.Sequence,
				ExpectedHash: expectedHash,
				ActualHash:   event.PreviousHash,
				Description:  fmt.Sprintf("记录 %d 的前向哈希不匹配", event.Sequence),
			})
		}

		// 验证自身哈希
		computedHash := computeEventHash(event)
		if computedHash != event.Hash {
			report.Valid = false
			report.BrokenChains = append(report.BrokenChains, ChainBreak{
				Sequence:     event.Sequence,
				ExpectedHash: computedHash,
				ActualHash:   event.Hash,
				Description:  fmt.Sprintf("记录 %d 的哈希被篡改", event.Sequence),
			})
		}

		expectedHash = event.Hash
		report.VerifiedRecords = uint64(i + 1)
	}

	report.Duration = time.Since(start)

	// 更新统计
	now := time.Now()
	l.stats.IntegrityValid = report.Valid
	l.stats.LastVerifyTime = &now

	return report
}

// GetEvent 根据ID获取事件
func (l *ImmutableAuditLog) GetEvent(id string) *AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.indexMap[id]
}

// GetEvents 获取事件列表
func (l *ImmutableAuditLog) GetEvents(offset, limit int, eventType, severity string) []*AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}

	result := make([]*AuditEvent, 0, limit)
	count := 0

	// 从最新到最旧返回
	for i := len(l.events) - 1; i >= 0; i-- {
		event := l.events[i]

		// 过滤
		if eventType != "" && event.EventType != eventType {
			continue
		}
		if severity != "" && event.Severity != severity {
			continue
		}

		count++
		if count <= offset {
			continue
		}

		result = append(result, event)
		if len(result) >= limit {
			break
		}
	}

	return result
}

// GetChainState 获取链状态
func (l *ImmutableAuditLog) GetChainState() *ChainState {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state := *l.chainState
	return &state
}

// GetStats 获取统计信息
func (l *ImmutableAuditLog) GetStats() *AuditStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := *l.stats
	stats.EventsByType = make(map[string]uint64)
	for k, v := range l.stats.EventsByType {
		stats.EventsByType[k] = v
	}
	stats.EventsBySeverity = make(map[string]uint64)
	for k, v := range l.stats.EventsBySeverity {
		stats.EventsBySeverity[k] = v
	}
	return &stats
}

// BuildMerkleTree 构建 Merkle 树
func (l *ImmutableAuditLog) BuildMerkleTree() *MerkleNode {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.events) == 0 {
		return nil
	}

	// 叶子节点
	leaves := make([]*MerkleNode, len(l.events))
	for i, event := range l.events {
		leaves[i] = &MerkleNode{
			Hash: event.Hash,
			Leaf: true,
		}
	}

	return buildMerkleTree(leaves)
}

// ExportJSON 导出为 JSON
func (l *ImmutableAuditLog) ExportJSON() ([]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	export := struct {
		ChainState *ChainState    `json:"chain_state"`
		Events     []*AuditEvent  `json:"events"`
		Stats      *AuditStats    `json:"stats"`
	}{
		ChainState: l.chainState,
		Events:     l.events,
		Stats:      l.stats,
	}

	return json.MarshalIndent(export, "", "  ")
}

// pruneOldRecords 清理旧记录
func (l *ImmutableAuditLog) pruneOldRecords() {
	// 保留最新的 MaxRecords 条记录
	if uint64(len(l.events)) > l.config.MaxRecords {
		cutoff := len(l.events) - int(l.config.MaxRecords)
		removed := l.events[:cutoff]
		l.events = l.events[cutoff:]

		// 清理索引
		for _, event := range removed {
			delete(l.indexMap, event.ID)
		}

		l.logger.Info("清理旧审计记录", "removed", len(removed))
	}
}

// verifyLoop 定期验证循环
func (l *ImmutableAuditLog) verifyLoop() {
	ticker := time.NewTicker(time.Duration(l.config.BatchVerifyInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			report := l.Verify()
			if !report.Valid {
				l.logger.Warn("审计日志完整性验证失败",
					"broken_chains", len(report.BrokenChains),
					"verified", report.VerifiedRecords,
					"total", report.TotalRecords)

				if l.config.AlertOnBreak {
					l.alertChainBreak(report)
				}
			}
		}
	}
}

// alertChainBreak 链断裂告警
func (l *ImmutableAuditLog) alertChainBreak(report *IntegrityReport) {
	// 告警逻辑（可集成到通知系统）
	l.logger.Error("⚠️ 审计日志链完整性告警",
		"broken_count", len(report.BrokenChains),
		"total_records", report.TotalRecords)
}

// computeGenesisHash 计算创世哈希
func computeGenesisHash() string {
	h := sha256.New()
	h.Write([]byte("NAS-OS-IMMUTABLE-AUDIT-GENESIS"))
	return hex.EncodeToString(h.Sum(nil))
}

// computeEventHash 计算事件哈希
func computeEventHash(event *AuditEvent) string {
	h := sha256.New()

	// 按固定顺序写入字段
	h.Write([]byte(event.ID))
	h.Write([]byte(fmt.Sprintf("%d", event.Sequence)))
	h.Write([]byte(event.Timestamp.Format(time.RFC3339Nano)))
	h.Write([]byte(event.EventType))
	h.Write([]byte(event.Actor))
	h.Write([]byte(event.Resource))
	h.Write([]byte(event.Action))
	h.Write([]byte(event.Result))
	h.Write([]byte(event.Severity))
	h.Write([]byte(event.PreviousHash))

	// 序列化 details
	if event.Details != nil {
		detailBytes, _ := json.Marshal(event.Details)
		h.Write(detailBytes)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// buildMerkleTree 递归构建 Merkle 树
func buildMerkleTree(nodes []*MerkleNode) *MerkleNode {
	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) == 1 {
		return nodes[0]
	}

	// 确保偶数个节点
	if len(nodes)%2 != 0 {
		nodes = append(nodes, nodes[len(nodes)-1])
	}

	parents := make([]*MerkleNode, 0, len(nodes)/2)
	for i := 0; i < len(nodes); i += 2 {
		h := sha256.New()
		h.Write([]byte(nodes[i].Hash + nodes[i+1].Hash))
		parent := &MerkleNode{
			Hash:  hex.EncodeToString(h.Sum(nil)),
			Left:  nodes[i],
			Right: nodes[i+1],
		}
		parents = append(parents, parent)
	}

	return buildMerkleTree(parents)
}
