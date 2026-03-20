// Package quota 提供存储配额管理 API
// Version: v2.56.0 - 配额设置、查询、调整 API
package quota

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// API 配额管理 API 处理器
type API struct {
	manager *Manager
}

// NewAPI 创建配额 API 处理器
func NewAPI(mgr *Manager) *API {
	return &API{
		manager: mgr,
	}
}

// RegisterRoutes 注册路由
func (a *API) RegisterRoutes(r *gin.RouterGroup) {
	quotaAPI := r.Group("/quota/v2")
	{
		// 配额设置
		quotaAPI.POST("/set", a.SetQuota)
		quotaAPI.POST("/batch-set", a.BatchSetQuota)

		// 配额查询
		quotaAPI.GET("/get/:id", a.GetQuota)
		quotaAPI.GET("/list", a.ListQuotas)
		quotaAPI.GET("/usage/:id", a.GetQuotaUsage)
		quotaAPI.GET("/usage", a.GetAllUsage)

		// 配额调整
		quotaAPI.PUT("/adjust/:id", a.AdjustQuota)
		quotaAPI.PUT("/extend-grace/:id", a.ExtendGracePeriod)
		quotaAPI.DELETE("/delete/:id", a.DeleteQuota)

		// 配额限制管理
		quotaAPI.PUT("/limits/:id", a.SetLimits)
		quotaAPI.GET("/limits/:id", a.GetLimits)

		// 配额状态
		quotaAPI.GET("/status/:id", a.GetQuotaStatus)
		quotaAPI.GET("/violations", a.GetViolations)
	}
}

// ========== 配额设置 API ==========

// SetQuotaRequest 设置配额请求
type SetQuotaRequest struct {
	Type        QuotaType `json:"type" binding:"required"`      // user/group/directory
	TargetID    string    `json:"targetId" binding:"required"`  // 用户名/组名/路径
	VolumeName  string    `json:"volumeName"`                   // 卷名
	Path        string    `json:"path"`                         // 目录路径（目录配额必填）
	HardLimit   uint64    `json:"hardLimit" binding:"required"` // 硬限制（字节）
	SoftLimit   uint64    `json:"softLimit"`                    // 软限制（字节）
	GracePeriod int       `json:"gracePeriod"`                  // 宽限期（小时）
}

// SetQuota 设置配额
// @Summary 设置配额
// @Description 设置用户、组或目录的存储配额
// @Tags quota
// @Accept json
// @Produce json
// @Param request body SetQuotaRequest true "配额设置请求"
// @Success 200 {object} Response{data=Quota}
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /quota/v2/set [post]
// @Security BearerAuth
func (a *API) SetQuota(c *gin.Context) {
	var req SetQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的请求参数: "+err.Error()))
		return
	}

	// 验证软限制不能超过硬限制
	if req.SoftLimit > 0 && req.SoftLimit > req.HardLimit {
		c.JSON(http.StatusBadRequest, Error(400, "软限制不能超过硬限制"))
		return
	}

	// 默认软限制为硬限制的 80%
	if req.SoftLimit == 0 {
		req.SoftLimit = uint64(float64(req.HardLimit) * 0.8)
	}

	input := QuotaInput{
		Type:       req.Type,
		TargetID:   req.TargetID,
		VolumeName: req.VolumeName,
		Path:       req.Path,
		HardLimit:  req.HardLimit,
		SoftLimit:  req.SoftLimit,
	}

	quota, err := a.manager.CreateQuota(input)
	if err != nil {
		switch err {
		case ErrUserNotFound:
			c.JSON(http.StatusNotFound, Error(404, "用户不存在"))
		case ErrGroupNotFound:
			c.JSON(http.StatusNotFound, Error(404, "用户组不存在"))
		case ErrQuotaExists:
			c.JSON(http.StatusConflict, Error(409, "配额已存在"))
		case ErrInvalidLimit:
			c.JSON(http.StatusBadRequest, Error(400, "无效的配额限制"))
		default:
			c.JSON(http.StatusInternalServerError, Error(500, "创建配额失败: "+err.Error()))
		}
		return
	}

	// 设置宽限期
	if req.GracePeriod > 0 {
		a.manager.SetGracePeriod(quota.ID, time.Duration(req.GracePeriod)*time.Hour)
	}

	c.JSON(http.StatusOK, Success(quota))
}

