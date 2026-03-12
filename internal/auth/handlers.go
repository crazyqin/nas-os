package auth

import (
	"net/http"

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
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	mfa := api.Group("/mfa")
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

// ========== 通用响应 ==========

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== 状态查询 ==========

func (h *Handlers) getStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	status := h.manager.GetStatus(userID)
	c.JSON(http.StatusOK, Success(status))
}

// ========== TOTP ==========

type TOTPSetupResponse struct {
	Secret      string `json:"secret"`
	URI         string `json:"uri"`
	QRCode      string `json:"qr_code"` // base64 PNG
	Issuer      string `json:"issuer"`
	AccountName string `json:"account_name"`
}

type EnableTOTPRequest struct {
	Code string `json:"code" binding:"required"`
}

func (h *Handlers) setupTOTP(c *gin.Context) {
	userID := c.GetString("user_id")
	username := c.GetString("username")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	setup, err := h.manager.SetupTOTP(userID, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(TOTPSetupResponse{
		Secret:      setup.Secret,
		URI:         setup.URI,
		QRCode:      setup.QRCode,
		Issuer:      setup.Issuer,
		AccountName: setup.AccountName,
	}))
}

func (h *Handlers) enableTOTP(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var req EnableTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.EnableTOTP(userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) disableTOTP(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var req EnableTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.DisableTOTP(userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
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
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var req SendSMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.SendSMSCode(userID, req.Phone); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) enableSMS(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var req EnableSMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.EnableSMS(userID, req.Phone, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) disableSMS(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var req EnableTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.DisableSMS(userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// ========== 备份码 ==========

func (h *Handlers) generateBackupCodes(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	codes, err := h.manager.GenerateBackupCodes(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(BackupCodesResponse{
		Codes: codes,
	}))
}

func (h *Handlers) getBackupStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	status := h.manager.GetStatus(userID)
	c.JSON(http.StatusOK, Success(gin.H{
		"backup_codes_count": status.BackupCodesCount,
	}))
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
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var req WebAuthnRegisterStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = username
	}

	sessionID, options, err := h.manager.BeginWebAuthnRegistration(userID, username, displayName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(WebAuthnRegisterStartResponse{
		SessionID: sessionID,
		Options:   options,
	}))
}

func (h *Handlers) finishWebAuthnRegistration(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var responseData interface{}
	if err := c.ShouldBindJSON(&responseData); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 需要从请求中获取 sessionID
	sessionID := c.GetHeader("X-WebAuthn-Session-ID")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, Error(400, "缺少会话 ID"))
		return
	}

	if err := h.manager.FinishWebAuthnRegistration(sessionID, responseData); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) beginWebAuthnAuthentication(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	sessionID, options, err := h.manager.BeginWebAuthnAuthentication(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(WebAuthnRegisterStartResponse{
		SessionID: sessionID,
		Options:   options,
	}))
}

func (h *Handlers) finishWebAuthnAuthentication(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	sessionID := c.GetHeader("X-WebAuthn-Session-ID")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, Error(400, "缺少会话 ID"))
		return
	}

	var responseData interface{}
	if err := c.ShouldBindJSON(&responseData); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	resultUserID, err := h.manager.FinishWebAuthnAuthentication(sessionID, responseData)
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(gin.H{
		"user_id": resultUserID,
	}))
}

func (h *Handlers) getWebAuthnCredentials(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	credentials := h.manager.GetWebAuthnCredentials(userID)
	c.JSON(http.StatusOK, Success(credentials))
}

func (h *Handlers) removeWebAuthnCredential(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	credentialID := c.Param("id")
	if err := h.manager.RemoveWebAuthnCredential(userID, credentialID); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
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
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
		return
	}

	var req VerifyMFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.VerifyMFA(userID, req.MFAType, req.Code, req.ResponseData); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}
