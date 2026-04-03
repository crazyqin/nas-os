package cloudsync

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 云同步 API 处理器.
type Handlers struct {
	manager     *Manager
	oauth2Service *OAuth2Service
	realtimeSync *RealtimeSync
	resumableUpload *ResumableUpload
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// SetOAuth2Service 设置 OAuth2 服务.
func (h *Handlers) SetOAuth2Service(service *OAuth2Service) {
	h.oauth2Service = service
}

// SetRealtimeSync 设置实时同步服务.
func (h *Handlers) SetRealtimeSync(sync *RealtimeSync) {
	h.realtimeSync = sync
}

// SetResumableUpload 设置断点续传服务.
func (h *Handlers) SetResumableUpload(upload *ResumableUpload) {
	h.resumableUpload = upload
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cloudsync := r.Group("/cloudsync")
	{
		// 提供商管理
		cloudsync.POST("/providers", h.createProvider)
		cloudsync.GET("/providers", h.listProviders)
		cloudsync.GET("/providers/:id", h.getProvider)
		cloudsync.PUT("/providers/:id", h.updateProvider)
		cloudsync.DELETE("/providers/:id", h.deleteProvider)
		cloudsync.POST("/providers/:id/test", h.testProvider)

		// 同步任务管理
		cloudsync.POST("/tasks", h.createSyncTask)
		cloudsync.GET("/tasks", h.listSyncTasks)
		cloudsync.GET("/tasks/:id", h.getSyncTask)
		cloudsync.PUT("/tasks/:id", h.updateSyncTask)
		cloudsync.DELETE("/tasks/:id", h.deleteSyncTask)

		// 同步操作
		cloudsync.POST("/tasks/:id/run", h.runSyncTask)
		cloudsync.POST("/tasks/:id/pause", h.pauseSyncTask)
		cloudsync.POST("/tasks/:id/resume", h.resumeSyncTask)
		cloudsync.POST("/tasks/:id/cancel", h.cancelSyncTask)
		cloudsync.GET("/tasks/:id/status", h.getSyncStatus)

		// OAuth2 授权
		cloudsync.GET("/oauth2/auth-url/:providerType", h.getOAuth2AuthURL)
		cloudsync.POST("/oauth2/callback", h.handleOAuth2Callback)
		cloudsync.DELETE("/oauth2/token/:providerId", h.deleteOAuth2Token)
		cloudsync.GET("/oauth2/tokens", h.listOAuth2Tokens)

		// 实时同步
		cloudsync.GET("/realtime/status", h.getRealtimeSyncStatus)
		cloudsync.POST("/realtime/start", h.startRealtimeSync)
		cloudsync.POST("/realtime/stop", h.stopRealtimeSync)
		cloudsync.POST("/realtime/watch/:taskId", h.addRealtimeWatch)
		cloudsync.DELETE("/realtime/watch/:taskId", h.removeRealtimeWatch)

		// 断点续传
		cloudsync.GET("/resumable/status", h.getResumableUploadStatus)
		cloudsync.GET("/resumable/pending", h.getPendingUploads)
		cloudsync.POST("/resumable/resume/:fileId", h.resumeUpload)

		// 全局状态
		cloudsync.GET("/statuses", h.getAllStatuses)
		cloudsync.GET("/stats", h.getStats)
		cloudsync.GET("/providers-info", h.getProvidersInfo)
	}
}

// ==================== 提供商管理 ====================

func (h *Handlers) createProvider(c *gin.Context) {
	var config ProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	provider, err := h.manager.CreateProvider(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "提供商创建成功",
		"data":    provider,
	})
}

func (h *Handlers) getProvider(c *gin.Context) {
	id := c.Param("id")

	provider, err := h.manager.GetProvider(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    provider,
	})
}

func (h *Handlers) listProviders(c *gin.Context) {
	providers := h.manager.ListProviders()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    providers,
	})
}

func (h *Handlers) updateProvider(c *gin.Context) {
	id := c.Param("id")

	var config ProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.UpdateProvider(id, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "提供商更新成功",
	})
}

func (h *Handlers) deleteProvider(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteProvider(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "提供商已删除",
	})
}

func (h *Handlers) testProvider(c *gin.Context) {
	id := c.Param("id")

	result, err := h.manager.TestProvider(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// ==================== 同步任务管理 ====================

func (h *Handlers) createSyncTask(c *gin.Context) {
	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	createdTask, err := h.manager.CreateSyncTask(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务创建成功",
		"data":    createdTask,
	})
}

func (h *Handlers) getSyncTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetSyncTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

func (h *Handlers) listSyncTasks(c *gin.Context) {
	tasks := h.manager.ListSyncTasks()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

func (h *Handlers) updateSyncTask(c *gin.Context) {
	id := c.Param("id")

	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.UpdateSyncTask(id, task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务更新成功",
	})
}

func (h *Handlers) deleteSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已删除",
	})
}

// ==================== 同步操作 ====================

func (h *Handlers) runSyncTask(c *gin.Context) {
	id := c.Param("id")

	status, err := h.manager.RunSyncTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已启动",
		"data":    status,
	})
}

func (h *Handlers) pauseSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.PauseSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已暂停",
	})
}

func (h *Handlers) resumeSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResumeSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已恢复",
	})
}

func (h *Handlers) cancelSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已取消",
	})
}

