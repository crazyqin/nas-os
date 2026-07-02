package privacyshield

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册隐私保护盾的 HTTP 路由.
func RegisterRoutes(r *gin.RouterGroup) {
	shield := NewShield()
	h := &Handler{shield: shield}

	privacyGroup := r.Group("/privacyshield")
	{
		// 扫描相关
		privacyGroup.POST("/scan", h.Scan)
		privacyGroup.POST("/scan/file", h.ScanFile)

		// 脱敏相关
		privacyGroup.POST("/mask", h.Mask)
		privacyGroup.POST("/mask/batch", h.MaskBatch)

		// 合规检查
		privacyGroup.POST("/compliance", h.ComplianceCheck)

		// 风险评估
		privacyGroup.POST("/risk", h.RiskAssessment)

		// 模式管理
		privacyGroup.GET("/patterns", h.GetPatterns)
		privacyGroup.POST("/patterns", h.AddPattern)
		privacyGroup.DELETE("/patterns/:name", h.RemovePattern)

		// 健康检查
		privacyGroup.GET("/health", h.Health)
	}
}

// Handler HTTP API 处理器.
type Handler struct {
	shield *Shield
}

// Scan 扫描内容中的敏感数据
// @Summary 扫描内容
// @Description 扫描文本内容中的敏感数据
// @Tags privacy
// @Accept json
// @Produce json
// @Param request body ScanRequest true "扫描请求"
// @Success 200 {object} ScanResult
// @Router /privacyshield/scan [post].
func (h *Handler) Scan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容不能为空"})
		return
	}

	result, err := h.shield.ScanContent(req.Content, req.Categories...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描失败: " + err.Error()})
		return
	}

	result.FilePath = req.FilePath
	c.JSON(http.StatusOK, result)
}

// ScanFile 扫描文件内容
// @Summary 扫描文件
// @Description 扫描文件内容中的敏感数据
// @Tags privacy
// @Accept json
// @Produce json
// @Param request body ScanRequest true "扫描请求（包含文件路径）"
// @Success 200 {object} ScanResult
// @Router /privacyshield/scan/file [post].
func (h *Handler) ScanFile(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件路径不能为空"})
		return
	}

	result, err := h.shield.ScanContent(req.Content, req.Categories...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描失败: " + err.Error()})
		return
	}

	result.FilePath = req.FilePath
	c.JSON(http.StatusOK, result)
}

// Mask 对内容进行脱敏处理
// @Summary 脱敏处理
// @Description 对文本内容进行脱敏处理
// @Tags privacy
// @Accept json
// @Produce json
// @Param request body MaskRequest true "脱敏请求"
// @Success 200 {object} MaskResponse
// @Router /privacyshield/mask [post].
func (h *Handler) Mask(c *gin.Context) {
	var req MaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容不能为空"})
		return
	}

	response, err := h.shield.MaskContent(req.Content, req.Strategy, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "脱敏失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// MaskBatch 批量脱敏处理
// @Summary 批量脱敏
// @Description 批量对多个内容进行脱敏处理
// @Tags privacy
// @Accept json
// @Produce json
// @Param request body []MaskRequest true "批量脱敏请求"
// @Success 200 {object} []MaskResponse
// @Router /privacyshield/mask/batch [post].
func (h *Handler) MaskBatch(c *gin.Context) {
	var requests []MaskRequest
	if err := c.ShouldBindJSON(&requests); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if len(requests) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求列表不能为空"})
		return
	}

	responses := make([]MaskResponse, 0, len(requests))
	for _, req := range requests {
		if req.Content == "" {
			continue
		}
		response, err := h.shield.MaskContent(req.Content, req.Strategy, req.Options)
		if err != nil {
			responses = append(responses, MaskResponse{
				Original: req.Content,
				Masked:   "ERROR: " + err.Error(),
				Strategy: req.Strategy,
			})
			continue
		}
		responses = append(responses, *response)
	}

	c.JSON(http.StatusOK, responses)
}

// ComplianceCheck 合规检查
// @Summary 合规检查
// @Description 对内容进行合规性检查
// @Tags privacy
// @Accept json
// @Produce json
// @Param request body ComplianceRequest true "合规检查请求"
// @Success 200 {object} ComplianceReport
// @Router /privacyshield/compliance [post].
func (h *Handler) ComplianceCheck(c *gin.Context) {
	var req ComplianceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容不能为空"})
		return
	}

	if req.Framework == "" {
		req.Framework = "GDPR"
	}

	report, err := h.shield.GenerateComplianceReport(req.Content, req.Framework)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "合规检查失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// RiskAssessment 风险评估
// @Summary 风险评估
// @Description 对内容进行风险评估
// @Tags privacy
// @Accept json
// @Produce json
// @Param request body RiskAssessmentRequest true "风险评估请求"
// @Success 200 {object} RiskScore
// @Router /privacyshield/risk [post].
func (h *Handler) RiskAssessment(c *gin.Context) {
	var req RiskAssessmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容不能为空"})
		return
	}

	riskScore, err := h.shield.AssessRisk(req.Content, req.Encrypted, req.AccessLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "风险评估失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, riskScore)
}

// GetPatterns 获取所有敏感数据模式
// @Summary 获取模式列表
// @Description 获取所有敏感数据模式
// @Tags privacy
// @Produce json
// @Success 200 {array} SensitivePattern
// @Router /privacyshield/patterns [get].
func (h *Handler) GetPatterns(c *gin.Context) {
	patterns := h.shield.GetPatterns()
	c.JSON(http.StatusOK, patterns)
}

// AddPattern 添加敏感数据模式
// @Summary 添加模式
// @Description 添加新的敏感数据模式
// @Tags privacy
// @Accept json
// @Produce json
// @Param request body SensitivePattern true "模式定义"
// @Success 200 {object} map[string]string
// @Router /privacyshield/patterns [post].
func (h *Handler) AddPattern(c *gin.Context) {
	var pattern SensitivePattern
	if err := c.ShouldBindJSON(&pattern); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if pattern.Name == "" || pattern.Pattern == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称和模式不能为空"})
		return
	}

	h.shield.AddPattern(pattern)
	c.JSON(http.StatusOK, gin.H{"message": "模式添加成功", "name": pattern.Name})
}

// RemovePattern 删除敏感数据模式
// @Summary 删除模式
// @Description 根据名称删除敏感数据模式
// @Tags privacy
// @Produce json
// @Param name path string true "模式名称"
// @Success 200 {object} map[string]string
// @Router /privacyshield/patterns/{name} [delete].
func (h *Handler) RemovePattern(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模式名称不能为空"})
		return
	}

	if h.shield.RemovePattern(name) {
		c.JSON(http.StatusOK, gin.H{"message": "模式删除成功", "name": name})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到指定模式", "name": name})
	}
}

// Health 健康检查
// @Summary 健康检查
// @Description 检查隐私保护盾服务状态
// @Tags privacy
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /privacyshield/health [get].
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":   "healthy",
		"service":  "privacyshield",
		"patterns": len(h.shield.GetPatterns()),
	})
}
