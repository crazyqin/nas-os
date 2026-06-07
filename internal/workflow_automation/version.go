// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// VersionManager 版本管理器.
type VersionManager struct {
	mu       sync.RWMutex
	versions map[string][]*WorkflowVersion // workflowID -> versions
	store    Store
	logger   *zap.Logger
}

// NewVersionManager 创建版本管理器.
func NewVersionManager(store Store, logger *zap.Logger) *VersionManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &VersionManager{
		versions: make(map[string][]*WorkflowVersion),
		store:    store,
		logger:   logger,
	}
}

// SaveVersion 保存版本快照.
func (vm *VersionManager) SaveVersion(workflowID string, wf *Workflow, comment string, createdBy string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	version := &WorkflowVersion{
		WorkflowID: workflowID,
		Version:    wf.Version,
		Snapshot:   cloneWorkflow(wf),
		Comment:    comment,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now(),
	}

	// 内存缓存
	vm.versions[workflowID] = append(vm.versions[workflowID], version)

	// 持久化
	if vm.store != nil {
		if err := vm.store.SaveVersion(version); err != nil {
			return fmt.Errorf("save version: %w", err)
		}
	}

	vm.logger.Info("version saved",
		zap.String("workflow_id", workflowID),
		zap.Int("version", wf.Version),
	)
	return nil
}

// GetVersions 获取工作流的所有版本.
func (vm *VersionManager) GetVersions(workflowID string) ([]*WorkflowVersion, error) {
	vm.mu.RLock()
	versions, ok := vm.versions[workflowID]
	vm.mu.RUnlock()

	if !ok && vm.store != nil {
		var err error
		versions, err = vm.store.GetVersions(workflowID)
		if err != nil {
			return nil, fmt.Errorf("get versions from store: %w", err)
		}
		vm.mu.Lock()
		vm.versions[workflowID] = versions
		vm.mu.Unlock()
	}

	return versions, nil
}

// GetVersion 获取指定版本.
func (vm *VersionManager) GetVersion(workflowID string, version int) (*WorkflowVersion, error) {
	versions, err := vm.GetVersions(workflowID)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	// 尝试从存储加载
	if vm.store != nil {
		return vm.store.GetVersion(workflowID, version)
	}

	return nil, fmt.Errorf("version %d not found for workflow %s", version, workflowID)
}

// GetLatestVersion 获取最新版本.
func (vm *VersionManager) GetLatestVersion(workflowID string) (*WorkflowVersion, error) {
	versions, err := vm.GetVersions(workflowID)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for workflow %s", workflowID)
	}

	return versions[len(versions)-1], nil
}

// Rollback 回滚到指定版本.
func (vm *VersionManager) Rollback(engine *Engine, workflowID string, targetVersion int) error {
	version, err := vm.GetVersion(workflowID, targetVersion)
	if err != nil {
		return fmt.Errorf("get target version: %w", err)
	}

	if version.Snapshot == nil {
		return fmt.Errorf("version %d has no snapshot", targetVersion)
	}

	// 获取当前工作流（保存当前版本作为回滚前快照）
	current, err := engine.GetWorkflow(workflowID)
	if err != nil {
		return fmt.Errorf("get current workflow: %w", err)
	}

	// 保存当前版本作为回滚前快照
	if err := vm.SaveVersion(workflowID, current,
		fmt.Sprintf("before rollback to v%d", targetVersion), ""); err != nil {
		vm.logger.Warn("failed to save pre-rollback snapshot", zap.Error(err))
	}

	// 恢复目标版本
	restored := version.Snapshot
	restored.Version = current.Version + 1
	restored.UpdatedAt = time.Now()

	if err := engine.UpdateWorkflow(restored); err != nil {
		return fmt.Errorf("update workflow: %w", err)
	}

	vm.logger.Info("workflow rolled back",
		zap.String("workflow_id", workflowID),
		zap.Int("target_version", targetVersion),
		zap.Int("new_version", restored.Version),
	)
	return nil
}

// DiffVersions 比较两个版本的差异.
func (vm *VersionManager) DiffVersions(workflowID string, v1, v2 int) (*VersionDiff, error) {
	v1Snap, err := vm.GetVersion(workflowID, v1)
	if err != nil {
		return nil, err
	}

	v2Snap, err := vm.GetVersion(workflowID, v2)
	if err != nil {
		return nil, err
	}

	return vm.computeDiff(v1Snap, v2Snap), nil
}

// VersionDiff 版本差异.
type VersionDiff struct {
	WorkflowID   string      `json:"workflow_id"`
	Version1     int         `json:"version1"`
	Version2     int         `json:"version2"`
	AddedNodes   []string    `json:"added_nodes"`
	RemovedNodes []string    `json:"removed_nodes"`
	ChangedNodes []string    `json:"changed_nodes"`
	AddedEdges   int         `json:"added_edges"`
	RemovedEdges int         `json:"removed_edges"`
	StatusChange string      `json:"status_change,omitempty"`
}

