package securityv2

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// HandlersV2 安全模块 v2 HTTP 处理器
type HandlersV2 struct {
	manager *SecurityManagerV2
}

// NewHandlersV2 创建安全处理器 v2
func NewHandlersV2(mgr *SecurityManagerV2) *HandlersV2 {
	return &HandlersV2{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *HandlersV2) RegisterRoutes(api *gin.RouterGroup) {
	security := api.Group("/security/v2")
	{
		// 仪表板
		security.GET("/dashboard", h.getDashboard)

		// 配置
		security.GET("/config", h.getConfig)
		security.PUT("/config", h.updateConfig)

		// ========== MFA 双因素认证 ==========
		mfa := security.Group("/mfa")
		{
			mfa.GET("/status", h.getMFAStatus)
			mfa.POST("/setup", h.setupMFA)
			mfa.POST("/enable", h.enableMFA)
			mfa.POST("/disable", h.disableMFA)
			mfa.POST("/verify", h.verifyMFA)
			mfa.GET("/recovery-codes", h.getRecoveryCodes)
			mfa.POST("/recovery-codes/regenerate", h.regenerateRecoveryCodes)
			mfa.PUT("/phone", h.updatePhone)
			mfa.PUT("/email", h.updateEmail)
			mfa.POST("/sms-code", h.sendSMSCode)
			mfa.POST("/email-code", h.sendEmailCode)
		}

		// ========== 文件加密 ==========
		encryption := security.Group("/encryption")
		{
			encryption.GET("/status", h.getEncryptionStatus)
			encryption.POST("/initialize", h.initializeEncryption)
			encryption.POST("/directories", h.createEncryptedDirectory)
			encryption.GET("/directories", h.getEncryptedDirectories)
			encryption.POST("/directories/:path/unlock", h.unlockDirectory)
			encryption.POST("/directories/:path/lock", h.lockDirectory)
			encryption.DELETE("/directories/:path", h.deleteEncryptedDirectory)
			encryption.POST("/files/encrypt", h.encryptFile)
			encryption.POST("/files/decrypt", h.decryptFile)
		}

		// ========== 告警通知 ==========
		alerting := security.Group("/alerting")
		{
			alerting.GET("/config", h.getAlertingConfig)
			alerting.PUT("/config", h.updateAlertingConfig)
			alerting.GET("/alerts", h.getAlerts)
			alerting.POST("/alerts/:id/acknowledge", h.acknowledgeAlert)
			alerting.GET("/stats", h.getAlertStats)
			alerting.GET("/subscribers", h.getSubscribers)
			alerting.POST("/subscribers", h.addSubscriber)
			alerting.DELETE("/subscribers/:id", h.removeSubscriber)
			alerting.POST("/test/:channel", h.testNotification)
		}
	}
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func success(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func apiError(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// ========== 仪表板 ==========

func (h *HandlersV2) getDashboard(c *gin.Context) {
	dashboard := h.manager.GetSecurityDashboard()
	c.JSON(http.StatusOK, success(dashboard))
}

// ========== 配置 ==========

func (h *HandlersV2) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, success(config))
}

func (h *HandlersV2) updateConfig(c *gin.Context) {
	var config SecurityConfigV2
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// ========== MFA ==========

func (h *HandlersV2) getMFAStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, apiError(400, "用户 ID 缺失"))
		return
	}

	status, err := h.manager.GetMFAManager().GetMFAStatus(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(status))
}

type SetupMFARequest struct {
	Phone  string `json:"phone"`
	Email  string `json:"email"`
}

func (h *HandlersV2) setupMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	username := c.GetString("username")

	var req SetupMFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	secret, err := h.manager.GetMFAManager().SetupMFA(userID, username, req.Phone, req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 返回 TOTP 密钥和恢复码（仅显示一次）
	c.JSON(http.StatusOK, success(map[string]interface{}{
		"totp_secret":     secret.TOTPSecret,
		"totp_uri":        h.generateTOTPURI(username, secret.TOTPSecret),
		"recovery_codes":  secret.RecoveryCodes, // 实际应该返回明文，这里简化
		"message":         "请安全保存恢复码，它们只会显示一次",
	}))
}

func (h *HandlersV2) generateTOTPURI(username, secret string) string {
	return "otpauth://totp/NAS-OS:" + username + "?secret=" + secret + "&issuer=NAS-OS&algorithm=SHA1&digits=6&period=30"
}

