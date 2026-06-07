// Package cluster 提供统一仪表板数据聚合功能
// 对标群晖 Active Insight 和 TrueNAS 中央管理
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DashboardAggregator 仪表板数据聚合器
type DashboardAggregator struct {
	discovery  *DiscoveryService
	config     DashboardConfig
	data       *AggregatedData
	dataMutex  sync.RWMutex
	httpClient *http.Client

	// 数据更新回调
	onDataUpdate func(data *AggregatedData)

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger *zap.Logger
}

// DashboardConfig 仪表板配置
type DashboardConfig struct {
	// 数据收集间隔
	CollectInterval time.Duration `json:"collect_interval"`

	// 数据缓存时间
	CacheDuration time.Duration `json:"cache_duration"`

	// 是否启用历史数据
	EnableHistory bool `json:"enable_history"`

	// 历史数据保留时间
	HistoryRetention time.Duration `json:"history_retention"`

	// 数据存储目录
	DataDir string `json:"data_dir"`

	// 节点超时（数据采集）
	NodeTimeout time.Duration `json:"node_timeout"`
}

// AggregatedData 聚合数据
type AggregatedData struct {
	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// 集群概览
	ClusterOverview ClusterOverview `json:"cluster_overview"`

	// 节点汇总
	NodeSummary NodeSummary `json:"node_summary"`

	// 存储汇总
	StorageSummary StorageSummary `json:"storage_summary"`

	// 容器汇总
	ContainerSummary ContainerSummary `json:"container_summary"`

	// 服务汇总
	ServiceSummary ServiceSummary `json:"service_summary"`

	// 资源趋势
	ResourceTrend ResourceTrend `json:"resource_trend"`

	// 历史数据（可选）
	HistoricalData []TimeSeriesPoint `json:"historical_data,omitempty"`
}

// ClusterOverview 集群概览
type ClusterOverview struct {
	// 总节点数
	TotalNodes int `json:"total_nodes"`

	// 在线节点数
	OnlineNodes int `json:"online_nodes"`

	// 离线节点数
	OfflineNodes int `json:"offline_nodes"`

	// 可疑节点数
	SuspectNodes int `json:"suspect_nodes"`

	// 主节点 ID
	MasterNodeID string `json:"master_node_id"`

	// 集群健康状态
	HealthStatus string `json:"health_status"` // healthy, degraded, critical

	// 平均响应时间（毫秒）
	AvgResponseTime float64 `json:"avg_response_time"`
}

// NodeSummary 节点汇总
type NodeSummary struct {
	// 总 CPU 核心
	TotalCPUCores int `json:"total_cpu_cores"`

	// 总内存（GB）
	TotalMemoryGB float64 `json:"total_memory_gb"`

	// 总磁盘（GB）
	TotalDiskGB float64 `json:"total_disk_gb"`

	// 平均 CPU 使用率
	AvgCPUUsage float64 `json:"avg_cpu_usage"`

	// 平均内存使用率
	AvgMemoryUsage float64 `json:"avg_memory_usage"`

	// 平均磁盘使用率
	AvgDiskUsage float64 `json:"avg_disk_usage"`

	// 节点详情
	Nodes []NodeDetail `json:"nodes"`
}