// BatchSetQuotaRequest 批量设置配额请求
type BatchSetQuotaRequest struct {
	Quotas []SetQuotaRequest `json:"quotas" binding:"required"`
}

// BatchSetQuotaResponse 批量设置配额响应
type BatchSetQuotaResponse struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Quotas  []Quota  `json:"quotas"`
	Errors  []string `json:"errors,omitempty"`
}

// BatchSetQuota 批量设置配额
// @Summary 批量设置配额
// @Description 批量设置多个配额
// @Tags quota
// @Accept json
// @Produce json
// @Param request body BatchSetQuotaRequest true "批量配额设置请求"
// @Success 200 {object} Response{data=BatchSetQuotaResponse}
// @Router /quota/v2/batch-set [post]
// @Security BearerAuth
func (a *API) BatchSetQuota(c *gin.Context) {
	var req BatchSetQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的请求参数: "+err.Error()))
		return
	}

	resp := BatchSetQuotaResponse{
		Quotas: make([]Quota, 0, len(req.Quotas)),
		Errors: make([]string, 0),
	}

	for _, q := range req.Quotas {
		input := QuotaInput{
			Type:       q.Type,
			TargetID:   q.TargetID,
			VolumeName: q.VolumeName,
			Path:       q.Path,
			HardLimit:  q.HardLimit,
			SoftLimit:  q.SoftLimit,
		}

		quota, err := a.manager.CreateQuota(input)
		if err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %s", q.TargetID, err.Error()))
		} else {
			resp.Success++
			resp.Quotas = append(resp.Quotas, *quota)
		}
	}

	c.JSON(http.StatusOK, Success(resp))
}

// ========== 配额查询 API ==========

// GetQuota 获取配额详情
// @Summary 获取配额详情
// @Description 获取指定配额的详细信息
// @Tags quota
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response{data=Quota}
// @Failure 404 {object} Response
// @Router /quota/v2/get/{id} [get]
// @Security BearerAuth
func (a *API) GetQuota(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	quota, err := a.manager.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
		return
	}

	c.JSON(http.StatusOK, Success(quota))
}

// ListQuotasRequest 列出配额请求
type ListQuotasRequest struct {
	Type       QuotaType `form:"type"`
	TargetID   string    `form:"targetId"`
	VolumeName string    `form:"volumeName"`
	Page       int       `form:"page"`
	PageSize   int       `form:"pageSize"`
}

// ListQuotasResponse 列出配额响应
type ListQuotasResponse struct {
	Total  int     `json:"total"`
	Page   int     `json:"page"`
	Quotas []Quota `json:"quotas"`
}

// ListQuotas 列出配额
// @Summary 列出配额
// @Description 获取配额列表，支持按类型和目标过滤
// @Tags quota
// @Produce json
// @Param type query string false "配额类型 (user/group/directory)"
// @Param targetId query string false "目标 ID"
// @Param volumeName query string false "卷名"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Success 200 {object} Response{data=ListQuotasResponse}
// @Router /quota/v2/list [get]
// @Security BearerAuth
func (a *API) ListQuotas(c *gin.Context) {
	var req ListQuotasRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的查询参数: "+err.Error()))
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	var quotas []*Quota
	switch req.Type {
	case QuotaTypeUser:
		if req.TargetID != "" {
			quotas = a.manager.ListUserQuotas(req.TargetID)
		} else {
			quotas = a.manager.ListQuotas()
			// 过滤用户配额
			filtered := make([]*Quota, 0)
			for _, q := range quotas {
				if q.Type == QuotaTypeUser {
					filtered = append(filtered, q)
				}
			}
			quotas = filtered
		}
	case QuotaTypeGroup:
		if req.TargetID != "" {
			quotas = a.manager.ListGroupQuotas(req.TargetID)
		} else {
			quotas = a.manager.ListQuotas()
			// 过滤组配额
			filtered := make([]*Quota, 0)
			for _, q := range quotas {
				if q.Type == QuotaTypeGroup {
					filtered = append(filtered, q)
				}
			}
			quotas = filtered
		}
	case QuotaTypeDirectory:
		quotas = a.manager.ListDirectoryQuotas()
	default:
		quotas = a.manager.ListQuotas()
	}

	// 按卷过滤
	if req.VolumeName != "" {
		filtered := make([]*Quota, 0)
		for _, q := range quotas {
			if q.VolumeName == req.VolumeName {
				filtered = append(filtered, q)
			}
		}
		quotas = filtered
	}

	// 分页
	total := len(quotas)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	resp := ListQuotasResponse{
		Total:  total,
		Page:   req.Page,
		Quotas: make([]Quota, 0, end-start),
	}
	for i := start; i < end; i++ {
		resp.Quotas = append(resp.Quotas, *quotas[i])
	}

	c.JSON(http.StatusOK, Success(resp))
}

