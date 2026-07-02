package system

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// SystemStatus 系统状态摘要.
type SystemStatus struct {
	Status    string    `json:"status"`    // 系统状态：running, degraded, error
	Uptime    string    `json:"uptime"`    // 运行时间
	GoVersion string    `json:"goVersion"` // Go 版本
	NumCPU    int       `json:"numCPU"`    // CPU 核心数
	MemStats  MemStats  `json:"memStats"`  // 内存统计
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// MemStats 内存统计.
type MemStats struct {
	Alloc      uint64 `json:"alloc"`      // 当前内存分配
	TotalAlloc uint64 `json:"totalAlloc"` // 总内存分配
	Sys        uint64 `json:"sys"`        // 系统内存
	NumGC      uint32 `json:"numGC"`      // GC 次数
}

// GetSystemStatus 获取系统状态摘要.
func GetSystemStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := SystemStatus{
		Status:    "running",
		Uptime:    time.Since(startTime).String(),
		GoVersion: runtime.Version(),
		NumCPU:    runtime.NumCPU(),
		MemStats: MemStats{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
		},
		Timestamp: time.Now(),
	}

	c.JSON(http.StatusOK, status)
}

// startTime 程序启动时间.
var startTime = time.Now()
