// Package datalifecycle 数据生命周期管理模块
package datalifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ========== 每层级每 GB 月度成本（元） ==========

var tierCostPerGB = map[Tier]float64{
	TierHot:     0.50,
	TierWarm:    0.20,
	TierCold:    0.08,
	TierArchive: 0.03,
}

// Manager 数据生命周期管理器.
type Manager struct {
	mu        sync.RWMutex
	auditMu   sync.Mutex
	store     Store
	logger    *zap.Logger
	policies  map[string]*LifecyclePolicy
	retPolicies map[string]*RetentionPolicy
	lineages  map[string]*DataLineage
	items     map[string]*DataItem
	audits    []*AuditEvent
	costSugs  []*CostSuggestion
	migrations []*MigrationRecord
	running   bool
	stopCh    chan struct{}
}

// NewManager 创建数据生命周期管理器.
func NewManager(store Store, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		store:       store,
		logger:      logger,
		policies:    make(map[string]*LifecyclePolicy),
		retPolicies: make(map[string]*RetentionPolicy),
		lineages:    make(map[string]*DataLineage),
		items:       make(map[string]*DataItem),
		audits:      make([]*AuditEvent, 0),
		costSugs:    make([]*CostSuggestion, 0),
		migrations:  make([]*MigrationRecord, 0),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.logger.Info("datalifecycle manager started")
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	m.logger.Info("datalifecycle manager stopped")
}

// ========== 生命周期策略管理 ==========

// CreatePolicy 创建生命周期策略.
func (m *Manager) CreatePolicy(req CreatePolicyRequest) (*LifecyclePolicy, error) {
	// 验证动作
	for _, a := range req.Actions {
		if !IsValidAction(a.Type) {
			return nil, ErrInvalidAction
		}
		if a.Type == ActionTierDown || a.Type == ActionTierUp {
			if !IsValidTier(a.TargetTier) {
				return nil, ErrInvalidTier
			}
		}
	}
	if req.SourceTier != "" && !IsValidTier(req.SourceTier) {
		return nil, ErrInvalidTier
	}

	now := time.Now()
	policy := &LifecyclePolicy{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		Priority:    req.Priority,
		PathPattern: req.PathPattern,
		Extensions:  req.Extensions,
		MinSize:     req.MinSize,
		MaxSize:     req.MaxSize,
		Tags:        req.Tags,
		SourceTier:  req.SourceTier,
		TriggerDays: req.TriggerDays,
		Schedule:    req.Schedule,
		Actions:     req.Actions,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.mu.Lock()
	m.policies[policy.ID] = policy
	m.mu.Unlock()

	if m.store != nil {
		if err := m.store.SavePolicy(policy); err != nil {
			m.logger.Warn("failed to persist policy", zap.Error(err))
		}
	}

	// 审计
	m.addAuditEvent(EventRetentionPolicy, "", fmt.Sprintf("创建策略: %s (%s)", policy.Name, policy.ID), "", policy.ID)

	m.logger.Info("lifecycle policy created",
		zap.String("id", policy.ID),
		zap.String("name", policy.Name),
	)

	return policy, nil
}

// GetPolicy 获取生命周期策略.
func (m *Manager) GetPolicy(id string) (*LifecyclePolicy, error) {
	m.mu.RLock()
	p, ok := m.policies[id]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListPolicies 列出所有生命周期策略.
func (m *Manager) ListPolicies() []*LifecyclePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*LifecyclePolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority < policies[j].Priority
	})
	return policies
}

// UpdatePolicy 更新生命周期策略.
func (m *Manager) UpdatePolicy(id string, req CreatePolicyRequest) (*LifecyclePolicy, error) {
	m.mu.Lock()
	p, ok := m.policies[id]
	if !ok {
		m.mu.Unlock()
		return nil, ErrPolicyNotFound
	}

	p.Name = req.Name
	p.Description = req.Description
	p.Enabled = req.Enabled
	p.Priority = req.Priority
	p.PathPattern = req.PathPattern
	p.Extensions = req.Extensions
	p.MinSize = req.MinSize
	p.MaxSize = req.MaxSize
	p.Tags = req.Tags
	p.SourceTier = req.SourceTier
	p.TriggerDays = req.TriggerDays
	p.Schedule = req.Schedule
	p.Actions = req.Actions
	p.UpdatedAt = time.Now()
	m.mu.Unlock()

	if m.store != nil {
		if err := m.store.SavePolicy(p); err != nil {
			m.logger.Warn("failed to persist policy update", zap.Error(err))
		}
	}

	m.addAuditEvent(EventRetentionPolicy, "", fmt.Sprintf("更新策略: %s (%s)", p.Name, p.ID), "", p.ID)
	return p, nil
}

