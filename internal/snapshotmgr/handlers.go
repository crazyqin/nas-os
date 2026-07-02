package snapshotmgr

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 快照 HTTP API 处理器.
type Handlers struct {
	manager      *Manager
	zfsManager   *ZFSSnapshotManager
	teamManager  *TeamSnapshotManager
	quotaManager *QuotaManager
	policyStore  *PolicyStore
	logger       *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	h := &Handlers{
		manager:     manager,
		zfsManager:  NewZFSSnapshotManager(logger),
		teamManager: NewTeamSnapshotManager(logger, manager),
		policyStore: NewPolicyStore(logger),
		logger:      logger,
	}
	h.quotaManager = NewQuotaManager(logger, manager, 20, 0)
	return h
}

// SetZFSSnapshotManager 设置 ZFS 快照管理器（可选注入）.
func (h *Handlers) SetZFSSnapshotManager(zfs *ZFSSnapshotManager) {
	h.zfsManager = zfs
}

// SetTeamSnapshotManager 设置团队快照管理器（可选注入）.
func (h *Handlers) SetTeamSnapshotManager(team *TeamSnapshotManager) {
	h.teamManager = team
}

// SetQuotaManager 设置配额管理器（可选注入）.
func (h *Handlers) SetQuotaManager(qm *QuotaManager) {
	h.quotaManager = qm
}

// SetPolicyStore 设置策略存储（可选注入）.
func (h *Handlers) SetPolicyStore(ps *PolicyStore) {
	h.policyStore = ps
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	snap := rg.Group("/snapshots")
	{
		snap.POST("", h.CreateSnapshot)
		snap.GET("", h.ListSnapshots)
		snap.GET("/stats", h.GetStats)
		snap.GET("/:id", h.GetSnapshot)
		snap.DELETE("/:id", h.DeleteSnapshot)
		snap.POST("/:id/restore", h.RestoreSnapshot)

		// 保留策略
		snap.POST("/policies", h.CreatePolicy)
		snap.GET("/policies", h.ListPolicies)

		// ZFS 快照增强
		snap.POST("/zfs/bookmark", h.CreateZFSBookmark)
		snap.GET("/zfs/bookmarks", h.ListZFSBookmarks)
		snap.DELETE("/zfs/bookmarks/:name", h.DeleteZFSBookmark)
		snap.POST("/zfs/hold", h.AddZFSHold)
		snap.DELETE("/zfs/hold", h.RemoveZFSHold)
		snap.GET("/zfs/holds", h.ListZFSHolds)

		// 快照差异对比
		snap.GET("/diff", h.GetSnapshotDiff)

		// 配额管理
		snap.GET("/quota", h.GetQuotaStatus)
		snap.PUT("/quota", h.UpdateQuota)
		snap.POST("/quota/enforce", h.EnforceQuota)

		// 团队快照
		snap.POST("/team/policies", h.CreateTeamPolicy)
		snap.GET("/team/policies", h.ListTeamPolicies)
		snap.POST("/team/snapshots", h.CreateTeamSnapshot)
		snap.GET("/team/snapshots", h.ListTeamSnapshots)
		snap.POST("/team/snapshots/:id/restore", h.RestoreTeamSnapshot)
		snap.DELETE("/team/snapshots/:id", h.DeleteTeamSnapshot)
		snap.POST("/team/snapshots/:id/lock", h.LockTeamSnapshot)
		snap.POST("/team/snapshots/:id/unlock", h.UnlockTeamSnapshot)
	}
}

// createSnapshotReq 创建快照请求.
type createSnapshotReq struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Source      string         `json:"source"`
	Items       []SnapshotItem `json:"items"`
}

// CreateSnapshot POST /api/v1/snapshots.
func (h *Handlers) CreateSnapshot(c *gin.Context) {
	var req createSnapshotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	source := req.Source
	if source == "" {
		source = "manual"
	}

	snap, err := h.manager.CreateSnapshot(req.Name, req.Description, source, req.Items)
	if err != nil {
		h.logger.Error("failed to create snapshot", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, snap)
}

// ListSnapshots GET /api/v1/snapshots.
func (h *Handlers) ListSnapshots(c *gin.Context) {
	snapshots := h.manager.ListSnapshots()
	c.JSON(http.StatusOK, gin.H{"snapshots": snapshots})
}

// GetSnapshot GET /api/v1/snapshots/:id.
func (h *Handlers) GetSnapshot(c *gin.Context) {
	id := c.Param("id")
	snap, err := h.manager.GetSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap)
}

