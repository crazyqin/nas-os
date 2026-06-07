// Package compliancereport CCPA 数据访问报告
package compliancereport

import (
	"fmt"
	"time"
)

// CCPADataCategory CCPA 数据分类.
type CCPADataCategory string

const (
	CCPACategoryIdentifiers      CCPADataCategory = "identifiers"       // 标识符
	CCPACategoryPersonalInfo     CCPADataCategory = "personal_info"     // 个人信息
	CCPACategoryCommercial       CCPADataCategory = "commercial"        // 商业信息
	CCPACategoryBiometric        CCPADataCategory = "biometric"         // 生物识别
	CCPACategoryInternetActivity CCPADataCategory = "internet_activity" // 网络活动
	CCPACategoryGeolocation      CCPADataCategory = "geolocation"       // 地理位置
	CCPACategoryProfessional     CCPADataCategory = "professional"      // 职业信息
	CCPACategoryEducation        CCPADataCategory = "education"         // 教育信息
	CCPACategoryInferences       CCPADataCategory = "inferences"        // 推断信息
)

// ThirdPartyType 第三方类型.
type ThirdPartyType string

const (
	ThirdPartyService   ThirdPartyType = "service_provider" // 服务提供商
	ThirdPartyBusiness  ThirdPartyType = "business_partner" // 商业伙伴
	ThirdPartyAd        ThirdPartyType = "advertising"      // 广告商
	ThirdPartyAnalytics ThirdPartyType = "analytics"        // 分析服务
)

// UserDataRecord 用户数据记录.
type UserDataRecord struct {
	Category    CCPADataCategory `json:"category"`
	FieldName   string           `json:"field_name"`
	Description string           `json:"description"`
	CollectedAt time.Time        `json:"collected_at"`
	RetainDays  int              `json:"retain_days"`
	IsSold      bool             `json:"is_sold"`   // 是否出售
	IsShared    bool             `json:"is_shared"` // 是否共享
}

// ThirdPartySharingRecord 第三方共享记录.
type ThirdPartySharingRecord struct {
	PartyID     string             `json:"party_id"`
	PartyName   string             `json:"party_name"`
	PartyType   ThirdPartyType     `json:"party_type"`
	DataTypes   []CCPADataCategory `json:"data_types"`
	Purpose     string             `json:"purpose"`
	StartDate   time.Time          `json:"start_date"`
	IsActive    bool               `json:"is_active"`
	HasContract bool               `json:"has_contract"` // 是否有合同约束
}

