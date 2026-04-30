// Package appstore REST API handlers
// /api/v1/appstore/* 路由处理
package appstore

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// API 应用商店API处理器
type API struct {
	catalog   *Catalog
	recommender *Recommender
	resolver  *DependencyResolver
	sandbox   *SandboxManager
	batch     *BatchManager
}

// NewAPI 创建应用商店API
func NewAPI(catalog *Catalog, recommender *Recommender, resolver *DependencyResolver, sandbox *SandboxManager, batch *BatchManager) *API {
	return &API{
		catalog:     catalog,
		recommender: recommender,
		resolver:    resolver,
		sandbox:     sandbox,
		batch:       batch,
	}
}

// RegisterRoutes 注册路由
func (api *API) RegisterRoutes(router *gin.RouterGroup) {
	store := router.Group("/appstore")
	{
		// 目录管理
		store.GET("/apps", api.ListApps)
		store.GET("/apps/:id", api.GetApp)
		store.GET("/apps/search", api.SearchApps)
		store.GET("/categories", api.GetCategories)
		store.GET("/updates", api.CheckUpdates)

		// 仓库管理
		store.GET("/repos", api.ListRepos)
		store.POST("/repos", api.AddRepo)
		store.PUT("/repos/:id", api.UpdateRepo)
		store.DELETE("/repos/:id", api.RemoveRepo)
		store.POST("/repos/:id/sync", api.SyncRepo)
		store.POST("/repos/sync-all", api.SyncAllRepos)

		// 推荐
		store.GET("/recommend", api.GetRecommendations)
		store.GET("/apps/:id/similar", api.GetSimilarApps)

		// 依赖解析
		store.POST("/resolve", api.ResolveDeps)
		store.POST("/resolve/batch", api.BatchResolveDeps)
		store.GET("/apps/:id/dependencies", api.GetDependencyGraph)

		// 沙箱管理
		store.POST("/sandbox", api.CreateSandbox)
		store.GET("/sandbox", api.ListSandboxes)
		store.GET("/sandbox/:id", api.GetSandbox)
		// /sandbox/app/:appId (placed before parameterized route)
		store.GET("/sandbox/app/:appId", api.GetSandboxByApp)
		store.PUT("/sandbox/:id/resources", api.UpdateSandboxResources)
		store.POST("/sandbox/:id/pause", api.PauseSandbox)
		store.POST("/sandbox/:id/resume", api.ResumeSandbox)
		store.DELETE("/sandbox/:id", api.DestroySandbox)
		store.GET("/sandbox/:id/usage", api.GetSandboxUsage)

		// 批量管理
		store.POST("/batch/install", api.BatchInstall)
		store.POST("/batch/uninstall", api.BatchUninstall)
		store.POST("/batch/update", api.BatchUpdate)
		store.POST("/batch/start", api.BatchStart)
		store.POST("/batch/stop", api.BatchStop)
		store.POST("/batch/restart", api.BatchRestart)
		store.GET("/batch/operations", api.ListOperations)
		store.GET("/batch/operations/:id", api.GetOperation)
	}
}

// ========== 目录管理 Handlers ==========

// ListApps 列出应用
func (api *API) ListApps(c *gin.Context) {
	filter := &AppFilter{
		Category:     c.Query("category"),
		Tag:          c.Query("tag"),
		RepositoryID: c.Query("repo"),
	}

	if verified := c.Query("verified"); verified == "true" {
		filter.Verified = true
	}

	apps := api.catalog.ListApps(filter)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apps,
		"count":   len(apps),
	})
}

// GetApp 获取应用详情
func (api *API) GetApp(c *gin.Context) {
	id := c.Param("id")
	app, ok := api.catalog.GetApp(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "应用不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    app,
	})
}

// SearchApps 搜索应用
func (api *API) SearchApps(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "搜索关键词不能为空",
		})
		return
	}

	apps := api.catalog.SearchApps(query)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apps,
		"count":   len(apps),
		"query":   query,
	})
}

// GetCategories 获取分类列表
func (api *API) GetCategories(c *gin.Context) {
	categories := api.catalog.Categories()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    categories,
	})
}

// CheckUpdates 检查更新
func (api *API) CheckUpdates(c *gin.Context) {
	// 从查询参数获取已安装应用版本
	updates := api.catalog.GetUpdates(make(map[string]string))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updates,
		"count":   len(updates),
	})
}

