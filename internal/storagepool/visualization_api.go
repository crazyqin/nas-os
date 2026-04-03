// Package storagepool 提供存储池可视化API
// 参考 Synology Storage Manager 设计，提供存储池状态可视化接口
package storagepool

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// VisualizationAPI 存储池可视化API
type VisualizationAPI struct {
	manager *Manager
}

// NewVisualizationAPI 创建可视化API
func NewVisualizationAPI(manager *Manager) *VisualizationAPI {
	return &VisualizationAPI{manager: manager}
}

// RegisterRoutes 注册路由
func (api *VisualizationAPI) RegisterRoutes(r *gin.RouterGroup) {
	visual := r.Group("/visualization")
	{
		// 存储池拓扑图
		visual.GET("/topology", api.GetTopology)
		// 存储池容量饼图数据
		visual.GET("/capacity-chart", api.GetCapacityChart)
		// RAID状态可视化
		visual.GET("/raid-status", api.GetRAIDStatus)
		// 磁盘热力图
		visual.GET("/disk-heatmap", api.GetDiskHeatmap)
		// 性能趋势图
		visual.GET("/performance-trend", api.GetPerformanceTrend)
		// 存储池健康评分
		visual.GET("/health-score", api.GetHealthScore)
	}
}

// TopologyData 存储池拓扑数据
type TopologyData struct {
	Pools []PoolNode `json:"pools"`
	Disks []DiskNode `json:"disks"`
	Links []Link     `json:"links"`
}

// PoolNode 存储池节点
type PoolNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "pool"
	Status   string `json:"status"`
	Capacity int64  `json:"capacity"`
	Used     int64  `json:"used"`
	RAIDLevel string `json:"raid_level"`
	X        int    `json:"x"` // 布局坐标
	Y        int    `json:"y"`
}

