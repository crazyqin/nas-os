// Package encryption Vault HTTP API 处理器
// 刑部 Round 241 - Vault Password 加密卷
package encryption

import (
	"net/http"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== 请求/响应类型 ==========

// CreateVaultRequest 创建 vault 请求.
type CreateVaultRequest struct {
	Name     string `json:"name" binding:"required" validate:"min=1,max=64"`
	Password string `json:"password" binding:"required" validate:"min=8,max=128"`
}

// UnlockVaultRequest 解锁 vault 请求.
type UnlockVaultRequest struct {
	Password string `json:"password" binding:"required"`
}

// VaultInfo vault 信息响应.
type VaultInfo struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Algorithm    Algorithm  `json:"algorithm"`
	State        VaultState `json:"state"`
	CreatedAt    string     `json:"created_at"`
	LastAccessed string     `json:"last_accessed"`
}

// VaultHandlers vault HTTP 处理器.
type VaultHandlers struct {
	manager *VaultManager
}

// NewVaultHandlers 创建 vault 处理器.
func NewVaultHandlers(manager *VaultManager) *VaultHandlers {
	return &VaultHandlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *VaultHandlers) RegisterRoutes(r *gin.RouterGroup) {
	vaultGroup := r.Group("/vaults")
	{
		vaultGroup.POST("", h.createVault)
		vaultGroup.GET("", h.listVaults)
		vaultGroup.POST("/:id/unlock", h.unlockVault)
		vaultGroup.POST("/:id/lock", h.lockVault)
		vaultGroup.DELETE("/:id", h.deleteVault)
	}
}

// createVault POST /api/vaults - 创建 vault.
func (h *VaultHandlers) createVault(c *gin.Context) {
	var req CreateVaultRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	vault, err := h.manager.CreateVault(req.Name, req.Password)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, toVaultInfo(vault))
}

// listVaults GET /api/vaults - 列出所有 vault.
func (h *VaultHandlers) listVaults(c *gin.Context) {
	vaults := h.manager.ListVaults()

	infos := make([]VaultInfo, 0, len(vaults))
	for _, v := range vaults {
		infos = append(infos, toVaultInfo(v))
	}

	api.OK(c, gin.H{
		"vaults": infos,
		"total":  len(infos),
	})
}

// unlockVault POST /api/vaults/:id/unlock - 解锁 vault.
func (h *VaultHandlers) unlockVault(c *gin.Context) {
	vaultID := c.Param("id")
	if vaultID == "" {
		api.BadRequest(c, "vault ID 不能为空")
		return
	}

	var req UnlockVaultRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	_, err := h.manager.UnlockVault(vaultID, req.Password)
	if err != nil {
		// 密码错误返回 401，vault 不存在返回 404
		if isNotFoundError(err) {
			api.NotFound(c, "vault 不存在")
			return
		}
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, err.Error()))
		return
	}

	vault, _ := h.manager.GetVault(vaultID)
	api.OK(c, vault)
}

// lockVault POST /api/vaults/:id/lock - 锁定 vault.
func (h *VaultHandlers) lockVault(c *gin.Context) {
	vaultID := c.Param("id")
	if vaultID == "" {
		api.BadRequest(c, "vault ID 不能为空")
		return
	}

	err := h.manager.LockVault(vaultID)
	if err != nil {
		if isNotFoundError(err) {
			api.NotFound(c, "vault 不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "vault 已锁定", nil)
}

// deleteVault DELETE /api/vaults/:id - 删除 vault.
func (h *VaultHandlers) deleteVault(c *gin.Context) {
	vaultID := c.Param("id")
	if vaultID == "" {
		api.BadRequest(c, "vault ID 不能为空")
		return
	}

	err := h.manager.DeleteVault(vaultID)
	if err != nil {
		if isNotFoundError(err) {
			api.NotFound(c, "vault 不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "vault 已删除", nil)
}

// ========== 辅助函数 ==========

// toVaultInfo 转换为响应结构.
func toVaultInfo(v *Vault) VaultInfo {
	return VaultInfo{
		ID:           v.ID,
		Name:         v.Name,
		Algorithm:    v.Algorithm,
		State:        v.State,
		CreatedAt:    v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastAccessed: v.LastAccessed.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// isNotFoundError 判断错误是否为"不存在"类型.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "不存在")
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString 在字符串中搜索子串.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
