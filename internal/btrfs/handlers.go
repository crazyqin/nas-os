package btrfs

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler Btrfs API handler
type Handler struct {
	manager *BtrfsManager
	logger  *zap.Logger
}

// NewHandler 创建handler
func NewHandler(manager *BtrfsManager, logger *zap.Logger) *Handler {
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	btrfs := rg.Group("/btrfs")
	{
		// 池管理
		btrfs.POST("/pool", h.CreatePool)
		btrfs.GET("/pool/:mountpoint/info", h.GetPoolInfo)
		btrfs.POST("/pool/:mountpoint/mount", h.MountPool)
		btrfs.POST("/pool/:mountpoint/unmount", h.UnmountPool)

		// 子卷管理
		btrfs.POST("/subvolume", h.CreateSubvolume)
		btrfs.DELETE("/subvolume", h.DeleteSubvolume)
		btrfs.GET("/subvolume/:mountpoint/list", h.ListSubvolumes)

		// 快照
		btrfs.POST("/snapshot", h.Snapshot)
		btrfs.POST("/send-receive", h.SendReceive)

		// Balance (在线RAID转换)
		btrfs.POST("/balance/:mountpoint/start", h.StartBalance)
		btrfs.POST("/balance/:mountpoint/cancel", h.CancelBalance)
		btrfs.GET("/balance/:mountpoint/status", h.BalanceStatus)

		// 设备管理
		btrfs.POST("/device/:mountpoint/add", h.AddDevice)
		btrfs.POST("/device/:mountpoint/remove", h.RemoveDevice)

		// 碎片整理
		btrfs.POST("/defragment", h.Defragment)

		// 使用情况
		btrfs.GET("/usage/:mountpoint", h.GetUsage)
	}
}

type CreatePoolRequest struct {
	Label    string   `json:"label" binding:"required"`
	Devices  []string `json:"devices" binding:"required"`
	Profile  string   `json:"profile"` // single, raid1, raid5, raid6, raid10
}

func (h *Handler) CreatePool(c *gin.Context) {
	var req CreatePoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile := RAIDProfile(req.Profile)
	if profile == "" {
		profile = RAIDSingle
	}

	if err := h.manager.CreatePool(c.Request.Context(), req.Label, req.Devices, profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "created"})
}

func (h *Handler) GetPoolInfo(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	info, err := h.manager.GetPoolInfo(c.Request.Context(), mountpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

type MountRequest struct {
	Device     string            `json:"device" binding:"required"`
	Mountpoint string            `json:"mountpoint" binding:"required"`
	Options    map[string]string `json:"options"`
}

func (h *Handler) MountPool(c *gin.Context) {
	var req MountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.MountPool(c.Request.Context(), req.Device, req.Mountpoint, req.Options); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "mounted"})
}

func (h *Handler) UnmountPool(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	if err := h.manager.UnmountPool(c.Request.Context(), mountpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "unmounted"})
}

type SubvolumeRequest struct {
	Path string `json:"path" binding:"required"`
}

func (h *Handler) CreateSubvolume(c *gin.Context) {
	var req SubvolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.CreateSubvolume(c.Request.Context(), req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (h *Handler) DeleteSubvolume(c *gin.Context) {
	var req SubvolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.DeleteSubvolume(c.Request.Context(), req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) ListSubvolumes(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	subvols, err := h.manager.ListSubvolumes(c.Request.Context(), mountpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, subvols)
}

type SnapshotRequest struct {
	Source   string `json:"source" binding:"required"`
	Dest     string `json:"dest" binding:"required"`
	ReadOnly bool   `json:"readonly"`
}

func (h *Handler) Snapshot(c *gin.Context) {
	var req SnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.Snapshot(c.Request.Context(), req.Source, req.Dest, req.ReadOnly); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "snapshot_created"})
}

type SendReceiveRequest struct {
	ParentSnap  string `json:"parent_snap" binding:"required"`
	CurrentSnap string `json:"current_snap" binding:"required"`
	Dest        string `json:"dest" binding:"required"`
}

func (h *Handler) SendReceive(c *gin.Context) {
	var req SendReceiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.SendReceive(c.Request.Context(), req.ParentSnap, req.CurrentSnap, req.Dest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

type BalanceRequest struct {
	Profile string            `json:"profile"` // raid1, raid5, raid6, raid10
	Filters map[string]string `json:"filters"`
}

func (h *Handler) StartBalance(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	var req BalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile := RAIDProfile(req.Profile)
	if err := h.manager.Balance(c.Request.Context(), mountpoint, profile, req.Filters); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "balance_started"})
}

func (h *Handler) CancelBalance(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	if err := h.manager.BalanceCancel(c.Request.Context(), mountpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "balance_cancelled"})
}

func (h *Handler) BalanceStatus(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	status, err := h.manager.BalanceStatus(c.Request.Context(), mountpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

type DeviceRequest struct {
	Device string `json:"device" binding:"required"`
}

func (h *Handler) AddDevice(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	var req DeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddDevice(c.Request.Context(), mountpoint, req.Device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "device_added"})
}

func (h *Handler) RemoveDevice(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	var req DeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.RemoveDevice(c.Request.Context(), mountpoint, req.Device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "device_removed"})
}

type DefragRequest struct {
	Path     string `json:"path" binding:"required"`
	Compress bool   `json:"compress"`
}

func (h *Handler) Defragment(c *gin.Context) {
	var req DefragRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.Defragment(c.Request.Context(), req.Path, req.Compress); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "defrag_started"})
}

func (h *Handler) GetUsage(c *gin.Context) {
	mountpoint := c.Param("mountpoint")
	usage, err := h.manager.GetUsage(c.Request.Context(), mountpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"usage": usage})
}