// GetQuotaUsage 获取配额使用情况
// @Summary 获取配额使用情况
// @Description 获取指定配额的使用统计
// @Tags quota
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response{data=QuotaUsage}
// @Failure 404 {object} Response
// @Router /quota/v2/usage/{id} [get]
// @Security BearerAuth
func (a *API) GetQuotaUsage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	usage, err := a.manager.GetUsage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
		return
	}

	c.JSON(http.StatusOK, Success(usage))
}

// GetAllUsage 获取所有配额使用情况
// @Summary 获取所有配额使用情况
// @Description 获取所有配额的使用统计
// @Tags quota
// @Produce json
// @Success 200 {object} Response{data=[]QuotaUsage}
// @Router /quota/v2/usage [get]
// @Security BearerAuth
func (a *API) GetAllUsage(c *gin.Context) {
	usages, err := a.manager.GetAllUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "获取使用情况失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(usages))
}

// ========== 配额调整 API ==========

// AdjustQuotaRequest 调整配额请求
type AdjustQuotaRequest struct {
	HardLimitDelta int64  `json:"hardLimitDelta"` // 硬限制增量（可正可负）
	SoftLimitDelta int64  `json:"softLimitDelta"` // 软限制增量（可正可负）
	Reason         string `json:"reason"`         // 调整原因
}

// AdjustQuota 调整配额
// @Summary 调整配额
// @Description 增量调整配额限制
// @Tags quota
// @Accept json
// @Produce json
// @Param id path string true "配额 ID"
// @Param request body AdjustQuotaRequest true "调整请求"
// @Success 200 {object} Response{data=Quota}
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /quota/v2/adjust/{id} [put]
// @Security BearerAuth
func (a *API) AdjustQuota(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	var req AdjustQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的请求参数: "+err.Error()))
		return
	}

	// 获取当前配额
	quota, err := a.manager.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
		return
	}

	// 安全的类型转换：检查 uint64 -> int64 是否会溢出
	// int64 最大值为 9223372036854775807
	const maxInt64 = uint64(1<<63 - 1)

	var newHardLimit, newSoftLimit int64

	// 安全计算新硬限制
	if quota.HardLimit > maxInt64 {
		// uint64 值超过 int64 最大值，直接使用最大值
		newHardLimit = int64(maxInt64) + req.HardLimitDelta
	} else {
		newHardLimit = int64(quota.HardLimit) + req.HardLimitDelta
	}

	// 安全计算新软限制
	if quota.SoftLimit > maxInt64 {
		newSoftLimit = int64(maxInt64) + req.SoftLimitDelta
	} else {
		newSoftLimit = int64(quota.SoftLimit) + req.SoftLimitDelta
	}

	// 验证限制
	if newHardLimit <= 0 {
		c.JSON(http.StatusBadRequest, Error(400, "硬限制必须大于 0"))
		return
	}
	if newSoftLimit < 0 {
		newSoftLimit = 0
	}
	if newSoftLimit > newHardLimit {
		c.JSON(http.StatusBadRequest, Error(400, "软限制不能超过硬限制"))
		return
	}

	// 更新配额
	input := QuotaInput{
		Type:       quota.Type,
		TargetID:   quota.TargetID,
		VolumeName: quota.VolumeName,
		Path:       quota.Path,
		HardLimit:  uint64(newHardLimit),
		SoftLimit:  uint64(newSoftLimit),
	}

	updated, err := a.manager.UpdateQuota(id, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "更新配额失败: "+err.Error()))
		return
	}

	// 记录调整历史
	a.manager.RecordAdjustment(id, req.HardLimitDelta, req.SoftLimitDelta, req.Reason)

	c.JSON(http.StatusOK, Success(updated))
}

// ExtendGracePeriodRequest 延长宽限期请求
type ExtendGracePeriodRequest struct {
	ExtendHours int    `json:"extendHours" binding:"required,min=1"` // 延长小时数
	Reason      string `json:"reason"`                               // 延长原因
}

