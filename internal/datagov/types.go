// Package datagov 数据治理模块 - 数据合规、分类、保留策略、审计
// 支持 GDPR/CCPA/等保2.0 等合规框架
package datagov

import (
	"time"
)

// ============================================================
// 数据分类类型
// ============================================================

// DataClassification 数据分类等级
type DataClassification string

const (
	ClassificationPublic       DataClassification = "public"       // 公开数据
	ClassificationInternal     DataClassification = "internal"     // 内部数据
	ClassificationConfidential DataClassification = "confidential" // 机密数据
	ClassificationRestricted   DataClassification = "restricted"   // 受限数据（高度敏感）
)

// SensitiveDataType 敏感数据类型
type SensitiveDataType string

const (
	SensitivePII        SensitiveDataType = "pii"        // 个人身份信息
	SensitivePHI        SensitiveDataType = "phi"        // 健康信息
	SensitiveFinancial  SensitiveDataType = "financial"  // 财务数据
	SensitiveCredential SensitiveDataType = "credential" // 凭证/密码
	SensitiveBiometric  SensitiveDataType = "biometric"  // 生物识别
	SensitiveNone       SensitiveDataType = "none"       // 非敏感
)

// DataAsset 数据资产定义
type DataAsset struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Path            string              `json:"path"`    // 数据存储路径
	Type            string              `json:"type"`    // 文件/数据库/API等
	Owner           string              `json:"owner"`   // 数据所有者
	Steward         string              `json:"steward"` // 数据管理员
	Classification  DataClassification  `json:"classification"`
	SensitiveTypes  []SensitiveDataType `json:"sensitive_types"` // 包含的敏感数据类型
	Tags            []string            `json:"tags"`
	Size            int64               `json:"size"`             // 数据大小(bytes)
	RecordCount     int64               `json:"record_count"`     // 记录数
	RetentionPolicy string              `json:"retention_policy"` // 关联的保留策略ID
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	LastAccessedAt  *time.Time          `json:"last_accessed_at,omitempty"`
	Metadata        map[string]string   `json:"metadata,omitempty"`
}

// ScanRule 数据扫描规则
type ScanRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	Pattern     string            `json:"pattern"`    // 正则表达式模式
	DataType    SensitiveDataType `json:"data_type"`  // 识别的敏感数据类型
	Confidence  float64           `json:"confidence"` // 置信度阈值 (0-1)
	Action      ScanAction        `json:"action"`     // 发现后的动作
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ScanAction 扫描动作
type ScanAction string

const (
	ActionTag        ScanAction = "tag"        // 仅标记
	ActionAlert      ScanAction = "alert"      // 发送告警
	ActionQuarantine ScanAction = "quarantine" // 隔离
	ActionEncrypt    ScanAction = "encrypt"    // 自动加密
	ActionMask       ScanAction = "mask"       // 自动脱敏
)

// ScanResult 扫描结果
type ScanResult struct {
	ID            string            `json:"id"`
	AssetID       string            `json:"asset_id"`
	AssetPath     string            `json:"asset_path"`
	RuleID        string            `json:"rule_id"`
	DataType      SensitiveDataType `json:"data_type"`
	MatchCount    int               `json:"match_count"`    // 匹配数量
	SampleMatches []string          `json:"sample_matches"` // 示例匹配（已脱敏）
	Confidence    float64           `json:"confidence"`
	Action        ScanAction        `json:"action"`
	ActionStatus  string            `json:"action_status"` // pending/completed/failed
	ScannedAt     time.Time         `json:"scanned_at"`
}

// ============================================================
// 保留策略类型
// ============================================================

