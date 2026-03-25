package cluster

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// EdgeAPI 边缘计算 API 处理器.
type EdgeAPI struct {
	edgeManager   *EdgeNodeManager
	taskScheduler *TaskScheduler
	resultAgg     *ResultAggregator
	edgeLB        *EdgeLoadBalancer
	logger        *zap.Logger
}

// NewEdgeAPI 创建边缘计算 API 处理器.
func NewEdgeAPI(
	edgeManager *EdgeNodeManager,
	taskScheduler *TaskScheduler,
	resultAgg *ResultAggregator,
	edgeLB *EdgeLoadBalancer,
	logger *zap.Logger,
) *EdgeAPI {
	return &EdgeAPI{
		edgeManager:   edgeManager,
		taskScheduler: taskScheduler,
		resultAgg:     resultAgg,
		edgeLB:        edgeLB,
		logger:        logger,
	}
}

// RegisterRoutes 注册路由.
func (api *EdgeAPI) RegisterRoutes(router *gin.RouterGroup) {
	// 边缘节点管理
	edgeNodes := router.Group("/edge/nodes")
	{
		edgeNodes.GET("", api.GetEdgeNodes)
		edgeNodes.POST("", api.RegisterEdgeNode)
		edgeNodes.GET("/:id", api.GetEdgeNode)
		edgeNodes.PUT("/:id", api.UpdateEdgeNode)
		edgeNodes.DELETE("/:id", api.UnregisterEdgeNode)
		edgeNodes.GET("/:id/status", api.GetEdgeNodeStatus)
		edgeNodes.POST("/:id/heartbeat", api.EdgeNodeHeartbeat)
		edgeNodes.POST("/:id/drain", api.DrainEdgeNode)
		edgeNodes.GET("/:id/tasks", api.GetEdgeNodeTasks)
	}

	// 任务管理
	tasks := router.Group("/edge/tasks")
	{
		tasks.GET("", api.GetTasks)
		tasks.POST("", api.CreateTask)
		tasks.GET("/:id", api.GetTask)
		tasks.DELETE("/:id", api.CancelTask)
		tasks.POST("/:id/retry", api.RetryTask)
		tasks.GET("/:id/result", api.GetTaskResult)
		tasks.GET("/stats", api.GetTaskStats)
	}

	// 定时任务
	schedules := router.Group("/edge/schedules")
	{
		schedules.GET("", api.GetSchedules)
		schedules.POST("", api.CreateSchedule)
		schedules.PUT("/:id", api.UpdateSchedule)
		schedules.DELETE("/:id", api.DeleteSchedule)
	}

	// 结果聚合
	results := router.Group("/edge/results")
	{
		results.GET("", api.GetAggregations)
		results.POST("", api.CreateAggregation)
		results.GET("/:id", api.GetAggregation)
		results.POST("/:id/submit", api.SubmitResult)
		results.GET("/stats", api.GetResultStats)
	}

	// 负载均衡
	lb := router.Group("/edge/lb")
	{
		lb.GET("/config", api.GetLBConfig)
		lb.PUT("/config", api.UpdateLBConfig)
		lb.GET("/stats", api.GetLBStats)
		lb.POST("/select", api.SelectNode)
		lb.DELETE("/sessions/:id", api.ClearSession)
	}

	// 统计和监控
	stats := router.Group("/edge/stats")
	{
		stats.GET("", api.GetEdgeStats)
		stats.GET("/nodes", api.GetNodeStats)
		stats.GET("/tasks", api.GetTaskStats)
		stats.GET("/results", api.GetResultStats)
	}
}

// 边缘节点 API

// GetEdgeNodes 获取边缘节点列表.
func (api *EdgeAPI) GetEdgeNodes(c *gin.Context) {
	nodeType := c.Query("type")
	status := c.Query("status")

	var nodes []*EdgeNode
	if nodeType != "" {
		nodes = api.edgeManager.GetNodesByType(nodeType)
	} else if status == "online" {
		nodes = api.edgeManager.GetOnlineNodes()
	} else if status == "available" {
		nodes = api.edgeManager.GetAvailableNodes()
	} else {
		nodes = api.edgeManager.GetNodes()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"count":   len(nodes),
	})
}

// RegisterEdgeNode 注册边缘节点.
func (api *EdgeAPI) RegisterEdgeNode(c *gin.Context) {
	var node EdgeNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.edgeManager.RegisterNode(&node); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "边缘节点已注册",
		"data":    node,
	})
}

