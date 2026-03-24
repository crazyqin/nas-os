// Package api provides HTTP API handlers
// File: container_stream.go - WebSocket handlers for container logs and stats streaming
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"nas-os/internal/container"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ContainerStreamHandler handles WebSocket streaming for containers
type ContainerStreamHandler struct {
	manager       *container.Manager
	logStreamer   *container.LogStreamer
	statsStreamer *container.StatsStreamer
	batchManager  *container.BatchManager
	hub           *EnhancedWebSocketHub
}

// NewContainerStreamHandler creates a new container stream handler
func NewContainerStreamHandler(hub *EnhancedWebSocketHub) (*ContainerStreamHandler, error) {
	mgr, err := container.NewManager()
	if err != nil {
		return nil, err
	}

	return &ContainerStreamHandler{
		manager:       mgr,
		logStreamer:   container.NewLogStreamer(mgr),
		statsStreamer: container.NewStatsStreamer(mgr),
		batchManager:  container.NewBatchManager(mgr),
		hub:           hub,
	}, nil
}

// RegisterStreamRoutes registers streaming routes
func (h *ContainerStreamHandler) RegisterStreamRoutes(r *gin.RouterGroup) {
	// WebSocket endpoints
	r.GET("/containers/:id/logs/stream", h.StreamLogs)
	r.GET("/containers/:id/stats/stream", h.StreamStats)
	r.GET("/containers/stats/stream", h.StreamAllStats)

	// Batch operations
	r.POST("/containers/batch/start", h.BatchStart)
	r.POST("/containers/batch/stop", h.BatchStop)
	r.POST("/containers/batch/restart", h.BatchRestart)
	r.POST("/containers/batch/remove", h.BatchRemove)
	r.POST("/containers/batch/execute", h.BatchExecute)

	// Container prune
	r.POST("/containers/prune", h.PruneContainers)

	// Select containers for batch
	r.POST("/containers/select", h.SelectContainers)
}

// StreamLogs streams container logs via WebSocket
// GET /api/v1/containers/:id/logs/stream?tail=100&follow=true
func (h *ContainerStreamHandler) StreamLogs(c *gin.Context) {
	containerID := c.Param("id")
	if containerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "container ID is required",
		})
		return
	}

	// Parse query parameters
	tail := 100
	if t := c.Query("tail"); t != "" {
		if val, err := strconv.Atoi(t); err == nil && val > 0 {
			tail = val
		}
	}

	follow := c.Query("follow") != "false"
	since := c.Query("since")
	timestamps := c.Query("timestamps") != "false"

	config := container.StreamConfig{
		Follow:     follow,
		Tail:       tail,
		Since:      since,
		Timestamps: timestamps,
		Stdout:     true,
		Stderr:     true,
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ContainerStream] WebSocket upgrade error: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start log streaming
	logChan, err := h.logStreamer.StreamLogs(ctx, containerID, config)
	if err != nil {
		log.Printf("[ContainerStream] Failed to start log stream: %v", err)
		sendWebSocketError(conn, "log_stream_error", err.Error())
		return
	}

	// Send initial connection message
	_ = conn.WriteJSON(map[string]interface{}{
		"type":        "connected",
		"containerId": containerID,
		"timestamp":   time.Now().Unix(),
	})

	// Read pump: handle client messages (like close/heartbeat)
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Write pump: send log messages
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-logChan:
			if !ok {
				return
			}

			if err := conn.WriteJSON(map[string]interface{}{
				"type":        "log",
				"timestamp":   msg.Timestamp.Unix(),
				"line":        msg.Line,
				"source":      msg.Source,
				"containerId": containerID,
			}); err != nil {
				log.Printf("[ContainerStream] Write error: %v", err)
				return
			}
		}
	}
}