// DeletePolicy 删除生命周期策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	p, ok := m.policies[id]
	if !ok {
		m.mu.Unlock()
		return ErrPolicyNotFound
	}
	delete(m.policies, id)
	m.mu.Unlock()

	if m.store != nil {
		if err := m.store.DeletePolicy(id); err != nil {
			m.logger.Warn("failed to delete policy from store", zap.Error(err))
		}
	}

	m.addAuditEvent(EventRetentionPolicy, "", fmt.Sprintf("删除策略: %s (%s)", p.Name, p.ID), "", p.ID)
	return nil
}

// ========== 数据项管理 ==========

// RegisterDataItem 注册数据项.
func (m *Manager) RegisterDataItem(item *DataItem) error {
	if item.Path == "" {
		return ErrPathRequired
	}
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if !IsValidTier(item.CurrentTier) {
		item.CurrentTier = TierHot
	}
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.ModifiedAt.IsZero() {
		item.ModifiedAt = now
	}
	if item.AccessedAt.IsZero() {
		item.AccessedAt = now
	}

	m.mu.Lock()
	m.items[item.ID] = item
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.SaveDataItem(item)
	}

	return nil
}

// GetDataItem 获取数据项.
func (m *Manager) GetDataItem(id string) (*DataItem, error) {
	m.mu.RLock()
	item, ok := m.items[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("数据项不存在: %s", id)
	}
	return item, nil
}

// ListDataItems 列出数据项.
func (m *Manager) ListDataItems(pathPrefix string, tier Tier) []*DataItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DataItem
	for _, item := range m.items {
		if pathPrefix != "" && !strings.HasPrefix(item.Path, pathPrefix) {
			continue
		}
		if tier != "" && item.CurrentTier != tier {
			continue
		}
		result = append(result, item)
	}
	return result
}

// ========== 策略评估与执行 ==========

// EvaluatePolicy 评估策略匹配的数据项.
func (m *Manager) EvaluatePolicy(ctx context.Context, policyID string, dryRun bool) (*EvaluateResult, error) {
	m.mu.RLock()
	policy, ok := m.policies[policyID]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrPolicyNotFound
	}
	if !policy.Enabled {
		return nil, ErrPolicyDisabled
	}

	// 查找匹配的数据项
	matched := m.matchItems(ctx, policy)

	result := &EvaluateResult{
		MatchedItems: len(matched),
		DryRun:       dryRun,
	}

	if dryRun {
		// 干跑模式：只返回匹配结果
		for _, item := range matched {
			for _, action := range policy.Actions {
				result.Actions = append(result.Actions, fmt.Sprintf("%s: %s -> %s", action.Type, item.Path, action.TargetTier))
			}
		}
		return result, nil
	}

	// 实际执行
	for _, item := range matched {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		recs := m.executeActions(ctx, item, policy)
		result.Details = append(result.Details, recs...)
	}

	return result, nil
}

