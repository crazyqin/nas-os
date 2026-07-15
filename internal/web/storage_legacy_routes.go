package web

import "github.com/gin-gonic/gin"

// registerLegacyStorageRoutes 注册历史存储 API 契约。
func registerLegacyStorageRoutes(api *gin.RouterGroup, h *LegacyStorageHandlers) {
	api.GET("/volumes", h.listVolumes)
	api.POST("/volumes", h.createVolume)
	api.GET("/volumes/:name", h.getVolume)
	api.DELETE("/volumes/:name", h.deleteVolume)
	api.POST("/volumes/:name/mount", h.mountVolume)
	api.POST("/volumes/:name/unmount", h.unmountVolume)
	api.GET("/volumes/:name/usage", h.getVolumeUsage)
	api.POST("/volumes/:name/devices", h.addDevice)
	api.DELETE("/volumes/:name/devices/:device", h.removeDevice)
	api.GET("/volumes/:name/devices", h.getDeviceStats)
	api.GET("/volumes/:name/subvolumes", h.listSubVolumes)
	api.POST("/volumes/:name/subvolumes", h.createSubVolume)
	api.GET("/volumes/:name/subvolumes/:subvol", h.getSubVolume)
	api.DELETE("/volumes/:name/subvolumes/:subvol", h.deleteSubVolume)
	api.PUT("/volumes/:name/subvolumes/:subvol/readonly", h.setSubVolumeReadOnly)
	api.GET("/volumes/:name/snapshots", h.listSnapshots)
	api.POST("/volumes/:name/snapshots", h.createSnapshot)
	api.DELETE("/volumes/:name/snapshots/:snapshot", h.deleteSnapshot)
	api.POST("/volumes/:name/snapshots/:snapshot/restore", h.restoreSnapshot)
	api.GET("/raid-configs", h.getRAIDConfigs)
	api.POST("/volumes/:name/convert", h.convertRAID)
	api.POST("/volumes/:name/balance", h.startBalance)
	api.GET("/volumes/:name/balance", h.getBalanceStatus)
	api.POST("/volumes/:name/scrub", h.startScrub)
	api.GET("/volumes/:name/scrub", h.getScrubStatus)
}
