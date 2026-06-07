// Package compliancereport GDPR 数据处理报告
package compliancereport

import (
	"fmt"
	"time"
)

// DataCategory 数据分类.
type DataCategory string

const (
	DataCategoryPersonal   DataCategory = "personal"   // 个人身份信息
	DataCategorySensitive  DataCategory = "sensitive"  // 敏感个人信息
	DataCategoryFinancial  DataCategory = "financial"  // 财务数据
	DataCategoryHealth     DataCategory = "health"     // 健康数据
	DataCategoryBehavioral DataCategory = "behavioral" // 行为数据
	DataCategoryTechnical  DataCategory = "technical"  // 技术数据
)

// ProcessingPurpose 处理目的.
type ProcessingPurpose string

const (
	PurposeServiceProvision ProcessingPurpose = "service_provision" // 服务提供
	PurposeAnalytics        ProcessingPurpose = "analytics"         // 数据分析
	PurposeMarketing        ProcessingPurpose = "marketing"         // 营销推广
	PurposeLegalCompliance  ProcessingPurpose = "legal_compliance"  // 法律合规
	PurposeSecurity         ProcessingPurpose = "security"          // 安全防护
	PurposeBackup           ProcessingPurpose = "backup"            // 数据备份
)

// LegalBasis 合法处理基础.
type LegalBasis string

const (
	LegalBasisConsent       LegalBasis = "consent"             // 同意
	LegalBasisContract      LegalBasis = "contract"            // 合同履行
	LegalBasisLegalOblig    LegalBasis = "legal_obligation"    // 法律义务
	LegalBasisVitalInterest LegalBasis = "vital_interest"      // 重大利益
	LegalBasisPublicTask    LegalBasis = "public_task"         // 公共任务
	LegalBasisLegitInterest LegalBasis = "legitimate_interest" // 合法利益
)

// DataStorageLocation 数据存储位置.
type DataStorageLocation struct {
	LocationID  string `json:"location_id"`
	Name        string `json:"name"`
	Region      string `json:"region"`       // 存储区域（如 EU, CN, US）
	Country     string `json:"country"`      // 国家代码
	IsEncrypted bool   `json:"is_encrypted"` // 是否加密存储
	Provider    string `json:"provider"`     // 存储提供商
	Description string `json:"description"`
}