// RetentionPolicy 数据保留策略
type RetentionPolicy struct {
	ID                       string               `json:"id"`
	Name                     string               `json:"name"`
	Description              string               `json:"description"`
	RetentionDays            int                  `json:"retention_days"`            // 保留天数
	ArchiveDays              int                  `json:"archive_days"`              // 归档天数（保留期前归档）
	AutoDelete               bool                 `json:"auto_delete"`               // 过期自动删除
	CompressArchive          bool                 `json:"compress_archive"`          // 归档时压缩
	ApplicableTags           []string             `json:"applicable_tags"`           // 适用的数据标签
	ApplicableClassification []DataClassification `json:"applicable_classification"` // 适用的数据分类
	RegulationRefs           []string             `json:"regulation_refs"`           // 关联的法规条款
	CreatedAt                time.Time            `json:"created_at"`
	UpdatedAt                time.Time            `json:"updated_at"`
	Enabled                  bool                 `json:"enabled"`
}

// RetentionTask 保留策略执行任务
type RetentionTask struct {
	ID          string     `json:"id"`
	PolicyID    string     `json:"policy_id"`
	AssetID     string     `json:"asset_id"`
	Action      string     `json:"action"` // archive/delete
	Status      string     `json:"status"` // pending/running/completed/failed
	ScheduledAt time.Time  `json:"scheduled_at"`
	ExecutedAt  *time.Time `json:"executed_at,omitempty"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
}

// ============================================================
// 访问审计类型
// ============================================================

// AccessEvent 数据访问事件
type AccessEvent struct {
	ID           string       `json:"id"`
	AssetID      string       `json:"asset_id"`
	AssetPath    string       `json:"asset_path"`
	UserID       string       `json:"user_id"`
	UserName     string       `json:"user_name"`
	Action       AccessAction `json:"action"`
	Source       string       `json:"source"` // 来源IP/系统
	UserAgent    string       `json:"user_agent"`
	Success      bool         `json:"success"`
	ErrorCode    string       `json:"error_code,omitempty"`
	DataSize     int64        `json:"data_size"`   // 访问的数据量(bytes)
	Duration     int64        `json:"duration_ms"` // 耗时(毫秒)
	Timestamp    time.Time    `json:"timestamp"`
	RiskScore    float64      `json:"risk_score"` // 风险评分 (0-100)
	AnomalyFlags []string     `json:"anomaly_flags,omitempty"`
}

// AccessAction 访问动作类型
type AccessAction string

const (
	AccessRead     AccessAction = "read"
	AccessWrite    AccessAction = "write"
	AccessDelete   AccessAction = "delete"
	AccessExport   AccessAction = "export"
	AccessShare    AccessAction = "share"
	AccessModify   AccessAction = "modify"
	AccessBulkRead AccessAction = "bulk_read" // 批量读取
)

// AccessPattern 用户访问模式
type AccessPattern struct {
	UserID          string        `json:"user_id"`
	UserName        string        `json:"user_name"`
	TotalAccess     int           `json:"total_access"`
	UniqueAssets    int           `json:"unique_assets"`
	AvgAccessPerDay float64       `json:"avg_access_per_day"`
	TopActions      []ActionCount `json:"top_actions"`
	AccessHours     map[int]int   `json:"access_hours"` // 小时 -> 访问次数
	RiskLevel       string        `json:"risk_level"`   // low/medium/high/critical
	LastAccess      time.Time     `json:"last_access"`
}

// ActionCount 动作计数
type ActionCount struct {
	Action AccessAction `json:"action"`
	Count  int          `json:"count"`
}

// AnomalyRule 异常访问检测规则
type AnomalyRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Type        string    `json:"type"` // volume_threshold/time_pattern/bulk_export/unusual_access
	Threshold   float64   `json:"threshold"`
	TimeWindow  int       `json:"time_window"` // 检测时间窗口(分钟)
	Severity    string    `json:"severity"`    // low/medium/high/critical
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// AnomalyAlert 异常访问告警
type AnomalyAlert struct {
	ID          string     `json:"id"`
	RuleID      string     `json:"rule_id"`
	RuleName    string     `json:"rule_name"`
	UserID      string     `json:"user_id"`
	UserName    string     `json:"user_name"`
	AssetID     string     `json:"asset_id"`
	Severity    string     `json:"severity"`
	Description string     `json:"description"`
	Evidence    []string   `json:"evidence"` // 异常证据
	RiskScore   float64    `json:"risk_score"`
	Status      string     `json:"status"` // open/investigating/resolved/false_positive
	DetectedAt  time.Time  `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// ============================================================
// 合规报告类型
// ============================================================

// ComplianceFramework 合规框架
type ComplianceFramework string

const (
	FrameworkGDPR     ComplianceFramework = "gdpr"     // 欧盟通用数据保护条例
	FrameworkCCPA     ComplianceFramework = "ccpa"     // 加州消费者隐私法案
	FrameworkPIPL     ComplianceFramework = "pipl"     // 中国个人信息保护法
	FrameworkMLPS2    ComplianceFramework = "mlps2"    // 等保2.0
	FrameworkISO27001 ComplianceFramework = "iso27001" // ISO 27001
	FrameworkHIPAA    ComplianceFramework = "hipaa"    // 健康保险可携性和责任法案
)

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID              string                  `json:"id"`
	Framework       ComplianceFramework     `json:"framework"`
	Title           string                  `json:"title"`
	GeneratedAt     time.Time               `json:"generated_at"`
	PeriodStart     time.Time               `json:"period_start"`
	PeriodEnd       time.Time               `json:"period_end"`
	Summary         ComplianceSummary       `json:"summary"`
	Requirements    []ComplianceRequirement `json:"requirements"`
	Findings        []ComplianceFinding     `json:"findings"`
	Recommendations []string                `json:"recommendations"`
	Status          string                  `json:"status"` // draft/final
}

