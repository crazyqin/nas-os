package aidatamasking

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 数据脱敏管理器
type Manager struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *MaskingEngineConfig
	engine *Engine
	logs   []*MaskingLog
	audit  []*AuditLog
}

// NewManager 创建数据脱敏管理器
func NewManager(logger *zap.Logger, config *MaskingEngineConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultMaskingEngineConfig()
	}

	return &Manager{
		logger: logger,
		config: config,
		engine: NewEngine(config),
		logs:   make([]*MaskingLog, 0),
		audit:  make([]*AuditLog, 0),
	}
}

// MaskText 对文本进行脱敏
func (m *Manager) MaskText(req *MaskingRequest) (*MaskingResponse, error) {
	m.mu.RLock()
	enabled := m.config.Enabled
	m.mu.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("masking engine is disabled")
	}

	start := time.Now()

	// 执行脱敏
	resp, err := m.engine.MaskText(req)
	if err != nil {
		m.addMaskingLog(&MaskingLog{
			ID:        generateLogID(),
			StartTime: start,
			EndTime:   time.Now(),
			Success:   false,
			Error:     err.Error(),
		})
		return nil, err
	}

	// 记录日志
	if m.config.LogEnabled {
		m.addMaskingLog(&MaskingLog{
			ID:        generateLogID(),
			StartTime: start,
			EndTime:   time.Now(),
			Success:   true,
		})
	}

	// 记录审计日志
	if m.config.AuditEnabled {
		m.addAuditLog(&AuditLog{
			ID:        generateLogID(),
			Action:    "mask",
			Details:   fmt.Sprintf("masked text with %d results", len(resp.Results)),
			Timestamp: time.Now(),
		})
	}

	return resp, nil
}

// BatchMaskText 批量文本脱敏
func (m *Manager) BatchMaskText(req *BatchMaskingRequest) (*BatchMaskingResponse, error) {
	m.mu.RLock()
	enabled := m.config.Enabled
	m.mu.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("masking engine is disabled")
	}

	start := time.Now()
	results := make([]*MaskingResponse, 0, len(req.Texts))
	totalMasked := 0

	for _, text := range req.Texts {
		maskReq := &MaskingRequest{
			Text:     text,
			Rules:    req.Rules,
			TestMode: req.TestMode,
		}

		resp, err := m.engine.MaskText(maskReq)
		if err != nil {
			m.logger.Error("batch masking failed for text",
				zap.Error(err),
				zap.Int("index", len(results)))
			continue
		}

		results = append(results, resp)
		if resp.Summary != nil && resp.Summary.TotalMatches > 0 {
			totalMasked++
		}
	}

	// 记录审计日志
	if m.config.AuditEnabled {
		m.addAuditLog(&AuditLog{
			ID:        generateLogID(),
			Action:    "batch_mask",
			Details:   fmt.Sprintf("processed %d texts, %d with sensitive data", len(req.Texts), totalMasked),
			Timestamp: time.Now(),
		})
	}

	return &BatchMaskingResponse{
		Results:     results,
		TotalTexts:  len(req.Texts),
		TotalMasked: totalMasked,
		Duration:    time.Since(start),
	}, nil
}