// DataProcessingActivity 数据处理活动.
type DataProcessingActivity struct {
	ActivityID    string              `json:"activity_id"`
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	DataTypes     []DataCategory      `json:"data_types"`        // 处理的数据类型
	Purposes      []ProcessingPurpose `json:"purposes"`          // 处理目的
	LegalBasis    LegalBasis          `json:"legal_basis"`       // 合法基础
	StorageLocs   []string            `json:"storage_locations"` // 存储位置 ID
	RetentionDays int                 `json:"retention_days"`    // 保留天数
	RetentionDesc string              `json:"retention_desc"`    // 保留策略描述
	IsActive      bool                `json:"is_active"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// GDPRReport GDPR 数据处理报告.
type GDPRReport struct {
	ReportID          string                   `json:"report_id"`
	GeneratedAt       time.Time                `json:"generated_at"`
	ReportPeriod      ReportPeriod             `json:"report_period"`
	DataController    DataController           `json:"data_controller"`
	DataProcessor     *DataProcessor           `json:"data_processor,omitempty"`
	StorageLocations  []DataStorageLocation    `json:"storage_locations"`
	Activities        []DataProcessingActivity `json:"processing_activities"`
	DataSubjectRights DataSubjectRightsSummary `json:"data_subject_rights"`
	Summary           GDPRSummary              `json:"summary"`
}

// ReportPeriod 报告周期.
type ReportPeriod struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// DataController 数据控制者.
type DataController struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	ContactEmail string `json:"contact_email"`
	DPOName      string `json:"dpo_name,omitempty"` // 数据保护官
	DPOContact   string `json:"dpo_contact,omitempty"`
}

// DataProcessor 数据处理者.
type DataProcessor struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	ContactEmail string `json:"contact_email"`
	Activities   string `json:"activities"` // 处理活动描述
}

// DataSubjectRightsSummary 数据主体权利摘要.
type DataSubjectRightsSummary struct {
	AccessRequests      int `json:"access_requests"`      // 访问请求数
	ErasureRequests     int `json:"erasure_requests"`     // 删除请求数
	PortabilityRequests int `json:"portability_requests"` // 可携性请求数
	ObjectionRequests   int `json:"objection_requests"`   // 异议请求数
	TotalRequests       int `json:"total_requests"`
	AvgResponseDays     int `json:"avg_response_days"` // 平均响应天数
}

// GDPRSummary GDPR 报告摘要.
type GDPRSummary struct {
	TotalActivities     int      `json:"total_activities"`
	ActiveActivities    int      `json:"active_activities"`
	TotalStorageLocs    int      `json:"total_storage_locations"`
	EUStorageLocs       int      `json:"eu_storage_locations"`
	NonEUStorageLocs    int      `json:"non_eu_storage_locations"`
	EncryptedStoragePct int      `json:"encrypted_storage_pct"` // 加密存储百分比
	ComplianceStatus    string   `json:"compliance_status"`
	Recommendations     []string `json:"recommendations"`
}

// GDPRReportGenerator GDPR 报告生成器.
type GDPRReportGenerator struct{}

// NewGDPRReportGenerator 创建 GDPR 报告生成器.
func NewGDPRReportGenerator() *GDPRReportGenerator {
	return &GDPRReportGenerator{}
}

// GenerateGDPRReport 生成 GDPR 数据处理报告.
func (g *GDPRReportGenerator) GenerateGDPRReport(config GDPRReportConfig) *GDPRReport {
	report := &GDPRReport{
		ReportID:    GenerateID("gdpr"),
		GeneratedAt: time.Now(),
		ReportPeriod: ReportPeriod{
			StartDate: config.StartDate,
			EndDate:   config.EndDate,
		},
		DataController: config.Controller,
		DataProcessor:  config.Processor,
	}

	// 填充存储位置
	report.StorageLocations = config.StorageLocations

	// 填充处理活动
	report.Activities = config.Activities

	// 填充数据主体权利摘要
	report.DataSubjectRights = config.SubjectRights

	// 生成摘要
	report.Summary = g.generateGDPRSummary(report)

	return report
}

// GDPRReportConfig GDPR 报告配置.
type GDPRReportConfig struct {
	StartDate        time.Time
	EndDate          time.Time
	Controller       DataController
	Processor        *DataProcessor
	StorageLocations []DataStorageLocation
	Activities       []DataProcessingActivity
	SubjectRights    DataSubjectRightsSummary
}

// generateGDPRSummary 生成 GDPR 报告摘要.
func (g *GDPRReportGenerator) generateGDPRSummary(report *GDPRReport) GDPRSummary {
	summary := GDPRSummary{
		TotalActivities:  len(report.Activities),
		TotalStorageLocs: len(report.StorageLocations),
	}

	for _, act := range report.Activities {
		if act.IsActive {
			summary.ActiveActivities++
		}
	}

	encryptedCount := 0
	for _, loc := range report.StorageLocations {
		if loc.Region == "EU" || loc.Region == "EEA" {
			summary.EUStorageLocs++
		} else {
			summary.NonEUStorageLocs++
		}
		if loc.IsEncrypted {
			encryptedCount++
		}
	}

	if summary.TotalStorageLocs > 0 {
		summary.EncryptedStoragePct = (encryptedCount * 100) / summary.TotalStorageLocs
	}

	// 确定合规状态
	summary.ComplianceStatus = g.determineGDPRCompliance(report, summary)

	// 生成建议
	summary.Recommendations = g.generateRecommendations(summary)

	return summary
}

// determineGDPRCompliance 确定 GDPR 合规状态.
func (g *GDPRReportGenerator) determineGDPRCompliance(report *GDPRReport, summary GDPRSummary) string {
	issues := 0

	// 检查是否有非 EU 存储但无适当保障
	if summary.NonEUStorageLocs > 0 {
		issues++
	}

	// 检查加密率
	if summary.EncryptedStoragePct < 100 {
		issues++
	}

	// 检查处理活动是否有合法基础
	for _, act := range report.Activities {
		if act.LegalBasis == "" {
			issues++
		}
	}

	if issues == 0 {
		return "compliant"
	} else if issues <= 2 {
		return "needs_review"
	}
	return "non_compliant"
}

// generateRecommendations 生成合规建议.
func (g *GDPRReportGenerator) generateRecommendations(summary GDPRSummary) []string {
	var recs []string

	if summary.NonEUStorageLocs > 0 {
		recs = append(recs, "非欧盟存储位置需确保有适当的数据传输保障措施（如标准合同条款）")
	}

	if summary.EncryptedStoragePct < 100 {
		recs = append(recs, fmt.Sprintf("当前加密存储覆盖率 %d%%，建议对所有存储位置启用加密", summary.EncryptedStoragePct))
	}

	if summary.TotalActivities > 0 && summary.ActiveActivities < summary.TotalActivities {
		recs = append(recs, "存在非活跃的处理活动，建议清理或归档")
	}

	if len(recs) == 0 {
		recs = append(recs, "当前 GDPR 合规状态良好，建议定期审查")
	}

	return recs
}
