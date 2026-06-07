package federatednas

import (
	"sync"
	"time"
)

// NodeStatus represents the operational status of a federation node.
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusSyncing NodeStatus = "syncing"
	NodeStatusError   NodeStatus = "error"
)

// ConflictResolution defines how conflicts are resolved.
type ConflictResolution string

const (
	ConflictResolutionAuto   ConflictResolution = "auto"
	ConflictResolutionManual ConflictResolution = "manual"
	ConflictResolutionNewest ConflictResolution = "newest"
	ConflictResolutionOldest ConflictResolution = "oldest"
	ConflictResolutionSource ConflictResolution = "source"
)

// FederationNode represents a NAS device in the federation.
type FederationNode struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Address        string             `json:"address"`
	Port           int                `json:"port"`
	Status         NodeStatus         `json:"status"`
	Capacity       int64              `json:"capacity"`
	UsedSpace      int64              `json:"used_space"`
	LastSeen       time.Time          `json:"last_seen"`
	RegisteredAt   time.Time          `json:"registered_at"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
	SyncVersion    int64              `json:"sync_version"`
	ConflictPolicy ConflictResolution `json:"conflict_policy"`
	mu             sync.RWMutex
}

// UpdateStatus updates the node's status and last seen time.
func (n *FederationNode) UpdateStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
	n.LastSeen = time.Now()
}

// SyncJob represents a synchronization job between nodes.
type SyncJob struct {
	ID           string     `json:"id"`
	SourceNodeID string     `json:"source_node_id"`
	TargetNodeID string     `json:"target_node_id"`
	Status       string     `json:"status"`
	TotalFiles   int        `json:"total_files"`
	SyncedFiles  int        `json:"synced_files"`
	FailedFiles  int        `json:"failed_files"`
	BytesTotal   int64      `json:"bytes_total"`
	BytesSynced  int64      `json:"bytes_synced"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Incremental  bool       `json:"incremental"`
	Resumable    bool       `json:"resumable"`
	LastOffset   int64      `json:"last_offset"`
	mu           sync.RWMutex
}

// Progress returns sync progress as a percentage (0-100).
func (j *SyncJob) Progress() float64 {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if j.TotalFiles == 0 {
		return 0
	}
	return float64(j.SyncedFiles) / float64(j.TotalFiles) * 100
}

// ConflictRecord represents a detected conflict during synchronization.
type ConflictRecord struct {
	ID            string     `json:"id"`
	SyncJobID     string     `json:"sync_job_id"`
	FilePath      string     `json:"file_path"`
	SourceNodeID  string     `json:"source_node_id"`
	TargetNodeID  string     `json:"target_node_id"`
	SourceModTime time.Time  `json:"source_mod_time"`
	TargetModTime time.Time  `json:"target_mod_time"`
	SourceHash    string     `json:"source_hash"`
	TargetHash    string     `json:"target_hash"`
	Resolution    string     `json:"resolution,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy    string     `json:"resolved_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// FederationPolicy defines the federation synchronization policy.
type FederationPolicy struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	SyncInterval       time.Duration      `json:"sync_interval"`
	ConflictResolution ConflictResolution `json:"conflict_resolution"`
	BandwidthLimit     int64              `json:"bandwidth_limit"`
	IncludePatterns    []string           `json:"include_patterns,omitempty"`
	ExcludePatterns    []string           `json:"exclude_patterns,omitempty"`
	AutoResolve        bool               `json:"auto_resolve"`
	RetryAttempts      int                `json:"retry_attempts"`
	RetryDelay         time.Duration      `json:"retry_delay"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// DistributedNamespace represents a unified view across federated nodes.
type DistributedNamespace struct {
	Path     string            `json:"path"`
	NodeID   string            `json:"node_id"`
	IsDir    bool              `json:"is_dir"`
	Size     int64             `json:"size"`
	ModTime  time.Time         `json:"mod_time"`
	Hash     string            `json:"hash,omitempty"`
	Replicas []string          `json:"replicas,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NodeHealth represents health metrics of a federation node.
type NodeHealth struct {
	NodeID        string    `json:"node_id"`
	CPUUsage      float64   `json:"cpu_usage"`
	MemoryUsage   float64   `json:"memory_usage"`
	DiskUsage     float64   `json:"disk_usage"`
	NetworkIn     int64     `json:"network_in"`
	NetworkOut    int64     `json:"network_out"`
	IOPS          int64     `json:"iops"`
	LatencyMs     float64   `json:"latency_ms"`
	Uptime        int64     `json:"uptime"`
	LastCheckTime time.Time `json:"last_check_time"`
}

// FederationStatus represents the overall status of the federation.
type FederationStatus struct {
	TotalNodes    int        `json:"total_nodes"`
	OnlineNodes   int        `json:"online_nodes"`
	OfflineNodes  int        `json:"offline_nodes"`
	SyncingNodes  int        `json:"syncing_nodes"`
	ErrorNodes    int        `json:"error_nodes"`
	ActiveJobs    int        `json:"active_jobs"`
	TotalCapacity int64      `json:"total_capacity"`
	UsedSpace     int64      `json:"used_space"`
	Conflicts     int        `json:"pending_conflicts"`
	LastSyncTime  *time.Time `json:"last_sync_time,omitempty"`
}
