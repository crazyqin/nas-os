// Package filededup 存储去重
// 文件哈希比对、重复检测、空间回收建议
package filededup

import (
	"sync"
	"time"
)

// ScanStatus 扫描状态类型.
type ScanStatus string

const (
	// ScanStatusPending 等待中.
	ScanStatusPending ScanStatus = "pending"
	// ScanStatusRunning 运行中.
	ScanStatusRunning ScanStatus = "running"
	// ScanStatusCompleted 已完成.
	ScanStatusCompleted ScanStatus = "completed"
	// ScanStatusFailed 失败.
	ScanStatusFailed ScanStatus = "failed"
)

// ScanConfig 扫描配置.
type ScanConfig struct {
	Paths        []string `json:"paths"`
	MinFileSize  int64    `json:"minFileSize,omitempty"`
	MaxFileSize  int64    `json:"maxFileSize,omitempty"`
	IncludeExts  []string `json:"includeExts,omitempty"`
	ExcludePaths []string `json:"excludePaths,omitempty"`
	Algorithm    string   `json:"algorithm,omitempty"` // sha256, md5
}

// ScanResult 扫描结果.
type ScanResult struct {
	ID             string            `json:"id"`
	Status         ScanStatus        `json:"status"`
	TotalFiles     int               `json:"totalFiles"`
	DuplicateFiles int               `json:"duplicateFiles"`
	WastedSpace    int64             `json:"wastedSpace"`
	Groups         []*DuplicateGroup `json:"groups"`
	StartedAt      time.Time         `json:"startedAt"`
	CompletedAt    *time.Time        `json:"completedAt,omitempty"`
}

// Recommendation 清理建议.
type Recommendation struct {
	GroupID     string   `json:"groupId"`
	Hash        string   `json:"hash"`
	Files       []string `json:"files"`
	KeepFile    string   `json:"keepFile"`
	WastedSpace int64    `json:"wastedSpace"`
	Reason      string   `json:"reason"`
}

// ExtendedManager 扩展管理器，添加任务要求的方法.
type ExtendedManager struct {
	*Manager
	mu       sync.RWMutex
	scans    map[string]*ScanResult
	scanList []*ScanResult
}

// NewExtendedManager 创建扩展管理器.
func NewExtendedManager(config *ManagerConfig) *ExtendedManager {
	return &ExtendedManager{
		Manager:  NewManager(config),
		scans:    make(map[string]*ScanResult),
		scanList: make([]*ScanResult, 0),
	}
}

// StartScan 启动扫描任务.
func (m *ExtendedManager) StartScan(config *ScanConfig) (*ScanResult, error) {
	// 转换配置
	algorithm := HashSHA256
	if config.Algorithm == "md5" {
		algorithm = HashMD5
	}

	// 执行扫描
	task, err := m.Scan(config.Paths, ScanModeFull, algorithm)
	if err != nil {
		return nil, err
	}

	// 构建结果
	now := time.Now()
	result := &ScanResult{
		ID:        task.TaskID,
		Status:    ScanStatusCompleted,
		StartedAt: task.StartTime,
		Groups:    m.GetDuplicateGroups(),
	}

	if task.Status == "failed" {
		result.Status = ScanStatusFailed
	} else {
		result.CompletedAt = &now
	}

	// 统计信息
	report := m.GenerateReport()
	result.TotalFiles = int(report.TotalFiles)
	result.DuplicateFiles = int(report.DuplicateFiles)
	result.WastedSpace = report.WastedSpace

	// 保存结果
	m.mu.Lock()
	m.scans[result.ID] = result
	m.scanList = append(m.scanList, result)
	m.mu.Unlock()

	return result, nil
}

// GetScanResult 获取扫描结果.
func (m *ExtendedManager) GetScanResult(id string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.scans[id]
	if !ok {
		return nil, ErrFileNotFound
	}
	return result, nil
}

// ListScans 列出所有扫描任务.
func (m *ExtendedManager) ListScans() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ScanResult, len(m.scanList))
	copy(result, m.scanList)
	return result
}

// DeleteDuplicate 删除重复文件（保留第一个）.
func (m *ExtendedManager) DeleteDuplicate(groupID string, keepIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找重复组
	groups := m.GetDuplicateGroups()
	for _, group := range groups {
		if group.GroupID == groupID {
			if keepIndex < 0 || keepIndex >= len(group.Files) {
				return ErrFileNotFound
			}

			// 删除除保留文件外的所有文件
			for i, file := range group.Files {
				if i == keepIndex {
					continue
				}
				if err := m.SoftDeleteFile(file.Path); err != nil {
					return err
				}
			}
			return nil
		}
	}

	return ErrNoDuplicates
}

// GetRecommendations 获取清理建议.
func (m *ExtendedManager) GetRecommendations() []*Recommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := m.GetDuplicateGroups()
	recommendations := make([]*Recommendation, 0, len(groups))

	for _, group := range groups {
		if len(group.Files) < 2 {
			continue
		}

		// 构建文件列表
		files := make([]string, len(group.Files))
		for i, f := range group.Files {
			files[i] = f.Path
		}

		// 选择保留文件（保留最新的）
		keepFile := group.Files[0].Path

		rec := &Recommendation{
			GroupID:     group.GroupID,
			Hash:        group.Hash,
			Files:       files,
			KeepFile:    keepFile,
			WastedSpace: group.WastedSpace,
			Reason:      "保留最新的文件，删除其他重复文件可回收空间",
		}

		recommendations = append(recommendations, rec)
	}

	return recommendations
}
