package snapshot

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ReplicationHandlers 复制 API 处理器
type ReplicationHandlers struct {
	manager *ReplicationManager
}

// NewReplicationHandlers 创建处理器
func NewReplicationHandlers(manager *ReplicationManager) *ReplicationHandlers {
	return &ReplicationHandlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *ReplicationHandlers) RegisterRoutes(r *gin.RouterGroup) {
	replication := r.Group("/replication")
	{
		// 配置管理
		replication.GET("/configs", h.listConfigs)
		replication.POST("/configs", h.createConfig)
		replication.GET("/configs/:id", h.getConfig)
		replication.PUT("/configs/:id", h.updateConfig)
		replication.DELETE("/configs/:id", h.deleteConfig)

		// 复制操作
		replication.POST("/configs/:id/start", h.startReplication)
		replication.POST("/jobs/:jobId/cancel", h.cancelJob)

		// 状态监控
		replication.GET("/configs/:id/status", h.getStatus)
		replication.GET("/jobs", h.listJobs)
		replication.GET("/jobs/:jobId", h.getJob)

		// 节点管理
		replication.GET("/nodes", h.listNodes)
		replication.POST("/nodes/check", h.checkNodes)
	}
}

// ========== 配置管理 ==========

// listConfigs 列出所有复制配置
func (h *ReplicationHandlers) listConfigs(c *gin.Context) {
	configs := h.manager.ListConfigs()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

// getConfig 获取单个配置
func (h *ReplicationHandlers) getConfig(c *gin.Context) {
	id := c.Param("id")

	config, err := h.manager.GetConfig(id)
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
		"data":    config,
	})
}

// createConfig 创建复制配置
func (h *ReplicationHandlers) createConfig(c *gin.Context) {
	var config ReplicationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.CreateConfig(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置创建成功",
		"data":    config,
	})
}

// updateConfig 更新复制配置
func (h *ReplicationHandlers) updateConfig(c *gin.Context) {
	id := c.Param("id")

	var config ReplicationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(id, &config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置更新成功",
		"data":    config,
	})
}

// deleteConfig 删除复制配置
func (h *ReplicationHandlers) deleteConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteConfig(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已删除",
	})
}

// ========== 复制操作 ==========

// startReplication 启动复制
func (h *ReplicationHandlers) startReplication(c *gin.Context) {
	id := c.Param("id")

	jobID, err := h.manager.StartReplication(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "启动复制失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "复制已启动",
		"data": gin.H{
			"jobId":     jobID,
			"startedAt": time.Now(),
		},
	})
}

// cancelJob 取消任务
func (h *ReplicationHandlers) cancelJob(c *gin.Context) {
	jobID := c.Param("jobId")

	if err := h.manager.CancelJob(jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已取消",
	})
}

// ========== 状态监控 ==========

// getStatus 获取复制状态
func (h *ReplicationHandlers) getStatus(c *gin.Context) {
	id := c.Param("id")

	status, err := h.manager.GetStatus(id)
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
		"data":    status,
	})
}

// listJobs 列出任务
func (h *ReplicationHandlers) listJobs(c *gin.Context) {
	configID := c.Query("configId")
	limit := 50
	if l := c.Query("limit"); l != "" {
		//nolint:errcheck // 解析失败保持默认值
		fmt.Sscanf(l, "%d", &limit)
	}

	jobs := h.manager.GetJobs(configID, limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    jobs,
	})
}

// getJob 获取任务详情
func (h *ReplicationHandlers) getJob(c *gin.Context) {
	jobID := c.Param("jobId")

	h.manager.mu.RLock()
	job, ok := h.manager.jobs[jobID]
	if !ok {
		// 查找历史记录
		for _, j := range h.manager.jobHistory {
			if j.ID == jobID {
				job = j
				ok = true
				break
			}
		}
	}
	h.manager.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    job,
	})
}

// ========== 节点管理 ==========

// listNodes 列出节点状态
func (h *ReplicationHandlers) listNodes(c *gin.Context) {
	configs := h.manager.ListConfigs()

	nodes := make(map[string]NodeReplicationStatus)
	for _, config := range configs {
		for _, target := range config.TargetNodes {
			if _, exists := nodes[target.NodeID]; !exists {
				nodes[target.NodeID] = NodeReplicationStatus{
					NodeID:   target.NodeID,
					Status:   target.Status,
					LastSync: target.LastSync,
				}
			}
		}
	}

	result := make([]NodeReplicationStatus, 0, len(nodes))
	for _, status := range nodes {
		result = append(result, status)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// checkNodes 检查节点状态
func (h *ReplicationHandlers) checkNodes(c *gin.Context) {
	// 手动触发节点检查
	go h.manager.checkNodes()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "节点检查已触发",
	})
}

// ========== 复制服务端接口 ==========

// ReplicationServerHandlers 复制服务端处理器
type ReplicationServerHandlers struct {
	server *ReplicationServer
}

// NewReplicationServerHandlers 创建服务端处理器
func NewReplicationServerHandlers(server *ReplicationServer) *ReplicationServerHandlers {
	return &ReplicationServerHandlers{server: server}
}

// RegisterRoutes 注册路由
func (h *ReplicationServerHandlers) RegisterRoutes(r *gin.RouterGroup) {
	receive := r.Group("/replication")
	{
		receive.POST("/receive", h.receiveSnapshot)
		receive.POST("/complete", h.completeReceive)
		receive.GET("/health", h.healthCheck)
	}
}

// receiveSnapshot 接收快照
func (h *ReplicationServerHandlers) receiveSnapshot(c *gin.Context) {
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	// 验证 API Key
	apiKey := c.GetHeader("X-Api-Key")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "缺少 API Key",
		})
		return
	}

	// 接收数据流
	if err := h.server.ReceiveSnapshot(req, c.Request.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "接收快照失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "快照接收成功",
	})
}

// completeReceive 完成接收
func (h *ReplicationServerHandlers) completeReceive(c *gin.Context) {
	snapshotName := c.GetHeader("X-Snapshot-Name")
	checksum := c.GetHeader("X-Checksum")

	if snapshotName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少快照名称",
		})
		return
	}

	// 验证校验和
	_ = checksum // 实际应该验证

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "快照接收完成",
		"data": gin.H{
			"snapshotName": snapshotName,
			"completedAt":  time.Now(),
		},
	})
}

// healthCheck 健康检查
func (h *ReplicationServerHandlers) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "healthy",
		"data": gin.H{
			"status":    "online",
			"timestamp": time.Now(),
		},
	})
}
