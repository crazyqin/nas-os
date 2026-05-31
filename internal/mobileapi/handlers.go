// Package mobileapi 提供移动端远程管理API服务
package mobileapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 移动端API处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// NewHandlersCompat 兼容旧接口创建处理器.
// Deprecated: 使用 NewHandlers(manager) 替代.
func NewHandlersCompat(authService *AuthService, pushService *PushService, syncService *SyncService) *Handlers {
	manager := &Manager{
		authService: authService,
		pushService: pushService,
		syncService: syncService,
		preferences: make(map[string]*NotificationPreference),
		history:     make([]*NotificationHistoryItem, 0),
		conflicts:   make(map[string]*ConflictRecord),
		bindings:    make(map[string]*DeviceBinding),
		maxHistory:  5000,
	}
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	mobile := r.Group("/mobile-api")
	{
		// 认证接口
		auth := mobile.Group("/auth")
		{
			auth.POST("/register", h.registerDevice)
			auth.POST("/login", h.login)
			auth.POST("/refresh", h.refreshToken)
			auth.POST("/logout", h.logout)
		}

		// 设备管理（需要认证）
		devices := mobile.Group("/devices")
		devices.Use(h.authMiddleware())
		{
			devices.GET("", h.listDevices)
			devices.GET("/:id", h.getDevice)
			devices.PUT("/:id", h.updateDevice)
			devices.DELETE("/:id", h.removeDevice)
			devices.POST("/:id/block", h.blockDevice)
			devices.POST("/:id/unblock", h.unblockDevice)
			devices.GET("/bindings", h.listBindings)
		}

		// 远程控制（需要认证）
		control := mobile.Group("/control")
		control.Use(h.authMiddleware())
		{
			control.POST("/shutdown", h.shutdown)
			control.POST("/reboot", h.reboot)
			control.POST("/sleep", h.sleep)
			control.GET("/status", h.systemStatus)
		}

		// 文件访问（需要认证）
		files := mobile.Group("/files")
		files.Use(h.authMiddleware())
		{
			files.GET("/list", h.listFiles)
			files.GET("/info", h.getFileInfo)
			files.GET("/download", h.downloadFile)
			files.POST("/upload", h.uploadFile)
			files.DELETE("/delete", h.deleteFile)
			files.POST("/mkdir", h.createDirectory)
			files.POST("/process-image", h.processImage)
		}

		// 推送通知（需要认证）
		push := mobile.Group("/push")
		push.Use(h.authMiddleware())
		{
			push.POST("/send", h.sendPush)
			push.POST("/broadcast", h.broadcastPush)
			push.GET("/history", h.pushHistory)
		}

		// 通知管理（需要认证）
		notification := mobile.Group("/notifications")
		notification.Use(h.authMiddleware())
		{
			notification.GET("/history", h.notificationHistory)
			notification.GET("/unread-count", h.unreadCount)
			notification.POST("/read", h.markNotificationsRead)
			notification.POST("/read-all", h.markAllNotificationsRead)
			notification.GET("/preferences", h.getNotificationPreferences)
			notification.PUT("/preferences", h.updateNotificationPreference)
		}

		// 数据同步（需要认证）
		sync := mobile.Group("/sync")
		sync.Use(h.authMiddleware())
		{
			sync.POST("/start", h.startSync)
			sync.POST("/stop", h.stopSync)
			sync.GET("/status", h.syncStatus)
			sync.GET("/items", h.listSyncItems)
			sync.POST("/config", h.updateSyncConfig)
			sync.GET("/config", h.getSyncConfig)
			sync.GET("/delta", h.syncDelta)
			sync.GET("/conflicts", h.listConflicts)
			sync.POST("/conflicts/:id/resolve", h.resolveConflict)
		}
	}
}

// response 标准API响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// authMiddleware JWT认证中间件.
func (h *Handlers) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header获取令牌
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, response{
				Code:    401,
				Message: "missing authorization header",
			})
			c.Abort()
			return
		}

		// 解析Bearer令牌
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			c.JSON(http.StatusUnauthorized, response{
				Code:    401,
				Message: "invalid authorization format",
			})
			c.Abort()
			return
		}

		tokenString := authHeader[7:]
		claims, err := h.manager.authService.ValidateToken(tokenString)
		if err != nil {
			code := http.StatusUnauthorized
			message := "invalid token"

			switch err {
			case ErrTokenExpired:
				message = "token expired"
			case ErrTokenRevoked:
				message = "token revoked"
			case ErrDeviceBlocked:
				code = http.StatusForbidden
				message = "device blocked"
			}

			c.JSON(code, response{
				Code:    code,
				Message: message,
			})
			c.Abort()
			return
		}

		// 设置上下文
		c.Set("userId", claims.UserID)
		c.Set("deviceId", claims.DeviceID)
		c.Next()
	}
}