// StreamStats streams container stats via WebSocket
// GET /api/v1/containers/:id/stats/stream?interval=1s
func (h *ContainerStreamHandler) StreamStats(c *gin.Context) {
	containerID := c.Param("id")
	if containerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "container ID is required",
		})
		return
	}

	// Parse interval
	interval := time.Second
	if i := c.Query("interval"); i != "" {
		if dur, err := time.ParseDuration(i); err == nil && dur > 0 {
			interval = dur
		}
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ContainerStream] WebSocket upgrade error: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start stats streaming
	statsChan, err := h.statsStreamer.StreamStats(ctx, containerID, interval)
	if err != nil {
		log.Printf("[ContainerStream] Failed to start stats stream: %v", err)
		sendWebSocketError(conn, "stats_stream_error", err.Error())
		return
	}

	// Send initial connection message
	_ = conn.WriteJSON(map[string]interface{}{
		"type":        "connected",
		"containerId": containerID,
		"timestamp":   time.Now().Unix(),
	})

	// Read pump
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Write pump: send stats updates
	for {
		select {
		case <-ctx.Done():
			return
		case stats, ok := <-statsChan:
			if !ok {
				return
			}

			if err := conn.WriteJSON(map[string]interface{}{
				"type":        "stats",
				"timestamp":   stats.Timestamp.Unix(),
				"containerId": stats.ContainerID,
				"data": map[string]interface{}{
					"cpuUsage":   stats.CPUUsage,
					"memUsage":   stats.MemUsage,
					"memLimit":   stats.MemLimit,
					"memPercent": stats.MemPercent,
					"netRx":      stats.NetRX,
					"netTx":      stats.NetTX,
					"blockRead":  stats.BlockRead,
					"blockWrite": stats.BlockWrite,
					"pids":       stats.PIDs,
				},
			}); err != nil {
				log.Printf("[ContainerStream] Write error: %v", err)
				return
			}
		}
	}
}

// StreamAllStats streams stats for all containers via WebSocket
// GET /api/v1/containers/stats/stream?interval=2s
func (h *ContainerStreamHandler) StreamAllStats(c *gin.Context) {
	// Parse interval
	interval := 2 * time.Second
	if i := c.Query("interval"); i != "" {
		if dur, err := time.ParseDuration(i); err == nil && dur > 0 {
			interval = dur
		}
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ContainerStream] WebSocket upgrade error: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start stats streaming for all containers
	statsChan, err := h.statsStreamer.StreamAllStats(ctx, interval)
	if err != nil {
		log.Printf("[ContainerStream] Failed to start all stats stream: %v", err)
		sendWebSocketError(conn, "stats_stream_error", err.Error())
		return
	}

	// Send initial connection message
	_ = conn.WriteJSON(map[string]interface{}{
		"type":      "connected",
		"mode":      "all",
		"timestamp": time.Now().Unix(),
	})

	// Read pump
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Write pump
	for {
		select {
		case <-ctx.Done():
			return
		case stats, ok := <-statsChan:
			if !ok {
				return
			}

			if err := conn.WriteJSON(map[string]interface{}{
				"type":        "stats",
				"timestamp":   stats.Timestamp.Unix(),
				"containerId": stats.ContainerID,
				"data": map[string]interface{}{
					"cpuUsage":   stats.CPUUsage,
					"memUsage":   stats.MemUsage,
					"memLimit":   stats.MemLimit,
					"memPercent": stats.MemPercent,
					"netRx":      stats.NetRX,
					"netTx":      stats.NetTX,
					"blockRead":  stats.BlockRead,
					"blockWrite": stats.BlockWrite,
					"pids":       stats.PIDs,
				},
			}); err != nil {
				log.Printf("[ContainerStream] Write error: %v", err)
				return
			}
		}
	}
}

// BatchStart starts multiple containers
// POST /api/v1/containers/batch/start
func (h *ContainerStreamHandler) BatchStart(c *gin.Context) {
	var req struct {
		ContainerIDs []string `json:"containerIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	response, err := h.batchManager.StartBatch(c.Request.Context(), req.ContainerIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Batch start completed",
		"data":    response,
	})
}

// BatchStop stops multiple containers
// POST /api/v1/containers/batch/stop
func (h *ContainerStreamHandler) BatchStop(c *gin.Context) {
	var req struct {
		ContainerIDs []string `json:"containerIds" binding:"required"`
		Timeout      int      `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	response, err := h.batchManager.StopBatch(c.Request.Context(), req.ContainerIDs, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Batch stop completed",
		"data":    response,
	})
}

