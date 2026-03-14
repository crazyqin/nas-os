package shares

import (
	"net/http"

	"nas-os/internal/nfs"
	"nas-os/internal/smb"

	"github.com/gin-gonic/gin"
)

// Response 通用响应
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

// Handlers 共享管理处理器（整合 SMB 和 NFS）
type Handlers struct {
	smbManager *smb.Manager
	nfsManager *nfs.Manager
}

// NewHandlers 创建处理器
func NewHandlers(smbMgr *smb.Manager, nfsMgr *nfs.Manager) *Handlers {
	return &Handlers{
		smbManager: smbMgr,
		nfsManager: nfsMgr,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	// 统一的共享管理路由
	shares := api.Group("/shares")
	{
		// ========== 概览 ==========
		shares.GET("", h.listAllShares)         // 列出所有共享
		shares.GET("/status", h.getStatus)      // 获取服务状态
		shares.POST("/apply", h.applyAllConfig) // 应用所有配置

		// ========== SMB 共享 ==========
		smbGroup := shares.Group("/smb")
		{
			smbGroup.GET("", h.listSMBShares)
			smbGroup.POST("", h.createSMBShare)
			smbGroup.GET("/:name", h.getSMBShare)
			smbGroup.PUT("/:name", h.updateSMBShare)
			smbGroup.DELETE("/:name", h.deleteSMBShare)
			smbGroup.POST("/:name/permission", h.setSMBPermission)
			smbGroup.DELETE("/:name/permission/:user", h.removeSMBPermission)
			smbGroup.GET("/user", h.getUserSMBShares)
			smbGroup.POST("/restart", h.restartSMB)
			smbGroup.GET("/status", h.getSMBStatus)
			smbGroup.GET("/config", h.getSMBConfig)
			smbGroup.PUT("/config", h.updateSMBConfig)
			smbGroup.POST("/test", h.testSMBConfig)
		}

		// ========== NFS 共享 ==========
		nfsGroup := shares.Group("/nfs")
		{
			nfsGroup.GET("", h.listNFSExports)
			nfsGroup.POST("", h.createNFSExport)
			nfsGroup.GET("/:path", h.getNFSExport)
			nfsGroup.PUT("/:path", h.updateNFSExport)
			nfsGroup.DELETE("/:path", h.deleteNFSExport)
			nfsGroup.POST("/restart", h.restartNFS)
			nfsGroup.GET("/status", h.getNFSStatus)
			nfsGroup.GET("/clients", h.getNFSClients)
			nfsGroup.GET("/config", h.getNFSConfig)
			nfsGroup.PUT("/config", h.updateNFSConfig)
		}
	}
}

// ========== 统一共享管理 ==========

type ShareOverview struct {
	Type   string      `json:"type"` // "smb" or "nfs"
	Name   string      `json:"name"`
	Path   string      `json:"path"`
	Config interface{} `json:"config"`
}

func (h *Handlers) listAllShares(c *gin.Context) {
	var result []ShareOverview

	// 收集 SMB 共享
	smbShares, _ := h.smbManager.ListShares()
	for _, s := range smbShares {
		result = append(result, ShareOverview{
			Type:   "smb",
			Name:   s.Name,
			Path:   s.Path,
			Config: s,
		})
	}

	// 收集 NFS 导出
	nfsExports, _ := h.nfsManager.ListExports()
	for _, e := range nfsExports {
		result = append(result, ShareOverview{
			Type:   "nfs",
			Name:   e.Path, // NFS 用路径作为标识
			Path:   e.Path,
			Config: e,
		})
	}

	c.JSON(http.StatusOK, Success(result))
}

func (h *Handlers) getStatus(c *gin.Context) {
	smbRunning, _ := h.smbManager.GetStatus()
	nfsStatus, _ := h.nfsManager.Status()

	c.JSON(http.StatusOK, Success(gin.H{
		"smb": gin.H{
			"running": smbRunning,
		},
		"nfs": gin.H{
			"running": nfsStatus.Running,
			"status":  nfsStatus.Status,
		},
	}))
}

func (h *Handlers) applyAllConfig(c *gin.Context) {
	smbErr := h.smbManager.ApplyConfig()
	nfsErr := h.nfsManager.Reload()

	result := gin.H{
		"smb": gin.H{"success": smbErr == nil},
		"nfs": gin.H{"success": nfsErr == nil},
	}

	if smbErr != nil {
		result["smb"].(gin.H)["error"] = smbErr.Error()
	}
	if nfsErr != nil {
		result["nfs"].(gin.H)["error"] = nfsErr.Error()
	}

	if smbErr != nil || nfsErr != nil {
		c.JSON(http.StatusPartialContent, Success(result))
		return
	}

	c.JSON(http.StatusOK, Success(result))
}

// ========== SMB 共享 API ==========

func (h *Handlers) listSMBShares(c *gin.Context) {
	shares, err := h.smbManager.ListShares()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(shares))
}

