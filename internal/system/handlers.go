package system

import (
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Handlers 系统监控处理器
type Handlers struct {
	monitor    *Monitor
	clientID   uint64
	clientIDMu sync.Mutex
}

// NewHandlers 创建处理器
func NewHandlers(monitor *Monitor) *Handlers {
	return &Handlers{
		monitor: monitor,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	system := r.Group("/system")
	{
		// 实时数据 WebSocket
		system.GET("/ws", h.websocketHandler)

		// 系统信息
		system.GET("/stats", h.getSystemStats)
		system.GET("/info", h.getSystemInfo)

		// 磁盘信息
		system.GET("/disks", h.getDiskStats)
		system.GET("/disks/smart/:device", h.getSMARTInfo)
		system.POST("/disks/check", h.checkAllDisks)

		// 网络信息
		system.GET("/network", h.getNetworkStats)

		// 进程信息
		system.GET("/processes", h.getTopProcesses)

		// 历史数据
		system.GET("/history", h.getHistoryData)

		// 告警管理
		system.GET("/alerts", h.getAlerts)
		system.POST("/alerts/:id/acknowledge", h.acknowledgeAlert)
	}
}

// websocketHandler WebSocket 连接处理
func (h *Handlers) websocketHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "WebSocket 升级失败",
		})
		return
	}

	// 生成客户端 ID（用于统计）
	h.clientIDMu.Lock()
	h.clientID++
	h.clientIDMu.Unlock()

	clientKey := c.ClientIP() + "-" + time.Now().Format("150405")

	// 注册客户端
	h.monitor.RegisterClient(clientKey, conn)

	// 立即发送一次当前数据
	systemStats, _ := h.monitor.GetSystemStats()
	diskStats, _ := h.monitor.GetDiskStats()
	networkStats, _ := h.monitor.GetNetworkStats(nil)

	h.monitor.Broadcast(&RealTimeData{
		Type:      "init",
		System:    systemStats,
		Disks:     diskStats,
		Network:   networkStats,
		Timestamp: time.Now(),
	})

	// 保持连接
	defer func() {
		h.monitor.UnregisterClient(clientKey)
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// getSystemStats 获取系统统计
func (h *Handlers) getSystemStats(c *gin.Context) {
	stats, err := h.monitor.GetSystemStats()
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
		"data":    stats,
	})
}

// getSystemInfo 获取系统信息
func (h *Handlers) getSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"hostname":  h.monitor.GetHostname(),
			"cores":     runtime.NumCPU(),
			"goVersion": runtime.Version(),
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
		},
	})
}

// getDiskStats 获取磁盘统计
func (h *Handlers) getDiskStats(c *gin.Context) {
	stats, err := h.monitor.GetDiskStats()
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
		"data":    stats,
	})
}

// getSMARTInfo 获取 SMART 信息
func (h *Handlers) getSMARTInfo(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备名称不能为空",
		})
		return
	}

	info, err := h.monitor.GetSMARTInfo("/dev/" + device)
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
		"data":    info,
	})
}

// checkAllDisks 检查所有磁盘
func (h *Handlers) checkAllDisks(c *gin.Context) {
	results, err := h.monitor.CheckAllDisks()
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
		"data":    results,
	})
}

// getNetworkStats 获取网络统计
func (h *Handlers) getNetworkStats(c *gin.Context) {
	stats, err := h.monitor.GetNetworkStats(nil)
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
		"data":    stats,
	})
}

// getTopProcesses 获取 Top 进程
func (h *Handlers) getTopProcesses(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	processes, err := h.monitor.GetTopProcesses(limit)
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
		"data":    processes,
	})
}

// getHistoryData 获取历史数据
func (h *Handlers) getHistoryData(c *gin.Context) {
	duration := c.DefaultQuery("duration", "24h")
	interval := c.DefaultQuery("interval", "1m")

	data, err := h.monitor.GetHistoryData(duration, interval)
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
		"data":    data,
	})
}

// getAlerts 获取告警列表
func (h *Handlers) getAlerts(c *gin.Context) {
	alerts, err := h.monitor.GetAlerts()
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
		"data":    alerts,
	})
}

// acknowledgeAlert 确认告警
func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.monitor.AcknowledgeAlert(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已确认",
	})
}
