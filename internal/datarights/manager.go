package datarights

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 数据权利管理器
type Manager struct {
	config    *Config
	requests  map[string]*DataRightRequest
	pias      map[string]*PrivacyImpactAssessment
	deletions map[string]*DeletionResult
	exports   map[string]*ExportResult
	mu        sync.RWMutex
}

// NewManager 创建数据权利管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{
			Enabled:              true,
			MaxRequests:          10000,
			ResponseDeadlineDays: 30,
			AuditEnabled:         true,
		}
	}
	if config.ResponseDeadlineDays == 0 {
		config.ResponseDeadlineDays = 30
	}
	return &Manager{
		config:    config,
		requests:  make(map[string]*DataRightRequest),
		pias:      make(map[string]*PrivacyImpactAssessment),
		deletions: make(map[string]*DeletionResult),
		exports:   make(map[string]*ExportResult),
	}
}

// CreateRequest 创建数据权利请求
func (m *Manager) CreateRequest(req *DataRightRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return fmt.Errorf("datarights module is disabled")
	}

	if len(m.requests) >= m.config.MaxRequests {
		return fmt.Errorf("maximum number of requests reached (%d)", m.config.MaxRequests)
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	req.Status = StatusPending
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	m.requests[req.ID] = req
	return nil
}

// GetRequest 获取数据权利请求
func (m *Manager) GetRequest(id string) (*DataRightRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, exists := m.requests[id]
	if !exists {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	return req, nil
}

// ListRequests 列出所有数据权利请求
func (m *Manager) ListRequests() []*DataRightRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]*DataRightRequest, 0, len(m.requests))
	for _, r := range m.requests {
		requests = append(requests, r)
	}
	return requests
}

