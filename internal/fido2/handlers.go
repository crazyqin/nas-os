// Package fido2 提供 FIDO2/WebAuthn API 处理器
package fido2

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers FIDO2 API 处理器
type Handlers struct {
	credManager    *CredentialManager
	recoveryManager *RecoveryCodeManager
	authenticator  *Authenticator
	sessions       map[string]*Session
	pendingRegs    map[string]*pendingRegistration
	pendingAuths   map[string]*pendingAuthentication
	config         *Config
}

// NewHandlers 创建处理器
func NewHandlers(
	credManager *CredentialManager,
	recoveryManager *RecoveryCodeManager,
	config *Config,
) *Handlers {
	if config == nil {
		config = DefaultConfig()
	}

	authenticator := NewAuthenticator(config)

	if credManager == nil {
		credManager = NewCredentialManager(nil, authenticator, config)
	}
	if recoveryManager == nil {
		recoveryManager = NewRecoveryCodeManager(nil, authenticator, config)
	}

	return &Handlers{
		credManager:    credManager,
		recoveryManager: recoveryManager,
		authenticator:  authenticator,
		sessions:       make(map[string]*Session),
		pendingRegs:    make(map[string]*pendingRegistration),
		pendingAuths:   make(map[string]*pendingAuthentication),
		config:         config,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	fido2 := r.Group("/fido2")
	{
		// 注册流程
		fido2.POST("/register/begin", h.beginRegistration)
		fido2.POST("/register/finish", h.finishRegistration)

		// 登录流程
		fido2.POST("/login/begin", h.beginLogin)
		fido2.POST("/login/finish", h.finishLogin)

		// 凭据管理
		fido2.GET("/credentials", h.listCredentials)
		fido2.DELETE("/credentials/:id", h.deleteCredential)

		// 恢复码
		fido2.POST("/recovery/generate", h.generateRecoveryCodes)
		fido2.POST("/recovery/verify", h.verifyRecoveryCode)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ==================== 注册流程 ====================

// beginRegistrationRequest 开始注册请求
type beginRegistrationRequest struct {
	UserID      string `json:"user_id" binding:"required"`      // 用户 ID
	UserName    string `json:"user_name" binding:"required"`    // 用户名
	DisplayName string `json:"display_name" binding:"required"` // 显示名称
}

// beginRegistration 开始注册
func (h *Handlers) beginRegistration(c *gin.Context) {
	var req beginRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	// 获取用户已有的凭据（用于排除）
	existingCredsPtr, err := h.credManager.GetActiveUserCredentials(req.UserID)
	var existingCreds []Credential
	if err == nil {
		for _, c := range existingCredsPtr {
			existingCreds = append(existingCreds, *c)
		}
	}

	// 创建注册挑战
	challenge, err := h.authenticator.CreateRegistrationChallenge(
		req.UserID,
		req.UserName,
		req.DisplayName,
		existingCreds,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: "创建注册挑战失败: " + err.Error(),
		})
		return
	}

	// 保存待处理的注册请求
	h.pendingRegs[challenge.Challenge] = &pendingRegistration{
		Challenge: challenge.Challenge,
		UserID:    req.UserID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(h.config.Timeout) * time.Millisecond),
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "注册挑战已创建",
		Data:    challenge,
	})
}

// finishRegistrationRequest 完成注册请求
type finishRegistrationRequest struct {
	UserID   string              `json:"user_id" binding:"required"`   // 用户 ID
	Name     string              `json:"name" binding:"required"`      // 凭据名称
	Response RegistrationResponse `json:"response" binding:"required"` // 注册响应
}

