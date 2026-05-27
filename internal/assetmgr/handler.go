// Package assetmgr 提供IT资产管理功能
package assetmgr

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 资产管理 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	assetGroup := api.Group("/assetmgr")
	{
		// 资产管理
		assetGroup.POST("/assets", h.addAsset)
		assetGroup.GET("/assets", h.listAssets)
		assetGroup.GET("/assets/search", h.searchAssets)
		assetGroup.GET("/assets/:id", h.getAsset)
		assetGroup.PUT("/assets/:id", h.updateAsset)
		assetGroup.DELETE("/assets/:id", h.deleteAsset)

		// 硬件/软件清单
		assetGroup.GET("/inventory/hardware", h.hardwareInventory)
		assetGroup.GET("/inventory/software", h.softwareInventory)

		// 资产分组
		assetGroup.POST("/groups", h.createGroup)
		assetGroup.GET("/groups", h.listGroups)
		assetGroup.GET("/groups/:id", h.getGroup)
		assetGroup.POST("/groups/:id/assets/:assetId", h.addAssetToGroup)
		assetGroup.DELETE("/groups/:id/assets/:assetId", h.removeAssetFromGroup)
		assetGroup.DELETE("/groups/:id", h.deleteGroup)

		// 维护计划
		assetGroup.POST("/schedules", h.createSchedule)
		assetGroup.GET("/schedules", h.listSchedules)
		assetGroup.GET("/schedules/:id", h.getSchedule)
		assetGroup.PUT("/schedules/:id/maintenance", h.recordMaintenance)
		assetGroup.GET("/schedules/upcoming", h.upcomingMaintenance)

		// 网络扫描
		assetGroup.POST("/scan", h.scanNetwork)
		assetGroup.GET("/scan/results", h.scanResults)

		// 生命周期
		assetGroup.PUT("/assets/:id/lifecycle", h.updateLifecycle)
		assetGroup.GET("/lifecycle/aging", h.agingAssets)
		assetGroup.GET("/lifecycle/warranty-expired", h.expiredWarranty)

		// 统计
		assetGroup.GET("/summary", h.summary)
	}
}

// addAsset 添加资产.
func (h *Handlers) addAsset(c *gin.Context) {
	var asset Asset
	if err := c.ShouldBindJSON(&asset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.AddAsset(&asset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, asset)
}

// listAssets 列出资产.
func (h *Handlers) listAssets(c *gin.Context) {
	assetType := AssetType(c.Query("type"))
	status := AssetStatus(c.Query("status"))
	assets := h.manager.ListAssets(assetType, status)
	c.JSON(http.StatusOK, gin.H{"assets": assets, "total": len(assets)})
}

// searchAssets 搜索资产.
func (h *Handlers) searchAssets(c *gin.Context) {
	query := c.Query("q")
	assets := h.manager.SearchAssets(query)
	c.JSON(http.StatusOK, gin.H{"assets": assets, "total": len(assets)})
}

// getAsset 获取资产详情.
func (h *Handlers) getAsset(c *gin.Context) {
	asset, err := h.manager.GetAsset(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, asset)
}

// updateAsset 更新资产.
func (h *Handlers) updateAsset(c *gin.Context) {
	var asset Asset
	if err := c.ShouldBindJSON(&asset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	asset.ID = c.Param("id")
	if err := h.manager.UpdateAsset(&asset); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, asset)
}

// deleteAsset 删除资产.
func (h *Handlers) deleteAsset(c *gin.Context) {
	if err := h.manager.DeleteAsset(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "资产已删除"})
}

// hardwareInventory 硬件清单.
func (h *Handlers) hardwareInventory(c *gin.Context) {
	assets := h.manager.ListHardwareInventory()
	c.JSON(http.StatusOK, gin.H{"assets": assets, "total": len(assets)})
}

// softwareInventory 软件清单.
func (h *Handlers) softwareInventory(c *gin.Context) {
	assets := h.manager.ListSoftwareInventory()
	c.JSON(http.StatusOK, gin.H{"assets": assets, "total": len(assets)})
}