// matchItems 根据策略条件匹配数据项.
func (m *Manager) matchItems(_ context.Context, policy *LifecyclePolicy) []*DataItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []*DataItem
	now := time.Now()

	for _, item := range m.items {
		// 路径匹配
		if policy.PathPattern != "" {
			if !matchPathPattern(policy.PathPattern, item.Path, item.Name) {
				continue
			}
		}

		// 扩展名过滤
		if len(policy.Extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(item.Name))
			found := false
			for _, e := range policy.Extensions {
				if strings.ToLower(e) == ext {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 大小过滤
		if policy.MinSize > 0 && item.Size < policy.MinSize {
			continue
		}
		if policy.MaxSize > 0 && item.Size > policy.MaxSize {
			continue
		}

		// 源层级过滤
		if policy.SourceTier != "" && item.CurrentTier != policy.SourceTier {
			continue
		}

		// 标签过滤
		if len(policy.Tags) > 0 {
			if !hasAnyTag(item.Tags, policy.Tags) {
				continue
			}
		}

		// 访问时间条件
		if policy.TriggerDays > 0 {
			threshold := now.AddDate(0, 0, -policy.TriggerDays)
			if item.AccessedAt.After(threshold) {
				continue
			}
		}

		matched = append(matched, item)
	}

	return matched
}

// executeActions 执行策略动作.
func (m *Manager) executeActions(_ context.Context, item *DataItem, policy *LifecyclePolicy) []*MigrationRecord {
	var records []*MigrationRecord

	for _, action := range policy.Actions {
		rec := &MigrationRecord{
			ID:         uuid.New().String(),
			FilePath:   item.Path,
			SourceTier: item.CurrentTier,
			TargetTier: action.TargetTier,
			Action:     action.Type,
			PolicyID:   policy.ID,
			Status:     "completed",
			BytesMoved: item.Size,
			StartedAt:  time.Now(),
		}

		switch action.Type {
		case ActionTierDown, ActionTierUp:
			if action.TargetTier != "" && action.TargetTier != item.CurrentTier {
				oldTier := item.CurrentTier
				item.CurrentTier = action.TargetTier
				rec.CompletedAt = time.Now()
				m.addAuditEvent(EventMigration, item.Path,
					fmt.Sprintf("数据迁移: %s -> %s (策略: %s)", oldTier, action.TargetTier, policy.Name),
					"", policy.ID)
			}
		case ActionArchive:
			oldTier := item.CurrentTier
			item.CurrentTier = TierArchive
			rec.TargetTier = TierArchive
			rec.CompletedAt = time.Now()
			m.addAuditEvent(EventArchive, item.Path,
				fmt.Sprintf("数据归档: %s -> archive (策略: %s)", oldTier, policy.Name),
				"", policy.ID)
		case ActionCompress:
			rec.CompletedAt = time.Now()
			m.addAuditEvent(EventCompress, item.Path,
				fmt.Sprintf("数据压缩 (策略: %s)", policy.Name),
				"", policy.ID)
		case ActionDelete:
			rec.Status = "completed"
			rec.CompletedAt = time.Now()
			m.addAuditEvent(EventDelete, item.Path,
				fmt.Sprintf("数据删除 (策略: %s)", policy.Name),
				"", policy.ID)
		case ActionNotify:
			rec.CompletedAt = time.Now()
		case ActionSnapshot:
			rec.CompletedAt = time.Now()
		}

		m.mu.Lock()
		m.migrations = append(m.migrations, rec)
		m.mu.Unlock()

		if m.store != nil {
			_ = m.store.SaveMigration(rec)
		}

		records = append(records, rec)
	}

	return records
}

// ========== 保留策略管理 ==========

// CreateRetentionPolicy 创建保留策略.
func (m *Manager) CreateRetentionPolicy(req CreateRetentionPolicyRequest) (*RetentionPolicy, error) {
	now := time.Now()
	policy := &RetentionPolicy{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Description:   req.Description,
		Enabled:       req.Enabled,
		Mode:          req.Mode,
		RetentionDays: req.RetentionDays,
		MaxVersions:   req.MaxVersions,
		MaxSizeBytes:  req.MaxSizeBytes,
		MaxCount:      req.MaxCount,
		PathPattern:   req.PathPattern,
		Extensions:    req.Extensions,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	m.mu.Lock()
	m.retPolicies[policy.ID] = policy
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.SaveRetentionPolicy(policy)
	}

	m.addAuditEvent(EventRetentionPolicy, "", fmt.Sprintf("创建保留策略: %s (%s)", policy.Name, policy.ID), "", policy.ID)
	return policy, nil
}

// GetRetentionPolicy 获取保留策略.
func (m *Manager) GetRetentionPolicy(id string) (*RetentionPolicy, error) {
	m.mu.RLock()
	p, ok := m.retPolicies[id]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// ListRetentionPolicies 列出所有保留策略.
func (m *Manager) ListRetentionPolicies() []*RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*RetentionPolicy, 0, len(m.retPolicies))
	for _, p := range m.retPolicies {
		policies = append(policies, p)
	}
	return policies
}

// DeleteRetentionPolicy 删除保留策略.
func (m *Manager) DeleteRetentionPolicy(id string) error {
	m.mu.Lock()
	if _, ok := m.retPolicies[id]; !ok {
		m.mu.Unlock()
		return ErrPolicyNotFound
	}
	delete(m.retPolicies, id)
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.DeleteRetentionPolicy(id)
	}

	return nil
}

