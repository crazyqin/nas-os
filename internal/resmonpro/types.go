package resmonpro

// ProcessInfo 进程资源信息
type ProcessInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	CPU        float64 `json:"cpu_percent"`
	Memory     float64 `json:"memory_mb"`
	MemoryPerc float64 `json:"memory_percent"`
	ReadBps    int64   `json:"read_bps"`
	WriteBps   int64   `json:"write_bps"`
	NetBps     int64   `json:"net_bps"`
	Status     string  `json:"status"`
}

// GPUInfo GPU 信息
type GPUInfo struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Temp      int     `json:"temp_celsius"`
	Util      int     `json:"utilization_percent"`
	MemUsed   int64   `json:"mem_used_mb"`
	MemTotal  int64   `json:"mem_total_mb"`
	PowerDraw float64 `json:"power_draw_w"`
	FanSpeed  int     `json:"fan_speed_percent"`
}

// NetworkFlow 网络流量信息
type NetworkFlow struct {
	Interface  string `json:"interface"`
	SrcIP      string `json:"src_ip"`
	DstIP      string `json:"dst_ip"`
	Protocol   string `json:"protocol"`
	BytesIn    int64  `json:"bytes_in"`
	BytesOut   int64  `json:"bytes_out"`
	PacketsIn  int64  `json:"packets_in"`
	PacketsOut int64  `json:"packets_out"`
}

// DiskIOInfo 磁盘 I/O 信息
type DiskIOInfo struct {
	Device     string  `json:"device"`
	ReadIOPS   int64   `json:"read_iops"`
	WriteIOPS  int64   `json:"write_iops"`
	ReadBps    int64   `json:"read_bps"`
	WriteBps   int64   `json:"write_bps"`
	ReadLatMs  float64 `json:"read_latency_ms"`
	WriteLatMs float64 `json:"write_latency_ms"`
	QueueDepth int     `json:"queue_depth"`
	BusyPerc   float64 `json:"busy_percent"`
}

// BottleneckDiagnosis 瓶颈诊断
type BottleneckDiagnosis struct {
	Component  string `json:"component"`
	Severity   string `json:"severity"`
	Issue      string `json:"issue"`
	Suggestion string `json:"suggestion"`
}

// ResmonProResponse 通用响应结构
type ResmonProResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
