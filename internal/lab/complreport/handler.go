package complreport

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 合规审计报告 HTTP 处理器.
type Handlers struct {
	svc *Service
}

// NewHandlers 创建合规审计报告处理器.
func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// RegisterRoutes 注册合规审计报告路由.
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	cr := apiGroup.Group("/complreport")
	{
		cr.POST("/generate", h.generateReport)
		cr.GET("/list", h.listReports)
		cr.GET("/:id", h.getReport)
		cr.POST("/schedule", h.createSchedule)
	}
}

// generateReport 生成合规审计报告
// @Summary 生成合规审计报告
// @Description 根据合规标准生成审计报告，自动收集合规证据
// @Tags complreport
// @Accept json
// @Produce json
// @Param request body GenerateRequest true "生成请求"
// @Success 201 {object} api.Response{data=Report}
// @Failure 400 {object} api.Response
// @Router /complreport/generate [post].
func (h *Handlers) generateReport(c *gin.Context) {
	var req GenerateRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	report, err := h.svc.GenerateReport(req)
	if err != nil {
		if err == ErrInvalidStandard {
			api.BadRequest(c, "无效的合规标准")
			return
		}
		if err == ErrInvalidFormat {
			api.BadRequest(c, "无效的报告格式")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, report)
}

// listReports 列出所有报告
// @Summary 列出合规审计报告
// @Description 获取所有已生成的合规审计报告
// @Tags complreport
// @Produce json
// @Success 200 {object} api.Response{data=ListResponse}
// @Router /complreport/list [get].
func (h *Handlers) listReports(c *gin.Context) {
	reports := h.svc.ListReports()
	api.OK(c, ListResponse{
		Reports: reports,
		Total:   len(reports),
	})
}

// getReport 获取报告详情
// @Summary 获取合规审计报告
// @Description 根据 ID 获取报告详情
// @Tags complreport
// @Produce json
// @Param id path string true "报告 ID"
// @Success 200 {object} api.Response{data=Report}
// @Failure 404 {object} api.Response
// @Router /complreport/{id} [get].
func (h *Handlers) getReport(c *gin.Context) {
	reportID := c.Param("id")
	if reportID == "" {
		api.BadRequest(c, "报告 ID 不能为空")
		return
	}

	report, err := h.svc.GetReport(reportID)
	if err != nil {
		if err == ErrReportNotFound {
			api.NotFound(c, "合规报告未找到")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, report)
}

// createSchedule 创建定期报告计划
// @Summary 创建定期报告计划
// @Description 创建定期合规审计报告生成计划
// @Tags complreport
// @Accept json
// @Produce json
// @Param request body ScheduleRequest true "计划信息"
// @Success 201 {object} api.Response{data=Schedule}
// @Failure 400 {object} api.Response
// @Router /complreport/schedule [post].
func (h *Handlers) createSchedule(c *gin.Context) {
	var req ScheduleRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	schedule, err := h.svc.CreateSchedule(req)
	if err != nil {
		if err == ErrInvalidStandard {
			api.BadRequest(c, "无效的合规标准")
			return
		}
		if err == ErrInvalidFormat {
			api.BadRequest(c, "无效的报告格式")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, schedule)
}
