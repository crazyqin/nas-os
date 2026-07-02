package secureboot

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler Secure Boot HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建 Secure Boot 处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	sb := rg.Group("/secureboot")
	{
		sb.GET("/status", h.GetStatus)
		sb.GET("/tpm", h.GetTPMInfo)
		sb.POST("/tpm/detect", h.DetectTPM)

		sb.GET("/policy", h.GetPolicy)
		sb.PUT("/policy", h.SetPolicy)
		sb.GET("/config", h.GetConfig)

		sb.GET("/keys", h.ListTrustedKeys)
		sb.POST("/keys", h.AddTrustedKey)
		sb.DELETE("/keys/:fingerprint", h.RemoveTrustedKey)

		sb.POST("/hashes", h.AddTrustedHash)

		sb.GET("/entries", h.ListBootEntries)
		sb.POST("/entries", h.RegisterBootEntry)
		sb.POST("/verify", h.VerifySignature)
	}
}

// GetStatus 获取安全启动状态
// GET /api/v1/secureboot/status.
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    status,
	})
}

// GetTPMInfo 获取 TPM 信息
// GET /api/v1/secureboot/tpm.
func (h *Handler) GetTPMInfo(c *gin.Context) {
	info := h.manager.GetTPMInfo()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    info,
	})
}

// DetectTPM 检测 TPM
// POST /api/v1/secureboot/tpm/detect.
func (h *Handler) DetectTPM(c *gin.Context) {
	info, err := h.manager.DetectTPM(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    info,
	})
}

// GetPolicy 获取启动策略
// GET /api/v1/secureboot/policy.
func (h *Handler) GetPolicy(c *gin.Context) {
	policy := h.manager.GetBootPolicy()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    gin.H{"policy": policy},
	})
}

// SetPolicy 设置启动策略
// PUT /api/v1/secureboot/policy.
func (h *Handler) SetPolicy(c *gin.Context) {
	var req struct {
		Policy BootPolicy `json:"policy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.SetBootPolicy(req.Policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// GetConfig 获取安全启动配置
// GET /api/v1/secureboot/config.
func (h *Handler) GetConfig(c *gin.Context) {
	cfg := h.manager.GetSecureBootConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    cfg,
	})
}

// ListTrustedKeys 列出信任密钥
// GET /api/v1/secureboot/keys.
func (h *Handler) ListTrustedKeys(c *gin.Context) {
	cfg := h.manager.GetSecureBootConfig()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    cfg.TrustedKeys,
	})
}

// AddTrustedKey 添加信任密钥
// POST /api/v1/secureboot/keys.
func (h *Handler) AddTrustedKey(c *gin.Context) {
	var key KeyInfo
	if err := c.ShouldBindJSON(&key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.AddTrustedKey(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// RemoveTrustedKey 移除信任密钥
// DELETE /api/v1/secureboot/keys/:fingerprint.
func (h *Handler) RemoveTrustedKey(c *gin.Context) {
	fp := c.Param("fingerprint")
	if err := h.manager.RemoveTrustedKey(fp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// AddTrustedHash 添加信任哈希
// POST /api/v1/secureboot/hashes.
func (h *Handler) AddTrustedHash(c *gin.Context) {
	var req struct {
		Hash string `json:"hash"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.AddTrustedHash(req.Hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// ListBootEntries 列出启动项
// GET /api/v1/secureboot/entries.
func (h *Handler) ListBootEntries(c *gin.Context) {
	entries := h.manager.ListBootEntries()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    entries,
	})
}

// RegisterBootEntry 注册启动项
// POST /api/v1/secureboot/entries.
func (h *Handler) RegisterBootEntry(c *gin.Context) {
	var entry BootEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.RegisterBootEntry(entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// VerifySignature 验证签名
// POST /api/v1/secureboot/verify.
func (h *Handler) VerifySignature(c *gin.Context) {
	var req struct {
		Component string `json:"component"`
		Hash      string `json:"hash"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	result, err := h.manager.VerifySignature(c.Request.Context(), req.Component, req.Hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    result,
	})
}
