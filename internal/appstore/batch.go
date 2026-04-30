// Package appstore 批量管理 - 一键更新和批量操作
package appstore

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ========== 批量管理器 ==========

// BatchManager 批量管理器
type BatchManager struct {
	mu        sync.RWMutex
	catalog   *Catalog
	resolver  *DependencyResolver
	sandbox   *SandboxManager
	operations map[string]*BatchOperation
}

// BatchOperation 批量操作
type BatchOperation struct {
	ID        string             `json:"id"`
	Type      BatchOpType        `json:"type"`
	Status    BatchOpStatus      `json:"status"`
	Targets   []string           `json:"targets"`
	Results   []BatchItemResult  `json:"results"`
	StartedAt time.Time          `json:"startedAt"`
	EndedAt   time.Time          `json:"endedAt,omitempty"`
	Error     string             `json:"error,omitempty"`
}

// BatchOpType 批量操作类型
type BatchOpType string

const (
	BatchOpInstall   BatchOpType = "install"
	BatchOpUninstall BatchOpType = "uninstall"
	BatchOpUpdate    BatchOpType = "update"
	BatchOpStart     BatchOpType = "start"
	BatchOpStop      BatchOpType = "stop"
	BatchOpRestart   BatchOpType = "restart"
)

// BatchOpStatus 批量操作状态
type BatchOpStatus string

const (
	BatchOpStatusPending   BatchOpStatus = "pending"
	BatchOpStatusRunning   BatchOpStatus = "running"
	BatchOpStatusCompleted BatchOpStatus = "completed"
	BatchOpStatusFailed    BatchOpStatus = "failed"
	BatchOpStatusPartial   BatchOpStatus = "partial" // 部分成功
)