func (h *Handlers) getSyncStatus(c *gin.Context) {
	id := c.Param("id")

	status, err := h.manager.GetSyncStatus(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// ==================== 全局状态 ====================

func (h *Handlers) getAllStatuses(c *gin.Context) {
	statuses := h.manager.GetAllSyncStatuses()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    statuses,
	})
}

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

func (h *Handlers) getProvidersInfo(c *gin.Context) {
	providers := SupportedProviders()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    providers,
	})
}

// ==================== OAuth2 授权 ====================

func (h *Handlers) getOAuth2AuthURL(c *gin.Context) {
	providerTypeStr := c.Param("providerType")
	providerType := ProviderType(providerTypeStr)

	redirectURL := c.Query("redirect_url")
	providerID := c.Query("provider_id")

	if h.oauth2Service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "OAuth2 服务未初始化",
		})
		return
	}

	authURL, state, err := h.oauth2Service.GenerateAuthURL(providerType, providerID, redirectURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"authUrl": authURL,
			"state":   state,
		},
	})
}

func (h *Handlers) handleOAuth2Callback(c *gin.Context) {
	var req struct {
		ProviderType string `json:"providerType" binding:"required"`
		ProviderID   string `json:"providerId" binding:"required"`
		Code         string `json:"code" binding:"required"`
		State        string `json:"state"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if h.oauth2Service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "OAuth2 服务未初始化",
		})
		return
	}

	token, err := h.oauth2Service.HandleAuthCallback(
		c.Request.Context(),
		ProviderType(req.ProviderType),
		req.ProviderID,
		req.Code,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "授权成功",
		"data": gin.H{
			"providerId":   token.ProviderID,
			"providerType": token.ProviderType,
			"expiresAt":    token.ExpiresAt,
		},
	})
}

func (h *Handlers) deleteOAuth2Token(c *gin.Context) {
	providerID := c.Param("providerId")

	if h.oauth2Service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "OAuth2 服务未初始化",
		})
		return
	}

	if err := h.oauth2Service.DeleteToken(providerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "令牌已删除",
	})
}

func (h *Handlers) listOAuth2Tokens(c *gin.Context) {
	if h.oauth2Service == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []interface{}{},
		})
		return
	}

	tokens := h.oauth2Service.ListTokens()

	// 不返回敏感信息
	result := make([]gin.H, len(tokens))
	for i, token := range tokens {
		result[i] = gin.H{
			"providerId":   token.ProviderID,
			"providerType": token.ProviderType,
			"expiresAt":    token.ExpiresAt,
			"updatedAt":    token.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// ==================== 实时同步 ====================

func (h *Handlers) getRealtimeSyncStatus(c *gin.Context) {
	if h.realtimeSync == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"running": false,
			},
		})
		return
	}

	status := h.realtimeSync.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

func (h *Handlers) startRealtimeSync(c *gin.Context) {
	if h.realtimeSync == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "实时同步服务未初始化",
		})
		return
	}

	if err := h.realtimeSync.Start(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "实时同步已启动",
	})
}

func (h *Handlers) stopRealtimeSync(c *gin.Context) {
	if h.realtimeSync == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "实时同步服务未初始化",
		})
		return
	}

	if err := h.realtimeSync.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "实时同步已停止",
	})
}

func (h *Handlers) addRealtimeWatch(c *gin.Context) {
	taskID := c.Param("taskId")

	if h.realtimeSync == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "实时同步服务未初始化",
		})
		return
	}

	task, err := h.manager.GetSyncTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if err := h.realtimeSync.AddWatch(task.LocalPath, taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "监控已添加",
	})
}

func (h *Handlers) removeRealtimeWatch(c *gin.Context) {
	taskID := c.Param("taskId")

	if h.realtimeSync == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "实时同步服务未初始化",
		})
		return
	}

	task, err := h.manager.GetSyncTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if err := h.realtimeSync.RemoveWatch(task.LocalPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "监控已移除",
	})
}

// ==================== 断点续传 ====================

func (h *Handlers) getResumableUploadStatus(c *gin.Context) {
	if h.resumableUpload == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"totalUploads": 0,
			},
		})
		return
	}

	stats := h.resumableUpload.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

func (h *Handlers) getPendingUploads(c *gin.Context) {
	if h.resumableUpload == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []interface{}{},
		})
		return
	}

	uploads := h.resumableUpload.GetPendingUploads()

	result := make([]gin.H, len(uploads))
	for i, upload := range uploads {
		result[i] = gin.H{
			"fileId":        upload.FileID,
			"localPath":     upload.LocalPath,
			"remotePath":    upload.RemotePath,
			"fileSize":      upload.FileSize,
			"uploadedSize":  upload.UploadedSize,
			"uploadedChunks": upload.UploadedChunks,
			"totalChunks":   upload.TotalChunks,
			"status":        upload.Status,
			"startTime":     upload.StartTime,
			"lastError":     upload.LastError,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

func (h *Handlers) resumeUpload(c *gin.Context) {
	fileID := c.Param("fileId")

	if h.resumableUpload == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "断点续传服务未初始化",
		})
		return
	}

	progress, err := h.resumableUpload.GetProgress(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if !h.resumableUpload.CanResume(fileID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "该上传无法恢复",
		})
		return
	}

	// 触发同步任务继续上传
	status, err := h.manager.RunSyncTask(progress.TaskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传已恢复",
		"data":    status,
	})
}
