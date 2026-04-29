// Package compliancereport 合规状态仪表盘 API
package compliancereport

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// DashboardMetrics 仪表盘指标.
type DashboardMetrics struct {
	OverallComplianceScore  int                      `json:"overall_compliance_score"`
	OverallStatus           string                   `json:"overall_status"`
	StandardsCompliance     []StandardCompliance     `json:"standards_compliance"`
	GDPRMetrics             *GDPRDashboardMetrics    `json:"gdpr,omitempty"`
	CCPAMetrics             *CCPADashboardMetrics    `json:"ccpa,omitempty"`
	BreachMetrics           *BreachDashboardMetrics  `json:"breach,omitempty"`
	RecentReports           []ReportSummary          `json:"recent_reports"`
	PendingRemediations     int                      `json:"pending_remediations"`
	ActiveBreachCount       int                      `json:"active_breach_count"`
	OpenAccessRequests      int                      `json:"open_access_requests"`
	ComplianceTrend         []TrendDataPoint         `json:"compliance_trend"`
	LastUpdated             time.Time                `json:"last_updated"`
}

// StandardCompliance 标准合规状态.
type StandardCompliance struct {
	Standard    string    `json:"standard"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Score       int       `json:"score"`
	LastScanAt  *time.Time `json:"last_scan_at,omitempty"`
	CheckTotal  int       `json:"check_total"`
	CheckPassed int       `json:"check_passed"`
	CheckFailed int       `json:"check_failed"`
}

// GDPRDashboardMetrics GDPR 仪表盘指标.
type GDPRDashboardMetrics struct {
	DataProcessingActivities int     `json:"data_processing_activities"`
	ActiveActivities         int     `json:"active_activities"`
	StorageLocations         int     `json:"storage_locations"`
	EncryptedStoragePct      int     `json:"encrypted_storage_pct"`
	EUStorageLocations       int     `json:"eu_storage_locations"`
	DataSubjectRequests      int     `json:"data_subject_requests"`
	AvgResponseDays          int     `json:"avg_response_days"`
	ComplianceStatus         string  `json:"compliance_status"`
}

// CCPADashboardMetrics CCPA 仪表盘指标.
type CCPADashboardMetrics struct {
	DataCategories       int    `json:"data_categories"`
	ThirdPartyPartners   int    `json:"third_party_partners"`
	SharedDataFields     int    `json:"shared_data_fields"`
	SoldDataFields       int    `json:"sold_data_fields"`
	AccessRequests       int    `json:"access_requests"`
	CompletedRequests    int    `json:"completed_requests"`
	PendingRequests      int    `json:"pending_requests"`
	ComplianceStatus     string `json:"compliance_status"`
}

// BreachDashboardMetrics 泄露仪表盘指标.
type BreachDashboardMetrics struct {
	TotalBreaches       int     `json:"total_breaches"`
	ActiveBreaches      int     `json:"active_breaches"`
	ClosedBreaches      int     `json:"closed_breaches"`
	AvgResponseHours    float64 `json:"avg_response_hours"`
	AvgNotificationHours float64 `json:"avg_notification_hours"`
	TotalRecordsAffected int    `json:"total_records_affected"`
	NotificationCompliant int   `json:"notification_compliant"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	ReportID    string    `json:"report_id"`
	Type        string    `json:"type"` // "compliance", "gdpr", "ccpa", "breach"
	Standard    string    `json:"standard,omitempty"`
	Status      string    `json:"status"`
	Score       int       `json:"score,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// TrendDataPoint 趋势数据点.
type TrendDataPoint struct {
	Date   string `json:"date"`   // YYYY-MM-DD
	Score  int    `json:"score"`
	Status string `json:"status"`
}

// DashboardService 仪表盘服务.
type DashboardService struct {
	reportGen    *ReportGenerator
	gdprGen      *GDPRReportGenerator
	ccpaGen      *CCPAReportGenerator
	breachGen    *BreachReportGenerator
	gdprReports  map[string]*GDPRReport
	ccpaReports  map[string]*CCPAReport
	breachReports map[string]*BreachNotificationReport
	standards    *StandardsManager
	mu           sync.RWMutex
}

// NewDashboardService 创建仪表盘服务.
func NewDashboardService(reportGen *ReportGenerator, standards *StandardsManager) *DashboardService {
	return &DashboardService{
		reportGen:     reportGen,
		gdprGen:       NewGDPRReportGenerator(),
		ccpaGen:       NewCCPAReportGenerator(),
		breachGen:     NewBreachReportGenerator(),
		gdprReports:   make(map[string]*GDPRReport),
		ccpaReports:   make(map[string]*CCPAReport),
		breachReports: make(map[string]*BreachNotificationReport),
		standards:     standards,
	}
}

// SaveGDPRReport 保存 GDPR 报告.
func (s *DashboardService) SaveGDPRReport(report *GDPRReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gdprReports[report.ReportID] = report
}

// SaveCCPAReport 保存 CCPA 报告.
func (s *DashboardService) SaveCCPAReport(report *CCPAReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ccpaReports[report.ReportID] = report
}

// SaveBreachReport 保存泄露报告.
func (s *DashboardService) SaveBreachReport(report *BreachNotificationReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.breachReports[report.ReportID] = report
}

// GetMetrics 获取仪表盘指标.
func (s *DashboardService) GetMetrics(ctx context.Context) *DashboardMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := &DashboardMetrics{
		LastUpdated: time.Now(),
	}

	// 基础合规指标
	status := s.reportGen.GetStatus()
	metrics.OverallComplianceScore = status.OverallScore
	metrics.OverallStatus = string(status.OverallStatus)
	metrics.PendingRemediations = status.PendingRemediation

	// 各标准状态
	for _, std := range status.Standards {
		stdInfo, _ := s.standards.GetStandard(std.Standard)
		sc := StandardCompliance{
			Standard: string(std.Standard),
			Name:     stdInfo.Name,
			Status:   string(std.Status),
			Score:    std.Score,
			LastScanAt: std.LastScan,
		}
		metrics.StandardsCompliance = append(metrics.StandardsCompliance, sc)
	}

	// 最近报告
	metrics.RecentReports = s.getRecentReports()

	// GDPR 指标
	metrics.GDPRMetrics = s.getGDPRMetrics()

	// CCPA 指标
	metrics.CCPAMetrics = s.getCCPAMetrics()

	// 泄露指标
	breachMetrics := s.getBreachMetrics()
	metrics.BreachMetrics = breachMetrics
	metrics.ActiveBreachCount = breachMetrics.ActiveBreaches

	// 趋势数据（模拟）
	metrics.ComplianceTrend = s.generateTrend(status)

	return metrics
}

// getRecentReports 获取最近报告.
func (s *DashboardService) getRecentReports() []ReportSummary {
	var summaries []ReportSummary

	// 合规报告
	for _, r := range s.reportGen.ListReports(nil) {
		summaries = append(summaries, ReportSummary{
			ReportID:  r.ID,
			Type:      "compliance",
			Standard:  string(r.Standard),
			Status:    string(r.ComplianceStatus),
			Score:     r.Score,
			CreatedAt: r.CreatedAt,
		})
	}

	// GDPR 报告
	for _, r := range s.gdprReports {
		summaries = append(summaries, ReportSummary{
			ReportID:  r.ReportID,
			Type:      "gdpr",
			Status:    r.Summary.ComplianceStatus,
			CreatedAt: r.GeneratedAt,
		})
	}

	// CCPA 报告
	for _, r := range s.ccpaReports {
		summaries = append(summaries, ReportSummary{
			ReportID:  r.ReportID,
			Type:      "ccpa",
			Status:    r.Summary.ComplianceStatus,
			CreatedAt: r.GeneratedAt,
		})
	}

	// 泄露报告
	for _, r := range s.breachReports {
		summaries = append(summaries, ReportSummary{
			ReportID:  r.ReportID,
			Type:      "breach",
			Status:    string(r.Status),
			CreatedAt: r.GeneratedAt,
		})
	}

	return summaries
}

// getGDPRMetrics 获取 GDPR 指标.
func (s *DashboardService) getGDPRMetrics() *GDPRDashboardMetrics {
	if len(s.gdprReports) == 0 {
		return nil
	}

	// 取最新报告
	var latest *GDPRReport
	for _, r := range s.gdprReports {
		if latest == nil || r.GeneratedAt.After(latest.GeneratedAt) {
			latest = r
		}
	}

	if latest == nil {
		return nil
	}

	return &GDPRDashboardMetrics{
		DataProcessingActivities: latest.Summary.TotalActivities,
		ActiveActivities:         latest.Summary.ActiveActivities,
		StorageLocations:         latest.Summary.TotalStorageLocs,
		EncryptedStoragePct:      latest.Summary.EncryptedStoragePct,
		EUStorageLocations:       latest.Summary.EUStorageLocs,
		DataSubjectRequests:      latest.DataSubjectRights.TotalRequests,
		AvgResponseDays:          latest.DataSubjectRights.AvgResponseDays,
		ComplianceStatus:         latest.Summary.ComplianceStatus,
	}
}

// getCCPAMetrics 获取 CCPA 指标.
func (s *DashboardService) getCCPAMetrics() *CCPADashboardMetrics {
	if len(s.ccpaReports) == 0 {
		return nil
	}

	var latest *CCPAReport
	for _, r := range s.ccpaReports {
		if latest == nil || r.GeneratedAt.After(latest.GeneratedAt) {
			latest = r
		}
	}

	if latest == nil {
		return nil
	}

	return &CCPADashboardMetrics{
		DataCategories:    latest.Summary.TotalDataCategories,
		ThirdPartyPartners: latest.Summary.ActiveThirdParties,
		SharedDataFields:  latest.Summary.SharedDataFields,
		SoldDataFields:    latest.Summary.SoldDataFields,
		AccessRequests:    latest.Summary.TotalAccessRequests,
		CompletedRequests: latest.Summary.CompletedRequests,
		PendingRequests:   latest.Summary.PendingRequests,
		ComplianceStatus:  latest.Summary.ComplianceStatus,
	}
}

// getBreachMetrics 获取泄露指标.
func (s *DashboardService) getBreachMetrics() *BreachDashboardMetrics {
	metrics := &BreachDashboardMetrics{}

	var totalResponseHours, totalNotifyHours float64
	responseCount, notifyCount := 0, 0

	for _, r := range s.breachReports {
		metrics.TotalBreaches++
		metrics.TotalRecordsAffected += r.TotalRecordsAffected

		switch r.Status {
		case BreachStatusDetected, BreachStatusContained, BreachStatusInvestigating, BreachStatusRemediating:
			metrics.ActiveBreaches++
		case BreachStatusClosed:
			metrics.ClosedBreaches++
		}

		if r.Summary.ResponseTimeHours > 0 {
			totalResponseHours += r.Summary.ResponseTimeHours
			responseCount++
		}

		if r.Summary.NotificationTimeHours > 0 {
			totalNotifyHours += r.Summary.NotificationTimeHours
			notifyCount++
		}

		if r.NotificationCompliant {
			metrics.NotificationCompliant++
		}
	}

	if responseCount > 0 {
		metrics.AvgResponseHours = totalResponseHours / float64(responseCount)
	}
	if notifyCount > 0 {
		metrics.AvgNotificationHours = totalNotifyHours / float64(notifyCount)
	}

	return metrics
}

// generateTrend 生成趋势数据.
func (s *DashboardService) generateTrend(status *ComplianceStatusOverview) []TrendDataPoint {
	trend := make([]TrendDataPoint, 7)
	now := time.Now()

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -(6 - i))
		// 模拟趋势，实际应从历史数据获取
		score := status.OverallScore
		if i > 0 {
			// 模拟历史波动
			score = score - (6-i)*2
			if score < 0 {
				score = 0
			}
		}

		trend[6-i] = TrendDataPoint{
			Date:   date.Format("2006-01-02"),
			Score:  score,
			Status: determineStatus(score),
		}
	}

	return trend
}

// determineStatus 根据分数确定状态.
func determineStatus(score int) string {
	switch {
	case score >= 90:
		return "compliant"
	case score >= 60:
		return "pending_review"
	default:
		return "non_compliant"
	}
}

// DashboardHandlers 仪表盘 API 处理器.
type DashboardHandlers struct {
	service *DashboardService
}

// NewDashboardHandlers 创建仪表盘处理器.
func NewDashboardHandlers(service *DashboardService) *DashboardHandlers {
	return &DashboardHandlers{service: service}
}

// RegisterDashboardRoutes 注册仪表盘路由.
func (h *DashboardHandlers) RegisterDashboardRoutes(api *gin.RouterGroup) {
	dashboard := api.Group("/compliance-dashboard")
	{
		dashboard.GET("/metrics", h.getMetrics)
		dashboard.GET("/gdpr", h.getGDPRReport)
		dashboard.POST("/gdpr/generate", h.generateGDPRReport)
		dashboard.GET("/ccpa", h.getCCPAReport)
		dashboard.POST("/ccpa/generate", h.generateCCPAReport)
		dashboard.GET("/breach", h.listBreachReports)
		dashboard.POST("/breach/generate", h.generateBreachReport)
		dashboard.GET("/export/json/:type/:id", h.exportJSON)
	}
}

// GET /api/v1/compliance-dashboard/metrics
func (h *DashboardHandlers) getMetrics(c *gin.Context) {
	metrics := h.service.GetMetrics(c.Request.Context())
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "success", Data: metrics})
}

// GET /api/v1/compliance-dashboard/gdpr
func (h *DashboardHandlers) getGDPRReport(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	var latest *GDPRReport
	for _, r := range h.service.gdprReports {
		if latest == nil || r.GeneratedAt.After(latest.GeneratedAt) {
			latest = r
		}
	}

	if latest == nil {
		c.JSON(http.StatusNotFound, errResp(404, "没有 GDPR 报告"))
		return
	}

	c.JSON(http.StatusOK, okResp(latest))
}

// POST /api/v1/compliance-dashboard/gdpr/generate
func (h *DashboardHandlers) generateGDPRReport(c *gin.Context) {
	var config GDPRReportConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, errResp(400, "请求参数错误: "+err.Error()))
		return
	}

	report := h.service.gdprGen.GenerateGDPRReport(config)
	h.service.SaveGDPRReport(report)
	c.JSON(http.StatusOK, okResp(report))
}

// GET /api/v1/compliance-dashboard/ccpa
func (h *DashboardHandlers) getCCPAReport(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	var latest *CCPAReport
	for _, r := range h.service.ccpaReports {
		if latest == nil || r.GeneratedAt.After(latest.GeneratedAt) {
			latest = r
		}
	}

	if latest == nil {
		c.JSON(http.StatusNotFound, errResp(404, "没有 CCPA 报告"))
		return
	}

	c.JSON(http.StatusOK, okResp(latest))
}

// POST /api/v1/compliance-dashboard/ccpa/generate
func (h *DashboardHandlers) generateCCPAReport(c *gin.Context) {
	var config CCPAReportConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, errResp(400, "请求参数错误: "+err.Error()))
		return
	}

	report := h.service.ccpaGen.GenerateCCPAReport(config)
	h.service.SaveCCPAReport(report)
	c.JSON(http.StatusOK, okResp(report))
}

// GET /api/v1/compliance-dashboard/breach
func (h *DashboardHandlers) listBreachReports(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	reports := make([]*BreachNotificationReport, 0, len(h.service.breachReports))
	for _, r := range h.service.breachReports {
		reports = append(reports, r)
	}

	c.JSON(http.StatusOK, okResp(reports))
}

// POST /api/v1/compliance-dashboard/breach/generate
func (h *DashboardHandlers) generateBreachReport(c *gin.Context) {
	var config BreachReportConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, errResp(400, "请求参数错误: "+err.Error()))
		return
	}

	report := h.service.breachGen.GenerateBreachReport(config)
	h.service.SaveBreachReport(report)
	c.JSON(http.StatusOK, okResp(report))
}

// GET /api/v1/compliance-dashboard/export/json/:type/:id
func (h *DashboardHandlers) exportJSON(c *gin.Context) {
	reportType := c.Param("type")
	reportID := c.Param("id")

	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	switch reportType {
	case "compliance":
		report, ok := h.service.reportGen.GetReport(reportID)
		if !ok {
			c.JSON(http.StatusNotFound, errResp(404, "报告不存在"))
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=compliance_%s.json", reportID))
		c.JSON(http.StatusOK, report)

	case "gdpr":
		report, ok := h.service.gdprReports[reportID]
		if !ok {
			c.JSON(http.StatusNotFound, errResp(404, "GDPR 报告不存在"))
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=gdpr_%s.json", reportID))
		c.JSON(http.StatusOK, report)

	case "ccpa":
		report, ok := h.service.ccpaReports[reportID]
		if !ok {
			c.JSON(http.StatusNotFound, errResp(404, "CCPA 报告不存在"))
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=ccpa_%s.json", reportID))
		c.JSON(http.StatusOK, report)

	case "breach":
		report, ok := h.service.breachReports[reportID]
		if !ok {
			c.JSON(http.StatusNotFound, errResp(404, "泄露报告不存在"))
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=breach_%s.json", reportID))
		c.JSON(http.StatusOK, report)

	default:
		c.JSON(http.StatusBadRequest, errResp(400, fmt.Sprintf("不支持的报告类型: %s，支持: compliance, gdpr, ccpa, breach", reportType)))
	}
}
