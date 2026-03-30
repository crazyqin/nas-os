package sensitivemask

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServiceGuard protects cloud AI services from receiving sensitive data
// This is the main entry point for the sensitivity masking system,
// inspired by Synology DSM 7.3 AI Console's data protection mechanism.
type ServiceGuard struct {
	detector      *Detector
	masker        *Masker
	policyManager *PolicyManager
	auditLogger   *AuditLogger
	services      map[string]*ServiceConfig
	mu            sync.RWMutex
}

// ServiceGuardConfig holds configuration for ServiceGuard.
type ServiceGuardConfig struct {
	PolicyStoragePath string `json:"policy_storage_path"`
	AuditStoragePath  string `json:"audit_storage_path"`
	MaxAuditLogs      int    `json:"max_audit_logs"`
	EnableAudit       bool   `json:"enable_audit"`
}

// NewServiceGuard creates a new ServiceGuard instance.
func NewServiceGuard(config ServiceGuardConfig) *ServiceGuard {
	sg := &ServiceGuard{
		policyManager: NewPolicyManager(config.PolicyStoragePath),
		auditLogger:   NewAuditLogger(config.AuditStoragePath, config.MaxAuditLogs),
		services:      make(map[string]*ServiceConfig),
	}

	sg.auditLogger.Enable(config.EnableAudit)
	sg.detector = NewDetector(DefaultDetectorConfig)
	sg.masker = NewMasker(DefaultMaskerConfig)

	// Create default policy
	sg.createDefaultPolicy()

	return sg
}