// ComplianceSummary 合规摘要
type ComplianceSummary struct {
	TotalRequirements int     `json:"total_requirements"`
	Compliant         int     `json:"compliant"`
	PartialCompliant  int     `json:"partial_compliant"`
	NonCompliant      int     `json:"non_compliant"`
	ComplianceScore   float64 `json:"compliance_score"` // 0-100
	RiskLevel         string  `json:"risk_level"`
}

// ComplianceRequirement 合规要求
type ComplianceRequirement struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"` // 条款编号
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Status      string   `json:"status"`      // compliant/partial/non_compliant/not_applicable
	Evidence    string   `json:"evidence"`    // 合规证据
	Gaps        []string `json:"gaps"`        // 差距
	Remediation string   `json:"remediation"` // 整改建议
}

// ComplianceFinding 合规发现
type ComplianceFinding struct {
	ID          string     `json:"id"`
	Requirement string     `json:"requirement"` // 关联要求ID
	Severity    string     `json:"severity"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Evidence    string     `json:"evidence"`
	Remediation string     `json:"remediation"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Status      string     `json:"status"` // open/in_progress/resolved
}

// ============================================================
// 数据地图类型
// ============================================================

// DataFlow 数据流向
type DataFlow struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"` // 源数据资产ID
	TargetID    string     `json:"target_id"` // 目标数据资产ID
	FlowType    string     `json:"flow_type"` // sync/async/etl/backup/share
	Description string     `json:"description"`
	Frequency   string     `json:"frequency"` // real-time/hourly/daily/manual
	Protocol    string     `json:"protocol"`  // smb/nfs/api/etl
	Encrypted   bool       `json:"encrypted"`
	LastSync    *time.Time `json:"last_sync,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// DataMapNode 数据地图节点
type DataMapNode struct {
	Asset      DataAsset  `json:"asset"`
	InFlows    []DataFlow `json:"in_flows"`   // 流入
	OutFlows   []DataFlow `json:"out_flows"`  // 流出
	Downstream []string   `json:"downstream"` // 下游资产ID
	Upstream   []string   `json:"upstream"`   // 上游资产ID
}

// DataLineage 数据血缘
type DataLineage struct {
	AssetID     string        `json:"asset_id"`
	AssetName   string        `json:"asset_name"`
	Ancestors   []LineageNode `json:"ancestors"`   // 祖先节点
	Descendants []LineageNode `json:"descendants"` // 后代节点
	Depth       int           `json:"depth"`
}

// LineageNode 血缘节点
type LineageNode struct {
	AssetID   string `json:"asset_id"`
	AssetName string `json:"asset_name"`
	Level     int    `json:"level"` // 距离
	FlowType  string `json:"flow_type"`
}

// ============================================================
// 隐私计算类型
// ============================================================

// MaskingRule 数据脱敏规则
type MaskingRule struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	DataType       SensitiveDataType `json:"data_type"`
	Method         MaskingMethod     `json:"method"`
	Pattern        string            `json:"pattern"`         // 匹配模式
	Replacement    string            `json:"replacement"`     // 替换内容/格式
	PreserveFormat bool              `json:"preserve_format"` // 保持格式
	Enabled        bool              `json:"enabled"`
	CreatedAt      time.Time         `json:"created_at"`
}

// MaskingMethod 脱敏方法
type MaskingMethod string

const (
	MaskingMask       MaskingMethod = "mask"       // 掩码替换 (e.g. 138****5678)
	MaskingHash       MaskingMethod = "hash"       // 哈希替换
	MaskingRedact     MaskingMethod = "redact"     // 完全遮蔽 (e.g. ****)
	MaskingPseudonym  MaskingMethod = "pseudonym"  // 假名化
	MaskingGeneralize MaskingMethod = "generalize" // 泛化 (e.g. 年龄->年龄段)
	MaskingShuffle    MaskingMethod = "shuffle"    // 乱序
	MaskingNull       MaskingMethod = "null"       // 置空
)

// AnonymizationTask 匿名化任务
type AnonymizationTask struct {
	ID               string     `json:"id"`
	AssetID          string     `json:"asset_id"`
	AssetPath        string     `json:"asset_path"`
	RuleIDs          []string   `json:"rule_ids"`    // 应用的脱敏规则
	OutputPath       string     `json:"output_path"` // 输出路径
	Status           string     `json:"status"`      // pending/running/completed/failed
	Progress         float64    `json:"progress"`    // 进度 0-100
	RecordsProcessed int64      `json:"records_processed"`
	RecordsTotal     int64      `json:"records_total"`
	ErrorMsg         string     `json:"error_msg,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// PrivacyConfig 隐私计算配置