// finishRegistration 完成注册
func (h *Handlers) finishRegistration(c *gin.Context) {
	var req finishRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	// 从客户端数据中提取挑战值
	clientData, err := h.authenticator.ParseClientDataJSON(req.Response.Response.ClientDataJSON)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "解析客户端数据失败: " + err.Error(),
		})
		return
	}

	// 验证待处理的注册请求
	pending, exists := h.pendingRegs[clientData.Challenge]
	if !exists {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "未找到待处理的注册请求",
		})
		return
	}

	// 检查是否过期
	if time.Now().After(pending.ExpiresAt) {
		delete(h.pendingRegs, clientData.Challenge)
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "注册挑战已过期",
		})
		return
	}

	// 验证用户 ID
	if pending.UserID != req.UserID {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "用户 ID 不匹配",
		})
		return
	}

	// 注册凭据
	cred, err := h.credManager.RegisterCredential(
		req.UserID,
		req.Name,
		&req.Response,
		clientData.Challenge,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: "注册凭据失败: " + err.Error(),
		})
		return
	}

	// 清除待处理的注册请求
	delete(h.pendingRegs, clientData.Challenge)

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "凭据注册成功",
		Data: CredentialInfo{
			ID:            cred.ID,
			Name:          cred.Name,
			Authenticator: cred.Authenticator,
			Transports:    cred.Transports,
			CreatedAt:     cred.CreatedAt,
			LastUsedAt:    cred.LastUsedAt,
			UsageCount:    cred.UsageCount,
			Revoked:       cred.Revoked,
		},
	})
}

// ==================== 登录流程 ====================

// beginLoginRequest 开始登录请求
type beginLoginRequest struct {
	UserID string `json:"user_id" binding:"required"` // 用户 ID
}

// beginLogin 开始登录
func (h *Handlers) beginLogin(c *gin.Context) {
	var req beginLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	// 获取用户的活跃凭据
	credsPtr, err := h.credManager.GetActiveUserCredentials(req.UserID)
	if err != nil || len(credsPtr) == 0 {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: "用户没有可用的凭据",
		})
		return
	}

	var creds []Credential
	for _, c := range credsPtr {
		creds = append(creds, *c)
	}

	// 创建认证挑战
	challenge, err := h.authenticator.CreateAuthenticationChallenge(creds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: "创建认证挑战失败: " + err.Error(),
		})
		return
	}

	// 保存待处理的认证请求
	h.pendingAuths[challenge.Challenge] = &pendingAuthentication{
		Challenge: challenge.Challenge,
		UserID:    req.UserID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(h.config.Timeout) * time.Millisecond),
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "认证挑战已创建",
		Data:    challenge,
	})
}

// finishLoginRequest 完成登录请求
type finishLoginRequest struct {
	UserID   string                `json:"user_id" binding:"required"`   // 用户 ID
	Response AuthenticationResponse `json:"response" binding:"required"` // 认证响应
}

// finishLogin 完成登录
func (h *Handlers) finishLogin(c *gin.Context) {
	var req finishLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	// 从客户端数据中提取挑战值
	clientData, err := h.authenticator.ParseClientDataJSON(req.Response.Response.ClientDataJSON)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "解析客户端数据失败: " + err.Error(),
		})
		return
	}

	// 验证待处理的认证请求
	pending, exists := h.pendingAuths[clientData.Challenge]
	if !exists {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "未找到待处理的认证请求",
		})
		return
	}

	// 检查是否过期
	if time.Now().After(pending.ExpiresAt) {
		delete(h.pendingAuths, clientData.Challenge)
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "认证挑战已过期",
		})
		return
	}

	// 验证用户 ID
	if pending.UserID != req.UserID {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "用户 ID 不匹配",
		})
		return
	}

	// 解码凭据 ID
	credIDBytes, err := base64URLEncodeDecode(req.Response.ID)
	if err != nil {
		credIDBytes, err = base64URLEncodeDecode(req.Response.RawID)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{
				Code:    http.StatusBadRequest,
				Message: "解码凭据 ID 失败",
			})
			return
		}
	}

	// 查找凭据
	cred, err := h.credManager.FindCredentialByWebAuthnID(credIDBytes)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: "凭据不存在",
		})
		return
	}

	// 验证凭据属于用户
	if cred.UserID != req.UserID {
		c.JSON(http.StatusForbidden, response{
			Code:    http.StatusForbidden,
			Message: "凭据不属于该用户",
		})
		return
	}

	// 验证认证响应
	session, err := h.authenticator.VerifyAuthentication(
		&req.Response,
		cred,
		clientData.Challenge,
	)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response{
			Code:    http.StatusUnauthorized,
			Message: "认证失败: " + err.Error(),
		})
		return
	}

	// 更新凭据使用信息
	if err := h.credManager.UpdateCredentialUsage(cred.ID, session.SignCount); err != nil {
		// 记录错误但不影响登录
		_ = err
	}

	// 设置会话信息
	session.UserID = req.UserID
	session.IPAddress = c.ClientIP()
	session.UserAgent = c.GetHeader("User-Agent")

	// 保存会话
	h.sessions[session.ID] = session

	// 清除待处理的认证请求
	delete(h.pendingAuths, clientData.Challenge)

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "认证成功",
		Data: map[string]interface{}{
			"session_id": session.ID,
			"expires_at": session.ExpiresAt,
			"user_id":    session.UserID,
		},
	})
}