// computeDiff 计算版本差异.
func (vm *VersionManager) computeDiff(v1, v2 *WorkflowVersion) *VersionDiff {
	diff := &VersionDiff{
		WorkflowID: v1.WorkflowID,
		Version1:   v1.Version,
		Version2:   v2.Version,
	}

	snap1 := v1.Snapshot
	snap2 := v2.Snapshot

	// 比较节点
	for id := range snap2.Nodes {
		if _, ok := snap1.Nodes[id]; !ok {
			diff.AddedNodes = append(diff.AddedNodes, id)
		}
	}
	for id := range snap1.Nodes {
		if _, ok := snap2.Nodes[id]; !ok {
			diff.RemovedNodes = append(diff.RemovedNodes, id)
		} else {
			// 检查节点是否变化（简单比较）
			if snap1.Nodes[id].Name != snap2.Nodes[id].Name ||
				snap1.Nodes[id].Type != snap2.Nodes[id].Type {
				diff.ChangedNodes = append(diff.ChangedNodes, id)
			}
		}
	}

	// 比较边数量
	diff.AddedEdges = len(snap2.Edges) - len(snap1.Edges)
	if diff.AddedEdges < 0 {
		diff.RemovedEdges = -diff.AddedEdges
		diff.AddedEdges = 0
	}

	// 比较状态
	if snap1.Status != snap2.Status {
		diff.StatusChange = fmt.Sprintf("%s -> %s", snap1.Status, snap2.Status)
	}

	return diff
}

// AutoSave 自动保存版本（用于工作流更新时）.
func (vm *VersionManager) AutoSave(engine *Engine, workflowID string) error {
	wf, err := engine.GetWorkflow(workflowID)
	if err != nil {
		return err
	}

	return vm.SaveVersion(workflowID, wf, "auto-save", "")
}

// CleanupOldVersions 清理旧版本（保留最近 N 个）.
func (vm *VersionManager) CleanupOldVersions(workflowID string, keepCount int) int {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	versions := vm.versions[workflowID]
	if len(versions) <= keepCount {
		return 0
	}

	removeCount := len(versions) - keepCount
	vm.versions[workflowID] = versions[removeCount:]

	vm.logger.Info("cleaned up old versions",
		zap.String("workflow_id", workflowID),
		zap.Int("removed", removeCount),
		zap.Int("kept", keepCount),
	)
	return removeCount
}

// ========== 辅助函数 ==========

// CreateAutoSaveVersion 创建自动保存版本.
func CreateAutoSaveVersion(wf *Workflow) *WorkflowVersion {
	return &WorkflowVersion{
		WorkflowID: wf.ID,
		Version:    wf.Version,
		Snapshot:   cloneWorkflow(wf),
		Comment:    "auto-save",
		CreatedAt:  time.Now(),
	}
}

// cloneWorkflow 深拷贝工作流.
func cloneWorkflow(wf *Workflow) *Workflow {
	clone := &Workflow{
		ID:          wf.ID,
		Name:        wf.Name,
		Description: wf.Description,
		Version:     wf.Version,
		Status:      wf.Status,
		CreatedAt:   wf.CreatedAt,
		UpdatedAt:   wf.UpdatedAt,
		CreatedBy:   wf.CreatedBy,
	}

	// 拷贝节点
	clone.Nodes = make(map[string]*Node, len(wf.Nodes))
	for id, node := range wf.Nodes {
		nodeCopy := *node
		if node.Config != nil {
			nodeCopy.Config = make(map[string]string, len(node.Config))
			for k, v := range node.Config {
				nodeCopy.Config[k] = v
			}
		}
		if node.Position != nil {
			posCopy := *node.Position
			nodeCopy.Position = &posCopy
		}
		clone.Nodes[id] = &nodeCopy
	}

	// 拷贝边
	clone.Edges = make([]*Edge, len(wf.Edges))
	for i, edge := range wf.Edges {
		edgeCopy := *edge
		clone.Edges[i] = &edgeCopy
	}

	// 拷贝变量
	if wf.Variables != nil {
		clone.Variables = make(map[string]string, len(wf.Variables))
		for k, v := range wf.Variables {
			clone.Variables[k] = v
		}
	}

	// 拷贝标签
	if wf.Labels != nil {
		clone.Labels = make(map[string]string, len(wf.Labels))
		for k, v := range wf.Labels {
			clone.Labels[k] = v
		}
	}

	return clone
}

// NewWorkflowVersionID 生成版本 ID.
func NewWorkflowVersionID() string {
	return uuid.New().String()
}