type PrivacyConfig struct {
	EnableAutoMasking    bool     `json:"enable_auto_masking"`    // 自动脱敏
	EnableEncryption     bool     `json:"enable_encryption"`      // 传输加密
	DefaultMaskingChar   string   `json:"default_masking_char"`   // 默认掩码字符
	RetainOriginalDays   int      `json:"retain_original_days"`   // 原始数据保留天数
	AllowedExportFormats []string `json:"allowed_export_formats"` // 允许导出格式
}

// DefaultPrivacyConfig 默认隐私配置
func DefaultPrivacyConfig() PrivacyConfig {
	return PrivacyConfig{
		EnableAutoMasking:    true,
		EnableEncryption:     true,
		DefaultMaskingChar:   "*",
		RetainOriginalDays:   30,
		AllowedExportFormats: []string{"csv", "json", "parquet"},
	}
}

// ============================================================
// 配置类型
// ============================================================

// Config 数据治理模块配置
type Config struct {
	// 扫描配置
	ScanEnabled     bool  `json:"scan_enabled"`
	ScanInterval    int   `json:"scan_interval"`      // 扫描间隔(小时)
	MaxScanFileSize int64 `json:"max_scan_file_size"` // 最大扫描文件大小(bytes)

	// 审计配置
	AuditRetentionDays  int  `json:"audit_retention_days"`  // 审计日志保留天数
	EnableAnomalyDetect bool `json:"enable_anomaly_detect"` // 启用异常检测

	// 合规配置
	DefaultFramework ComplianceFramework `json:"default_framework"`

	// 隐私配置
	Privacy PrivacyConfig `json:"privacy"`

	// 存储配置
	DataDir string `json:"data_dir"` // 数据存储目录
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		ScanEnabled:         true,
		ScanInterval:        24,
		MaxScanFileSize:     1024 * 1024 * 100, // 100MB
		AuditRetentionDays:  365,
		EnableAnomalyDetect: true,
		DefaultFramework:    FrameworkMLPS2,
		Privacy:             DefaultPrivacyConfig(),
		DataDir:             "/var/lib/nas-os/datagov",
	}
}