// ProcessAIPrompt 处理AI提示词
func (m *Manager) ProcessAIPrompt(req *AIPromptRequest) (*AIPromptResponse, error) {
	m.mu.RLock()
	aiConfig := m.config.AIIntegration
	m.mu.RUnlock()

	if aiConfig == nil || !aiConfig.Enabled {
		return &AIPromptResponse{
			OriginalPrompt:   req.Prompt,
			MaskedPrompt:     req.Prompt,
			HasSensitiveData: false,
			MaskingApplied:   false,
		}, nil
	}

	// 检查提示词长度
	if aiConfig.MaxPromptLength > 0 && len(req.Prompt) > aiConfig.MaxPromptLength {
		return nil, fmt.Errorf("prompt length exceeds maximum: %d > %d", len(req.Prompt), aiConfig.MaxPromptLength)
	}

	// 检测敏感数据
	hasSensitive, detectedTypes := m.engine.HasSensitiveData(req.Prompt)

	if !hasSensitive {
		return &AIPromptResponse{
			OriginalPrompt:   req.Prompt,
			MaskedPrompt:     req.Prompt,
			HasSensitiveData: false,
			MaskingApplied:   false,
		}, nil
	}

	// 如果启用了预处理脱敏
	if aiConfig.PreProcess {
		maskResp, err := m.engine.MaskText(&MaskingRequest{
			Text:     req.Prompt,
			TestMode: false,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to mask prompt: %w", err)
		}

		// 记录日志
		if aiConfig.LogPrompts && m.config.LogEnabled {
			m.logger.Info("AI prompt masked",
				zap.String("session_id", req.SessionID),
				zap.Int("detected_types", len(detectedTypes)),
				zap.Int("mask_count", maskResp.Summary.TotalMatches))
		}

		return &AIPromptResponse{
			OriginalPrompt:   req.Prompt,
			MaskedPrompt:     maskResp.MaskedText,
			HasSensitiveData: true,
			MaskingApplied:   true,
		}, nil
	}

	return &AIPromptResponse{
		OriginalPrompt:   req.Prompt,
		MaskedPrompt:     req.Prompt,
		HasSensitiveData: true,
		MaskingApplied:   false,
	}, nil
}

// AddRule 添加脱敏规则
func (m *Manager) AddRule(rule *MaskingRule) error {
	m.mu.Lock()
	err := m.engine.AddRule(rule)
	shouldAudit := err == nil && m.config.AuditEnabled
	m.mu.Unlock()

	if err != nil {
		return err
	}

	if shouldAudit {
		m.addAuditLog(&AuditLog{
			ID:        generateLogID(),
			Action:    "rule_create",
			Details:   fmt.Sprintf("created rule: %s", rule.Name),
			Timestamp: time.Now(),
		})
	}

	m.logger.Info("masking rule added",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name))

	return nil
}

// UpdateRule 更新脱敏规则
func (m *Manager) UpdateRule(id string, rule *MaskingRule) error {
	m.mu.Lock()
	err := m.engine.UpdateRule(id, rule)
	shouldAudit := err == nil && m.config.AuditEnabled
	m.mu.Unlock()

	if err != nil {
		return err
	}

	if shouldAudit {
		m.addAuditLog(&AuditLog{
			ID:        generateLogID(),
			Action:    "rule_update",
			Details:   fmt.Sprintf("updated rule: %s", id),
			Timestamp: time.Now(),
		})
	}

	m.logger.Info("masking rule updated",
		zap.String("id", id))

	return nil
}

// DeleteRule 删除脱敏规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	err := m.engine.DeleteRule(id)
	shouldAudit := err == nil && m.config.AuditEnabled
	m.mu.Unlock()

	if err != nil {
		return err
	}

	if shouldAudit {
		m.addAuditLog(&AuditLog{
			ID:        generateLogID(),
			Action:    "rule_delete",
			Details:   fmt.Sprintf("deleted rule: %s", id),
			Timestamp: time.Now(),
		})
	}

	m.logger.Info("masking rule deleted",
		zap.String("id", id))

	return nil
}

// GetRule 获取脱敏规则
func (m *Manager) GetRule(id string) (*MaskingRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.engine.GetRule(id)
}

// ListRules 列出所有脱敏规则
func (m *Manager) ListRules() []*MaskingRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.engine.ListRules()
}

// HasSensitiveData 检查文本是否包含敏感数据
func (m *Manager) HasSensitiveData(text string) (bool, []SensitiveDataType) {
	return m.engine.HasSensitiveData(text)
}

// GetMaskingLogs 获取脱敏日志
func (m *Manager) GetMaskingLogs(limit int) []*MaskingLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.logs) {
		limit = len(m.logs)
	}

	start := len(m.logs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*MaskingLog, limit)
	copy(result, m.logs[start:])
	return result
}

// GetAuditLogs 获取审计日志
func (m *Manager) GetAuditLogs(limit int) []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.audit) {
		limit = len(m.audit)
	}

	start := len(m.audit) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*AuditLog, limit)
	copy(result, m.audit[start:])
	return result
}

// addMaskingLog 添加脱敏日志
func (m *Manager) addMaskingLog(log *MaskingLog) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logs = append(m.logs, log)

	// 限制日志数量
	maxLogs := 10000
	if len(m.logs) > maxLogs {
		m.logs = m.logs[len(m.logs)-maxLogs:]
	}
}

// addAuditLog 添加审计日志
func (m *Manager) addAuditLog(log *AuditLog) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.audit = append(m.audit, log)

	// 限制日志数量
	maxLogs := 10000
	if len(m.audit) > maxLogs {
		m.audit = m.audit[len(m.audit)-maxLogs:]
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *MaskingEngineConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *MaskingEngineConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config != nil {
		m.config = config
		m.engine.UpdateConfig(config)
	}
}

// generateLogID 生成日志ID
func generateLogID() string {
	return fmt.Sprintf("log-%d", time.Now().UnixNano())
}
