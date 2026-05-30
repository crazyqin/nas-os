// Package quantumsafe 提供 REST API 处理器
package quantumsafe

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 量子安全加密 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	qs := r.Group("/quantumsafe")
	{
		// 密钥管理
		qs.GET("/keys", h.listKeys)
		qs.POST("/keys", h.generateKey)
		qs.GET("/keys/:id", h.getKey)
		qs.DELETE("/keys/:id", h.revokeKey)
		qs.POST("/keys/:id/rotate", h.rotateKey)

		// 加密操作
		qs.POST("/encrypt", h.encrypt)
		qs.POST("/decrypt", h.decrypt)

		// 签名操作
		qs.POST("/sign", h.sign)
		qs.POST("/verify", h.verify)

		// 加密器管理
		qs.GET("/ciphers", h.listCiphers)
		qs.POST("/ciphers", h.createCipher)
		qs.GET("/ciphers/:id", h.getCipher)

		// 迁移管理
		qs.GET("/migrations", h.listMigrations)
		qs.POST("/migrations", h.migrateKeys)
		qs.GET("/migrations/:id", h.getMigration)

		// 审计日志
		qs.GET("/audit-log", h.getAuditLog)

		// 统计信息
		qs.GET("/stats", h.getStats)

		// 配置
		qs.GET("/config", h.getConfig)
		qs.PUT("/config", h.updateConfig)

		// 算法信息
		qs.GET("/algorithms", h.listAlgorithms)
		qs.GET("/algorithms/:name", h.getAlgorithmInfo)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// generateKey 生成密钥
func (h *Handlers) generateKey(c *gin.Context) {
	var req struct {
		Name          string        `json:"name" binding:"required"`
		Algorithm     Algorithm     `json:"algorithm"`
		SecurityLevel SecurityLevel `json:"security_level"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	key, err := h.manager.GenerateKey(req.Name, req.Algorithm, req.SecurityLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "key generated",
		Data:    key,
	})
}

// listKeys 列出密钥
func (h *Handlers) listKeys(c *gin.Context) {
	keys := h.manager.ListKeys()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    keys,
	})
}

// getKey 获取密钥
func (h *Handlers) getKey(c *gin.Context) {
	id := c.Param("id")
	key, err := h.manager.GetKey(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    key,
	})
}

// revokeKey 吊销密钥
func (h *Handlers) revokeKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RevokeKey(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "key revoked",
	})
}

// rotateKey 轮换密钥
func (h *Handlers) rotateKey(c *gin.Context) {
	id := c.Param("id")
	var req KeyRotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有请求体，使用默认值
		req = KeyRotationRequest{
			KeyID:        id,
			RetainOldKey: true,
		}
	} else {
		req.KeyID = id
	}

	key, err := h.manager.RotateKey(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "key rotated",
		Data:    key,
	})
}

// encrypt 加密
func (h *Handlers) encrypt(c *gin.Context) {
	var req EncryptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.EncryptHybrid(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// decrypt 解密
func (h *Handlers) decrypt(c *gin.Context) {
	var req DecryptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 简化的解密实现
	result := &DecryptResponse{
		Plaintext: req.Ciphertext, // 模拟解密
		KeyID:     req.KeyID,
		Algorithm: AlgorithmClassic,
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// sign 签名
func (h *Handlers) sign(c *gin.Context) {
	var req SignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 验证密钥存在
	_, err := h.manager.GetKey(req.KeyID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	// 模拟签名
	signature := make([]byte, 64)
	copy(signature, req.Message[:min(64, len(req.Message))])

	result := &SignResponse{
		Signature: signature,
		KeyID:     req.KeyID,
		Algorithm: req.Algorithm,
	}

	// 审计
	h.manager.AuditCrypto(AuditSign, req.KeyID, map[string]interface{}{
		"message_size": len(req.Message),
	})

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// verify 验证
func (h *Handlers) verify(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 验证密钥存在
	_, err := h.manager.GetKey(req.KeyID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	// 模拟验证
	valid := len(req.Signature) > 0 && len(req.Message) > 0

	result := &VerifyResponse{
		Valid: valid,
		KeyID: req.KeyID,
	}

	// 审计
	h.manager.AuditCrypto(AuditVerify, req.KeyID, map[string]interface{}{
		"valid":        valid,
		"message_size": len(req.Message),
	})

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// listCiphers 列出加密器
func (h *Handlers) listCiphers(c *gin.Context) {
	ciphers := h.manager.ListCiphers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    ciphers,
	})
}

// createCipher 创建加密器
func (h *Handlers) createCipher(c *gin.Context) {
	var req HybridCipher
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	cipher, err := h.manager.CreateCipher(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "cipher created",
		Data:    cipher,
	})
}

// getCipher 获取加密器
func (h *Handlers) getCipher(c *gin.Context) {
	id := c.Param("id")
	cipher, err := h.manager.GetCipher(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cipher,
	})
}

// listMigrations 列出迁移计划
func (h *Handlers) listMigrations(c *gin.Context) {
	migrations := h.manager.ListMigrations()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    migrations,
	})
}

// migrateKeys 迁移密钥
func (h *Handlers) migrateKeys(c *gin.Context) {
	var req struct {
		SourceKeyID     string    `json:"source_key_id" binding:"required"`
		TargetAlgorithm Algorithm `json:"target_algorithm" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	plan, err := h.manager.MigrateKeys(req.SourceKeyID, req.TargetAlgorithm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "migration started",
		Data:    plan,
	})
}

// getMigration 获取迁移计划
func (h *Handlers) getMigration(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetMigration(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    plan,
	})
}

// getAuditLog 获取审计日志
func (h *Handlers) getAuditLog(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	entries := h.manager.GetAuditLog(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    entries,
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg QuantumSafeConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}

// listAlgorithms 列出算法
func (h *Handlers) listAlgorithms(c *gin.Context) {
	algos := h.manager.ListAlgorithms()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    algos,
	})
}

// getAlgorithmInfo 获取算法信息
func (h *Handlers) getAlgorithmInfo(c *gin.Context) {
	name := c.Param("name")
	algo := Algorithm(name)

	info := h.manager.GetAlgorithmInfo(algo)
	if info == nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "algorithm not found: " + name,
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// min 返回两个整数中较小的一个
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