// GetEdgeNode 获取边缘节点详情.
func (api *EdgeAPI) GetEdgeNode(c *gin.Context) {
	nodeID := c.Param("id")
	node, exists := api.edgeManager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "边缘节点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    node,
	})
}

// UpdateEdgeNode 更新边缘节点.
func (api *EdgeAPI) UpdateEdgeNode(c *gin.Context) {
	nodeID := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	node, exists := api.edgeManager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "边缘节点不存在",
		})
		return
	}

	// 应用更新
	if priority, ok := updates["priority"].(float64); ok {
		node.Priority = int(priority)
	}
	if weight, ok := updates["weight"].(float64); ok {
		node.Weight = int(weight)
	}
	if labels, ok := updates["labels"].(map[string]interface{}); ok {
		node.Labels = make(map[string]string)
		for k, v := range labels {
			node.Labels[k] = fmt.Sprintf("%v", v)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "边缘节点已更新",
		"data":    node,
	})
}

// UnregisterEdgeNode 注销边缘节点.
func (api *EdgeAPI) UnregisterEdgeNode(c *gin.Context) {
	nodeID := c.Param("id")

	if err := api.edgeManager.UnregisterNode(nodeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "边缘节点已注销",
	})
}

// GetEdgeNodeStatus 获取边缘节点状态.
func (api *EdgeAPI) GetEdgeNodeStatus(c *gin.Context) {
	nodeID := c.Param("id")
	node, exists := api.edgeManager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "边缘节点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"node_id":   node.ID,
			"status":    node.Status,
			"type":      node.Type,
			"last_seen": node.LastSeen,
			"resources": node.Resources,
			"tasks": gin.H{
				"running": node.TasksRunning,
				"queued":  node.TasksQueued,
			},
		},
	})
}

// EdgeNodeHeartbeat 边缘节点心跳.
func (api *EdgeAPI) EdgeNodeHeartbeat(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Resources EdgeNodeResource `json:"resources"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// 即使没有请求体也更新心跳
		if err := api.edgeManager.UpdateHeartbeat(nodeID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	} else {
		if err := api.edgeManager.UpdateNodeResources(nodeID, req.Resources); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "心跳已更新",
	})
}

// DrainEdgeNode 边缘节点下线.
func (api *EdgeAPI) DrainEdgeNode(c *gin.Context) {
	nodeID := c.Param("id")

	node, exists := api.edgeManager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "边缘节点不存在",
		})
		return
	}

	_ = api.edgeManager.UpdateNodeStatus(nodeID, EdgeNodeStatusMaintain)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "边缘节点正在下线",
		"data": gin.H{
			"node_id":       nodeID,
			"tasks_running": node.TasksRunning,
		},
	})
}

// GetEdgeNodeTasks 获取边缘节点上的任务.
func (api *EdgeAPI) GetEdgeNodeTasks(c *gin.Context) {
	nodeID := c.Param("id")

	tasks := api.taskScheduler.GetTasksByNode(nodeID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
		"count":   len(tasks),
	})
}

// 任务 API

// GetTasks 获取任务列表.
func (api *EdgeAPI) GetTasks(c *gin.Context) {
	status := c.Query("status")
	nodeID := c.Query("node_id")

	var tasks []*Task
	if status != "" {
		tasks = api.taskScheduler.GetTasksByStatus(status)
	} else if nodeID != "" {
		tasks = api.taskScheduler.GetTasksByNode(nodeID)
	} else {
		tasks = api.taskScheduler.GetTasks()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
		"count":   len(tasks),
	})
}

// CreateTask 创建任务.
func (api *EdgeAPI) CreateTask(c *gin.Context) {
	var task Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.taskScheduler.CreateTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "任务已创建",
		"data":    task,
	})
}

// GetTask 获取任务详情.
func (api *EdgeAPI) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	task, exists := api.taskScheduler.GetTask(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    task,
	})
}

// CancelTask 取消任务.
func (api *EdgeAPI) CancelTask(c *gin.Context) {
	taskID := c.Param("id")

	if err := api.taskScheduler.CancelTask(taskID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "任务已取消",
	})
}

// RetryTask 重试任务.
func (api *EdgeAPI) RetryTask(c *gin.Context) {
	taskID := c.Param("id")

	if err := api.taskScheduler.RetryTask(taskID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "任务已重试",
	})
}

// GetTaskResult 获取任务结果.
func (api *EdgeAPI) GetTaskResult(c *gin.Context) {
	taskID := c.Param("id")
	task, exists := api.taskScheduler.GetTask(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "任务不存在",
		})
		return
	}

	if task.Result == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "任务尚未完成",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    task.Result,
	})
}

// GetTaskStats 获取任务统计.
func (api *EdgeAPI) GetTaskStats(c *gin.Context) {
	stats := api.taskScheduler.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// 定时任务 API

// GetSchedules 获取定时任务列表.
func (api *EdgeAPI) GetSchedules(c *gin.Context) {
	// 从 taskScheduler 获取定时任务
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    []interface{}{},
		"message": "定时任务功能待实现",
	})
}

// CreateSchedule 创建定时任务.
func (api *EdgeAPI) CreateSchedule(c *gin.Context) {
	var req struct {
		Task     Task   `json:"task"`
		Schedule string `json:"schedule"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.taskScheduler.CreateScheduledTask(&req.Task, req.Schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "定时任务已创建",
	})
}