// DeviceRegisterRequest 设备注册请求.
type DeviceRegisterRequest struct {
	UserID       string       `json:"userId" binding:"required"`
	DeviceName   string       `json:"deviceName" binding:"required"`
	Platform     Platform     `json:"platform" binding:"required"`
	OSVersion    string       `json:"osVersion"`
	AppVersion   string       `json:"appVersion"`
	PushToken    string       `json:"pushToken"`
	PushProvider PushProvider `json:"pushProvider"`
}

// registerDevice 注册设备.
func (h *Handlers) registerDevice(c *gin.Context) {
	var req DeviceRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	device := &MobileDevice{
		UserID:       req.UserID,
		DeviceName:   req.DeviceName,
		Platform:     req.Platform,
		OSVersion:    req.OSVersion,
		AppVersion:   req.AppVersion,
		PushToken:    req.PushToken,
		PushProvider: req.PushProvider,
	}

	token, err := h.manager.RegisterDevice(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "registration failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "device registered",
		Data: gin.H{
			"device": device,
			"token":  token,
		},
	})
}

// LoginRequest 登录请求.
type LoginRequest struct {
	DeviceID string `json:"deviceId" binding:"required"`
	UserID   string `json:"userId" binding:"required"`
}

// login 设备登录.
func (h *Handlers) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	token, err := h.manager.authService.Authenticate(req.DeviceID, req.UserID)
	if err != nil {
		code := http.StatusUnauthorized
		if err == ErrDeviceBlocked {
			code = http.StatusForbidden
		}
		c.JSON(code, response{
			Code:    code,
			Message: "authentication failed: " + err.Error(),
		})
		return
	}

	// 创建会话
	session := h.manager.authService.CreateSession(
		req.UserID,
		req.DeviceID,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "login successful",
		Data: gin.H{
			"token":   token,
			"session": session,
		},
	})
}

// RefreshRequest 刷新令牌请求.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// refreshToken 刷新访问令牌.
func (h *Handlers) refreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	token, err := h.manager.authService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		code := http.StatusUnauthorized
		message := "refresh failed"

		switch err {
		case ErrRefreshExpired:
			message = "refresh token expired"
		case ErrTokenRevoked:
			message = "refresh token revoked"
		case ErrDeviceBlocked:
			code = http.StatusForbidden
			message = "device blocked"
		}

		c.JSON(code, response{
			Code:    code,
			Message: message,
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "token refreshed",
		Data:    token,
	})
}

// LogoutRequest 登出请求.
type LogoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// logout 设备登出.
func (h *Handlers) logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.authService.RevokeToken(req.RefreshToken); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "logout failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "logged out",
	})
}

// listDevices 列出用户设备.
func (h *Handlers) listDevices(c *gin.Context) {
	userID := c.GetString("userId")
	devices := h.manager.authService.ListDevices(userID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(devices),
			"devices": devices,
		},
	})
}

// getDevice 获取设备详情.
func (h *Handlers) getDevice(c *gin.Context) {
	deviceID := c.Param("id")
	device, ok := h.manager.authService.GetDevice(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "device not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    device,
	})
}

// UpdateDeviceRequest 更新设备请求.
type UpdateDeviceRequest struct {
	DeviceName string `json:"deviceName"`
	AppVersion string `json:"appVersion"`
	PushToken  string `json:"pushToken"`
}

// updateDevice 更新设备信息.
func (h *Handlers) updateDevice(c *gin.Context) {
	deviceID := c.Param("id")

	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	device, ok := h.manager.authService.GetDevice(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "device not found",
		})
		return
	}

	// 更新字段
	if req.DeviceName != "" {
		device.DeviceName = req.DeviceName
	}
	if req.AppVersion != "" {
		device.AppVersion = req.AppVersion
	}
	if req.PushToken != "" {
		device.PushToken = req.PushToken
	}
	device.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "device updated",
		Data:    device,
	})
}

// removeDevice 移除设备.
func (h *Handlers) removeDevice(c *gin.Context) {
	deviceID := c.Param("id")

	if err := h.manager.RemoveDevice(deviceID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "device removed",
	})
}

// blockDevice 封禁设备.
func (h *Handlers) blockDevice(c *gin.Context) {
	deviceID := c.Param("id")

	if err := h.manager.authService.BlockDevice(deviceID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "device blocked",
	})
}

// unblockDevice 解封设备.
func (h *Handlers) unblockDevice(c *gin.Context) {
	deviceID := c.Param("id")

	if err := h.manager.authService.UnblockDevice(deviceID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "device unblocked",
	})
}

