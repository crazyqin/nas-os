// Package datasovereignty 数据主权管理模块
// 自动识别数据地理位置，确保符合 GDPR/CCPA/PIPL 等各地法规
package datasovereignty

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Region 数据存储区域
type Region string

const (
	RegionChina     Region = "china"
	RegionEU        Region = "eu"
	RegionUS        Region = "us"
	RegionAPAC      Region = "apac"
	RegionGlobal    Region = "global"
	RegionLocal     Region = "local"
)

// ComplianceFramework 合规框架
type ComplianceFramework string

const (
	FrameworkGDPR   ComplianceFramework = "gdpr"
	FrameworkCCPA   ComplianceFramework = "ccpa"
	FrameworkPIPL   ComplianceFramework = "pipl"
	FrameworkHIPAA  ComplianceFramework = "hipaa"
	FrameworkSOX    ComplianceFramework = "sox"
	FrameworkISO27001 ComplianceFramework = "iso27001"
)

// DataClassification 数据分类
type DataClassification string

const (
	ClassPublic       DataClassification = "public"
	ClassInternal     DataClassification = "internal"
	ClassConfidential DataClassification = "confidential"
	ClassRestricted   DataClassification = "restricted"
	ClassPII          DataClassification = "pii"
	ClassSensitive    DataClassification = "sensitive"
)

// TransferStatus 数据传输状态
type TransferStatus string

const (
	TransferPending    TransferStatus = "pending"
	TransferApproved   TransferStatus = "approved"
	TransferRejected   TransferStatus = "rejected"
	TransferCompleted  TransferStatus = "completed"
	TransferViolated   TransferStatus = "violated"
)