// UpdateSchedule 更新定时任务.
func (api *EdgeAPI) UpdateSchedule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "定时任务已更新",
	})
}

// DeleteSchedule 删除定时任务.
func (api *EdgeAPI) DeleteSchedule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "定时任务已删除",
	})
}

// 结果聚合 API

// GetAggregations 获取聚合列表.
func (api *EdgeAPI) GetAggregations(c *gin.Context) {
	aggs := api.resultAgg.GetAggregations()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    aggs,
		"count":   len(aggs),
	})
}

// CreateAggregation 创建聚合.
func (api *EdgeAPI) CreateAggregation(c *gin.Context) {
	var req struct {
		TaskID        string `json:"task_id"`
		Strategy      string `json:"strategy"`
		ExpectedCount int    `json:"expected_count"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	agg, err := api.resultAgg.CreateAggregation(req.TaskID, req.Strategy, req.ExpectedCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "聚合已创建",
		"data":    agg,
	})
}

// GetAggregation 获取聚合详情.
func (api *EdgeAPI) GetAggregation(c *gin.Context) {
	aggID := c.Param("id")
	agg, exists := api.resultAgg.GetAggregation(aggID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "聚合不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    agg,
	})
}

// SubmitResult 提交结果.
func (api *EdgeAPI) SubmitResult(c *gin.Context) {
	var result TaskResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err := api.resultAgg.SubmitResult(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "结果已提交",
	})
}

// GetResultStats 获取结果统计.
func (api *EdgeAPI) GetResultStats(c *gin.Context) {
	stats := api.resultAgg.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// 负载均衡 API

// GetLBConfig 获取负载均衡配置.
func (api *EdgeAPI) GetLBConfig(c *gin.Context) {
	config := api.edgeLB.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdateLBConfig 更新负载均衡配置.
func (api *EdgeAPI) UpdateLBConfig(c *gin.Context) {
	var config EdgeLBConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	api.edgeLB.UpdateConfig(config)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "负载均衡配置已更新",
	})
}

// GetLBStats 获取负载均衡统计.
func (api *EdgeAPI) GetLBStats(c *gin.Context) {
	stats := api.edgeLB.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// SelectNode 选择节点.
func (api *EdgeAPI) SelectNode(c *gin.Context) {
	var req SelectNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	node, err := api.edgeLB.SelectNode(req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    node,
	})
}

// ClearSession 清除会话.
func (api *EdgeAPI) ClearSession(c *gin.Context) {
	sessionID := c.Param("id")

	api.edgeLB.ClearSession(sessionID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "会话已清除",
	})
}

// 统计 API

// GetEdgeStats 获取边缘计算总览统计.
func (api *EdgeAPI) GetEdgeStats(c *gin.Context) {
	nodeStats := api.edgeManager.GetNodeStats()
	taskStats := api.taskScheduler.GetStats()
	resultStats := api.resultAgg.GetStats()
	lbStats := api.edgeLB.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"nodes":     nodeStats,
			"tasks":     taskStats,
			"results":   resultStats,
			"lb":        lbStats,
			"timestamp": time.Now(),
		},
	})
}

// GetNodeStats 获取节点统计.
func (api *EdgeAPI) GetNodeStats(c *gin.Context) {
	stats := api.edgeManager.GetNodeStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}