// EnforceRetentionPolicy 执行保留策略，返回被清理的数据项列表.
func (m *Manager) EnforceRetentionPolicy(ctx context.Context, policyID string) ([]string, error) {
	m.mu.RLock()
	policy, ok := m.retPolicies[policyID]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrPolicyNotFound
	}
	if !policy.Enabled {
		return nil, ErrPolicyDisabled
	}

	// 匹配数据项
	m.mu.RLock()
	var matched []*DataItem
	now := time.Now()
	for _, item := range m.items {
		if !m.matchRetentionPolicy(item, policy) {
			continue
		}

		switch policy.Mode {
		case RetentionModeTime:
			if policy.RetentionDays > 0 {
				threshold := now.AddDate(0, 0, -policy.RetentionDays)
				if item.ModifiedAt.Before(threshold) {
					matched = append(matched, item)
				}
			}
		case RetentionModeSpace:
			// 空间模式下，匹配所有项，后续按大小排序删除
			matched = append(matched, item)
		case RetentionModeCount:
			// 数量模式下，匹配所有项，后续按时间排序保留最新的
			matched = append(matched, item)
		default:
			matched = append(matched, item)
		}
	}
	m.mu.RUnlock()

	var removed []string

	switch policy.Mode {
	case RetentionModeSpace:
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].AccessedAt.Before(matched[j].AccessedAt)
		})
		var totalSize int64
		for _, item := range matched {
			totalSize += item.Size
		}
		for _, item := range matched {
			if totalSize <= policy.MaxSizeBytes {
				break
			}
			select {
			case <-ctx.Done():
				return removed, ctx.Err()
			default:
			}
			removed = append(removed, item.Path)
			totalSize -= item.Size
			m.addAuditEvent(EventDelete, item.Path,
				fmt.Sprintf("保留策略空间清理: %s", policy.Name), "", policy.ID)
		}

	case RetentionModeCount:
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].ModifiedAt.Before(matched[j].ModifiedAt)
		})
		if policy.MaxCount > 0 && len(matched) > policy.MaxCount {
			toRemove := matched[:len(matched)-policy.MaxCount]
			for _, item := range toRemove {
				select {
				case <-ctx.Done():
					return removed, ctx.Err()
				default:
				}
				removed = append(removed, item.Path)
				m.addAuditEvent(EventDelete, item.Path,
					fmt.Sprintf("保留策略数量清理: %s", policy.Name), "", policy.ID)
			}
		}

	default:
		for _, item := range matched {
			select {
			case <-ctx.Done():
				return removed, ctx.Err()
			default:
			}
			removed = append(removed, item.Path)
			m.addAuditEvent(EventDelete, item.Path,
				fmt.Sprintf("保留策略时间清理: %s", policy.Name), "", policy.ID)
		}
	}

	return removed, nil
}