// NodeDetail 节点详情
type NodeDetail struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Address      string    `json:"address"`
	Status       string    `json:"status"`
	Role         string    `json:"role"`
	Version      string    `json:"version"`
	CPUCores     int       `json:"cpu_cores"`
	MemoryGB     float64   `json:"memory_gb"`
	DiskGB       float64   `json:"disk_gb"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	DiskUsage    float64   `json:"disk_usage"`
	LastSeen     time.Time `json:"last_seen"`
	Uptime       string    `json:"uptime"`
	Capabilities []string  `json:"capabilities"`
}

// StorageSummary 存储汇总
type StorageSummary struct {
	// 总存储池数
	TotalPools int `json:"total_pools"`

	// 总卷数
	TotalVolumes int `json:"total_volumes"`

	// 总容量（GB）
	TotalCapacityGB float64 `json:"total_capacity_gb"`

	// 已用容量（GB）
	UsedCapacityGB float64 `json:"used_capacity_gb"`

	// 可用容量（GB）
	FreeCapacityGB float64 `json:"free_capacity_gb"`

	// 使用率
	UsagePercent float64 `json:"usage_percent"`

	// 各节点存储详情
	NodeStorage []NodeStorageInfo `json:"node_storage"`
}

// NodeStorageInfo 节点存储信息
type NodeStorageInfo struct {
	NodeID       string       `json:"node_id"`
	NodeName     string       `json:"node_name"`
	Pools        []PoolInfo   `json:"pools"`
	Volumes      []VolumeInfo `json:"volumes"`
	TotalGB      float64      `json:"total_gb"`
	UsedGB       float64      `json:"used_gb"`
	FreeGB       float64      `json:"free_gb"`
	UsagePercent float64      `json:"usage_percent"`
}

// PoolInfo 存储池信息
type PoolInfo struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"` // zfs, btrfs, lvm
	Status  string  `json:"status"`
	TotalGB float64 `json:"total_gb"`
	UsedGB  float64 `json:"used_gb"`
	FreeGB  float64 `json:"free_gb"`
	Health  string  `json:"health"`
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name       string  `json:"name"`
	Pool       string  `json:"pool"`
	SizeGB     float64 `json:"size_gb"`
	UsedGB     float64 `json:"used_gb"`
	Status     string  `json:"status"`
	MountPoint string  `json:"mount_point"`
}

// ContainerSummary 容器汇总
type ContainerSummary struct {
	// 总容器数
	TotalContainers int `json:"total_containers"`

	// 运行容器数
	RunningContainers int `json:"running_containers"`

	// 停止容器数
	StoppedContainers int `json:"stopped_containers"`

	// 异常容器数
	ErrorContainers int `json:"error_containers"`

	// 总镜像数
	TotalImages int `json:"total_images"`

	// 各节点容器详情
	NodeContainers []NodeContainerInfo `json:"node_containers"`
}

// NodeContainerInfo 节点容器信息
type NodeContainerInfo struct {
	NodeID        string          `json:"node_id"`
	NodeName      string          `json:"node_name"`
	Containers    []ContainerInfo `json:"containers"`
	RunningCount  int             `json:"running_count"`
	StoppedCount  int             `json:"stopped_count"`
	TotalCPUUsage float64         `json:"total_cpu_usage"`
	TotalMemUsage uint64          `json:"total_mem_usage"`
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Image    string            `json:"image"`
	Status   string            `json:"status"`
	CPUUsage float64           `json:"cpu_usage"`
	MemUsage uint64            `json:"mem_usage"`
	Ports    []string          `json:"ports"`
	Labels   map[string]string `json:"labels"`
}

// ServiceSummary 服务汇总
type ServiceSummary struct {
	// 总服务数
	TotalServices int `json:"total_services"`

	// 运行服务数
	RunningServices int `json:"running_services"`

	// 停止服务数
	StoppedServices int `json:"stopped_services"`

	// 异常服务数
	ErrorServices int `json:"error_services"`

	// 各节点服务详情
	NodeServices []NodeServiceInfo `json:"node_services"`
}