// CCPADataAccessRequest CCPA 数据访问请求.
type CCPADataAccessRequest struct {
	RequestID   string     `json:"request_id"`
	UserID      string     `json:"user_id"`
	UserName    string     `json:"user_name"`
	Email       string     `json:"email"`
	RequestType string     `json:"request_type"` // "know", "delete", "opt_out"
	Status      string     `json:"status"`       // "pending", "processing", "completed", "denied"
	SubmittedAt time.Time  `json:"submitted_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

// CCPAReport CCPA 数据访问报告.
type CCPAReport struct {
	ReportID       string                    `json:"report_id"`
	GeneratedAt    time.Time                 `json:"generated_at"`
	ReportPeriod   ReportPeriod              `json:"report_period"`
	BusinessInfo   CCPABusinessInfo          `json:"business_info"`
	UserData       []UserDataRecord          `json:"user_data"`
	ThirdParty     []ThirdPartySharingRecord `json:"third_party_sharing"`
	AccessRequests []CCPADataAccessRequest   `json:"access_requests"`
	Summary        CCPASummary               `json:"summary"`
}

// CCPABusinessInfo 企业信息.
type CCPABusinessInfo struct {
	Name             string `json:"name"`
	Address          string `json:"address"`
	Website          string `json:"website"`
	PrivacyPolicyURL string `json:"privacy_policy_url"`
	ContactEmail     string `json:"contact_email"`
	HasPrivacyPolicy bool   `json:"has_privacy_policy"`
	DoNotSellLink    bool   `json:"do_not_sell_link"` // 是否有"不要出售"链接
}

// CCPASummary CCPA 报告摘要.
type CCPASummary struct {
	TotalDataCategories int      `json:"total_data_categories"`
	TotalDataFields     int      `json:"total_data_fields"`
	SharedDataFields    int      `json:"shared_data_fields"`
	SoldDataFields      int      `json:"sold_data_fields"`
	TotalThirdParties   int      `json:"total_third_parties"`
	ActiveThirdParties  int      `json:"active_third_parties"`
	TotalAccessRequests int      `json:"total_access_requests"`
	CompletedRequests   int      `json:"completed_requests"`
	PendingRequests     int      `json:"pending_requests"`
	AvgCompletionDays   int      `json:"avg_completion_days"`
	ComplianceStatus    string   `json:"compliance_status"`
	Recommendations     []string `json:"recommendations"`
}

// CCPAReportGenerator CCPA 报告生成器.
type CCPAReportGenerator struct{}

// NewCCPAReportGenerator 创建 CCPA 报告生成器.
func NewCCPAReportGenerator() *CCPAReportGenerator {
	return &CCPAReportGenerator{}
}

// CCPAReportConfig CCPA 报告配置.
type CCPAReportConfig struct {
	StartDate      time.Time
	EndDate        time.Time
	BusinessInfo   CCPABusinessInfo
	UserData       []UserDataRecord
	ThirdParty     []ThirdPartySharingRecord
	AccessRequests []CCPADataAccessRequest
}

// GenerateCCPAReport 生成 CCPA 数据访问报告.
func (g *CCPAReportGenerator) GenerateCCPAReport(config CCPAReportConfig) *CCPAReport {
	report := &CCPAReport{
		ReportID:    GenerateID("ccpa"),
		GeneratedAt: time.Now(),
		ReportPeriod: ReportPeriod{
			StartDate: config.StartDate,
			EndDate:   config.EndDate,
		},
		BusinessInfo:   config.BusinessInfo,
		UserData:       config.UserData,
		ThirdParty:     config.ThirdParty,
		AccessRequests: config.AccessRequests,
	}

	report.Summary = g.generateCCPASummary(report)

	return report
}

// generateCCPASummary 生成 CCPA 摘要.
func (g *CCPAReportGenerator) generateCCPASummary(report *CCPAReport) CCPASummary {
	summary := CCPASummary{
		TotalDataFields:     len(report.UserData),
		TotalThirdParties:   len(report.ThirdParty),
		TotalAccessRequests: len(report.AccessRequests),
	}

	// 统计数据分类
	categories := make(map[CCPADataCategory]bool)
	for _, d := range report.UserData {
		categories[d.Category] = true
		if d.IsShared {
			summary.SharedDataFields++
		}
		if d.IsSold {
			summary.SoldDataFields++
		}
	}
	summary.TotalDataCategories = len(categories)

	// 统计第三方
	for _, tp := range report.ThirdParty {
		if tp.IsActive {
			summary.ActiveThirdParties++
		}
	}

	// 统计请求
	totalCompletionDays := 0
	for _, req := range report.AccessRequests {
		switch req.Status {
		case "completed":
			summary.CompletedRequests++
			if req.CompletedAt != nil {
				days := int(req.CompletedAt.Sub(req.SubmittedAt).Hours() / 24)
				totalCompletionDays += days
			}
		case "pending", "processing":
			summary.PendingRequests++
		}
	}

	if summary.CompletedRequests > 0 {
		summary.AvgCompletionDays = totalCompletionDays / summary.CompletedRequests
	}

	// 确定合规状态
	summary.ComplianceStatus = g.determineCCPACompliance(report, summary)

	// 生成建议
	summary.Recommendations = g.generateCCPARecommendations(report, summary)

	return summary
}

// determineCCPACompliance 确定 CCPA 合规状态.
func (g *CCPAReportGenerator) determineCCPACompliance(report *CCPAReport, summary CCPASummary) string {
	issues := 0

	// 检查隐私政策
	if !report.BusinessInfo.HasPrivacyPolicy {
		issues += 2
	}

	// 检查"不要出售"链接
	if !report.BusinessInfo.DoNotSellLink {
		issues++
	}

	// 检查第三方是否有合同
	for _, tp := range report.ThirdParty {
		if tp.IsActive && !tp.HasContract {
			issues++
		}
	}

	// 检查请求响应时间（CCPA 要求 45 天内响应）
	if summary.AvgCompletionDays > 45 {
		issues++
	}

	if issues == 0 {
		return "compliant"
	} else if issues <= 2 {
		return "needs_review"
	}
	return "non_compliant"
}

// generateCCPARecommendations 生成 CCPA 合规建议.
func (g *CCPAReportGenerator) generateCCPARecommendations(report *CCPAReport, summary CCPASummary) []string {
	var recs []string

	if !report.BusinessInfo.HasPrivacyPolicy {
		recs = append(recs, "必须发布符合 CCPA 要求的隐私政策")
	}

	if !report.BusinessInfo.DoNotSellLink {
		recs = append(recs, "网站必须提供\"不要出售我的个人信息\"链接")
	}

	if summary.SoldDataFields > 0 {
		recs = append(recs, fmt.Sprintf("当前有 %d 个数据字段涉及出售，需确保用户有权选择退出", summary.SoldDataFields))
	}

	if summary.PendingRequests > 0 {
		recs = append(recs, fmt.Sprintf("有 %d 个待处理的用户请求，CCPA 要求 45 天内完成响应", summary.PendingRequests))
	}

	if summary.AvgCompletionDays > 30 {
		recs = append(recs, "平均请求完成时间较长，建议优化请求处理流程")
	}

	for _, tp := range report.ThirdParty {
		if tp.IsActive && !tp.HasContract {
			recs = append(recs, fmt.Sprintf("与第三方 %s 的数据共享缺少合同约束", tp.PartyName))
			break
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "当前 CCPA 合规状态良好，建议持续监控")
	}

	return recs
}