// matchRetentionPolicy 检查数据项是否匹配保留策略.
func (m *Manager) matchRetentionPolicy(item *DataItem, policy *RetentionPolicy) bool {
	if policy.PathPattern != "" {
		if matched, _ := filepath.Match(policy.PathPattern, item.Path); !matched {
			if matched, _ := filepath.Match(policy.PathPattern, item.Name); !matched {
				return false
			}
		}
	}
	if len(policy.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(item.Name))
		found := false
		for _, e := range policy.Extensions {
			if strings.ToLower(e) == ext {
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

// ========== 数据血缘追踪 ==========

// CreateLineage 创建数据血缘记录.
func (m *Manager) CreateLineage(req CreateLineageRequest) (*DataLineage, error) {
	if req.FilePath == "" {
		return nil, ErrPathRequired
	}

	lineage := &DataLineage{
		ID:         uuid.New().String(),
		FilePath:   req.FilePath,
		SourcePath: req.SourcePath,
		Operation:  req.Operation,
		Operator:   req.Operator,
		Timestamp:  time.Now(),
		Metadata:   req.Metadata,
	}

	m.mu.Lock()
	m.lineages[lineage.ID] = lineage
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.SaveLineage(lineage)
	}

	m.addAuditEvent(EventLineageUpdate, req.FilePath,
		fmt.Sprintf("血缘记录: %s -> %s (%s)", req.SourcePath, req.FilePath, req.Operation),
		req.Operator, "")

	return lineage, nil
}

// GetLineage 获取血缘记录.
func (m *Manager) GetLineage(id string) (*DataLineage, error) {
	m.mu.RLock()
	l, ok := m.lineages[id]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrLineageNotFound
	}
	return l, nil
}

// GetLineageByPath 按路径获取血缘记录.
func (m *Manager) GetLineageByPath(filePath string) []*DataLineage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DataLineage
	for _, l := range m.lineages {
		if l.FilePath == filePath || l.SourcePath == filePath {
			result = append(result, l)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}

// GetLineageGraph 获取数据血缘关系图.
func (m *Manager) GetLineageGraph(filePath string) *LineageGraph {
	m.mu.RLock()
	defer m.mu.RUnlock()

	graph := &LineageGraph{}
	nodeMap := make(map[string]*LineageNode)

	// 收集所有相关记录
	var all []*DataLineage
	for _, l := range m.lineages {
		if l.FilePath == filePath || l.SourcePath == filePath {
			all = append(all, l)
		}
	}

	// 递归扩展（找上下游）
	visited := make(map[string]bool)
	queue := []string{filePath}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if visited[path] {
			continue
		}
		visited[path] = true

		for _, l := range m.lineages {
			if l.FilePath == path || l.SourcePath == path {
				if !visited[l.FilePath] {
					queue = append(queue, l.FilePath)
				}
				if l.SourcePath != "" && !visited[l.SourcePath] {
					queue = append(queue, l.SourcePath)
				}
				all = append(all, l)
			}
		}
	}

	// 构建节点
	for _, l := range all {
		if _, ok := nodeMap[l.FilePath]; !ok {
			nodeMap[l.FilePath] = &LineageNode{
				FilePath:  l.FilePath,
				Timestamp: l.Timestamp,
				Operation: l.Operation,
			}
		}
		if l.SourcePath != "" {
			if _, ok := nodeMap[l.SourcePath]; !ok {
				nodeMap[l.SourcePath] = &LineageNode{
					FilePath: l.SourcePath,
				}
			}
		}
	}

	// 构建父子关系
	for _, l := range all {
		if l.SourcePath != "" {
			parent, ok := nodeMap[l.SourcePath]
			if ok {
				child := nodeMap[l.FilePath]
				child.Parent = l.SourcePath
				parent.Children = append(parent.Children, child)
			}
		}
	}

	// 根节点
	graph.Root = nodeMap[filePath]
	graph.All = make([]*LineageNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		graph.All = append(graph.All, n)
	}

	return graph
}

// DeleteLineage 删除血缘记录.
func (m *Manager) DeleteLineage(id string) error {
	m.mu.Lock()
	if _, ok := m.lineages[id]; !ok {
		m.mu.Unlock()
		return ErrLineageNotFound
	}
	delete(m.lineages, id)
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.DeleteLineage(id)
	}
	return nil
}

// ========== 存储成本优化 ==========

// AnalyzeCosts 分析存储成本并生成优化建议.
func (m *Manager) AnalyzeCosts(ctx context.Context) *CostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &CostSummary{
		ByTier:      make(map[Tier]TierCost),
		GeneratedAt: time.Now(),
	}

	for _, item := range m.items {
		select {
		case <-ctx.Done():
			return summary
		default:
		}

		summary.TotalItems++
		summary.TotalSize += item.Size

		cost := calculateCost(item.Size, item.CurrentTier)
		summary.TotalCost += cost

		tc := summary.ByTier[item.CurrentTier]
		tc.Tier = item.CurrentTier
		tc.ItemCount++
		tc.TotalSize += item.Size
		tc.Cost += cost
		summary.ByTier[item.CurrentTier] = tc

		// 生成优化建议
		sug := m.generateSuggestion(item, cost)
		if sug != nil {
			summary.Suggestions = append(summary.Suggestions, sug)
			summary.TotalSavings += sug.Savings
		}
	}

	// 保存建议
	m.costSugs = summary.Suggestions
	if m.store != nil {
		for _, s := range summary.Suggestions {
			_ = m.store.SaveCostSuggestion(s)
		}
	}

	if summary.TotalSavings > 0 {
		m.addAuditEvent(EventCostAlert, "",
			fmt.Sprintf("发现可优化项: %d 个, 预计节省: %.2f 元/月", len(summary.Suggestions), summary.TotalSavings),
			"", "")
	}

	return summary
}

