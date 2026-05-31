package fedcluster

import "time"

// ClusterInfo 集群信息摘要
type ClusterInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	NodeCount   int       `json:"node_count"`
	OnlineNodes int       `json:"online_nodes"`
	TotalTB     float64   `json:"total_tb"`
	UsedTB      float64   `json:"used_tb"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// NodeInfo 节点信息摘要
type NodeInfo struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Role     ClusterRole `json:"role"`
	Status   NodeStatus  `json:"status"`
	StorageTB float64    `json:"storage_tb"`
	Usage    float64     `json:"usage_percent"`
}

// SyncRequest 同步请求
type SyncRequest struct {
	ClusterID  string `json:"cluster_id" binding:"required"`
	SourceNode string `json:"source_node" binding:"required"`
	TargetNode string `json:"target_node" binding:"required"`
	SourcePath string `json:"source_path" binding:"required"`
	TargetPath string `json:"target_path" binding:"required"`
	Recursive  bool   `json:"recursive"`
	DeleteExcess bool `json:"delete_excess"`
}

// ClusterCreateRequest 创建集群请求
type ClusterCreateRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	SyncPolicy  SyncPolicy `json:"sync_policy"`
	AutoHeal    bool       `json:"auto_heal"`
	LoadBalance bool       `json:"load_balance"`
}

// NodeJoinRequest 节点加入请求
type NodeJoinRequest struct {
	Hostname string   `json:"hostname" binding:"required"`
	Port     int      `json:"port"`
	Name     string   `json:"name"`
	Role     ClusterRole `json:"role"`
	Tags     []string `json:"tags"`
}

// FailoverRequest 故障转移请求
type FailoverRequest struct {
	ClusterID   string `json:"cluster_id" binding:"required"`
	FailedNode  string `json:"failed_node" binding:"required"`
	TargetNode  string `json:"target_node"`
	AutoSelect  bool   `json:"auto_select"`
}

// RebalanceRequest 重新平衡请求
type RebalanceRequest struct {
	ClusterID string `json:"cluster_id" binding:"required"`
	DryRun    bool   `json:"dry_run"`
}

// RebalanceResult 重新平衡结果
type RebalanceResult struct {
	ClusterID    string            `json:"cluster_id"`
	DryRun       bool              `json:"dry_run"`
	Actions      []*RebalanceAction `json:"actions"`
	EstimatedTime int              `json:"estimated_time_seconds"`
}

// RebalanceAction 重新平衡动作
type RebalanceAction struct {
	Type       string  `json:"type"` // migrate, replicate, cleanup
	SourceNode string  `json:"source_node"`
	TargetNode string  `json:"target_node,omitempty"`
	Path       string  `json:"path"`
	SizeBytes  int64   `json:"size_bytes"`
	Priority   int     `json:"priority"`
}

// ClusterHealth 集群健康状态
type ClusterHealth struct {
	ClusterID    string         `json:"cluster_id"`
	OverallStatus string        `json:"overall_status"` // healthy, degraded, critical
	NodeHealth   map[string]bool `json:"node_health"`
	SyncStatus   string         `json:"sync_status"`
	LastCheck    time.Time      `json:"last_check"`
	Issues       []string       `json:"issues,omitempty"`
}

// ClusterMetrics 集群指标
type ClusterMetrics struct {
	ClusterID      string    `json:"cluster_id"`
	TotalStorage   float64   `json:"total_storage_tb"`
	UsedStorage    float64   `json:"used_storage_tb"`
	TotalNodes     int       `json:"total_nodes"`
	OnlineNodes    int       `json:"online_nodes"`
	AvgCPUUsage    float64   `json:"avg_cpu_usage"`
	AvgMemoryUsage float64   `json:"avg_memory_usage"`
	TotalIOPS      int       `json:"total_iops"`
	NetworkInMB    float64   `json:"network_in_mb"`
	NetworkOutMB   float64   `json:"network_out_mb"`
	Timestamp      time.Time `json:"timestamp"`
}

// ClusterAlert 集群告警
type ClusterAlert struct {
	ID        string    `json:"id"`
	ClusterID string    `json:"cluster_id"`
	NodeID    string    `json:"node_id,omitempty"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}