// shutdown 关机.
func (h *Handlers) shutdown(c *gin.Context) {
	// TODO: 实现关机逻辑
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "shutdown initiated",
	})
}

// reboot 重启.
func (h *Handlers) reboot(c *gin.Context) {
	// TODO: 实现重启逻辑
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "reboot initiated",
	})
}

// sleep 休眠.
func (h *Handlers) sleep(c *gin.Context) {
	// TODO: 实现休眠逻辑
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sleep initiated",
	})
}

// systemStatus 获取系统状态.
func (h *Handlers) systemStatus(c *gin.Context) {
	// TODO: 实现系统状态获取
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"status":     "running",
			"uptime":     "72h30m",
			"cpuUsage":   25.5,
			"memUsage":   45.2,
			"diskUsage":  68.7,
		},
	})
}

// ListFilesRequest 列出文件请求.
type ListFilesRequest struct {
	Path     string `form:"path" binding:"required"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=50"`
}

// listFiles 列出文件.
func (h *Handlers) listFiles(c *gin.Context) {
	var req ListFilesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// TODO: 实现文件列表获取
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"path":  req.Path,
			"files": []interface{}{},
			"total": 0,
		},
	})
}

// getFileInfo 获取文件信息.
func (h *Handlers) getFileInfo(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "path is required",
		})
		return
	}

	// TODO: 实现文件信息获取
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"path": path,
		},
	})
}

// downloadFile 下载文件.
func (h *Handlers) downloadFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "path is required",
		})
		return
	}

	// TODO: 实现文件下载
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "download initiated",
		Data: gin.H{
			"path": path,
		},
	})
}

// uploadFile 上传文件.
func (h *Handlers) uploadFile(c *gin.Context) {
	// TODO: 实现文件上传
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "upload completed",
	})
}

// deleteFile 删除文件.
func (h *Handlers) deleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "path is required",
		})
		return
	}

	// TODO: 实现文件删除
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "file deleted",
	})
}

// createDirectory 创建目录.
func (h *Handlers) createDirectory(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "path is required",
		})
		return
	}

	// TODO: 实现目录创建
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "directory created",
	})
}

// SendPushRequest 发送推送请求.
type SendPushRequest struct {
	DeviceID string            `json:"deviceId" binding:"required"`
	Title    string            `json:"title" binding:"required"`
	Body     string            `json:"body" binding:"required"`
	Data     map[string]string `json:"data,omitempty"`
	Image    string            `json:"image,omitempty"`
	Priority string            `json:"priority,omitempty"`
	Badge    int               `json:"badge,omitempty"`
}

// sendPush 发送推送通知.
func (h *Handlers) sendPush(c *gin.Context) {
	var req SendPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	notification := &PushNotification{
		DeviceID:  req.DeviceID,
		Title:     req.Title,
		Body:      req.Body,
		Data:      req.Data,
		Image:     req.Image,
		Priority:  req.Priority,
		Badge:     req.Badge,
		CreatedAt: time.Now(),
	}

	if err := h.manager.pushService.Send(notification); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "send push failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "push sent",
		Data:    notification,
	})
}

// BroadcastPushRequest 广播推送请求.
type BroadcastPushRequest struct {
	Title    string            `json:"title" binding:"required"`
	Body     string            `json:"body" binding:"required"`
	Data     map[string]string `json:"data,omitempty"`
	Image    string            `json:"image,omitempty"`
	Priority string            `json:"priority,omitempty"`
}

// broadcastPush 广播推送.
func (h *Handlers) broadcastPush(c *gin.Context) {
	var req BroadcastPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	userID := c.GetString("userId")
	devices := h.manager.authService.ListDevices(userID)

	sent := 0
	for _, device := range devices {
		if device.PushToken == "" {
			continue
		}

		notification := &PushNotification{
			DeviceID:  device.ID,
			Provider:  device.PushProvider,
			Title:     req.Title,
			Body:      req.Body,
			Data:      req.Data,
			Image:     req.Image,
			Priority:  req.Priority,
			CreatedAt: time.Now(),
		}

		if err := h.manager.pushService.Send(notification); err == nil {
			sent++
		}
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "broadcast sent",
		Data: gin.H{
			"totalDevices": len(devices),
			"sent":         sent,
		},
	})
}

// pushHistory 获取推送历史.
func (h *Handlers) pushHistory(c *gin.Context) {
	history := h.manager.pushService.GetHistory()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// startSync 开始同步.
func (h *Handlers) startSync(c *gin.Context) {
	userID := c.GetString("userId")
	deviceID := c.GetString("deviceId")

	if err := h.manager.syncService.StartSync(userID, deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: "start sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sync started",
	})
}