// NodeServiceInfo 节点服务信息
type NodeServiceInfo struct {
	NodeID       string        `json:"node_id"`
	NodeName     string        `json:"node_name"`
	Services     []ServiceInfo `json:"services"`
	RunningCount int           `json:"running_count"`
	StoppedCount int           `json:"stopped_count"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name    string            `json:"name"`
	Status  string            `json:"status"`
	Port    int               `json:"port,omitempty"`
	Version string            `json:"version,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// ResourceTrend 资源趋势
type ResourceTrend struct {
	// CPU 趋势（最近 24 小时）
	CPUTrend []TrendPoint `json:"cpu_trend"`

	// 内存趋势
	MemoryTrend []TrendPoint `json:"memory_trend"`

	// 磁盘趋势
	DiskTrend []TrendPoint `json:"disk_trend"`

	// 网络流量趋势
	NetworkTrend []TrendPoint `json:"network_trend"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Timestamp time.Time       `json:"timestamp"`
	Cluster   ClusterOverview `json:"cluster"`
	Nodes     NodeSummary     `json:"nodes"`
	Storage   StorageSummary  `json:"storage"`
}

// NewDashboardAggregator 创建仪表板聚合器
func NewDashboardAggregator(discovery *DiscoveryService, config DashboardConfig, logger *zap.Logger) (*DashboardAggregator, error) {
	// 设置默认值
	if config.CollectInterval == 0 {
		config.CollectInterval = 30 * time.Second
	}
	if config.CacheDuration == 0 {
		config.CacheDuration = 5 * time.Minute
	}
	if config.EnableHistory && config.HistoryRetention == 0 {
		config.HistoryRetention = 24 * time.Hour
	}
	if config.NodeTimeout == 0 {
		config.NodeTimeout = 10 * time.Second
	}
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/dashboard"
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	aggregator := &DashboardAggregator{
		discovery:  discovery,
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		httpClient: &http.Client{Timeout: config.NodeTimeout},
	}

	return aggregator, nil
}

// Initialize 初始化聚合器
func (da *DashboardAggregator) Initialize() error {
	// 初始化数据
	da.data = &AggregatedData{
		Timestamp: time.Now(),
	}

	// 启动数据收集循环
	da.wg.Add(1)
	go da.collectLoop()

	da.logger.Info("仪表板聚合器已启动",
		zap.Duration("collect_interval", da.config.CollectInterval))

	return nil
}

// collectLoop 数据收集循环
func (da *DashboardAggregator) collectLoop() {
	defer da.wg.Done()

	ticker := time.NewTicker(da.config.CollectInterval)
	defer ticker.Stop()

	// 首次立即收集
	da.collectData()

	for {
		select {
		case <-da.ctx.Done():
			return
		case <-ticker.C:
			da.collectData()
		}
	}
}

// collectData 收集所有节点数据
func (da *DashboardAggregator) collectData() {
	nodes := da.discovery.GetNodes()

	da.dataMutex.Lock()
	defer da.dataMutex.Unlock()

	// 更新时间戳
	da.data.Timestamp = time.Now()

	// 收集集群概览
	da.data.ClusterOverview = da.collectClusterOverview(nodes)

	// 收集节点汇总
	da.data.NodeSummary = da.collectNodeSummary(nodes)

	// 收集存储汇总
	da.data.StorageSummary = da.collectStorageSummary(nodes)

	// 收集容器汇总
	da.data.ContainerSummary = da.collectContainerSummary(nodes)

	// 收集服务汇总
	da.data.ServiceSummary = da.collectServiceSummary(nodes)

	// 收集资源趋势
	da.data.ResourceTrend = da.collectResourceTrend()

	// 如果启用历史数据，添加历史点
	if da.config.EnableHistory {
		da.addHistoryPoint()
	}

	// 触发更新回调
	if da.onDataUpdate != nil {
		go da.onDataUpdate(da.data)
	}

	// 持久化数据
	_ = da.saveData()
}

// collectClusterOverview 收集集群概览
func (da *DashboardAggregator) collectClusterOverview(nodes []*NodeInfo) ClusterOverview {
	overview := ClusterOverview{
		TotalNodes: len(nodes),
	}

	for _, node := range nodes {
		switch node.Status {
		case NodeStateActive:
			overview.OnlineNodes++
		case NodeStateFailed:
			overview.OfflineNodes++
		case NodeStateSuspect:
			overview.SuspectNodes++
		}

		if node.Role == "master" {
			overview.MasterNodeID = node.ID
		}
	}

	// 计算健康状态
	if overview.OfflineNodes > 0 || overview.SuspectNodes > overview.TotalNodes/2 {
		overview.HealthStatus = "critical"
	} else if overview.SuspectNodes > 0 {
		overview.HealthStatus = "degraded"
	} else {
		overview.HealthStatus = "healthy"
	}

	return overview
}

// collectNodeSummary 收集节点汇总
func (da *DashboardAggregator) collectNodeSummary(nodes []*NodeInfo) NodeSummary {
	summary := NodeSummary{
		Nodes: make([]NodeDetail, 0, len(nodes)),
	}

	totalCPUUsage := 0.0
	totalMemUsage := 0.0
	totalDiskUsage := 0.0

	for _, node := range nodes {
		// 收集详细数据
		detail := da.collectNodeDetail(node)
		summary.Nodes = append(summary.Nodes, detail)

		// 累加统计
		summary.TotalCPUCores += detail.CPUCores
		summary.TotalMemoryGB += detail.MemoryGB
		summary.TotalDiskGB += detail.DiskGB

		if node.Status == NodeStateActive {
			totalCPUUsage += node.CPUUsage
			totalMemUsage += node.MemoryUsage
			totalDiskUsage += node.DiskUsage
		}
	}

	// 计算平均值
	onlineNodes := da.data.ClusterOverview.OnlineNodes
	if onlineNodes > 0 {
		summary.AvgCPUUsage = totalCPUUsage / float64(onlineNodes)
		summary.AvgMemoryUsage = totalMemUsage / float64(onlineNodes)
		summary.AvgDiskUsage = totalDiskUsage / float64(onlineNodes)
	}

	return summary
}

// collectNodeDetail 收集节点详情
func (da *DashboardAggregator) collectNodeDetail(node *NodeInfo) NodeDetail {
	detail := NodeDetail{
		ID:           node.ID,
		Name:         node.Name,
		Address:      node.Address,
		Status:       string(node.Status),
		Role:         node.Role,
		Version:      node.Version,
		CPUCores:     node.CPUCores,
		CPUUsage:     node.CPUUsage,
		MemoryUsage:  node.MemoryUsage,
		DiskUsage:    node.DiskUsage,
		LastSeen:     node.LastSeen,
		Capabilities: node.Capabilities,
	}

	// 转换容量单位（bytes -> GB）
	detail.MemoryGB = float64(node.MemoryTotal) / (1024 * 1024 * 1024)
	detail.DiskGB = float64(node.DiskTotal) / (1024 * 1024 * 1024)

	// 计算运行时间
	if node.Status == NodeStateActive {
		detail.Uptime = formatUptime(time.Since(node.RegisterTime))
	} else {
		detail.Uptime = "offline"
	}

	// 从节点获取更多数据（可选）
	if node.Status == NodeStateActive {
		nodeDetail, err := da.fetchNodeDetail(node)
		if err == nil && nodeDetail != nil {
			// 合并获取的数据
			detail.CPUUsage = nodeDetail.CPUUsage
			detail.MemoryUsage = nodeDetail.MemoryUsage
			detail.DiskUsage = nodeDetail.DiskUsage
		}
	}

	return detail
}

// fetchNodeDetail 从节点获取详情
func (da *DashboardAggregator) fetchNodeDetail(node *NodeInfo) (*NodeDetail, error) {
	ctx, cancel := context.WithTimeout(da.ctx, da.config.NodeTimeout)
	defer cancel()

	url := fmt.Sprintf("http://%s:%d/api/v1/node/detail", node.Address, node.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Node-ID", node.ID)

	resp, err := da.httpClient.Do(req)
	if err != nil {
		da.logger.Debug("获取节点详情失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	var detail NodeDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}

	return &detail, nil
}

// collectStorageSummary 收集存储汇总
func (da *DashboardAggregator) collectStorageSummary(nodes []*NodeInfo) StorageSummary {
	summary := StorageSummary{
		NodeStorage: make([]NodeStorageInfo, 0, len(nodes)),
	}

	for _, node := range nodes {
		if node.Status != NodeStateActive {
			continue
		}

		// 从节点获取存储数据
		storageInfo, err := da.fetchNodeStorage(node)
		if err != nil {
			da.logger.Debug("获取节点存储失败",
				zap.String("node_id", node.ID),
				zap.Error(err))
			continue
		}

		summary.NodeStorage = append(summary.NodeStorage, *storageInfo)

		// 累加统计
		summary.TotalPools += len(storageInfo.Pools)
		summary.TotalVolumes += len(storageInfo.Volumes)
		summary.TotalCapacityGB += storageInfo.TotalGB
		summary.UsedCapacityGB += storageInfo.UsedGB
		summary.FreeCapacityGB += storageInfo.FreeGB
	}

	// 计算使用率
	if summary.TotalCapacityGB > 0 {
		summary.UsagePercent = summary.UsedCapacityGB / summary.TotalCapacityGB * 100
	}

	return summary
}

// fetchNodeStorage 从节点获取存储数据
func (da *DashboardAggregator) fetchNodeStorage(node *NodeInfo) (*NodeStorageInfo, error) {
	ctx, cancel := context.WithTimeout(da.ctx, da.config.NodeTimeout)
	defer cancel()

	url := fmt.Sprintf("http://%s:%d/api/v1/storage/summary", node.Address, node.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Node-ID", node.ID)

	resp, err := da.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info NodeStorageInfo
	info.NodeID = node.ID
	info.NodeName = node.Name

	// 解析响应
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		// 使用模拟数据
		info.TotalGB = float64(node.DiskTotal) / (1024 * 1024 * 1024)
		info.UsedGB = info.TotalGB * node.DiskUsage / 100
		info.FreeGB = info.TotalGB - info.UsedGB
		info.UsagePercent = node.DiskUsage
	}

	return &info, nil
}

// collectContainerSummary 收集容器汇总
func (da *DashboardAggregator) collectContainerSummary(nodes []*NodeInfo) ContainerSummary {
	summary := ContainerSummary{
		NodeContainers: make([]NodeContainerInfo, 0, len(nodes)),
	}

	for _, node := range nodes {
		if node.Status != NodeStateActive {
			continue
		}

		// 从节点获取容器数据
		containerInfo, err := da.fetchNodeContainers(node)
		if err != nil {
			da.logger.Debug("获取节点容器失败",
				zap.String("node_id", node.ID),
				zap.Error(err))
			continue
		}

		summary.NodeContainers = append(summary.NodeContainers, *containerInfo)

		// 累加统计
		summary.TotalContainers += len(containerInfo.Containers)
		summary.RunningContainers += containerInfo.RunningCount
		summary.StoppedContainers += containerInfo.StoppedCount
	}

	return summary
}

// fetchNodeContainers 从节点获取容器数据
func (da *DashboardAggregator) fetchNodeContainers(node *NodeInfo) (*NodeContainerInfo, error) {
	ctx, cancel := context.WithTimeout(da.ctx, da.config.NodeTimeout)
	defer cancel()

	url := fmt.Sprintf("http://%s:%d/api/v1/containers/summary", node.Address, node.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Node-ID", node.ID)

	resp, err := da.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info NodeContainerInfo
	info.NodeID = node.ID
	info.NodeName = node.Name

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		// 使用空数据
		info.Containers = []ContainerInfo{}
	}

	return &info, nil
}

// collectServiceSummary 收集服务汇总
func (da *DashboardAggregator) collectServiceSummary(nodes []*NodeInfo) ServiceSummary {
	summary := ServiceSummary{
		NodeServices: make([]NodeServiceInfo, 0, len(nodes)),
	}

	for _, node := range nodes {
		if node.Status != NodeStateActive {
			continue
		}

		// 使用节点注册的服务信息
		info := NodeServiceInfo{
			NodeID:   node.ID,
			NodeName: node.Name,
		}

		for _, svc := range node.Services {
			info.Services = append(info.Services, ServiceInfo{
				Name:    svc.Name,
				Status:  svc.Status,
				Port:    svc.Port,
				Version: svc.Version,
			})

			summary.TotalServices++
			if svc.Status == "running" {
				info.RunningCount++
				summary.RunningServices++
			} else {
				info.StoppedCount++
				summary.StoppedServices++
			}
		}

		summary.NodeServices = append(summary.NodeServices, info)
	}

	return summary
}

// collectResourceTrend 收集资源趋势
func (da *DashboardAggregator) collectResourceTrend() ResourceTrend {
	// 简化实现：使用历史数据
	trend := ResourceTrend{}

	if !da.config.EnableHistory || len(da.data.HistoricalData) == 0 {
		return trend
	}

	// 从历史数据提取趋势
	for _, point := range da.data.HistoricalData {
		trend.CPUTrend = append(trend.CPUTrend, TrendPoint{
			Timestamp: point.Timestamp,
			Value:     point.Nodes.AvgCPUUsage,
		})

		trend.MemoryTrend = append(trend.MemoryTrend, TrendPoint{
			Timestamp: point.Timestamp,
			Value:     point.Nodes.AvgMemoryUsage,
		})

		trend.DiskTrend = append(trend.DiskTrend, TrendPoint{
			Timestamp: point.Timestamp,
			Value:     point.Storage.UsagePercent,
		})
	}

	return trend
}

// addHistoryPoint 添加历史数据点
func (da *DashboardAggregator) addHistoryPoint() {
	point := TimeSeriesPoint{
		Timestamp: da.data.Timestamp,
		Cluster:   da.data.ClusterOverview,
		Nodes:     da.data.NodeSummary,
		Storage:   da.data.StorageSummary,
	}

	da.data.HistoricalData = append(da.data.HistoricalData, point)

	// 清理过期历史数据
	cutoff := time.Now().Add(-da.config.HistoryRetention)
	for i, p := range da.data.HistoricalData {
		if p.Timestamp.After(cutoff) {
			da.data.HistoricalData = da.data.HistoricalData[i:]
			break
		}
	}
}

// GetData 获取聚合数据
func (da *DashboardAggregator) GetData() *AggregatedData {
	da.dataMutex.RLock()
	defer da.dataMutex.RUnlock()

	return da.data
}

// GetCachedData 获取缓存数据（带过期检查）
func (da *DashboardAggregator) GetCachedData() (*AggregatedData, bool) {
	da.dataMutex.RLock()
	defer da.dataMutex.RUnlock()

	if time.Since(da.data.Timestamp) > da.config.CacheDuration {
		return nil, false
	}

	return da.data, true
}

// SetOnDataUpdate 设置数据更新回调
func (da *DashboardAggregator) SetOnDataUpdate(callback func(data *AggregatedData)) {
	da.onDataUpdate = callback
}

// Shutdown 关闭聚合器
func (da *DashboardAggregator) Shutdown() error {
	da.cancel()
	da.wg.Wait()

	_ = da.saveData()
	da.logger.Info("仪表板聚合器已关闭")
	return nil
}

// saveData 持久化数据
func (da *DashboardAggregator) saveData() error {
	dataFile := fmt.Sprintf("%s/dashboard_data.json", da.config.DataDir)

	data, err := json.MarshalIndent(da.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dataFile, data, 0640)
}

// formatUptime 格式化运行时间
func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分钟", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d小时", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%d天%d小时", days, hours)
}