// ExtendGracePeriod 延长宽限期
// @Summary 延长宽限期
// @Description 延长配额超限的宽限期
// @Tags quota
// @Accept json
// @Produce json
// @Param id path string true "配额 ID"
// @Param request body ExtendGracePeriodRequest true "延长请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /quota/v2/extend-grace/{id} [put]
// @Security BearerAuth
func (a *API) ExtendGracePeriod(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	var req ExtendGracePeriodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的请求参数: "+err.Error()))
		return
	}

	// 获取配额
	_, err := a.manager.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
		return
	}

	// 延长宽限期
	newExpiry := time.Now().Add(time.Duration(req.ExtendHours) * time.Hour)
	a.manager.ExtendGracePeriod(id, newExpiry)

	c.JSON(http.StatusOK, Success(map[string]interface{}{
		"extendedHours": req.ExtendHours,
		"reason":        req.Reason,
	}))
}

// DeleteQuota 删除配额
// @Summary 删除配额
// @Description 删除指定的配额
// @Tags quota
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /quota/v2/delete/{id} [delete]
// @Security BearerAuth
func (a *API) DeleteQuota(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	if err := a.manager.DeleteQuota(id); err != nil {
		if err == ErrQuotaNotFound {
			c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, Error(500, "删除配额失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

// ========== 配额限制管理 API ==========

// SetLimitsRequest 设置限制请求
type SetLimitsRequest struct {
	HardLimit   uint64 `json:"hardLimit" binding:"required"` // 硬限制
	SoftLimit   uint64 `json:"softLimit"`                    // 软限制
	GracePeriod int    `json:"gracePeriod"`                  // 宽限期（小时）
}

// SetLimits 设置配额限制
// @Summary 设置配额限制
// @Description 设置配额的硬限制、软限制和宽限期
// @Tags quota
// @Accept json
// @Produce json
// @Param id path string true "配额 ID"
// @Param request body SetLimitsRequest true "限制设置请求"
// @Success 200 {object} Response{data=Quota}
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /quota/v2/limits/{id} [put]
// @Security BearerAuth
func (a *API) SetLimits(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	var req SetLimitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的请求参数: "+err.Error()))
		return
	}

	// 验证软限制
	if req.SoftLimit > 0 && req.SoftLimit > req.HardLimit {
		c.JSON(http.StatusBadRequest, Error(400, "软限制不能超过硬限制"))
		return
	}

	// 默认软限制
	if req.SoftLimit == 0 {
		req.SoftLimit = uint64(float64(req.HardLimit) * 0.8)
	}

	// 获取当前配额
	quota, err := a.manager.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
		return
	}

	// 更新配额
	input := QuotaInput{
		Type:       quota.Type,
		TargetID:   quota.TargetID,
		VolumeName: quota.VolumeName,
		Path:       quota.Path,
		HardLimit:  req.HardLimit,
		SoftLimit:  req.SoftLimit,
	}

	updated, err := a.manager.UpdateQuota(id, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "更新限制失败: "+err.Error()))
		return
	}

	// 设置宽限期
	if req.GracePeriod > 0 {
		a.manager.SetGracePeriod(id, time.Duration(req.GracePeriod)*time.Hour)
	}

	c.JSON(http.StatusOK, Success(updated))
}

// LimitsResponse 限制响应
type LimitsResponse struct {
	QuotaID      string     `json:"quotaId"`
	HardLimit    uint64     `json:"hardLimit"`             // 硬限制（字节）
	SoftLimit    uint64     `json:"softLimit"`             // 软限制（字节）
	GracePeriod  int        `json:"gracePeriod"`           // 宽限期（小时）
	GraceExpiry  *time.Time `json:"graceExpiry,omitempty"` // 宽限期到期时间
	HardLimitStr string     `json:"hardLimitStr"`          // 可读格式
	SoftLimitStr string     `json:"softLimitStr"`          // 可读格式
}