// DeleteSnapshot DELETE /api/v1/snapshots/:id.
func (h *Handlers) DeleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteSnapshot(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "snapshot deleted"})
}

// RestoreSnapshot POST /api/v1/snapshots/:id/restore.
func (h *Handlers) RestoreSnapshot(c *gin.Context) {
	id := c.Param("id")
	items, err := h.manager.RestoreSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "restore initiated",
		"items":   items,
	})
}

// GetStats GET /api/v1/snapshots/stats.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// ==================== 保留策略 ====================

// createPolicyReq 创建保留策略请求.
type createPolicyReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	TargetScope string `json:"target_scope"` // global / pool / dataset / team
	TargetRef   string `json:"target_ref"`
	Minutely    int    `json:"minutely"`
	Hourly      int    `json:"hourly"`
	Daily       int    `json:"daily"`
	Weekly      int    `json:"weekly"`
	Monthly     int    `json:"monthly"`
	Yearly      int    `json:"yearly"`
}

// CreatePolicy POST /api/v1/snapshots/policies.
func (h *Handlers) CreatePolicy(c *gin.Context) {
	var req createPolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := &RetentionPolicy{
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		TargetScope: req.TargetScope,
		TargetRef:   req.TargetRef,
		Minutely:    req.Minutely,
		Hourly:      req.Hourly,
		Daily:       req.Daily,
		Weekly:      req.Weekly,
		Monthly:     req.Monthly,
		Yearly:      req.Yearly,
	}

	created, err := h.policyStore.Create(policy)
	if err != nil {
		h.logger.Error("failed to create policy", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// ListPolicies GET /api/v1/snapshots/policies.
func (h *Handlers) ListPolicies(c *gin.Context) {
	targetScope := c.Query("target_scope")
	targetRef := c.Query("target_ref")

	policies := h.policyStore.List(targetScope, targetRef)
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// ==================== ZFS Bookmark ====================

// createBookmarkReq 创建 ZFS bookmark 请求.
type createBookmarkReq struct {
	Pool         string `json:"pool" binding:"required"`
	Dataset      string `json:"dataset"`
	SnapshotName string `json:"snapshot_name" binding:"required"`
	BookmarkName string `json:"bookmark_name" binding:"required"`
}

// CreateZFSBookmark POST /api/v1/snapshots/zfs/bookmark.
func (h *Handlers) CreateZFSBookmark(c *gin.Context) {
	var req createBookmarkReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bm, err := h.zfsManager.CreateBookmark(req.Pool, req.Dataset, req.SnapshotName, req.BookmarkName)
	if err != nil {
		h.logger.Error("failed to create ZFS bookmark", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bm)
}

// ListZFSBookmarks GET /api/v1/snapshots/zfs/bookmarks.
func (h *Handlers) ListZFSBookmarks(c *gin.Context) {
	pool := c.Query("pool")
	dataset := c.Query("dataset")

	bookmarks := h.zfsManager.ListBookmarks(pool, dataset)
	c.JSON(http.StatusOK, gin.H{"bookmarks": bookmarks})
}

// DeleteZFSBookmark DELETE /api/v1/snapshots/zfs/bookmarks/:name.
func (h *Handlers) DeleteZFSBookmark(c *gin.Context) {
	name := c.Param("name")
	pool := c.Query("pool")
	dataset := c.Query("dataset")

	if err := h.zfsManager.DeleteBookmark(pool, dataset, name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bookmark deleted"})
}

// ==================== ZFS Hold ====================

// addHoldReq 添加 ZFS hold 请求.
type addHoldReq struct {
	SnapshotID string `json:"snapshot_id" binding:"required"`
	Tag        string `json:"tag" binding:"required"`
	Reason     string `json:"reason"`
	HolderRef  string `json:"holder_ref"`
}

// removeHoldReq 移除 ZFS hold 请求.
type removeHoldReq struct {
	SnapshotID string `json:"snapshot_id" binding:"required"`
	Tag        string `json:"tag" binding:"required"`
}

// AddZFSHold POST /api/v1/snapshots/zfs/hold.
func (h *Handlers) AddZFSHold(c *gin.Context) {
	var req addHoldReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.zfsManager.AddHold(req.SnapshotID, req.Tag, req.Reason, req.HolderRef); err != nil {
		h.logger.Error("failed to add ZFS hold", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "hold added"})
}

// RemoveZFSHold DELETE /api/v1/snapshots/zfs/hold.
func (h *Handlers) RemoveZFSHold(c *gin.Context) {
	var req removeHoldReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.zfsManager.RemoveHold(req.SnapshotID, req.Tag); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "hold removed"})
}

// ListZFSHolds GET /api/v1/snapshots/zfs/holds.
func (h *Handlers) ListZFSHolds(c *gin.Context) {
	snapshotID := c.Query("snapshot_id")
	if snapshotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "snapshot_id is required"})
		return
	}

	holds := h.zfsManager.ListHolds(snapshotID)
	c.JSON(http.StatusOK, gin.H{"holds": holds})
}

// ==================== 快照差异对比 ====================

// GetSnapshotDiff GET /api/v1/snapshots/diff.
func (h *Handlers) GetSnapshotDiff(c *gin.Context) {
	pool := c.Query("pool")
	dataset := c.Query("dataset")
	snapA := c.Query("snapshot_a")
	snapB := c.Query("snapshot_b")

	if pool == "" || snapA == "" || snapB == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pool, snapshot_a, and snapshot_b are required"})
		return
	}

	result, err := h.zfsManager.Diff(pool, dataset, snapA, snapB)
	if err != nil {
		h.logger.Error("failed to compute snapshot diff", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ==================== 配额管理 ====================

// GetQuotaStatus GET /api/v1/snapshots/quota.
func (h *Handlers) GetQuotaStatus(c *gin.Context) {
	status := h.quotaManager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// updateQuotaReq 更新配额请求.
type updateQuotaReq struct {
	MaxPercent float64 `json:"max_percent" binding:"required"`
	TotalBytes int64   `json:"total_bytes"`
}

// UpdateQuota PUT /api/v1/snapshots/quota.
func (h *Handlers) UpdateQuota(c *gin.Context) {
	var req updateQuotaReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.quotaManager.SetQuota(req.MaxPercent)
	if req.TotalBytes > 0 {
		h.quotaManager.SetTotalBytes(req.TotalBytes)
	}

	c.JSON(http.StatusOK, h.quotaManager.GetStatus())
}

// EnforceQuota POST /api/v1/snapshots/quota/enforce.
func (h *Handlers) EnforceQuota(c *gin.Context) {
	h.quotaManager.EnforceQuota()
	c.JSON(http.StatusOK, gin.H{"message": "quota enforcement completed"})
}

// ==================== 团队快照 ====================

// createTeamPolicyReq 创建团队快照策略请求.
type createTeamPolicyReq struct {
	TeamID           string `json:"team_id" binding:"required"`
	FolderPath       string `json:"folder_path" binding:"required"`
	PolicyName       string `json:"policy_name" binding:"required"`
	Enabled          bool   `json:"enabled"`
	Recursive        bool   `json:"recursive"`
	AutoCreate       bool   `json:"auto_create"`
	CronExpr         string `json:"cron_expr"`
	RetainHourly     int    `json:"retain_hourly"`
	RetainDaily      int    `json:"retain_daily"`
	RetainWeekly     int    `json:"retain_weekly"`
	RetainMonthly    int    `json:"retain_monthly"`
	OwnerVisible     bool   `json:"owner_visible"`
	MemberVisible    bool   `json:"member_visible"`
	GuestVisible     bool   `json:"guest_visible"`
	RestoreOwnerOnly bool   `json:"restore_owner_only"`
	DeleteAdminOnly  bool   `json:"delete_admin_only"`
}

// CreateTeamPolicy POST /api/v1/snapshots/team/policies.
func (h *Handlers) CreateTeamPolicy(c *gin.Context) {
	var req createTeamPolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := &TeamSnapshotPolicy{
		TeamID:        req.TeamID,
		FolderPath:    req.FolderPath,
		PolicyName:    req.PolicyName,
		Enabled:       req.Enabled,
		Recursive:     req.Recursive,
		AutoCreate:    req.AutoCreate,
		CronExpr:      req.CronExpr,
		RetainHourly:  req.RetainHourly,
		RetainDaily:   req.RetainDaily,
		RetainWeekly:  req.RetainWeekly,
		RetainMonthly: req.RetainMonthly,
		Visibility: TeamSnapshotVisibility{
			OwnerVisible:     req.OwnerVisible,
			MemberVisible:    req.MemberVisible,
			GuestVisible:     req.GuestVisible,
			AdminVisible:     true,
			RestoreOwnerOnly: req.RestoreOwnerOnly,
			DeleteAdminOnly:  req.DeleteAdminOnly,
		},
	}

	created, err := h.teamManager.CreatePolicy(policy)
	if err != nil {
		h.logger.Error("failed to create team policy", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// ListTeamPolicies GET /api/v1/snapshots/team/policies.
func (h *Handlers) ListTeamPolicies(c *gin.Context) {
	teamID := c.Query("team_id")
	if teamID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id is required"})
		return
	}

	policies := h.teamManager.ListPoliciesByTeam(teamID)
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// createTeamSnapshotReq 创建团队快照请求.
type createTeamSnapshotReq struct {
	TeamID     string `json:"team_id" binding:"required"`
	FolderPath string `json:"folder_path" binding:"required"`
	CreatedBy  string `json:"created_by" binding:"required"`
	Source     string `json:"source"`
}

// CreateTeamSnapshot POST /api/v1/snapshots/team/snapshots.
func (h *Handlers) CreateTeamSnapshot(c *gin.Context) {
	var req createTeamSnapshotReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	source := req.Source
	if source == "" {
		source = "manual"
	}

	snap, err := h.teamManager.CreateTeamSnapshot(req.TeamID, req.FolderPath, req.CreatedBy, source)
	if err != nil {
		h.logger.Error("failed to create team snapshot", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, snap)
}

// ListTeamSnapshots GET /api/v1/snapshots/team/snapshots.
func (h *Handlers) ListTeamSnapshots(c *gin.Context) {
	teamID := c.Query("team_id")
	userID := c.Query("user_id")
	userRole := c.DefaultQuery("user_role", "member")

	if teamID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id and user_id are required"})
		return
	}

	snapshots := h.teamManager.ListTeamSnapshots(teamID, userID, userRole)
	c.JSON(http.StatusOK, gin.H{"snapshots": snapshots})
}

// RestoreTeamSnapshot POST /api/v1/snapshots/team/snapshots/:id/restore.
func (h *Handlers) RestoreTeamSnapshot(c *gin.Context) {
	id := c.Param("id")
	userID := c.Query("user_id")
	userRole := c.DefaultQuery("user_role", "member")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if err := h.teamManager.RestoreTeamSnapshot(id, userID, userRole); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "team snapshot restore initiated"})
}

// DeleteTeamSnapshot DELETE /api/v1/snapshots/team/snapshots/:id.
func (h *Handlers) DeleteTeamSnapshot(c *gin.Context) {
	id := c.Param("id")
	userID := c.Query("user_id")
	userRole := c.DefaultQuery("user_role", "member")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	if err := h.teamManager.DeleteTeamSnapshot(id, userID, userRole); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "team snapshot deleted"})
}

// LockTeamSnapshot POST /api/v1/snapshots/team/snapshots/:id/lock.
func (h *Handlers) LockTeamSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.teamManager.LockSnapshot(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "team snapshot locked"})
}

// UnlockTeamSnapshot POST /api/v1/snapshots/team/snapshots/:id/unlock.
func (h *Handlers) UnlockTeamSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.teamManager.UnlockSnapshot(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "team snapshot unlocked"})
}