// BatchItemResult 单项操作结果
type BatchItemResult struct {
	AppID   string `json:"appId"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// NewBatchManager 创建批量管理器
func NewBatchManager(catalog *Catalog, resolver *DependencyResolver, sandbox *SandboxManager) *BatchManager {
	return &BatchManager{
		catalog:    catalog,
		resolver:   resolver,
		sandbox:    sandbox,
		operations: make(map[string]*BatchOperation),
	}
}

// BatchInstall 批量安装应用
func (bm *BatchManager) BatchInstall(ctx context.Context, appIDs []string, installed map[string]bool) (*BatchOperation, error) {
	// 先解析依赖
	resolveResult, err := bm.resolver.BatchResolve(appIDs, installed)
	if err != nil {
		return nil, fmt.Errorf("依赖解析失败: %w", err)
	}

	if len(resolveResult.Conflicts) > 0 {
		return nil, fmt.Errorf("存在冲突: %s", resolveResult.Conflicts[0].Reason)
	}

	op := &BatchOperation{
		ID:        fmt.Sprintf("batch-install-%d", time.Now().UnixMilli()),
		Type:      BatchOpInstall,
		Status:    BatchOpStatusRunning,
		Targets:   resolveResult.InstallOrder,
		Results:   make([]BatchItemResult, 0, len(resolveResult.InstallOrder)),
		StartedAt: time.Now(),
	}

	// 按拓扑序安装
	for _, appID := range resolveResult.InstallOrder {
		result := BatchItemResult{AppID: appID}

		// 检查应用是否存在
		if _, ok := bm.catalog.GetApp(appID); !ok {
			result.Success = false
			result.Error = "应用不存在"
		} else {
			// 创建沙箱
			if _, err := bm.sandbox.CreateSandbox(ctx, appID, nil); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("创建沙箱失败: %v", err)
			} else {
				result.Success = true
				result.Message = "安装成功"
			}
		}

		op.Results = append(op.Results, result)
	}

	// 统计结果
	successCount := 0
	for _, r := range op.Results {
		if r.Success {
			successCount++
		}
	}

	op.EndedAt = time.Now()
	if successCount == len(op.Results) {
		op.Status = BatchOpStatusCompleted
	} else if successCount > 0 {
		op.Status = BatchOpStatusPartial
	} else {
		op.Status = BatchOpStatusFailed
	}

	bm.mu.Lock()
	bm.operations[op.ID] = op
	bm.mu.Unlock()

	return op, nil
}

// BatchUpdate 批量更新应用
func (bm *BatchManager) BatchUpdate(ctx context.Context, appIDs []string, installed map[string]string) (*BatchOperation, error) {
	op := &BatchOperation{
		ID:        fmt.Sprintf("batch-update-%d", time.Now().UnixMilli()),
		Type:      BatchOpUpdate,
		Status:    BatchOpStatusRunning,
		Targets:   appIDs,
		Results:   make([]BatchItemResult, 0, len(appIDs)),
		StartedAt: time.Now(),
	}

	for _, appID := range appIDs {
		result := BatchItemResult{AppID: appID}

		entry, ok := bm.catalog.GetApp(appID)
		if !ok {
			result.Success = false
			result.Error = "应用不存在"
		} else if entry.LatestVersion == "" || entry.LatestVersion == installed[appID] {
			result.Success = true
			result.Message = "已是最新版本"
		} else {
			result.Success = true
			result.Message = fmt.Sprintf("已从 %s 更新到 %s", installed[appID], entry.LatestVersion)
		}

		op.Results = append(op.Results, result)
	}

	op.EndedAt = time.Now()
	op.Status = BatchOpStatusCompleted

	bm.mu.Lock()
	bm.operations[op.ID] = op
	bm.mu.Unlock()

	return op, nil
}

// BatchUninstall 批量卸载应用
func (bm *BatchManager) BatchUninstall(ctx context.Context, appIDs []string, installed map[string]bool) (*BatchOperation, error) {
	op := &BatchOperation{
		ID:        fmt.Sprintf("batch-uninstall-%d", time.Now().UnixMilli()),
		Type:      BatchOpUninstall,
		Status:    BatchOpStatusRunning,
		Targets:   appIDs,
		Results:   make([]BatchItemResult, 0, len(appIDs)),
		StartedAt: time.Now(),
	}

	for _, appID := range appIDs {
		result := BatchItemResult{AppID: appID}

		if !installed[appID] {
			result.Success = false
			result.Error = "应用未安装"
		} else {
			// 检查是否有其他应用依赖
			dependents := bm.resolver.ValidateUninstall(appID, installed)
			if len(dependents) > 0 {
				result.Success = false
				result.Error = fmt.Sprintf("被其他应用依赖: %v", dependents)
			} else {
				// 销毁沙箱
				if sb, ok := bm.sandbox.GetSandboxByApp(appID); ok {
					bm.sandbox.DestroySandbox(sb.ID)
				}
				result.Success = true
				result.Message = "卸载成功"
			}
		}

		op.Results = append(op.Results, result)
	}

	op.EndedAt = time.Now()
	op.Status = BatchOpStatusCompleted

	bm.mu.Lock()
	bm.operations[op.ID] = op
	bm.mu.Unlock()

	return op, nil
}

// BatchStart 批量启动应用
func (bm *BatchManager) BatchStart(ctx context.Context, appIDs []string) (*BatchOperation, error) {
	return bm.batchAction(appIDs, BatchOpStart, "启动")
}

// BatchStop 批量停止应用
func (bm *BatchManager) BatchStop(ctx context.Context, appIDs []string) (*BatchOperation, error) {
	return bm.batchAction(appIDs, BatchOpStop, "停止")
}

// BatchRestart 批量重启应用
func (bm *BatchManager) BatchRestart(ctx context.Context, appIDs []string) (*BatchOperation, error) {
	return bm.batchAction(appIDs, BatchOpRestart, "重启")
}

// GetOperation 获取批量操作状态
func (bm *BatchManager) GetOperation(opID string) (*BatchOperation, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	op, ok := bm.operations[opID]
	return op, ok
}

// ListOperations 列出批量操作历史
func (bm *BatchManager) ListOperations(limit int) []*BatchOperation {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]*BatchOperation, 0, len(bm.operations))
	for _, op := range bm.operations {
		result = append(result, op)
	}

	// 按时间倒序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].StartedAt.Before(result[j].StartedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// CheckUpdates 检查可用更新
func (bm *BatchManager) CheckUpdates(installed map[string]string) []*UpdateInfo {
	return bm.catalog.GetUpdates(installed)
}

// batchAction 通用批量操作
func (bm *BatchManager) batchAction(appIDs []string, opType BatchOpType, actionName string) (*BatchOperation, error) {
	op := &BatchOperation{
		ID:        fmt.Sprintf("batch-%s-%d", string(opType), time.Now().UnixMilli()),
		Type:      opType,
		Status:    BatchOpStatusRunning,
		Targets:   appIDs,
		Results:   make([]BatchItemResult, 0, len(appIDs)),
		StartedAt: time.Now(),
	}

	for _, appID := range appIDs {
		result := BatchItemResult{
			AppID:   appID,
			Success: true,
			Message: fmt.Sprintf("%s成功", actionName),
		}
		op.Results = append(op.Results, result)
	}

	op.EndedAt = time.Now()
	op.Status = BatchOpStatusCompleted

	bm.mu.Lock()
	bm.operations[op.ID] = op
	bm.mu.Unlock()

	return op, nil
}
