package auth

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers MFA HTTP 处理器
type Handlers struct {
	manager *MFAManager
}

// NewHandlers 创建 MFA 处理器
func NewHandlers(mgr *MFAManager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
// 注意：MFA 操作需要认证，调用方应在应用此路由组前添加认证中间件
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	mfa := apiGroup.Group("/mfa")
	// 所有 MFA 操作都需要认证
	{
		// 状态查询
		mfa.GET("/status", h.getStatus)

		// ========== TOTP ==========
		mfa.POST("/totp/setup", h.setupTOTP)
		mfa.POST("/totp/enable", h.enableTOTP)
		mfa.POST("/totp/disable", h.disableTOTP)

		// ========== 短信验证码 ==========
		mfa.POST("/sms/send", h.sendSMS)
		mfa.POST("/sms/enable", h.enableSMS)
		mfa.POST("/sms/disable", h.disableSMS)

		// ========== 备份码 ==========
		mfa.POST("/backup/generate", h.generateBackupCodes)
		mfa.GET("/backup/status", h.getBackupStatus)

		// ========== WebAuthn ==========
		mfa.POST("/webauthn/register/start", h.beginWebAuthnRegistration)
		mfa.POST("/webauthn/register/finish", h.finishWebAuthnRegistration)
		mfa.POST("/webauthn/authenticate/start", h.beginWebAuthnAuthentication)
		mfa.POST("/webauthn/authenticate/finish", h.finishWebAuthnAuthentication)
		mfa.GET("/webauthn/credentials", h.getWebAuthnCredentials)
		mfa.DELETE("/webauthn/credentials/:id", h.removeWebAuthnCredential)

		// ========== MFA 验证 ==========
		mfa.POST("/verify", h.verifyMFA)
	}
}

// ========== 状态查询 ==========

// getStatus 获取 MFA 状态
// @Summary 获取 MFA 状态
// @Description 获取当前用户的 MFA 配置状态
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=MFAStatus}
// @Failure 401 {object} api.Response
// @Router /mfa/status [get]
// @Security BearerAuth
func (h *Handlers) getStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	status := h.manager.GetStatus(userID)
	api.OK(c, status)
}

// ========== TOTP ==========

// TOTPSetupResponse TOTP 设置响应
type TOTPSetupResponse struct {
	Secret      string `json:"secret"`
	URI         string `json:"uri"`
	QRCode      string `json:"qr_code"` // base64 PNG
	Issuer      string `json:"issuer"`
	AccountName string `json:"account_name"`
}

// EnableTOTPRequest 启用 TOTP 请求
type EnableTOTPRequest struct {
	Code string `json:"code" binding:"required"`
}

// setupTOTP 设置 TOTP
// @Summary 设置 TOTP
// @Description 为当前用户设置 TOTP 二步验证，返回密钥和二维码
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=TOTPSetupResponse}
// @Failure 400 {object} api.Response
// @Failure 401 {object} api.Response
// @Router /mfa/totp/setup [post]
// @Security BearerAuth
func (h *Handlers) setupTOTP(c *gin.Context) {
	userID := c.GetString("user_id")
	username := c.GetString("username")

	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	setup, err := h.manager.SetupTOTP(userID, username)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, TOTPSetupResponse{
		Secret:      setup.Secret,
		URI:         setup.URI,
		QRCode:      setup.QRCode,
		Issuer:      setup.Issuer,
		AccountName: setup.AccountName,
	})
}

// enableTOTP 启用 TOTP
// @Summary 启用 TOTP
// @Description 使用验证码启用 TOTP 二步验证
// @Tags auth
// @Accept json
// @Produce json
// @Param request body EnableTOTPRequest true "验证码"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 401 {object} api.Response
// @Router /mfa/totp/enable [post]
// @Security BearerAuth
func (h *Handlers) enableTOTP(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var req EnableTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.EnableTOTP(userID, req.Code); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

func (h *Handlers) disableTOTP(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var req EnableTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.DisableTOTP(userID, req.Code); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

// ========== 短信验证码 ==========

type SendSMSRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type EnableSMSRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func (h *Handlers) sendSMS(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var req SendSMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.SendSMSCode(userID, req.Phone); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

func (h *Handlers) enableSMS(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var req EnableSMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.EnableSMS(userID, req.Phone, req.Code); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

func (h *Handlers) disableSMS(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var req EnableTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.DisableSMS(userID, req.Code); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

// ========== 备份码 ==========

func (h *Handlers) generateBackupCodes(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	codes, err := h.manager.GenerateBackupCodes(userID)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, BackupCodesResponse{
		Codes: codes,
	})
}

func (h *Handlers) getBackupStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	status := h.manager.GetStatus(userID)
	api.OK(c, gin.H{
		"backup_codes_count": status.BackupCodesCount,
	})
}

// ========== WebAuthn ==========

type WebAuthnRegisterStartRequest struct {
	DisplayName string `json:"display_name"`
}

type WebAuthnRegisterStartResponse struct {
	SessionID string      `json:"session_id"`
	Options   interface{} `json:"options"`
}

func (h *Handlers) beginWebAuthnRegistration(c *gin.Context) {
	userID := c.GetString("user_id")
	username := c.GetString("username")

	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var req WebAuthnRegisterStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = username
	}

	sessionID, options, err := h.manager.BeginWebAuthnRegistration(userID, username, displayName)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, WebAuthnRegisterStartResponse{
		SessionID: sessionID,
		Options:   options,
	})
}

func (h *Handlers) finishWebAuthnRegistration(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var responseData interface{}
	if err := c.ShouldBindJSON(&responseData); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 需要从请求中获取 sessionID
	sessionID := c.GetHeader("X-WebAuthn-Session-ID")
	if sessionID == "" {
		api.BadRequest(c, "缺少会话 ID")
		return
	}

	if err := h.manager.FinishWebAuthnRegistration(sessionID, responseData); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

func (h *Handlers) beginWebAuthnAuthentication(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	sessionID, options, err := h.manager.BeginWebAuthnAuthentication(userID)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, WebAuthnRegisterStartResponse{
		SessionID: sessionID,
		Options:   options,
	})
}

func (h *Handlers) finishWebAuthnAuthentication(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	sessionID := c.GetHeader("X-WebAuthn-Session-ID")
	if sessionID == "" {
		api.BadRequest(c, "缺少会话 ID")
		return
	}

	var responseData interface{}
	if err := c.ShouldBindJSON(&responseData); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	resultUserID, err := h.manager.FinishWebAuthnAuthentication(sessionID, responseData)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"user_id": resultUserID,
	})
}

func (h *Handlers) getWebAuthnCredentials(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	credentials := h.manager.GetWebAuthnCredentials(userID)
	api.OK(c, credentials)
}

func (h *Handlers) removeWebAuthnCredential(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	credentialID := c.Param("id")
	if err := h.manager.RemoveWebAuthnCredential(userID, credentialID); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}

// ========== MFA 验证 ==========

type VerifyMFARequest struct {
	MFAType      string      `json:"mfa_type" binding:"required"` // totp, sms, webauthn
	Code         string      `json:"code"`                        // TOTP 或短信验证码
	ResponseData interface{} `json:"response_data"`               // WebAuthn 响应数据
}

func (h *Handlers) verifyMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		api.Unauthorized(c, "未授权")
		return
	}

	var req VerifyMFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.VerifyMFA(userID, req.MFAType, req.Code, req.ResponseData); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, nil)
}