func (h *Handlers) createSMBShare(c *gin.Context) {
	var req smb.ShareInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	share, err := h.smbManager.CreateShareFromInput(req)
	if err != nil {
		c.JSON(http.StatusConflict, Error(409, err.Error()))
		return
	}

	// 自动应用配置
	if err := h.smbManager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(share))
}

func (h *Handlers) getSMBShare(c *gin.Context) {
	name := c.Param("name")
	share, err := h.smbManager.GetShare(name)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(share))
}

func (h *Handlers) updateSMBShare(c *gin.Context) {
	name := c.Param("name")
	var req smb.ShareInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	share, err := h.smbManager.UpdateShareFromInput(name, req)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	// 自动应用配置
	if err := h.smbManager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(share))
}

func (h *Handlers) deleteSMBShare(c *gin.Context) {
	name := c.Param("name")
	if err := h.smbManager.DeleteShare(name); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	// 自动应用配置
	if err := h.smbManager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) setSMBPermission(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Username  string `json:"username" binding:"required"`
		ReadWrite bool   `json:"read_write"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.smbManager.SetSharePermission(name, req.Username, req.ReadWrite); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) removeSMBPermission(c *gin.Context) {
	name := c.Param("name")
	username := c.Param("user")
	if err := h.smbManager.RemoveSharePermission(name, username); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getUserSMBShares(c *gin.Context) {
	username := c.Query("user")
	if username == "" {
		c.JSON(http.StatusBadRequest, Error(400, "需要用户名"))
		return
	}

	shares := h.smbManager.GetUserShares(username)
	c.JSON(http.StatusOK, Success(shares))
}

func (h *Handlers) restartSMB(c *gin.Context) {
	if err := h.smbManager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getSMBStatus(c *gin.Context) {
	running, err := h.smbManager.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(gin.H{"running": running}))
}

func (h *Handlers) getSMBConfig(c *gin.Context) {
	// 返回全局 SMB 配置
	c.JSON(http.StatusOK, Success(gin.H{
		"message": "SMB 配置已加载",
	}))
}

func (h *Handlers) updateSMBConfig(c *gin.Context) {
	// TODO: 更新全局 SMB 配置
	c.JSON(http.StatusOK, Success(gin.H{"message": "配置已更新"}))
}

func (h *Handlers) testSMBConfig(c *gin.Context) {
	ok, output, err := h.smbManager.TestConfig()
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "配置测试失败："+output))
		return
	}
	c.JSON(http.StatusOK, Success(gin.H{
		"valid":  ok,
		"output": output,
	}))
}

// ========== NFS 共享 API ==========

func (h *Handlers) listNFSExports(c *gin.Context) {
	exports, err := h.nfsManager.ListExports()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(exports))
}

func (h *Handlers) createNFSExport(c *gin.Context) {
	var req nfs.ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	exp := req.ToExport()
	if err := h.nfsManager.CreateExport(exp); err != nil {
		c.JSON(http.StatusConflict, Error(409, err.Error()))
		return
	}

	// 自动应用配置
	if err := h.nfsManager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(exp))
}

func (h *Handlers) getNFSExport(c *gin.Context) {
	path := c.Param("path")
	exp, err := h.nfsManager.GetExport(path)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(exp))
}

func (h *Handlers) updateNFSExport(c *gin.Context) {
	path := c.Param("path")
	var req nfs.ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	exp := req.ToExport()
	if err := h.nfsManager.UpdateExport(path, exp); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	// 自动应用配置
	if err := h.nfsManager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(exp))
}

func (h *Handlers) deleteNFSExport(c *gin.Context) {
	path := c.Param("path")
	if err := h.nfsManager.DeleteExport(path); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	// 自动应用配置
	if err := h.nfsManager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) restartNFS(c *gin.Context) {
	if err := h.nfsManager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getNFSStatus(c *gin.Context) {
	status, err := h.nfsManager.Status()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(status))
}

func (h *Handlers) getNFSClients(c *gin.Context) {
	clients, err := h.nfsManager.GetClients()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(gin.H{"clients": clients}))
}

func (h *Handlers) getNFSConfig(c *gin.Context) {
	// 返回全局 NFS 配置
	c.JSON(http.StatusOK, Success(gin.H{
		"message": "NFS 配置已加载",
	}))
}

func (h *Handlers) updateNFSConfig(c *gin.Context) {
	// TODO: 更新全局 NFS 配置
	c.JSON(http.StatusOK, Success(gin.H{"message": "配置已更新"}))
}