// ==================== 凭据管理 ====================

// listCredentials 列出用户凭据
func (h *Handlers) listCredentials(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "缺少 user_id 参数",
		})
		return
	}

	infos, err := h.credManager.GetUserCredentialInfos(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: "获取凭据列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "获取凭据列表成功",
		Data:    infos,
	})
}

// deleteCredential 删除凭据
func (h *Handlers) deleteCredential(c *gin.Context) {
	credID := c.Param("id")
	if credID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "缺少凭据 ID",
		})
		return
	}

	// 获取凭据信息
	cred, err := h.credManager.GetCredential(credID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: "凭据不存在: " + err.Error(),
		})
		return
	}

	// 验证用户权限（可选：检查请求用户是否为凭据所有者）
	userID := c.Query("user_id")
	if userID != "" && cred.UserID != userID {
		c.JSON(http.StatusForbidden, response{
			Code:    http.StatusForbidden,
			Message: "无权删除该凭据",
		})
		return
	}

	// 删除凭据
	if err := h.credManager.DeleteCredential(credID); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: "删除凭据失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "凭据已删除",
	})
}

// ==================== 恢复码 ====================

// generateRecoveryCodesRequest 生成恢复码请求
type generateRecoveryCodesRequest struct {
	UserID string `json:"user_id" binding:"required"` // 用户 ID
	Count  int    `json:"count"`                       // 恢复码数量（默认 8）
}

// generateRecoveryCodes 生成恢复码
func (h *Handlers) generateRecoveryCodes(c *gin.Context) {
	var req generateRecoveryCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	// 默认生成 8 个恢复码
	if req.Count == 0 {
		req.Count = 8
	}

	codes, err := h.recoveryManager.GenerateRecoveryCodes(req.UserID, req.Count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: "生成恢复码失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "恢复码已生成",
		Data: map[string]interface{}{
			"codes": codes,
			"count": len(codes),
			"warning": "请妥善保管恢复码，每个恢复码只能使用一次",
		},
	})
}

// verifyRecoveryCodeRequest 验证恢复码请求
type verifyRecoveryCodeRequest struct {
	UserID string `json:"user_id" binding:"required"` // 用户 ID
	Code   string `json:"code" binding:"required"`    // 恢复码
}

// verifyRecoveryCode 验证恢复码
func (h *Handlers) verifyRecoveryCode(c *gin.Context) {
	var req verifyRecoveryCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	valid, err := h.recoveryManager.VerifyRecoveryCode(req.UserID, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: "验证恢复码失败: " + err.Error(),
		})
		return
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, response{
			Code:    http.StatusUnauthorized,
			Message: "恢复码无效或已使用",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "恢复码验证成功",
	})
}

// base64URLEncodeDecode Base64 URL 解码
func base64URLEncodeDecode(s string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(s)
}