// createGroup 创建分组.
func (h *Handlers) createGroup(c *gin.Context) {
	var group AssetGroup
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.CreateGroup(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

// listGroups 列出分组.
func (h *Handlers) listGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, gin.H{"groups": groups, "total": len(groups)})
}

// getGroup 获取分组详情.
func (h *Handlers) getGroup(c *gin.Context) {
	group, err := h.manager.GetGroup(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

// addAssetToGroup 添加资产到分组.
func (h *Handlers) addAssetToGroup(c *gin.Context) {
	if err := h.manager.AddAssetToGroup(c.Param("id"), c.Param("assetId")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "资产已添加到分组"})
}

// removeAssetFromGroup 从分组移除资产.
func (h *Handlers) removeAssetFromGroup(c *gin.Context) {
	if err := h.manager.RemoveAssetFromGroup(c.Param("id"), c.Param("assetId")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "资产已从分组移除"})
}

// deleteGroup 删除分组.
func (h *Handlers) deleteGroup(c *gin.Context) {
	if err := h.manager.DeleteGroup(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "分组已删除"})
}

// createSchedule 创建维护计划.
func (h *Handlers) createSchedule(c *gin.Context) {
	var schedule MaintenanceSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.CreateMaintenanceSchedule(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, schedule)
}

// listSchedules 列出维护计划.
func (h *Handlers) listSchedules(c *gin.Context) {
	schedules := h.manager.ListMaintenanceSchedules()
	c.JSON(http.StatusOK, gin.H{"schedules": schedules, "total": len(schedules)})
}

// getSchedule 获取维护计划.
func (h *Handlers) getSchedule(c *gin.Context) {
	schedule, err := h.manager.GetMaintenanceSchedule(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, schedule)
}

// recordMaintenance 记录维护.
func (h *Handlers) recordMaintenance(c *gin.Context) {
	var req struct {
		Time string `json:"time"` // RFC3339
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.RecordMaintenance(c.Param("id"), timeParse(req.Time)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "维护已记录"})
}

func timeParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now()
	}
	return t
}

// upcomingMaintenance 即将到期的维护.
func (h *Handlers) upcomingMaintenance(c *gin.Context) {
	days := queryInt(c, "days", 30)
	schedules := h.manager.GetUpcomingMaintenance(days)
	c.JSON(http.StatusOK, gin.H{"schedules": schedules, "total": len(schedules)})
}

// scanNetwork 网络扫描.
func (h *Handlers) scanNetwork(c *gin.Context) {
	var req struct {
		Range string `json:"range"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	result, err := h.manager.ScanNetwork(req.Range)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// scanResults 扫描结果.
func (h *Handlers) scanResults(c *gin.Context) {
	results := h.manager.scanner.GetScanResults()
	c.JSON(http.StatusOK, gin.H{"results": results, "total": len(results)})
}

// updateLifecycle 更新生命周期.
func (h *Handlers) updateLifecycle(c *gin.Context) {
	var req struct {
		Stage LifecycleStage `json:"stage"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.manager.UpdateLifecycleStage(c.Param("id"), req.Stage); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "生命周期已更新"})
}

// agingAssets 老化资产.
func (h *Handlers) agingAssets(c *gin.Context) {
	years := queryInt(c, "years", 5)
	assets := h.manager.GetAgingAssets(years)
	c.JSON(http.StatusOK, gin.H{"assets": assets, "total": len(assets)})
}

// expiredWarranty 保修到期.
func (h *Handlers) expiredWarranty(c *gin.Context) {
	assets := h.manager.GetExpiredWarranty()
	c.JSON(http.StatusOK, gin.H{"assets": assets, "total": len(assets)})
}

// summary 统计摘要.
func (h *Handlers) summary(c *gin.Context) {
	summary := h.manager.GetAssetSummary()
	c.JSON(http.StatusOK, summary)
}

// queryInt 从查询参数获取整数.
func queryInt(c *gin.Context, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	val := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			val = val*10 + int(ch-'0')
		}
	}
	if val == 0 {
		return defaultVal
	}
	return val
}
