package perfmon

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP API handlers for the performance monitor.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new performance monitor HTTP handler.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes registers all performance monitor API routes under the given group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	perf := rg.Group("/perf")
	{
		perf.GET("/iops", h.GetIOPS)
		perf.GET("/latency", h.GetLatency)
		perf.GET("/bandwidth", h.GetBandwidth)
		perf.GET("/diskio", h.GetDiskIO)
		perf.GET("/netio", h.GetNetIO)
		perf.GET("/cpu", h.GetCPU)
		perf.GET("/memory", h.GetMemory)
		perf.GET("/summary", h.GetSummary)
	}
}

// GetIOPS handles GET /api/v1/perf/iops
// @Summary Get IOPS statistics
// @Description Returns current read/write IOPS statistics
// @Tags perfmon
// @Produce json
// @Success 200 {object} IOPSStats
// @Router /api/v1/perf/iops [get].
func (h *Handler) GetIOPS(c *gin.Context) {
	stats := h.manager.GetIOPSStats()
	c.JSON(http.StatusOK, stats)
}

// GetLatency handles GET /api/v1/perf/latency
// @Summary Get latency statistics
// @Description Returns read/write latency statistics (avg, P99, max)
// @Tags perfmon
// @Produce json
// @Success 200 {object} LatencyStats
// @Router /api/v1/perf/latency [get].
func (h *Handler) GetLatency(c *gin.Context) {
	stats := h.manager.GetLatencyStats()
	c.JSON(http.StatusOK, stats)
}

// GetBandwidth handles GET /api/v1/perf/bandwidth
// @Summary Get bandwidth statistics
// @Description Returns network and disk bandwidth statistics
// @Tags perfmon
// @Produce json
// @Success 200 {object} BandwidthStats
// @Router /api/v1/perf/bandwidth [get].
func (h *Handler) GetBandwidth(c *gin.Context) {
	stats := h.manager.GetBandwidthStats()
	c.JSON(http.StatusOK, stats)
}

// GetDiskIO handles GET /api/v1/perf/diskio
// @Summary Get disk I/O statistics
// @Description Returns per-disk read/write bytes, queue depth
// @Tags perfmon
// @Produce json
// @Success 200 {array} DiskIOStats
// @Router /api/v1/perf/diskio [get].
func (h *Handler) GetDiskIO(c *gin.Context) {
	stats := h.manager.GetDiskIOStats()
	c.JSON(http.StatusOK, stats)
}

// GetNetIO handles GET /api/v1/perf/netio
// @Summary Get network I/O statistics
// @Description Returns per-interface bytes, packets, errors
// @Tags perfmon
// @Produce json
// @Success 200 {array} NetIOStats
// @Router /api/v1/perf/netio [get].
func (h *Handler) GetNetIO(c *gin.Context) {
	stats := h.manager.GetNetIOStats()
	c.JSON(http.StatusOK, stats)
}

// GetCPU handles GET /api/v1/perf/cpu
// @Summary Get detailed CPU statistics
// @Description Returns user/system/iowait/irq/softirq percentages
// @Tags perfmon
// @Produce json
// @Success 200 {object} CPUDetailStats
// @Router /api/v1/perf/cpu [get].
func (h *Handler) GetCPU(c *gin.Context) {
	stats := h.manager.GetCPUDetailStats()
	c.JSON(http.StatusOK, stats)
}

// GetMemory handles GET /api/v1/perf/memory
// @Summary Get detailed memory statistics
// @Description Returns used/cached/available/swap memory stats
// @Tags perfmon
// @Produce json
// @Success 200 {object} MemoryDetailStats
// @Router /api/v1/perf/memory [get].
func (h *Handler) GetMemory(c *gin.Context) {
	stats := h.manager.GetMemoryDetailStats()
	c.JSON(http.StatusOK, stats)
}

// GetSummary handles GET /api/v1/perf/summary
// @Summary Get full performance summary
// @Description Returns all collected performance metrics in one response
// @Tags perfmon
// @Produce json
// @Success 200 {object} PerfSummary
// @Router /api/v1/perf/summary [get].
func (h *Handler) GetSummary(c *gin.Context) {
	summary := h.manager.GetSummary()
	c.JSON(http.StatusOK, summary)
}
