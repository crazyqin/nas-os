package web

import "github.com/gin-gonic/gin"

// registerLegacyStorageRoutes 注册历史存储 API 契约。
// 在客户端迁移完成前，路径、HTTP 方法和响应格式必须保持兼容。
func registerLegacyStorageRoutes(api *gin.RouterGroup, s *Server) {
	// ========== 卷管理 ==========
	api.GET("/volumes", s.listVolumes)
	api.POST("/volumes", s.createVolume)
	api.GET("/volumes/:name", s.getVolume)
	api.DELETE("/volumes/:name", s.deleteVolume)
	api.POST("/volumes/:name/mount", s.mountVolume)
	api.POST("/volumes/:name/unmount", s.unmountVolume)
	api.GET("/volumes/:name/usage", s.getVolumeUsage)
	api.POST("/volumes/:name/devices", s.addDevice)
	api.DELETE("/volumes/:name/devices/:device", s.removeDevice)
	api.GET("/volumes/:name/devices", s.getDeviceStats)

	// ========== 子卷管理 ==========
	api.GET("/volumes/:name/subvolumes", s.listSubVolumes)
	api.POST("/volumes/:name/subvolumes", s.createSubVolume)
	api.GET("/volumes/:name/subvolumes/:subvol", s.getSubVolume)
	api.DELETE("/volumes/:name/subvolumes/:subvol", s.deleteSubVolume)
	api.PUT("/volumes/:name/subvolumes/:subvol/readonly", s.setSubVolumeReadOnly)

	// ========== 快照管理 ==========
	api.GET("/volumes/:name/snapshots", s.listSnapshots)
	api.POST("/volumes/:name/snapshots", s.createSnapshot)
	api.DELETE("/volumes/:name/snapshots/:snapshot", s.deleteSnapshot)
	api.POST("/volumes/:name/snapshots/:snapshot/restore", s.restoreSnapshot)

	// ========== RAID 配置 ==========
	api.GET("/raid-configs", s.getRAIDConfigs)
	api.POST("/volumes/:name/convert", s.convertRAID)

	// ========== 维护操作 ==========
	api.POST("/volumes/:name/balance", s.startBalance)
	api.GET("/volumes/:name/balance", s.getBalanceStatus)
	api.POST("/volumes/:name/scrub", s.startScrub)
	api.GET("/volumes/:name/scrub", s.getScrubStatus)
}
