// Package websocket 提供消息队列管理 API
// Version: v2.45.0 - 消息队列监控和管理接口
package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// QueueHandlers 消息队列处理器
type QueueHandlers struct {
	queue *EnhancedMessageQueue
}

// NewQueueHandlers 创建消息队列处理器
func NewQueueHandlers(queue *EnhancedMessageQueue) *QueueHandlers {
	return &QueueHandlers{
		queue: queue,
	}
}

// RegisterRoutes 注册路由
func (h *QueueHandlers) RegisterRoutes(r *gin.RouterGroup) {
	mq := r.Group("/message-queue")
	{
		// 状态和统计
		mq.GET("/stats", h.getStats)

		// 配置
		mq.GET("/config", h.getConfig)
		mq.PUT("/config", h.updateConfig)

		// 操作
		mq.POST("/flush", h.flushQueue)
		mq.POST("/push", h.pushMessage)
		mq.POST("/batch-push", h.batchPushMessages)

		// 背压控制
		mq.GET("/backpressure", h.getBackpressureStats)
		mq.PUT("/backpressure/threshold", h.setBackpressureThreshold)

		// 去重控制
		mq.GET("/dedup/stats", h.getDedupStats)
		mq.DELETE("/dedup/cache", h.clearDedupCache)
	}
}

// getStats 获取队列统计
// @Summary 获取队列统计
// @Description 获取消息队列的详细统计信息
// @Tags message-queue
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/stats [get]
// @Security BearerAuth
func (h *QueueHandlers) getStats(c *gin.Context) {
	stats := h.queue.Stats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getConfig 获取队列配置
// @Summary 获取队列配置
// @Description 获取消息队列的当前配置
// @Tags message-queue
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/config [get]
// @Security BearerAuth
func (h *QueueHandlers) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.queue.config,
	})
}

// updateConfig 更新队列配置
// @Summary 更新队列配置
// @Description 更新消息队列配置（部分字段支持热更新）
// @Tags message-queue
// @Accept json
// @Produce json
// @Param request body MessageQueueConfig true "配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/config [put]
// @Security BearerAuth
func (h *QueueHandlers) updateConfig(c *gin.Context) {
	var req struct {
		EnableDedup           *bool    `json:"enableDedup,omitempty"`
		DedupTTL              *string  `json:"dedupTTL,omitempty"`
		EnableBackpressure    *bool    `json:"enableBackpressure,omitempty"`
		BackpressureThreshold *float64 `json:"backpressureThreshold,omitempty"`
		BackpressureStrategy  *string  `json:"backpressureStrategy,omitempty"`
		DefaultTTL            *string  `json:"defaultTTL,omitempty"`
		MaxRetries            *int     `json:"maxRetries,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 更新配置
	if req.EnableDedup != nil {
		h.queue.config.EnableDedup = *req.EnableDedup
	}
	if req.EnableBackpressure != nil {
		h.queue.config.EnableBackpressure = *req.EnableBackpressure
	}
	if req.BackpressureThreshold != nil {
		h.queue.config.BackpressureThreshold = *req.BackpressureThreshold
	}
	if req.BackpressureStrategy != nil {
		h.queue.config.BackpressureStrategy = *req.BackpressureStrategy
	}
	if req.MaxRetries != nil {
		h.queue.config.MaxRetries = *req.MaxRetries
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
		"data":    h.queue.config,
	})
}

// flushQueue 清空队列
// @Summary 清空队列
// @Description 清空消息队列中的所有消息
// @Tags message-queue
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/flush [post]
// @Security BearerAuth
func (h *QueueHandlers) flushQueue(c *gin.Context) {
	count := h.queue.Flush()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "队列已清空",
		"data": gin.H{
			"flushedCount": count,
		},
	})
}

// pushMessage 推送消息
// @Summary 推送消息
// @Description 向队列推送单条消息
// @Tags message-queue
// @Accept json
// @Produce json
// @Param request body Message true "消息"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/push [post]
// @Security BearerAuth
func (h *QueueHandlers) pushMessage(c *gin.Context) {
	var msg Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.queue.Push(&msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "消息已推送",
		"data": gin.H{
			"messageId": msg.ID,
		},
	})
}

// batchPushMessages 批量推送消息
// @Summary 批量推送消息
// @Description 向队列批量推送消息
// @Tags message-queue
// @Accept json
// @Produce json
// @Param request body []Message true "消息列表"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/batch-push [post]
// @Security BearerAuth
func (h *QueueHandlers) batchPushMessages(c *gin.Context) {
	var messages []*Message
	if err := c.ShouldBindJSON(&messages); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	successCount, errors := h.queue.BatchPush(messages)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量推送完成",
		"data": gin.H{
			"total":   len(messages),
			"success": successCount,
			"failed":  len(messages) - successCount,
			"errors":  errors,
		},
	})
}

// getBackpressureStats 获取背压统计
// @Summary 获取背压统计
// @Description 获取背压控制器的详细统计
// @Tags message-queue
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/backpressure [get]
// @Security BearerAuth
func (h *QueueHandlers) getBackpressureStats(c *gin.Context) {
	if h.queue.backpressure == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "背压控制未启用",
			"data":    nil,
		})
		return
	}

	stats := h.queue.backpressure.Stats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// setBackpressureThreshold 设置背压阈值
// @Summary 设置背压阈值
// @Description 设置背压触发的阈值
// @Tags message-queue
// @Accept json
// @Produce json
// @Param request body map[string]float64 true "阈值配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/backpressure/threshold [put]
// @Security BearerAuth
func (h *QueueHandlers) setBackpressureThreshold(c *gin.Context) {
	var req struct {
		Threshold float64 `json:"threshold"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Threshold < 0 || req.Threshold > 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "阈值必须在 0-1 之间",
		})
		return
	}

	h.queue.config.BackpressureThreshold = req.Threshold

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "阈值已更新",
		"data": gin.H{
			"threshold": req.Threshold,
		},
	})
}

// getDedupStats 获取去重统计
// @Summary 获取去重统计
// @Description 获取消息去重器的详细统计
// @Tags message-queue
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/dedup/stats [get]
// @Security BearerAuth
func (h *QueueHandlers) getDedupStats(c *gin.Context) {
	if h.queue.deduplicator == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "消息去重未启用",
			"data":    nil,
		})
		return
	}

	stats := h.queue.deduplicator.Stats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// clearDedupCache 清空去重缓存
// @Summary 清空去重缓存
// @Description 清空消息去重缓存
// @Tags message-queue
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /message-queue/dedup/cache [delete]
// @Security BearerAuth
func (h *QueueHandlers) clearDedupCache(c *gin.Context) {
	if h.queue.deduplicator == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "消息去重未启用",
		})
		return
	}

	h.queue.deduplicator.mu.Lock()
	h.queue.deduplicator.cache = make(map[string]*dedupEntry)
	h.queue.deduplicator.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "去重缓存已清空",
	})
}