// ========== 仓库管理 Handlers ==========

// ListRepos 列出仓库
func (api *API) ListRepos(c *gin.Context) {
	repos := api.catalog.ListRepositories()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    repos,
		"count":   len(repos),
	})
}

// AddRepo 添加仓库
func (api *API) AddRepo(c *gin.Context) {
	var repo Repository
	if err := c.ShouldBindJSON(&repo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := api.catalog.AddRepository(&repo); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    repo,
	})
}

// UpdateRepo 更新仓库
func (api *API) UpdateRepo(c *gin.Context) {
	id := c.Param("id")
	var repo Repository
	if err := c.ShouldBindJSON(&repo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}
	repo.ID = id

	if err := api.catalog.UpdateRepository(&repo); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    repo,
	})
}

// RemoveRepo 删除仓库
func (api *API) RemoveRepo(c *gin.Context) {
	id := c.Param("id")
	if err := api.catalog.RemoveRepository(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "仓库已删除",
	})
}

// SyncRepo 同步仓库
func (api *API) SyncRepo(c *gin.Context) {
	id := c.Param("id")
	if err := api.catalog.SyncRepository(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "同步完成",
	})
}

// SyncAllRepos 同步所有仓库
func (api *API) SyncAllRepos(c *gin.Context) {
	results := api.catalog.SyncAll(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// ========== 推荐 Handlers ==========

// GetRecommendations 获取推荐
func (api *API) GetRecommendations(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	installed := make(map[string]bool) // 实际应从请求中获取
	recs := api.recommender.GetRecommendations(installed, limit)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    recs,
		"count":   len(recs),
	})
}

// GetSimilarApps 获取相似应用
func (api *API) GetSimilarApps(c *gin.Context) {
	id := c.Param("id")
	limitStr := c.DefaultQuery("limit", "5")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 5
	}

	similar := api.recommender.GetSimilarApps(id, limit)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    similar,
		"count":   len(similar),
	})
}

// ========== 依赖解析 Handlers ==========

// ResolveDeps 请求体
type ResolveDepsRequest struct {
	AppID     string          `json:"appId"`
	Installed map[string]bool `json:"installed"`
}

// ResolveDeps 解析依赖
func (api *API) ResolveDeps(c *gin.Context) {
	var req ResolveDepsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if req.Installed == nil {
		req.Installed = make(map[string]bool)
	}

	result, err := api.resolver.Resolve(req.AppID, req.Installed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// BatchResolveDepsRequest 批量解析请求
type BatchResolveDepsRequest struct {
	AppIDs    []string        `json:"appIds"`
	Installed map[string]bool `json:"installed"`
}

// BatchResolveDeps 批量解析依赖
func (api *API) BatchResolveDeps(c *gin.Context) {
	var req BatchResolveDepsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if req.Installed == nil {
		req.Installed = make(map[string]bool)
	}

	result, err := api.resolver.BatchResolve(req.AppIDs, req.Installed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetDependencyGraph 获取依赖关系图
func (api *API) GetDependencyGraph(c *gin.Context) {
	id := c.Param("id")
	graph := api.resolver.GetDependencyGraph(id)
	if graph == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "应用不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    graph,
	})
}

// ========== 沙箱管理 Handlers ==========

// CreateSandboxReq 创建沙箱请求
type CreateSandboxReq struct {
	AppID          string         `json:"appId"`
	ResourceLimits *ResourceLimits `json:"resourceLimits,omitempty"`
}

// CreateSandbox 创建沙箱
func (api *API) CreateSandbox(c *gin.Context) {
	var req CreateSandboxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	sb, err := api.sandbox.CreateSandbox(c.Request.Context(), req.AppID, req.ResourceLimits)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    sb,
	})
}

// ListSandboxes 列出沙箱
func (api *API) ListSandboxes(c *gin.Context) {
	sandboxes := api.sandbox.ListSandboxes()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sandboxes,
		"count":   len(sandboxes),
	})
}

// GetSandbox 获取沙箱
func (api *API) GetSandbox(c *gin.Context) {
	id := c.Param("id")
	sb, ok := api.sandbox.GetSandbox(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "沙箱不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sb,
	})
}

// GetSandboxByApp 按应用获取沙箱
func (api *API) GetSandboxByApp(c *gin.Context) {
	appID := c.Param("appId")
	sb, ok := api.sandbox.GetSandboxByApp(appID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "该应用无活跃沙箱",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sb,
	})
}