// stopSync 停止同步.
func (h *Handlers) stopSync(c *gin.Context) {
	h.manager.syncService.StopSync()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sync stopped",
	})
}

// syncStatus 获取同步状态.
func (h *Handlers) syncStatus(c *gin.Context) {
	stats := h.manager.syncService.GetStats()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// listSyncItems 列出同步项.
func (h *Handlers) listSyncItems(c *gin.Context) {
	userID := c.GetString("userId")
	items := h.manager.syncService.ListItems(userID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(items),
			"items": items,
		},
	})
}

// updateSyncConfig 更新同步配置.
func (h *Handlers) updateSyncConfig(c *gin.Context) {
	var config SyncConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.syncService.UpdateConfig(&config)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sync config updated",
		Data:    config,
	})
}

// getSyncConfig 获取同步配置.
func (h *Handlers) getSyncConfig(c *gin.Context) {
	config := h.manager.syncService.GetConfig()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    config,
	})
}

// ========== 设备绑定 ==========

// listBindings 列出设备绑定.
func (h *Handlers) listBindings(c *gin.Context) {
	userID := c.GetString("userId")
	bindings := h.manager.ListBindings(userID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(bindings),
			"bindings": bindings,
		},
	})
}

// ========== 通知管理 ==========

// notificationHistory 获取通知历史.
func (h *Handlers) notificationHistory(c *gin.Context) {
	userID := c.GetString("userId")
	limit := 50
	offset := 0

	if v := c.Query("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if v := c.Query("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}

	history := h.manager.GetNotificationHistory(userID, limit, offset)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// unreadCount 获取未读通知数量.
func (h *Handlers) unreadCount(c *gin.Context) {
	userID := c.GetString("userId")
	count := h.manager.GetUnreadCount(userID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"count": count,
		},
	})
}

// markNotificationsRead 标记通知已读.
func (h *Handlers) markNotificationsRead(c *gin.Context) {
	var req NotificationReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	userID := c.GetString("userId")
	count := h.manager.MarkNotificationRead(userID, req.IDs)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "notifications marked as read",
		Data: gin.H{
			"count": count,
		},
	})
}

// markAllNotificationsRead 标记所有通知已读.
func (h *Handlers) markAllNotificationsRead(c *gin.Context) {
	userID := c.GetString("userId")
	count := h.manager.MarkAllNotificationsRead(userID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "all notifications marked as read",
		Data: gin.H{
			"count": count,
		},
	})
}

// getNotificationPreferences 获取通知偏好.
func (h *Handlers) getNotificationPreferences(c *gin.Context) {
	userID := c.GetString("userId")
	deviceID := c.GetString("deviceId")
	prefs := h.manager.ListNotificationPreferences(userID, deviceID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"preferences": prefs,
		},
	})
}

// updateNotificationPreference 更新通知偏好.
func (h *Handlers) updateNotificationPreference(c *gin.Context) {
	var pref NotificationPreference
	if err := c.ShouldBindJSON(&pref); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pref.UserID = c.GetString("userId")
	pref.DeviceID = c.GetString("deviceId")
	h.manager.SetNotificationPreference(&pref)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "preference updated",
		Data:    pref,
	})
}

// ========== 离线同步 ==========

// syncDelta 获取增量同步数据.
func (h *Handlers) syncDelta(c *gin.Context) {
	userID := c.GetString("userId")
	deviceID := c.GetString("deviceId")

	var lastSyncTime time.Time
	if v := c.Query("lastSync"); v != "" {
		lastSyncTime, _ = time.Parse(time.RFC3339, v)
	}

	delta := h.manager.GetSyncDelta(userID, deviceID, lastSyncTime)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    delta,
	})
}

// listConflicts 列出同步冲突.
func (h *Handlers) listConflicts(c *gin.Context) {
	userID := c.GetString("userId")
	conflicts := h.manager.ListConflicts(userID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(conflicts),
			"conflicts": conflicts,
		},
	})
}

// ResolveConflictRequest 解决冲突请求.
type ResolveConflictRequest struct {
	Resolution ConflictResolution `json:"resolution" binding:"required"`
}

// resolveConflict 解决同步冲突.
func (h *Handlers) resolveConflict(c *gin.Context) {
	conflictID := c.Param("id")

	var req ResolveConflictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.ResolveConflict(conflictID, req.Resolution); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "conflict resolved",
	})
}

// ========== 图片处理 ==========

// processImage 处理图片.
func (h *Handlers) processImage(c *gin.Context) {
	var req ImageProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// TODO: 实现图片处理逻辑
	result := &ImageProcessResult{
		OriginalPath:  req.Path,
		ProcessedPath: req.Path,
		Format:        string(req.Format),
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "image processed",
		Data:    result,
	})
}