// GetLimits 获取配额限制
// @Summary 获取配额限制
// @Description 获取配额的硬限制、软限制和宽限期
// @Tags quota
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response{data=LimitsResponse}
// @Failure 404 {object} Response
// @Router /quota/v2/limits/{id} [get]
// @Security BearerAuth
func (a *API) GetLimits(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	quota, err := a.manager.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
		return
	}

	resp := LimitsResponse{
		QuotaID:      quota.ID,
		HardLimit:    quota.HardLimit,
		SoftLimit:    quota.SoftLimit,
		HardLimitStr: formatBytes(quota.HardLimit),
		SoftLimitStr: formatBytes(quota.SoftLimit),
	}

	// 获取宽限期信息
	gracePeriod, graceExpiry := a.manager.GetGracePeriodInfo(id)
	resp.GracePeriod = int(gracePeriod.Hours())
	resp.GraceExpiry = graceExpiry

	c.JSON(http.StatusOK, Success(resp))
}

// ========== 配额状态 API ==========

// Status 配额状态
type Status struct {
	QuotaID        string     `json:"quotaId"`
	Type           QuotaType  `json:"type"`
	TargetID       string     `json:"targetId"`
	TargetName     string     `json:"targetName"`
	VolumeName     string     `json:"volumeName"`
	Path           string     `json:"path"`
	HardLimit      uint64     `json:"hardLimit"`
	SoftLimit      uint64     `json:"softLimit"`
	UsedBytes      uint64     `json:"usedBytes"`
	AvailableBytes uint64     `json:"availableBytes"`
	UsagePercent   float64    `json:"usagePercent"`
	IsOverSoft     bool       `json:"isOverSoft"`
	IsOverHard     bool       `json:"isOverHard"`
	InGracePeriod  bool       `json:"inGracePeriod"`
	GraceExpiry    *time.Time `json:"graceExpiry,omitempty"`
	Status         string     `json:"status"` // normal, warning, critical, exceeded
	Message        string     `json:"message"`
}

// GetQuotaStatus 获取配额状态
// @Summary 获取配额状态
// @Description 获取配额的详细状态信息
// @Tags quota
// @Produce json
// @Param id path string true "配额 ID"
// @Success 200 {object} Response{data=QuotaStatus}
// @Failure 404 {object} Response
// @Router /quota/v2/status/{id} [get]
// @Security BearerAuth
func (a *API) GetQuotaStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Error(400, "配额 ID 不能为空"))
		return
	}

	quota, err := a.manager.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, "配额不存在"))
		return
	}

	usage, err := a.manager.GetUsage(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "获取使用情况失败: "+err.Error()))
		return
	}

	status := Status{
		QuotaID:        quota.ID,
		Type:           quota.Type,
		TargetID:       quota.TargetID,
		TargetName:     quota.TargetName,
		VolumeName:     quota.VolumeName,
		Path:           quota.Path,
		HardLimit:      quota.HardLimit,
		SoftLimit:      quota.SoftLimit,
		UsedBytes:      usage.UsedBytes,
		AvailableBytes: usage.Available,
		UsagePercent:   usage.UsagePercent,
		IsOverSoft:     usage.IsOverSoft,
		IsOverHard:     usage.IsOverHard,
	}

	// 确定状态
	switch {
	case usage.IsOverHard:
		status.Status = "exceeded"
		status.Message = "已超过硬限制，写入将被拒绝"
	case usage.IsOverSoft:
		status.Status = "warning"
		status.Message = "已超过软限制，请注意存储空间"
	case usage.UsagePercent >= 80:
		status.Status = "critical"
		status.Message = "存储空间即将用尽"
	case usage.UsagePercent >= 60:
		status.Status = "warning"
		status.Message = "存储空间使用较高"
	default:
		status.Status = "normal"
		status.Message = "存储空间正常"
	}

	// 检查宽限期
	_, graceExpiry := a.manager.GetGracePeriodInfo(id)
	if graceExpiry != nil {
		status.GraceExpiry = graceExpiry
		status.InGracePeriod = time.Now().Before(*graceExpiry)
		if status.InGracePeriod && status.IsOverSoft {
			status.Message = fmt.Sprintf("宽限期内，将在 %s 后拒绝写入",
				graceExpiry.Format("2006-01-02 15:04:05"))
		}
	}

	c.JSON(http.StatusOK, Success(status))
}