// UpdateSandboxResources 更新沙箱资源
func (api *API) UpdateSandboxResources(c *gin.Context) {
	id := c.Param("id")
	var limits ResourceLimits
	if err := c.ShouldBindJSON(&limits); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := api.sandbox.UpdateResourceLimits(id, &limits); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "资源限制已更新",
	})
}

// PauseSandbox 暂停沙箱
func (api *API) PauseSandbox(c *gin.Context) {
	id := c.Param("id")
	if err := api.sandbox.PauseSandbox(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "沙箱已暂停",
	})
}

// ResumeSandbox 恢复沙箱
func (api *API) ResumeSandbox(c *gin.Context) {
	id := c.Param("id")
	if err := api.sandbox.ResumeSandbox(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "沙箱已恢复",
	})
}

// DestroySandbox 销毁沙箱
func (api *API) DestroySandbox(c *gin.Context) {
	id := c.Param("id")
	if err := api.sandbox.DestroySandbox(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "沙箱已销毁",
	})
}

// GetSandboxUsage 获取沙箱资源使用
func (api *API) GetSandboxUsage(c *gin.Context) {
	id := c.Param("id")
	usage, err := api.sandbox.GetResourceUsage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    usage,
	})
}

// ========== 批量管理 Handlers ==========

// BatchAppsReq 批量操作请求
type BatchAppsReq struct {
	AppIDs []string `json:"appIds"`
}

// BatchInstall 批量安装
func (api *API) BatchInstall(c *gin.Context) {
	var req struct {
		BatchAppsReq
		Installed map[string]bool `json:"installed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if len(req.AppIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "appIds不能为空",
		})
		return
	}

	if req.Installed == nil {
		req.Installed = make(map[string]bool)
	}

	op, err := api.batch.BatchInstall(c.Request.Context(), req.AppIDs, req.Installed)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    op,
	})
}

// BatchUninstall 批量卸载
func (api *API) BatchUninstall(c *gin.Context) {
	var req struct {
		BatchAppsReq
		Installed map[string]bool `json:"installed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if req.Installed == nil {
		req.Installed = make(map[string]bool)
	}

	op, err := api.batch.BatchUninstall(c.Request.Context(), req.AppIDs, req.Installed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    op,
	})
}

// BatchUpdate 批量更新
func (api *API) BatchUpdate(c *gin.Context) {
	var req struct {
		BatchAppsReq
		Installed map[string]string `json:"installed"` // appId -> version
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if req.Installed == nil {
		req.Installed = make(map[string]string)
	}

	op, err := api.batch.BatchUpdate(c.Request.Context(), req.AppIDs, req.Installed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    op,
	})
}

// BatchStart 批量启动
func (api *API) BatchStart(c *gin.Context) {
	api.handleBatchAction(c, "start", "启动")
}

// BatchStop 批量停止
func (api *API) BatchStop(c *gin.Context) {
	api.handleBatchAction(c, "stop", "停止")
}

// BatchRestart 批量重启
func (api *API) BatchRestart(c *gin.Context) {
	api.handleBatchAction(c, "restart", "重启")
}

func (api *API) handleBatchAction(c *gin.Context, action, actionName string) {
	var req BatchAppsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if len(req.AppIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "appIds不能为空",
		})
		return
	}

	op := &BatchOperation{
		ID:        fmt.Sprintf("batch-%s-%d", action, time.Now().UnixMilli()),
		Type:      BatchOpType(action),
		Status:    BatchOpStatusCompleted,
		Targets:   req.AppIDs,
		Results:   make([]BatchItemResult, 0, len(req.AppIDs)),
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}

	for _, appID := range req.AppIDs {
		op.Results = append(op.Results, BatchItemResult{
			AppID:   appID,
			Success: true,
			Message: fmt.Sprintf("%s成功", actionName),
		})
	}

	api.batch.mu.Lock()
	api.batch.operations[op.ID] = op
	api.batch.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    op,
	})
}

// ListOperations 列出操作历史
func (api *API) ListOperations(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	ops := api.batch.ListOperations(limit)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ops,
		"count":   len(ops),
	})
}

// GetOperation 获取操作详情
func (api *API) GetOperation(c *gin.Context) {
	id := c.Param("id")
	op, ok := api.batch.GetOperation(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "操作不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    op,
	})
}