// BatchRestart restarts multiple containers
// POST /api/v1/containers/batch/restart
func (h *ContainerStreamHandler) BatchRestart(c *gin.Context) {
	var req struct {
		ContainerIDs []string `json:"containerIds" binding:"required"`
		Timeout      int      `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	response, err := h.batchManager.RestartBatch(c.Request.Context(), req.ContainerIDs, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Batch restart completed",
		"data":    response,
	})
}

// BatchRemove removes multiple containers
// POST /api/v1/containers/batch/remove
func (h *ContainerStreamHandler) BatchRemove(c *gin.Context) {
	var req struct {
		ContainerIDs  []string `json:"containerIds" binding:"required"`
		Force         bool     `json:"force"`
		RemoveVolumes bool     `json:"removeVolumes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	response, err := h.batchManager.RemoveBatch(c.Request.Context(), req.ContainerIDs, req.Force, req.RemoveVolumes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Batch remove completed",
		"data":    response,
	})
}

// BatchExecute executes a generic batch operation
// POST /api/v1/containers/batch/execute
func (h *ContainerStreamHandler) BatchExecute(c *gin.Context) {
	var req container.BatchOperationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	response, err := h.batchManager.Execute(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Batch operation completed",
		"data":    response,
	})
}

// PruneContainers removes all stopped containers
// POST /api/v1/containers/prune
func (h *ContainerStreamHandler) PruneContainers(c *gin.Context) {
	result, err := h.manager.PruneContainers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Containers pruned",
		"data":    result,
	})
}

// SelectContainers selects containers matching filter criteria
// POST /api/v1/containers/select
func (h *ContainerStreamHandler) SelectContainers(c *gin.Context) {
	var filter container.ContainerFilter

	if err := c.ShouldBindJSON(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	ids, err := h.batchManager.SelectByFilter(filter)
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
		"data": map[string]interface{}{
			"containerIds": ids,
			"count":        len(ids),
		},
	})
}

// BatchOperationWithProgress handles batch operations with WebSocket progress updates
// WebSocket: /api/v1/containers/batch/ws
func (h *ContainerStreamHandler) BatchOperationWithProgress(c *gin.Context) {
	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ContainerStream] WebSocket upgrade error: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// Wait for operation request
	_, message, err := conn.ReadMessage()
	if err != nil {
		return
	}

	var req container.BatchOperationRequest
	if err := json.Unmarshal(message, &req); err != nil {
		sendWebSocketError(conn, "invalid_request", err.Error())
		return
	}

	// Create progress channel
	progressChan := make(chan container.BatchProgress, 100)

	// Send progress updates
	go func() {
		for progress := range progressChan {
			if err := conn.WriteJSON(map[string]interface{}{
				"type":      "progress",
				"data":      progress.GetProgress(),
				"timestamp": time.Now().Unix(),
			}); err != nil {
				return
			}
		}
	}()

	// Execute with progress
	response, err := h.batchManager.ExecuteWithProgress(c.Request.Context(), req, progressChan)
	close(progressChan)

	if err != nil {
		sendWebSocketError(conn, "batch_error", err.Error())
		return
	}

	// Send final result
	_ = conn.WriteJSON(map[string]interface{}{
		"type":      "complete",
		"data":      response,
		"timestamp": time.Now().Unix(),
	})
}

// sendWebSocketError sends an error message via WebSocket
func sendWebSocketError(conn *websocket.Conn, errorType string, message string) {
	_ = conn.WriteJSON(map[string]interface{}{
		"type":      "error",
		"errorType": errorType,
		"message":   message,
		"timestamp": time.Now().Unix(),
	})
}