// DataPolicy 数据主权策略
type DataPolicy struct {
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	Description        string               `json:"description"`
	AllowedRegions     []Region             `json:"allowed_regions"`
	BlockedRegions     []Region             `json:"blocked_regions"`
	Frameworks         []ComplianceFramework `json:"frameworks"`
	Classification     DataClassification   `json:"classification"`
	EncryptionRequired bool                 `json:"encryption_required"`
	ResidencyDays      int                  `json:"residency_days"`
	RetentionDays      int                  `json:"retention_days"`
	AuditRequired      bool                 `json:"audit_required"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
	Enabled            bool                 `json:"enabled"`
}

// DataAsset 数据资产
type DataAsset struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Path            string            `json:"path"`
	Size            int64             `json:"size"`
	Classification  DataClassification `json:"classification"`
	CurrentRegion   Region            `json:"current_region"`
	OriginRegion    Region            `json:"origin_region"`
	OwnerID         string            `json:"owner_id"`
	PolicyID        string            `json:"policy_id"`
	Encrypted       bool              `json:"encrypted"`
	Compliant       bool              `json:"compliant"`
	Violations      []string          `json:"violations,omitempty"`
	LastAuditAt     *time.Time        `json:"last_audit_at,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// TransferRequest 数据传输请求
type TransferRequest struct {
	ID              string           `json:"id"`
	AssetID         string           `json:"asset_id"`
	SourceRegion    Region           `json:"source_region"`
	TargetRegion    Region           `json:"target_region"`
	RequesterID     string           `json:"requester_id"`
	Reason          string           `json:"reason"`
	Status          TransferStatus   `json:"status"`
	ApprovedBy      string           `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time       `json:"approved_at,omitempty"`
	RejectedReason  string           `json:"rejected_reason,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	Violations      []string         `json:"violations,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID                string             `json:"id"`
	Region            Region             `json:"region"`
	Framework         ComplianceFramework `json:"framework"`
	TotalAssets       int                `json:"total_assets"`
	CompliantAssets   int                `json:"compliant_assets"`
	ViolatingAssets   int                `json:"violating_assets"`
	ComplianceRate    float64            `json:"compliance_rate"`
	Violations        []Violation        `json:"violations"`
	Recommendations   []string           `json:"recommendations"`
	GeneratedAt       time.Time          `json:"generated_at"`
	ValidUntil        time.Time          `json:"valid_until"`
}

// Violation 合规违规
type Violation struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"asset_id"`
	PolicyID    string    `json:"policy_id"`
	Framework   ComplianceFramework `json:"framework"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Region      Region    `json:"region"`
	DetectedAt  time.Time `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Resolved    bool      `json:"resolved"`
}

// SovereigntyStats 数据主权统计
type SovereigntyStats struct {
	TotalAssets       int                       `json:"total_assets"`
	CompliantAssets   int                       `json:"compliant_assets"`
	ViolatingAssets   int                       `json:"violating_assets"`
	TotalPolicies     int                       `json:"total_policies"`
	ActivePolicies    int                       `json:"active_policies"`
	RegionDistribution map[Region]int           `json:"region_distribution"`
	ClassDistribution map[DataClassification]int `json:"class_distribution"`
	ComplianceRate    float64                   `json:"compliance_rate"`
	PendingTransfers  int                       `json:"pending_transfers"`
	LastAuditTime     *time.Time                `json:"last_audit_time,omitempty"`
}

// DataSovereigntyManager 数据主权管理器
type DataSovereigntyManager struct {
	mu              sync.RWMutex
	policies        map[string]*DataPolicy
	assets          map[string]*DataAsset
	transfers       map[string]*TransferRequest
	violations      []Violation
	auditLog        []AuditEntry
	config          *SovereigntyConfig
}

// SovereigntyConfig 数据主权配置
type SovereigntyConfig struct {
	DefaultRegion        Region   `json:"default_region"`
	EnforceEncryption    bool     `json:"enforce_encryption"`
	AutoAudit            bool     `json:"auto_audit"`
	AuditIntervalHours   int      `json:"audit_interval_hours"`
	AlertOnViolation     bool     `json:"alert_on_violation"`
	RequireApproval      bool     `json:"require_approval"`
	AllowedFrameworks    []ComplianceFramework `json:"allowed_frameworks"`
}

// AuditEntry 审计日志
type AuditEntry struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	UserID    string    `json:"user_id"`
	AssetID   string    `json:"asset_id,omitempty"`
	Details   string    `json:"details"`
	Region    Region    `json:"region"`
	Timestamp time.Time `json:"timestamp"`
}

// NewDataSovereigntyManager 创建数据主权管理器
func NewDataSovereigntyManager(config *SovereigntyConfig) *DataSovereigntyManager {
	if config == nil {
		config = &SovereigntyConfig{
			DefaultRegion:     RegionLocal,
			EnforceEncryption: true,
			AutoAudit:         true,
			AuditIntervalHours: 24,
			AlertOnViolation:  true,
			RequireApproval:   true,
		}
	}
	return &DataSovereigntyManager{
		policies:   make(map[string]*DataPolicy),
		assets:     make(map[string]*DataAsset),
		transfers:  make(map[string]*TransferRequest),
		violations: make([]Violation, 0),
		auditLog:   make([]AuditEntry, 0),
		config:     config,
	}
}

// CreatePolicy 创建数据主权策略
func (dsm *DataSovereigntyManager) CreatePolicy(policy *DataPolicy) error {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	if _, exists := dsm.policies[policy.ID]; exists {
		return fmt.Errorf("policy %s already exists", policy.ID)
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	dsm.policies[policy.ID] = policy
	dsm.addAuditEntry("create_policy", "", policy.ID, fmt.Sprintf("Created policy: %s", policy.Name))
	return nil
}

// RegisterAsset 注册数据资产
func (dsm *DataSovereigntyManager) RegisterAsset(asset *DataAsset) error {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	if asset.ID == "" {
		return fmt.Errorf("asset ID is required")
	}

	if _, exists := dsm.assets[asset.ID]; exists {
		return fmt.Errorf("asset %s already exists", asset.ID)
	}

	now := time.Now()
	asset.CreatedAt = now
	asset.UpdatedAt = now

	// 自动应用策略
	if asset.PolicyID == "" {
		for _, policy := range dsm.policies {
			if policy.Enabled && dsm.matchesPolicy(asset, policy) {
				asset.PolicyID = policy.ID
				break
			}
		}
	}

	// 检查合规性
	dsm.checkCompliance(asset)

	dsm.assets[asset.ID] = asset
	dsm.addAuditEntry("register_asset", asset.OwnerID, asset.ID, fmt.Sprintf("Registered asset: %s", asset.Name))
	return nil
}

// RequestTransfer 请求数据传输
func (dsm *DataSovereigntyManager) RequestTransfer(req *TransferRequest) error {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	if req.AssetID == "" {
		return fmt.Errorf("asset ID is required")
	}

	asset, exists := dsm.assets[req.AssetID]
	if !exists {
		return fmt.Errorf("asset %s not found", req.AssetID)
	}

	// 检查传输合规性
	violations := dsm.checkTransferCompliance(asset, req.SourceRegion, req.TargetRegion)
	if len(violations) > 0 {
		req.Status = TransferViolated
		req.Violations = violations
	} else if dsm.config.RequireApproval {
		req.Status = TransferPending
	} else {
		req.Status = TransferApproved
	}

	now := time.Now()
	req.CreatedAt = now
	req.UpdatedAt = now

	dsm.transfers[req.ID] = req
	dsm.addAuditEntry("request_transfer", req.RequesterID, req.AssetID,
		fmt.Sprintf("Transfer from %s to %s: %s", req.SourceRegion, req.TargetRegion, req.Status))
	return nil
}

// ApproveTransfer 批准数据传输
func (dsm *DataSovereigntyManager) ApproveTransfer(transferID, approverID string) error {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	transfer, exists := dsm.transfers[transferID]
	if !exists {
		return fmt.Errorf("transfer %s not found", transferID)
	}

	if transfer.Status != TransferPending {
		return fmt.Errorf("transfer %s is not pending", transferID)
	}

	now := time.Now()
	transfer.Status = TransferApproved
	transfer.ApprovedBy = approverID
	transfer.ApprovedAt = &now
	transfer.UpdatedAt = now

	dsm.addAuditEntry("approve_transfer", approverID, transfer.AssetID,
		fmt.Sprintf("Approved transfer %s", transferID))
	return nil
}

// RunComplianceAudit 运行合规审计
func (dsm *DataSovereigntyManager) RunComplianceAudit(region Region, framework ComplianceFramework) (*ComplianceReport, error) {
	dsm.mu.RLock()
	defer dsm.mu.RUnlock()

	report := &ComplianceReport{
		ID:        fmt.Sprintf("audit_%s_%s_%d", region, framework, time.Now().Unix()),
		Region:    region,
		Framework: framework,
	}

	var violations []Violation
	for _, asset := range dsm.assets {
		if region != RegionGlobal && asset.CurrentRegion != region {
			continue
		}

		report.TotalAssets++

		policy, hasPolicy := dsm.policies[asset.PolicyID]
		if !hasPolicy {
			violations = append(violations, Violation{
				ID:          fmt.Sprintf("v_%s_%d", asset.ID, time.Now().UnixNano()),
				AssetID:     asset.ID,
				Description: fmt.Sprintf("Asset %s has no policy assigned", asset.Name),
				Severity:    "high",
				Region:      asset.CurrentRegion,
				DetectedAt:  time.Now(),
			})
			report.ViolatingAssets++
			continue
		}

		isCompliant := true
		for _, f := range policy.Frameworks {
			if f == framework {
				if !asset.Compliant {
					isCompliant = false
					break
				}
			}
		}

		if isCompliant {
			report.CompliantAssets++
		} else {
			report.ViolatingAssets++
			violations = append(violations, Violation{
				ID:          fmt.Sprintf("v_%s_%d", asset.ID, time.Now().UnixNano()),
				AssetID:     asset.ID,
				PolicyID:    asset.PolicyID,
				Framework:   framework,
				Description: fmt.Sprintf("Asset %s violates %s compliance", asset.Name, framework),
				Severity:    "medium",
				Region:      asset.CurrentRegion,
				DetectedAt:  time.Now(),
			})
		}
	}

	report.Violations = violations
	if report.TotalAssets > 0 {
		report.ComplianceRate = float64(report.CompliantAssets) / float64(report.TotalAssets) * 100
	}
	report.GeneratedAt = time.Now()
	report.ValidUntil = time.Now().Add(24 * time.Hour)

	report.Recommendations = dsm.generateRecommendations(report)
	return report, nil
}

// GetStats 获取统计信息
func (dsm *DataSovereigntyManager) GetStats() *SovereigntyStats {
	dsm.mu.RLock()
	defer dsm.mu.RUnlock()

	stats := &SovereigntyStats{
		RegionDistribution:   make(map[Region]int),
		ClassDistribution:    make(map[DataClassification]int),
	}

	for _, asset := range dsm.assets {
		stats.TotalAssets++
		if asset.Compliant {
			stats.CompliantAssets++
		} else {
			stats.ViolatingAssets++
		}
		stats.RegionDistribution[asset.CurrentRegion]++
		stats.ClassDistribution[asset.Classification]++
	}

	for _, policy := range dsm.policies {
		stats.TotalPolicies++
		if policy.Enabled {
			stats.ActivePolicies++
		}
	}

	for _, transfer := range dsm.transfers {
		if transfer.Status == TransferPending {
			stats.PendingTransfers++
		}
	}

	if stats.TotalAssets > 0 {
		stats.ComplianceRate = float64(stats.CompliantAssets) / float64(stats.TotalAssets) * 100
	}

	return stats
}

// ListViolations 获取违规列表
func (dsm *DataSovereigntyManager) ListViolations(resolved bool) []Violation {
	dsm.mu.RLock()
	defer dsm.mu.RUnlock()

	var result []Violation
	for _, v := range dsm.violations {
		if v.Resolved == resolved {
			result = append(result, v)
		}
	}
	return result
}

// GetDataAsset 获取数据资产
func (dsm *DataSovereigntyManager) GetDataAsset(assetID string) (*DataAsset, error) {
	dsm.mu.RLock()
	defer dsm.mu.RUnlock()

	asset, exists := dsm.assets[assetID]
	if !exists {
		return nil, fmt.Errorf("asset %s not found", assetID)
	}
	return asset, nil
}

// MarshalJSON 序列化
func (dsm *DataSovereigntyManager) MarshalJSON() ([]byte, error) {
	dsm.mu.RLock()
	defer dsm.mu.RUnlock()

	return json.Marshal(struct {
		Policies   map[string]*DataPolicy    `json:"policies"`
		Assets     map[string]*DataAsset     `json:"assets"`
		Transfers  map[string]*TransferRequest `json:"transfers"`
		Config     *SovereigntyConfig        `json:"config"`
	}{
		Policies:  dsm.policies,
		Assets:    dsm.assets,
		Transfers: dsm.transfers,
		Config:    dsm.config,
	})
}

// 内部辅助方法

func (dsm *DataSovereigntyManager) matchesPolicy(asset *DataAsset, policy *DataPolicy) bool {
	if len(policy.Classification) > 0 && asset.Classification != policy.Classification {
		return false
	}
	return true
}

func (dsm *DataSovereigntyManager) checkCompliance(asset *DataAsset) {
	asset.Compliant = true
	asset.Violations = nil

	policy, exists := dsm.policies[asset.PolicyID]
	if !exists {
		asset.Compliant = false
		asset.Violations = append(asset.Violations, "No policy assigned")
		return
	}

	// 检查区域限制
	for _, blocked := range policy.BlockedRegions {
		if asset.CurrentRegion == blocked {
			asset.Compliant = false
			asset.Violations = append(asset.Violations, fmt.Sprintf("Region %s is blocked by policy", asset.CurrentRegion))
		}
	}

	// 检查加密要求
	if policy.EncryptionRequired && !asset.Encrypted {
		asset.Compliant = false
		asset.Violations = append(asset.Violations, "Encryption required but not applied")
	}
}

func (dsm *DataSovereigntyManager) checkTransferCompliance(asset *DataAsset, source, target Region) []string {
	var violations []string

	policy, exists := dsm.policies[asset.PolicyID]
	if !exists {
		return violations
	}

	for _, blocked := range policy.BlockedRegions {
		if target == blocked {
			violations = append(violations, fmt.Sprintf("Transfer to region %s is blocked", target))
		}
	}

	allowed := false
	if len(policy.AllowedRegions) == 0 {
		allowed = true
	} else {
		for _, r := range policy.AllowedRegions {
			if r == target {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		violations = append(violations, fmt.Sprintf("Transfer to region %s is not allowed", target))
	}

	return violations
}

func (dsm *DataSovereigntyManager) generateRecommendations(report *ComplianceReport) []string {
	var recommendations []string

	if report.ComplianceRate < 80 {
		recommendations = append(recommendations, "Compliance rate is below 80%, immediate action required")
	}

	if report.ViolatingAssets > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Review %d violating assets", report.ViolatingAssets))
	}

	return recommendations
}

func (dsm *DataSovereigntyManager) addAuditEntry(action, userID, assetID, details string) {
	entry := AuditEntry{
		ID:        fmt.Sprintf("audit_%d", len(dsm.auditLog)+1),
		Action:    action,
		UserID:    userID,
		AssetID:   assetID,
		Details:   details,
		Region:    dsm.config.DefaultRegion,
		Timestamp: time.Now(),
	}
	dsm.auditLog = append(dsm.auditLog, entry)
}