// Violation 配额违规记录
type Violation struct {
	QuotaID      string     `json:"quotaId"`
	TargetID     string     `json:"targetId"`
	TargetName   string     `json:"targetName"`
	VolumeName   string     `json:"volumeName"`
	Type         string     `json:"type"` // soft_limit, hard_limit
	UsedBytes    uint64     `json:"usedBytes"`
	LimitBytes   uint64     `json:"limitBytes"`
	UsagePercent float64    `json:"usagePercent"`
	ViolatedAt   time.Time  `json:"violatedAt"`
	ResolvedAt   *time.Time `json:"resolvedAt,omitempty"`
}

// GetViolations 获取违规列表
// @Summary 获取违规列表
// @Description 获取当前所有配额违规
// @Tags quota
// @Produce json
// @Param type query string false "违规类型 (soft_limit/hard_limit)"
// @Success 200 {object} Response{data=[]Violation}
// @Router /quota/v2/violations [get]
// @Security BearerAuth
func (a *API) GetViolations(c *gin.Context) {
	filterType := c.Query("type")

	usages, err := a.manager.GetAllUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "获取使用情况失败: "+err.Error()))
		return
	}

	violations := make([]Violation, 0)
	for _, usage := range usages {
		// 检查硬限制违规
		if usage.IsOverHard {
			if filterType == "" || filterType == "hard_limit" {
				quota, _ := a.manager.GetQuota(usage.QuotaID)
				if quota != nil {
					violations = append(violations, Violation{
						QuotaID:      usage.QuotaID,
						TargetID:     usage.TargetID,
						TargetName:   usage.TargetName,
						VolumeName:   usage.VolumeName,
						Type:         "hard_limit",
						UsedBytes:    usage.UsedBytes,
						LimitBytes:   usage.HardLimit,
						UsagePercent: usage.UsagePercent,
						ViolatedAt:   time.Now(),
					})
				}
			}
		} else if usage.IsOverSoft {
			// 检查软限制违规
			if filterType == "" || filterType == "soft_limit" {
				quota, _ := a.manager.GetQuota(usage.QuotaID)
				if quota != nil {
					violations = append(violations, Violation{
						QuotaID:      usage.QuotaID,
						TargetID:     usage.TargetID,
						TargetName:   usage.TargetName,
						VolumeName:   usage.VolumeName,
						Type:         "soft_limit",
						UsedBytes:    usage.UsedBytes,
						LimitBytes:   usage.SoftLimit,
						UsagePercent: usage.UsagePercent,
						ViolatedAt:   time.Now(),
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, Success(violations))
}

// ========== 辅助功能 ==========

// Adjustment 配额调整记录
type Adjustment struct {
	QuotaID    string    `json:"quotaId"`
	HardDelta  int64     `json:"hardDelta"`
	SoftDelta  int64     `json:"softDelta"`
	Reason     string    `json:"reason"`
	AdjustedAt time.Time `json:"adjustedAt"`
}

// GracePeriodManager 宽限期管理器
type GracePeriodManager struct {
	gracePeriods map[string]*GracePeriodInfo
	mu           sync.RWMutex
}

// GracePeriodInfo 宽限期信息
type GracePeriodInfo struct {
	QuotaID   string        `json:"quotaId"`
	Duration  time.Duration `json:"duration"`
	Expiry    *time.Time    `json:"expiry,omitempty"`
	StartedAt time.Time     `json:"startedAt"`
}

// NewGracePeriodManager 创建宽限期管理器
func NewGracePeriodManager() *GracePeriodManager {
	return &GracePeriodManager{
		gracePeriods: make(map[string]*GracePeriodInfo),
	}
}

// SetGracePeriod 设置宽限期
func (g *GracePeriodManager) SetGracePeriod(quotaID string, duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.gracePeriods[quotaID] = &GracePeriodInfo{
		QuotaID:   quotaID,
		Duration:  duration,
		StartedAt: time.Now(),
	}
}

// ExtendGracePeriod 延长宽限期
func (g *GracePeriodManager) ExtendGracePeriod(quotaID string, expiry time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	info, exists := g.gracePeriods[quotaID]
	if !exists {
		info = &GracePeriodInfo{
			QuotaID:   quotaID,
			StartedAt: time.Now(),
		}
		g.gracePeriods[quotaID] = info
	}
	info.Expiry = &expiry
}

// GetGracePeriodInfo 获取宽限期信息
func (g *GracePeriodManager) GetGracePeriodInfo(quotaID string) (time.Duration, *time.Time) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	info, exists := g.gracePeriods[quotaID]
	if !exists {
		return 0, nil
	}
	return info.Duration, info.Expiry
}