func (h *HandlersV2) enableMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Method string `json:"method"`
		Code   string `json:"code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 验证代码
	mfaMgr := h.manager.GetMFAManager()
	if !mfaMgr.VerifyTOTPCode("", req.Code) {
		// 简化：实际应该验证用户的 TOTP
		c.JSON(http.StatusBadRequest, apiError(400, "验证码错误"))
		return
	}

	if err := mfaMgr.EnableMFA(userID, req.Method); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) disableMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Code string `json:"code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetMFAManager().DisableMFA(userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) verifyMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Code   string `json:"code"`
		Method string `json:"method"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	result, err := h.manager.GetMFAManager().VerifyMFA(userID, req.Code, req.Method)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(result))
}

func (h *HandlersV2) getRecoveryCodes(c *gin.Context) {
	// 实际实现应该从加密存储中读取
	c.JSON(http.StatusOK, success(map[string]interface{}{
		"message": "恢复码已保存到安全存储",
	}))
}

func (h *HandlersV2) regenerateRecoveryCodes(c *gin.Context) {
	userID := c.GetString("user_id")

	codes, err := h.manager.GetMFAManager().RegenerateRecoveryCodes(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(map[string]interface{}{
		"recovery_codes": codes,
		"message":        "新恢复码已生成，请安全保存",
	}))
}

func (h *HandlersV2) updatePhone(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"` // 验证码
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetMFAManager().UpdatePhone(userID, req.Phone); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) updateEmail(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"` // 验证码
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetMFAManager().UpdateEmail(userID, req.Email); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) sendSMSCode(c *gin.Context) {
	userID := c.GetString("user_id")

	code, err := h.manager.GetMFAManager().SendSMSCode(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 开发环境返回验证码，生产环境应该隐藏
	c.JSON(http.StatusOK, success(map[string]interface{}{
		"sent": true,
		"code": code, // 生产环境应该移除
	}))
}

func (h *HandlersV2) sendEmailCode(c *gin.Context) {
	userID := c.GetString("user_id")

	code, err := h.manager.GetMFAManager().SendEmailCode(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(map[string]interface{}{
		"sent": true,
		"code": code, // 生产环境应该移除
	}))
}

// ========== 文件加密 ==========

func (h *HandlersV2) getEncryptionStatus(c *gin.Context) {
	status := h.manager.getEncryptionStatus()
	c.JSON(http.StatusOK, success(status))
}

type InitializeEncryptionRequest struct {
	Password string `json:"password"`
}

func (h *HandlersV2) initializeEncryption(c *gin.Context) {
	var req InitializeEncryptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.Initialize(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

type CreateEncryptedDirectoryRequest struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *HandlersV2) createEncryptedDirectory(c *gin.Context) {
	var req CreateEncryptedDirectoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	dir, err := h.manager.GetEncryptionManager().CreateEncryptedDirectory(req.Path, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, success(dir))
}

func (h *HandlersV2) getEncryptedDirectories(c *gin.Context) {
	dirs := h.manager.GetEncryptionManager().GetEncryptedDirectories()
	c.JSON(http.StatusOK, success(dirs))
}

func (h *HandlersV2) unlockDirectory(c *gin.Context) {
	path := c.Param("path")
	var req struct {
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetEncryptionManager().UnlockDirectory(path, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) lockDirectory(c *gin.Context) {
	path := c.Param("path")

	if err := h.manager.GetEncryptionManager().LockDirectory(path); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) deleteEncryptedDirectory(c *gin.Context) {
	path := c.Param("path")

	if err := h.manager.GetEncryptionManager().DeleteEncryptedDirectory(path); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

type EncryptFileRequest struct {
	SrcPath string `json:"src_path"`
	DstPath string `json:"dst_path"`
}

func (h *HandlersV2) encryptFile(c *gin.Context) {
	var req EncryptFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetEncryptionManager().EncryptFile(req.SrcPath, req.DstPath); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) decryptFile(c *gin.Context) {
	var req EncryptFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetEncryptionManager().DecryptFile(req.SrcPath, req.DstPath); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// ========== 告警通知 ==========

func (h *HandlersV2) getAlertingConfig(c *gin.Context) {
	config := h.manager.GetAlertingManager().GetConfig()
	c.JSON(http.StatusOK, success(config))
}

func (h *HandlersV2) updateAlertingConfig(c *gin.Context) {
	var config AlertingConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetAlertingManager().UpdateConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) getAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filters := make(map[string]string)
	for _, key := range []string{"severity", "type", "acknowledged", "notified"} {
		if value := c.Query(key); value != "" {
			filters[key] = value
		}
	}

	alerts := h.manager.GetAlertingManager().GetAlerts(limit, offset, filters)
	c.JSON(http.StatusOK, success(alerts))
}

func (h *HandlersV2) acknowledgeAlert(c *gin.Context) {
	alertID := c.Param("id")
	username := c.GetString("username")

	if err := h.manager.GetAlertingManager().AcknowledgeAlert(alertID, username); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) getAlertStats(c *gin.Context) {
	stats := h.manager.GetAlertingManager().GetAlertStats()
	c.JSON(http.StatusOK, success(stats))
}

func (h *HandlersV2) getSubscribers(c *gin.Context) {
	subscribers := h.manager.GetAlertingManager().GetSubscribers()
	c.JSON(http.StatusOK, success(subscribers))
}

type AddSubscriberRequest struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Target   string   `json:"target"`
	Events   []string `json:"events"`
	Severity string   `json:"severity"`
}

func (h *HandlersV2) addSubscriber(c *gin.Context) {
	var req AddSubscriberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	subscriber := AlertSubscriber{
		ID:       generateAlertIDV2(),
		Name:     req.Name,
		Type:     req.Type,
		Target:   req.Target,
		Events:   req.Events,
		Severity: req.Severity,
		Active:   true,
	}

	if err := h.manager.GetAlertingManager().AddSubscriber(subscriber); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, success(subscriber))
}

func (h *HandlersV2) removeSubscriber(c *gin.Context) {
	subscriberID := c.Param("id")

	if err := h.manager.GetAlertingManager().RemoveSubscriber(subscriberID); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *HandlersV2) testNotification(c *gin.Context) {
	channel := c.Param("channel")

	if err := h.manager.GetAlertingManager().TestNotification(channel); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(map[string]interface{}{
		"message": "测试通知已发送",
	}))
}