// generateSuggestion 为单个数据项生成成本优化建议.
func (m *Manager) generateSuggestion(item *DataItem, currentCost float64) *CostSuggestion {
	now := time.Now()

	// 根据访问频率推荐层级
	var suggestedTier Tier
	var reason string

	daysSinceAccess := now.Sub(item.AccessedAt).Hours() / 24

	switch {
	case daysSinceAccess < 7:
		if item.CurrentTier != TierHot {
			suggestedTier = TierHot
			reason = "近7天内频繁访问，建议提升到热数据层"
		}
	case daysSinceAccess < 30:
		if item.CurrentTier == TierHot {
			suggestedTier = TierWarm
			reason = "近30天内有访问，建议迁移到温数据层降低成本"
		} else if item.CurrentTier != TierWarm {
			suggestedTier = TierWarm
			reason = "近30天内有访问，建议迁移到温数据层"
		}
	case daysSinceAccess < 90:
		if item.CurrentTier == TierHot || item.CurrentTier == TierWarm {
			suggestedTier = TierCold
			reason = "超过30天未访问，建议迁移到冷数据层"
		}
	default:
		if item.CurrentTier != TierArchive {
			suggestedTier = TierArchive
			reason = "超过90天未访问，建议归档以最大化节省成本"
		}
	}

	if suggestedTier == "" || suggestedTier == item.CurrentTier {
		return nil
	}

	suggestedCost := calculateCost(item.Size, suggestedTier)

	return &CostSuggestion{
		ID:            uuid.New().String(),
		FilePath:      item.Path,
		CurrentTier:   item.CurrentTier,
		SuggestedTier: suggestedTier,
		CurrentCost:   currentCost,
		SuggestedCost: suggestedCost,
		Savings:       currentCost - suggestedCost,
		Reason:        reason,
		CreatedAt:     now,
	}
}

// ListCostSuggestions 列出成本优化建议.
func (m *Manager) ListCostSuggestions() []*CostSuggestion {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*CostSuggestion, len(m.costSugs))
	copy(result, m.costSugs)
	return result
}

// ========== 审计日志 ==========

// ListAuditEvents 列出审计事件.
func (m *Manager) ListAuditEvents(eventType EventType, limit int) []*AuditEvent {
	m.auditMu.Lock()
	defer m.auditMu.Unlock()

	if limit <= 0 {
		limit = 100
	}

	var result []*AuditEvent
	for i := len(m.audits) - 1; i >= 0; i-- {
		if eventType != "" && m.audits[i].EventType != eventType {
			continue
		}
		result = append(result, m.audits[i])
		if len(result) >= limit {
			break
		}
	}
	return result
}

// addAuditEvent 添加审计事件（需持有锁或在锁外调用后自行持久化）.
func (m *Manager) addAuditEvent(eventType EventType, filePath, details, operator, policyID string) {
	event := &AuditEvent{
		ID:        uuid.New().String(),
		EventType: eventType,
		FilePath:  filePath,
		Details:   details,
		Operator:  operator,
		PolicyID:  policyID,
		Timestamp: time.Now(),
	}

	m.auditMu.Lock()
	m.audits = append(m.audits, event)
	m.auditMu.Unlock()

	if m.store != nil {
		_ = m.store.SaveAuditEvent(event)
	}
}

// GetMigrations 获取迁移记录.
func (m *Manager) GetMigrations(policyID string, limit int) []*MigrationRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	var result []*MigrationRecord
	for i := len(m.migrations) - 1; i >= 0; i-- {
		if policyID != "" && m.migrations[i].PolicyID != policyID {
			continue
		}
		result = append(result, m.migrations[i])
		if len(result) >= limit {
			break
		}
	}
	return result
}

// ========== 辅助函数 ==========

// matchPathPattern 匹配路径模式，支持 ** 递归匹配.
func matchPathPattern(pattern, path, name string) bool {
	// 处理 ** 通配符
	if strings.Contains(pattern, "**") {
		prefix := strings.TrimSuffix(pattern, "**")
		prefix = strings.TrimSuffix(prefix, "/")
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// 标准 glob 匹配
	if m, _ := filepath.Match(pattern, path); m {
		return true
	}
	if m, _ := filepath.Match(pattern, name); m {
		return true
	}
	return false
}

// calculateCost 计算存储成本.
func calculateCost(sizeBytes int64, tier Tier) float64 {
	costPerGB, ok := tierCostPerGB[tier]
	if !ok {
		costPerGB = tierCostPerGB[TierHot]
	}
	sizeGB := float64(sizeBytes) / (1024 * 1024 * 1024)
	return sizeGB * costPerGB
}

// hasAnyTag 检查是否包含任一标签.
func hasAnyTag(itemTags, requiredTags []string) bool {
	tagSet := make(map[string]bool, len(itemTags))
	for _, t := range itemTags {
		tagSet[strings.ToLower(t)] = true
	}
	for _, t := range requiredTags {
		if tagSet[strings.ToLower(t)] {
			return true
		}
	}
	return false
}