// ProcessAccessRequest 处理访问权请求（导出用户数据）
func (m *Manager) ProcessAccessRequest(id string) (*ExportResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[id]
	if !exists {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	if req.Type != RightAccess {
		return nil, fmt.Errorf("request is not an access request")
	}
	if req.Status == StatusCompleted {
		return nil, fmt.Errorf("request already completed")
	}

	req.Status = StatusProcessing
	req.UpdatedAt = time.Now()

	// 模拟数据收集
	now := time.Now()
	result := &ExportResult{
		RequestID:   id,
		Format:      FormatJSON,
		RecordCount: 42,
		FilePath:    fmt.Sprintf("/tmp/export_%s.json", id),
		FileSize:    1024 * 64,
		CompletedAt: now,
	}
	m.exports[id] = result

	req.Status = StatusCompleted
	req.ProcessedAt = &now
	req.UpdatedAt = now

	return result, nil
}

// ProcessDeletionRequest 处理删除权请求（被遗忘权）
func (m *Manager) ProcessDeletionRequest(id string) (*DeletionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[id]
	if !exists {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	if req.Type != RightErasure {
		return nil, fmt.Errorf("request is not an erasure request")
	}
	if req.Status == StatusCompleted {
		return nil, fmt.Errorf("request already completed")
	}

	req.Status = StatusProcessing
	req.UpdatedAt = time.Now()

	// 模拟数据删除
	now := time.Now()
	result := &DeletionResult{
		RequestID:      id,
		DeletedRecords: 156,
		DeletedTables:  []string{"user_profiles", "activity_logs", "preferences", "files"},
		CompletedAt:    now,
	}
	m.deletions[id] = result

	req.Status = StatusCompleted
	req.ProcessedAt = &now
	req.UpdatedAt = now

	return result, nil
}

// ProcessPortabilityRequest 处理可携带权请求
func (m *Manager) ProcessPortabilityRequest(id string) (*ExportResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[id]
	if !exists {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	if req.Type != RightPortability {
		return nil, fmt.Errorf("request is not a portability request")
	}
	if req.Status == StatusCompleted {
		return nil, fmt.Errorf("request already completed")
	}

	req.Status = StatusProcessing
	req.UpdatedAt = time.Now()

	// 模拟结构化数据导出
	now := time.Now()
	result := &ExportResult{
		RequestID:   id,
		Format:      FormatJSON,
		RecordCount: 230,
		FilePath:    fmt.Sprintf("/tmp/portability_%s.json", id),
		FileSize:    1024 * 256,
		CompletedAt: now,
	}
	m.exports[id] = result

	req.Status = StatusCompleted
	req.ProcessedAt = &now
	req.UpdatedAt = now

	return result, nil
}

// RejectRequest 拒绝数据权利请求
func (m *Manager) RejectRequest(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[id]
	if !exists {
		return fmt.Errorf("request not found: %s", id)
	}
	if req.Status == StatusCompleted {
		return fmt.Errorf("request already completed")
	}

	now := time.Now()
	req.Status = StatusRejected
	req.ProcessedAt = &now
	req.UpdatedAt = now
	return nil
}

// CreatePIA 创建隐私影响评估
func (m *Manager) CreatePIA(pia *PrivacyImpactAssessment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return fmt.Errorf("datarights module is disabled")
	}

	if pia.ID == "" {
		pia.ID = uuid.New().String()
	}
	if pia.Status == "" {
		pia.Status = "draft"
	}
	pia.CreatedAt = time.Now()
	pia.UpdatedAt = time.Now()

	m.pias[pia.ID] = pia
	return nil
}

// GetPIA 获取隐私影响评估
func (m *Manager) GetPIA(id string) (*PrivacyImpactAssessment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pia, exists := m.pias[id]
	if !exists {
		return nil, fmt.Errorf("PIA not found: %s", id)
	}
	return pia, nil
}

// ListPIAs 列出所有隐私影响评估
func (m *Manager) ListPIAs() []*PrivacyImpactAssessment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	piaList := make([]*PrivacyImpactAssessment, 0, len(m.pias))
	for _, p := range m.pias {
		piaList = append(piaList, p)
	}
	return piaList
}

// GenerateComplianceReport 生成合规报告
func (m *Manager) GenerateComplianceReport() *ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &ComplianceReport{
		ID:          uuid.New().String(),
		GeneratedAt: time.Now(),
	}

	totalResponseDays := 0.0
	processedCount := 0

	for _, req := range m.requests {
		report.TotalRequests++

		switch req.Type {
		case RightAccess:
			report.AccessRequests++
		case RightErasure:
			report.ErasureRequests++
		case RightPortability:
			report.PortabilityRequests++
		}

		switch req.Status {
		case StatusCompleted:
			report.CompletedRequests++
			if req.ProcessedAt != nil {
				days := req.ProcessedAt.Sub(req.CreatedAt).Hours() / 24
				totalResponseDays += days
				processedCount++
				if days > float64(m.config.ResponseDeadlineDays) {
					report.OverdueRequests++
				}
			}
		case StatusPending:
			report.PendingRequests++
			// 检查是否逾期
			daysSinceCreated := time.Since(req.CreatedAt).Hours() / 24
			if daysSinceCreated > float64(m.config.ResponseDeadlineDays) {
				report.OverdueRequests++
			}
		case StatusRejected:
			report.RejectedRequests++
		}
	}

	if processedCount > 0 {
		report.AverageResponseDays = totalResponseDays / float64(processedCount)
	}

	// 计算合规分数
	if report.TotalRequests > 0 {
		completionRate := float64(report.CompletedRequests) / float64(report.TotalRequests) * 100
		overdueRate := float64(report.OverdueRequests) / float64(report.TotalRequests) * 100
		report.ComplianceScore = completionRate - overdueRate
		if report.ComplianceScore < 0 {
			report.ComplianceScore = 0
		}
	} else {
		report.ComplianceScore = 100
	}

	// 统计活跃 PIA
	for _, pia := range m.pias {
		if pia.Status == "approved" {
			report.ActivePIAs++
		}
	}

	return report
}

// GetDeletionResult 获取删除结果
func (m *Manager) GetDeletionResult(requestID string) (*DeletionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.deletions[requestID]
	if !exists {
		return nil, fmt.Errorf("deletion result not found for request: %s", requestID)
	}
	return result, nil
}

// GetExportResult 获取导出结果
func (m *Manager) GetExportResult(requestID string) (*ExportResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.exports[requestID]
	if !exists {
		return nil, fmt.Errorf("export result not found for request: %s", requestID)
	}
	return result, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_requests": len(m.requests),
		"total_pias":     len(m.pias),
	}

	statusCounts := make(map[RequestStatus]int)
	typeCounts := make(map[RightType]int)
	for _, req := range m.requests {
		statusCounts[req.Status]++
		typeCounts[req.Type]++
	}
	stats["by_status"] = statusCounts
	stats["by_type"] = typeCounts

	return stats
}