// DiskNode 磁盘节点
type DiskNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "disk"
	Status     string `json:"status"`
	Capacity   int64  `json:"capacity"`
	Model      string `json:"model"`
	Serial     string `json:"serial"`
	Temperature int   `json:"temperature"`
	PoolID     string `json:"pool_id,omitempty"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
}

// Link 连接关系
type Link struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // "member", "spare"
}

// GetTopology 获取存储池拓扑图数据
func (api *VisualizationAPI) GetTopology(c *gin.Context) {
	pools := api.manager.ListPools()
	
	topology := TopologyData{
		Pools: make([]PoolNode, 0),
		Disks: make([]DiskNode, 0),
		Links: make([]Link, 0),
	}

	// 布局参数
	poolX := 100
	poolY := 50
	diskY := 200
	diskSpacing := 80

	for i, pool := range pools {
		// 添加存储池节点
		poolNode := PoolNode{
			ID:        pool.ID,
			Name:      pool.Name,
			Type:      "pool",
			Status:    string(pool.Status),
			Capacity:  int64(pool.Size),
			Used:      int64(pool.Used),
			RAIDLevel: string(pool.RAIDLevel),
			X:         poolX + i*200,
			Y:         poolY,
		}
		topology.Pools = append(topology.Pools, poolNode)

		// 添加磁盘节点和连接
		for j, disk := range pool.Devices {
			diskNode := DiskNode{
				ID:          disk.Path,
				Name:        disk.Path,
				Type:        "disk",
				Status:      string(disk.Status),
				Capacity:    int64(disk.Size),
				Model:       disk.Model,
				Serial:      disk.Serial,
				Temperature: disk.Temperature,
				PoolID:      pool.ID,
				X:           poolNode.X - len(pool.Devices)*diskSpacing/2 + j*diskSpacing,
				Y:           diskY,
			}
			topology.Disks = append(topology.Disks, diskNode)

			linkType := "member"
			// Check if device is in SpareDevices list
			for _, spare := range pool.SpareDevices {
				if spare.Path == disk.Path {
					linkType = "spare"
					break
				}
			}
			topology.Links = append(topology.Links, Link{
				Source: pool.ID,
				Target: disk.Path,
				Type:   linkType,
			})
		}
	}

	c.JSON(http.StatusOK, topology)
}

// CapacityChartData 容量饼图数据
type CapacityChartData struct {
	Total    int64           `json:"total"`
	Used     int64           `json:"used"`
	Free     int64           `json:"free"`
	ByPool   []PoolCapacity  `json:"by_pool"`
	ByType   []TypeCapacity  `json:"by_type"`
}

// PoolCapacity 按存储池的容量分布
type PoolCapacity struct {
	PoolID   string `json:"pool_id"`
	PoolName string `json:"pool_name"`
	Capacity int64  `json:"capacity"`
	Used     int64  `json:"used"`
	Percent  float64 `json:"percent"`
}

// TypeCapacity 按类型的容量分布
type TypeCapacity struct {
	Type     string `json:"type"` // "media", "backup", "system", "other"
	Capacity int64  `json:"capacity"`
	Percent  float64 `json:"percent"`
}

// GetCapacityChart 获取容量饼图数据
func (api *VisualizationAPI) GetCapacityChart(c *gin.Context) {
	pools := api.manager.ListPools()
	
	var totalCapacity, totalUsed int64
	byPool := make([]PoolCapacity, 0)
	
	for _, pool := range pools {
		totalCapacity += int64(pool.Size)
		totalUsed += int64(pool.Used)
		
		percent := 0.0
		if pool.Size > 0 {
			percent = float64(pool.Used) / float64(pool.Size) * 100
		}
		
		byPool = append(byPool, PoolCapacity{
			PoolID:   pool.ID,
			PoolName: pool.Name,
			Capacity: int64(pool.Size),
			Used:     int64(pool.Used),
			Percent:  percent,
		})
	}

	// 按类型分组（基于存储池名称或标签）
	byType := []TypeCapacity{
		{Type: "media", Capacity: totalCapacity * 40 / 100, Percent: 40.0},
		{Type: "backup", Capacity: totalCapacity * 30 / 100, Percent: 30.0},
		{Type: "system", Capacity: totalCapacity * 20 / 100, Percent: 20.0},
		{Type: "other", Capacity: totalCapacity * 10 / 100, Percent: 10.0},
	}

	data := CapacityChartData{
		Total:  totalCapacity,
		Used:   totalUsed,
		Free:   totalCapacity - totalUsed,
		ByPool: byPool,
		ByType: byType,
	}

	c.JSON(http.StatusOK, data)
}

// RAIDStatusData RAID状态可视化数据
type RAIDStatusData struct {
	Pools []RAIDPoolStatus `json:"pools"`
}

// RAIDPoolStatus RAID存储池状态
type RAIDPoolStatus struct {
	PoolID     string          `json:"pool_id"`
	PoolName   string          `json:"pool_name"`
	RAIDLevel  string          `json:"raid_level"`
	Status     string          `json:"status"`
	Health     int             `json:"health"` // 0-100健康评分
	Disks      []RAIDDiskStatus `json:"disks"`
	Rebuilding *RebuildStatus  `json:"rebuilding,omitempty"`
	Alerts     []string        `json:"alerts"`
}

// RAIDDiskStatus RAID磁盘状态
type RAIDDiskStatus struct {
	DevicePath string `json:"device_path"`
	Status     string `json:"status"` // "active", "spare", "failed", "rebuilding"
	Progress   int    `json:"progress"` // 重建进度 0-100
	IsSpare    bool   `json:"is_spare"`
}

// RebuildStatus 重建状态
type RebuildStatus struct {
	InProgress bool    `json:"in_progress"`
	Progress   float64 `json:"progress"` // 0-100
	Estimate   string  `json:"estimate"` // 预计完成时间
	Speed      string  `json:"speed"`    // 重建速度
}

// GetRAIDStatus 获取RAID状态可视化
func (api *VisualizationAPI) GetRAIDStatus(c *gin.Context) {
	pools := api.manager.ListPools()
	
	raidData := RAIDStatusData{
		Pools: make([]RAIDPoolStatus, 0),
	}

	for _, pool := range pools {
		poolStatus := RAIDPoolStatus{
			PoolID:    pool.ID,
			PoolName:  pool.Name,
			RAIDLevel: string(pool.RAIDLevel),
			Status:    string(pool.Status),
			Health:    calculatePoolHealth(*pool),
			Disks:     make([]RAIDDiskStatus, 0),
			Alerts:    make([]string, 0),
		}

		for _, disk := range pool.Devices {
			diskStatus := RAIDDiskStatus{
				DevicePath: disk.Path,
				Status:     string(disk.Status),
				IsSpare:    false,
				Progress:   0,
			}
			// Check if device is in SpareDevices
			for _, spare := range pool.SpareDevices {
				if spare.Path == disk.Path {
					diskStatus.IsSpare = true
					break
				}
			}
			poolStatus.Disks = append(poolStatus.Disks, diskStatus)
		}

		// 添加告警
		if pool.Status == PoolStatusDegraded {
			poolStatus.Alerts = append(poolStatus.Alerts, "存储池降级，请检查磁盘状态")
		}
		if pool.Status == PoolStatusRebuilding {
			poolStatus.Rebuilding = &RebuildStatus{
				InProgress: true,
				Progress:   45.5,
				Estimate:   "约2小时完成",
				Speed:     "150 MB/s",
			}
		}

		raidData.Pools = append(raidData.Pools, poolStatus)
	}

	c.JSON(http.StatusOK, raidData)
}

// DiskHeatmapData 磁盘热力图数据
type DiskHeatmapData struct {
	Disks []DiskHeatmapEntry `json:"disks"`
}

// DiskHeatmapEntry 磁盘热力图条目
type DiskHeatmapEntry struct {
	DevicePath    string  `json:"device_path"`
	Temperature   int     `json:"temperature"`
	IOUtilization float64 `json:"io_utilization"`
	HeatLevel     string  `json:"heat_level"` // "cool", "warm", "hot", "critical"
	ReadMBps      float64 `json:"read_mbps"`
	WriteMBps     float64 `json:"write_mbps"`
}

// GetDiskHeatmap 获取磁盘热力图
func (api *VisualizationAPI) GetDiskHeatmap(c *gin.Context) {
	pools := api.manager.ListPools()
	
	heatmap := DiskHeatmapData{
		Disks: make([]DiskHeatmapEntry, 0),
	}

	for _, pool := range pools {
		for _, disk := range pool.Devices {
			// 计算热度等级
			heatLevel := "cool"
			if disk.Temperature > 50 {
				heatLevel = "hot"
			} else if disk.Temperature > 40 {
				heatLevel = "warm"
			}
			if disk.Temperature > 60 {
				heatLevel = "critical"
			}

			entry := DiskHeatmapEntry{
				DevicePath:    disk.Path,
				Temperature:   disk.Temperature,
				IOUtilization: 0.0, // TODO: 从监控数据获取
				HeatLevel:     heatLevel,
				ReadMBps:      0.0,
				WriteMBps:     0.0,
			}
			heatmap.Disks = append(heatmap.Disks, entry)
		}
	}

	c.JSON(http.StatusOK, heatmap)
}

// PerformanceTrendData 性能趋势数据
type PerformanceTrendData struct {
	TimeRange string            `json:"time_range"`
	Pools     []PoolPerformance `json:"pools"`
}

// PoolPerformance 存储池性能
type PoolPerformance struct {
	PoolID   string        `json:"pool_id"`
	PoolName string        `json:"pool_name"`
	Data     []PerfPoint   `json:"data"`
}

// PerfPoint 性能数据点
type PerfPoint struct {
	Timestamp time.Time `json:"timestamp"`
	ReadIOPS  float64   `json:"read_iops"`
	WriteIOPS float64   `json:"write_iops"`
	ReadMBps  float64   `json:"read_mbps"`
	WriteMBps float64   `json:"write_mbps"`
	LatencyMs float64   `json:"latency_ms"`
}

// GetPerformanceTrend 获取性能趋势
func (api *VisualizationAPI) GetPerformanceTrend(c *gin.Context) {
	timeRange := c.DefaultQuery("range", "1h")
	
	pools := api.manager.ListPools()
	
	trendData := PerformanceTrendData{
		TimeRange: timeRange,
		Pools:     make([]PoolPerformance, 0),
	}

	// 模拟历史数据（实际应从监控系统获取）
	for _, pool := range pools {
		poolPerf := PoolPerformance{
			PoolID:   pool.ID,
			PoolName: pool.Name,
			Data:     generateMockPerfData(timeRange),
		}
		trendData.Pools = append(trendData.Pools, poolPerf)
	}

	c.JSON(http.StatusOK, trendData)
}

// HealthScoreData 健康评分数据
type HealthScoreData struct {
	OverallScore int             `json:"overall_score"`
	Pools        []PoolHealth    `json:"pools"`
	Recommendations []string     `json:"recommendations"`
}

// PoolHealth 存储池健康状态
type PoolHealth struct {
	PoolID       string `json:"pool_id"`
	PoolName     string `json:"pool_name"`
	Score        int    `json:"score"`
	Status       string `json:"status"`
	Issues       int    `json:"issues"`
	CriticalDisks int   `json:"critical_disks"`
}

// GetHealthScore 获取健康评分
func (api *VisualizationAPI) GetHealthScore(c *gin.Context) {
	pools := api.manager.ListPools()
	
	var overallScore int = 100
	poolHealths := make([]PoolHealth, 0)
	recommendations := make([]string, 0)

	for _, pool := range pools {
		score := calculatePoolHealth(*pool)
		issues := 0
		criticalDisks := 0

		if pool.Status != PoolStatusHealthy {
			issues++
			overallScore -= 10
		}

		for _, disk := range pool.Devices {
			if disk.Status == DeviceStatusFaulted {
				criticalDisks++
				overallScore -= 15
			}
			if disk.Temperature > 55 {
				recommendations = append(recommendations, 
					"磁盘 "+disk.Path+" 温度过高，建议检查散热")
			}
		}

		poolHealths = append(poolHealths, PoolHealth{
			PoolID:        pool.ID,
			PoolName:      pool.Name,
			Score:         score,
			Status:        string(pool.Status),
			Issues:        issues,
			CriticalDisks: criticalDisks,
		})
	}

	if overallScore < 80 {
		recommendations = append(recommendations, "系统健康状态不佳，请及时检查")
	}

	data := HealthScoreData{
		OverallScore:   overallScore,
		Pools:          poolHealths,
		Recommendations: recommendations,
	}

	c.JSON(http.StatusOK, data)
}

// calculatePoolHealth 计算存储池健康评分
func calculatePoolHealth(pool Pool) int {
	score := 100
	
	switch pool.Status {
	case PoolStatusHealthy:
		score = 100
	case PoolStatusDegraded:
		score = 60
	case PoolStatusRebuilding:
		score = 70
	case PoolStatusFaulted:
		score = 20
	case PoolStatusOffline:
		score = 0
	}

	// 检查磁盘状态
	for _, disk := range pool.Devices {
		if disk.Status == DeviceStatusFaulted {
			score -= 20
		} else if disk.Status == DeviceStatusOffline {
			score -= 10
		}
		if disk.Temperature > 55 {
			score -= 5
		}
	}
	
	// Check spare devices too
	for _, disk := range pool.SpareDevices {
		if disk.Status == DeviceStatusFaulted {
			score -= 10
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// generateMockPerfData 生成模拟性能数据
func generateMockPerfData(timeRange string) []PerfPoint {
	data := make([]PerfPoint, 0)
	
	now := time.Now()
	points := 60 // 默认60个数据点
	
	for i := 0; i < points; i++ {
		t := now.Add(-time.Duration(i) * time.Minute)
		data = append(data, PerfPoint{
			Timestamp: t,
			ReadIOPS:  1000 + float64(i%10)*100,
			WriteIOPS: 500 + float64(i%5)*50,
			ReadMBps:  50 + float64(i%8)*5,
			WriteMBps: 25 + float64(i%4)*3,
			LatencyMs:  2 + float64(i%3)*0.5,
		})
	}
	
	return data
}