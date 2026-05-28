package adaptivetwofa

import (
	"nas-os/internal/api"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 自适应2FA HTTP处理器
type Handlers struct {
	manager *AdaptiveManager
}

// NewHandlers 创建处理器
func NewHandlers(manager *AdaptiveManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	adaptive := apiGroup.Group("/adaptive-2fa")
	{
		// 评估登录
		adaptive.POST("/evaluate", h.evaluateLogin)

		// 信任设备管理
		adaptive.POST("/trust-device", h.trustDevice)
		adaptive.GET("/trusted-devices", h.getTrustedDevices)
		adaptive.DELETE("/trusted-devices/:device_id", h.revokeTrust)
		adaptive.DELETE("/trusted-devices", h.revokeAllTrust)

		// 挑战验证
		adaptive.POST("/verify-challenge", h.verifyChallenge)

		// 统计信息
		adaptive.GET("/stats", h.getStats)

		// 配置
		adaptive.GET("/config", h.getConfig)
	}
}

// ========== 请求/响应类型 ==========

// EvaluateLoginRequest 评估登录请求
type EvaluateLoginRequest struct {
	UserID            string      `json:"user_id" binding:"required"`
	Username          string      `json:"username" binding:"required"`
	IP                string      `json:"ip" binding:"required"`
	UserAgent         string      `json:"user_agent"`
	DeviceFingerprint string      `json:"device_fingerprint"`
	GeoLocation       *GeoLocation `json:"geo_location,omitempty"`
	FingerprintExtra  map[string]string `json:"fingerprint_extra,omitempty"`
}

// EvaluateLoginResponse 评估登录响应
type EvaluateLoginResponse struct {
	Allowed           bool          `json:"allowed"`
	RiskScore         *RiskScore    `json:"risk_score"`
	Challenges        []AuthChallenge `json:"challenges,omitempty"`
	TrustDevicePrompt bool          `json:"trust_device_prompt"`
	Message           string        `json:"message,omitempty"`
}

// TrustDeviceRequest 信任设备请求
type TrustDeviceRequest struct {
	UserID            string       `json:"user_id" binding:"required"`
	DeviceFingerprint string       `json:"device_fingerprint" binding:"required"`
	IP                string       `json:"ip" binding:"required"`
	UserAgent         string       `json:"user_agent"`
	GeoLocation       *GeoLocation `json:"geo_location,omitempty"`
}

// VerifyChallengeRequest 验证挑战请求
type VerifyChallengeRequest struct {
	ChallengeID string `json:"challenge_id" binding:"required"`
}

// ========== 处理器方法 ==========

// evaluateLogin 评估登录
// @Summary 评估登录风险
// @Description 根据登录上下文评估风险，决定是否需要双因素认证
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Param request body EvaluateLoginRequest true "登录上下文"
// @Success 200 {object} api.Response{data=EvaluateLoginResponse}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /adaptive-2fa/evaluate [post]
func (h *Handlers) evaluateLogin(c *gin.Context) {
	var req EvaluateLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 构建登录上下文
	ctx := &LoginContext{
		UserID:            req.UserID,
		Username:          req.Username,
		IP:                req.IP,
		UserAgent:         req.UserAgent,
		DeviceFingerprint: req.DeviceFingerprint,
		Timestamp:         time.Now(),
		GeoLocation:       req.GeoLocation,
	}

	// 如果没有设备指纹，使用简单指纹
	if ctx.DeviceFingerprint == "" && req.UserAgent != "" {
		ctx.DeviceFingerprint = GenerateSimpleFingerprint(req.IP, req.UserAgent)
	}

	// 如果有额外指纹组件，生成更详细的指纹
	if req.FingerprintExtra != nil {
		components := GetFingerprintComponents(req.IP, req.UserAgent, req.FingerprintExtra)
		fp := h.manager.GetFingerprintGenerator().Generate(components)
		ctx.DeviceFingerprint = fp.Fingerprint
	}

	// 评估登录
	result := h.manager.EvaluateLogin(ctx)

	api.OK(c, EvaluateLoginResponse{
		Allowed:           result.Allowed,
		RiskScore:         result.RiskScore,
		Challenges:        result.Challenges,
		TrustDevicePrompt: result.TrustDevicePrompt,
		Message:           result.Message,
	})
}

// trustDevice 信任设备
// @Summary 信任设备
// @Description 将设备标记为信任设备，未来登录将跳过双因素认证
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Param request body TrustDeviceRequest true "设备信息"
// @Success 200 {object} api.Response{data=TrustedDevice}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /adaptive-2fa/trust-device [post]
func (h *Handlers) trustDevice(c *gin.Context) {
	var req TrustDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	device, err := h.manager.TrustDevice(
		req.UserID,
		req.DeviceFingerprint,
		req.IP,
		req.UserAgent,
		req.GeoLocation,
	)

	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, device)
}

// getTrustedDevices 获取信任设备列表
// @Summary 获取信任设备列表
// @Description 获取用户的所有信任设备
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Param user_id query string true "用户ID"
// @Success 200 {object} api.Response{data=[]TrustedDevice}
// @Failure 400 {object} api.Response
// @Router /adaptive-2fa/trusted-devices [get]
func (h *Handlers) getTrustedDevices(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		api.BadRequest(c, "user_id 参数必填")
		return
	}

	devices := h.manager.GetTrustedDevices(userID)
	api.OK(c, devices)
}

// revokeTrust 撤销设备信任
// @Summary 撤销设备信任
// @Description 撤销指定设备的信任状态
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Param user_id query string true "用户ID"
// @Param device_id path string true "设备ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /adaptive-2fa/trusted-devices/{device_id} [delete]
func (h *Handlers) revokeTrust(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		api.BadRequest(c, "user_id 参数必填")
		return
	}

	deviceID := c.Param("device_id")
	if deviceID == "" {
		api.BadRequest(c, "device_id 参数必填")
		return
	}

	if err := h.manager.RevokeTrust(userID, deviceID); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, nil)
}

// revokeAllTrust 撤销所有信任设备
// @Summary 撤销所有信任设备
// @Description 撤销用户的所有信任设备
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Param user_id query string true "用户ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /adaptive-2fa/trusted-devices [delete]
func (h *Handlers) revokeAllTrust(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		api.BadRequest(c, "user_id 参数必填")
		return
	}

	if err := h.manager.RevokeAllTrust(userID); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, nil)
}

// verifyChallenge 验证挑战
// @Summary 验证认证挑战
// @Description 验证并完成认证挑战
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Param request body VerifyChallengeRequest true "挑战ID"
// @Success 200 {object} api.Response{data=AuthChallenge}
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /adaptive-2fa/verify-challenge [post]
func (h *Handlers) verifyChallenge(c *gin.Context) {
	var req VerifyChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 验证挑战
	challenge, err := h.manager.VerifyChallenge(req.ChallengeID)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	// 完成挑战
	if err := h.manager.CompleteChallenge(req.ChallengeID); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, challenge)
}

// getStats 获取统计信息
// @Summary 获取自适应2FA统计信息
// @Description 获取自适应2FA模块的统计信息
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Success 200 {object} api.Response
// @Router /adaptive-2fa/stats [get]
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	api.OK(c, stats)
}

// getConfig 获取配置
// @Summary 获取自适应2FA配置
// @Description 获取当前自适应2FA配置
// @Tags adaptive-2fa
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=AdaptiveConfig}
// @Router /adaptive-2fa/config [get]
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	api.OK(c, config)
}