// createDefaultPolicy creates the default protection policy.
func (sg *ServiceGuard) createDefaultPolicy() {
	sg.policyManager.mu.Lock()
	defer sg.policyManager.mu.Unlock()

	// 直接创建带固定ID的默认策略
	policy := &Policy{
		ID:          "default",
		Name:        "default",
		Description: "默认敏感信息保护策略 - 参考群晖DSM 7.3 AI Console设计",
		Detector:    DefaultDetectorConfig,
		Masker:      DefaultMaskerConfig,
		Actions: PolicyActions{
			BlockTransmission: false,
			MaskBeforeSend:    true,
			LogDetection:      true,
			AlertOnHighRisk:   true,
			NotifyAdmin:       false,
			AuditLevel:        AuditLevelDetailed,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sg.policyManager.policies["default"] = policy
	sg.policyManager.active = "default"
}

// RegisterService registers a cloud AI service for protection.
func (sg *ServiceGuard) RegisterService(config ServiceConfig) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if _, exists := sg.services[config.Name]; exists {
		return fmt.Errorf("service already registered: %s", config.Name)
	}

	sg.services[config.Name] = &config
	return nil
}

// UnregisterService removes a service from protection.
func (sg *ServiceGuard) UnregisterService(name string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	delete(sg.services, name)
}

// ProcessData processes data before sending to a cloud AI service
// This is the main method to call before any cloud AI interaction.
func (sg *ServiceGuard) ProcessData(ctx context.Context, serviceName string, text string, userID string) (*ProcessingResult, error) {
	// Get service config
	sg.mu.RLock()
	service, ok := sg.services[serviceName]
	sg.mu.RUnlock()

	if !ok || !service.Enabled {
		// No protection for unregistered or disabled services
		return &ProcessingResult{
			OriginalText:  text,
			ProcessedText: text,
			Protected:     false,
		}, nil
	}

	// Get policy
	policy, ok := sg.policyManager.GetPolicy(service.PolicyID)
	if !ok {
		policy, _ = sg.policyManager.GetActivePolicy()
		if policy == nil {
			return nil, fmt.Errorf("no valid policy found")
		}
	}

	// Update detector and masker with policy config
	sg.detector.UpdateConfig(policy.Detector)
	sg.masker.UpdateConfig(policy.Masker)

	// Detect sensitive information
	detectionResult, err := sg.detector.Detect(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("detection failed: %w", err)
	}

	result := &ProcessingResult{
		OriginalText:  text,
		Detections:    detectionResult.Matches,
		HasSensitive:  detectionResult.HasSensitive,
		HighRiskCount: detectionResult.HighRiskCount,
		Protected:     true,
	}

	// No sensitive data found
	if !detectionResult.HasSensitive {
		result.ProcessedText = text
		return result, nil
	}

	// Check if transmission should be blocked
	if policy.Actions.BlockTransmission && detectionResult.HighRiskCount > 0 {
		result.Blocked = true
		result.ProcessedText = ""
		result.BlockReason = "检测到高风险敏感信息，已阻止传输"

		// Log audit
		_ = sg.auditLogger.Log(ctx, AuditLog{
			SourceType:  "service",
			SourceID:    serviceName,
			Detections:  detectionResult.Matches,
			Action:      "blocked",
			UserID:      userID,
			ServiceName: serviceName,
			PolicyID:    policy.ID,
			Blocked:     true,
		})

		return result, nil
	}

	// Mask data if enabled
	processedText := text
	if policy.Actions.MaskBeforeSend {
		processedText, err = sg.masker.Mask(ctx, text, detectionResult.Matches)
		if err != nil {
			return nil, fmt.Errorf("masking failed: %w", err)
		}

		// Update masked values in detections
		for i := range result.Detections {
			result.Detections[i].MaskedValue = sg.masker.MaskWithType(
				result.Detections[i].Value,
				result.Detections[i].Type,
				policy.Masker.Strategies[result.Detections[i].Type],
			)
		}
	}

	result.ProcessedText = processedText

	// Log audit if enabled
	if policy.Actions.LogDetection {
		_ = sg.auditLogger.Log(ctx, AuditLog{
			SourceType:  "service",
			SourceID:    serviceName,
			Detections:  detectionResult.Matches,
			Action:      "masked",
			UserID:      userID,
			ServiceName: serviceName,
			PolicyID:    policy.ID,
			Blocked:     false,
		})
	}

	// Alert on high risk if enabled
	if policy.Actions.AlertOnHighRisk && detectionResult.HighRiskCount > 0 {
		sg.sendHighRiskAlert(serviceName, detectionResult)
	}

	return result, nil
}

// ProcessingResult represents the result of data processing.
type ProcessingResult struct {
	OriginalText  string           `json:"original_text"`
	ProcessedText string           `json:"processed_text"`
	Detections    []SensitiveMatch `json:"detections"`
	HasSensitive  bool             `json:"has_sensitive"`
	HighRiskCount int              `json:"high_risk_count"`
	Blocked       bool             `json:"blocked"`
	BlockReason   string           `json:"block_reason"`
	Protected     bool             `json:"protected"`
}

// sendHighRiskAlert sends an alert for high-risk detections.
func (sg *ServiceGuard) sendHighRiskAlert(serviceName string, result *DetectionResult) {
	// In production, this would integrate with notification systems
	// For now, we log it
	fmt.Printf("[SENSITIVE MASK ALERT] Service: %s, High-Risk Detections: %d\n",
		serviceName, result.HighRiskCount)
}

// CheckData performs a check without masking (for validation/preview).
func (sg *ServiceGuard) CheckData(ctx context.Context, text string) (*DetectionResult, error) {
	return sg.detector.Detect(ctx, text)
}

// PreviewMasking shows what the masked output would look like.
func (sg *ServiceGuard) PreviewMasking(ctx context.Context, text string) (string, []SensitiveMatch, error) {
	result, err := sg.detector.Detect(ctx, text)
	if err != nil {
		return text, nil, err
	}

	if !result.HasSensitive {
		return text, nil, nil
	}

	masked, err := sg.masker.Mask(ctx, text, result.Matches)
	if err != nil {
		return text, nil, err
	}

	return masked, result.Matches, nil
}

// GetPolicyManager returns the policy manager for configuration.
func (sg *ServiceGuard) GetPolicyManager() *PolicyManager {
	return sg.policyManager
}

// GetAuditLogger returns the audit logger for querying logs.
func (sg *ServiceGuard) GetAuditLogger() *AuditLogger {
	return sg.auditLogger
}

// UpdateServicePolicy updates the policy for a specific service.
func (sg *ServiceGuard) UpdateServicePolicy(serviceName, policyID string) error {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	service, ok := sg.services[serviceName]
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	service.PolicyID = policyID
	return nil
}

// ListServices lists all registered services.
func (sg *ServiceGuard) ListServices() []*ServiceConfig {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	services := make([]*ServiceConfig, 0, len(sg.services))
	for _, s := range sg.services {
		services = append(services, s)
	}
	return services
}

// QuickProtect is a convenience function for quick data protection
// Use this for simple use cases where you just need masked output.
func QuickProtect(text string) (string, bool, error) {
	result := QuickDetect(text)
	if len(result) == 0 {
		return text, false, nil
	}

	masked, _ := QuickMask(text)
	return masked, true, nil
}

// IsSafeToSend checks if data is safe to send to cloud AI services.
func (sg *ServiceGuard) IsSafeToSend(ctx context.Context, text string) (bool, *DetectionResult, error) {
	result, err := sg.detector.Detect(ctx, text)
	if err != nil {
		return false, nil, err
	}

	// Consider safe if no high-risk detections
	return result.HighRiskCount == 0, result, nil
}
