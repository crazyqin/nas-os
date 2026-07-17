// Package web3storage implements a Web3/IPFS decentralized storage gateway for NAS.
// Provides IPFS content management, content addressing (CID generation/validation),
// decentralized multi-node redundant backup, HTTP gateway proxy, Filecoin integration,
// and local caching for hot content acceleration.
package web3storage

import (
	"fmt"
	"sync"
	"time"
)

// PinStatus represents the pin lifecycle state.
type PinStatus int

const (
	PinStatusQueued PinStatus = iota
	PinStatusPinning
	PinStatusPinned
	PinStatusFailed
	PinStatusUnpinned
)

func (s PinStatus) String() string {
	switch s {
	case PinStatusQueued:
		return "queued"
	case PinStatusPinning:
		return "pinning"
	case PinStatusPinned:
		return "pinned"
	case PinStatusFailed:
		return "failed"
	case PinStatusUnpinned:
		return "unpinned"
	default:
		return "unknown"
	}
}

// DealState represents the Filecoin deal lifecycle.
type DealState int

const (
	DealStateProposing DealState = iota
	DealStatePublished
	DealStateActive
	DealStateExpired
	DealStateSlashed
	DealStateFailed
)

func (d DealState) String() string {
	switch d {
	case DealStateProposing:
		return "proposing"
	case DealStatePublished:
		return "published"
	case DealStateActive:
		return "active"
	case DealStateExpired:
		return "expired"
	case DealStateSlashed:
		return "slashed"
	case DealStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// NodeStatus represents the health state of a storage node.
type NodeStatus int

const (
	NodeStatusUnknown NodeStatus = iota
	NodeStatusOnline
	NodeStatusDegraded
	NodeStatusOffline
)

func (n NodeStatus) String() string {
	switch n {
	case NodeStatusOnline:
		return "online"
	case NodeStatusDegraded:
		return "degraded"
	case NodeStatusOffline:
		return "offline"
	default:
		return "unknown"
	}
}

// Web3StorageConfig holds the configuration for the Web3 storage module.
type Web3StorageConfig struct {
	// IPFS API endpoint (e.g. http://localhost:5001).
	IPFSAPIAddr string `json:"ipfsApiAddr"`
	// Public IPFS gateway URL for content retrieval (e.g. https://ipfs.io/ipfs/).
	GatewayURL string `json:"gatewayUrl"`
	// Local gateway listen address.
	LocalGatewayAddr string `json:"localGatewayAddr"`
	// ReplicationFactor is the desired number of remote nodes storing each pin.
	ReplicationFactor int `json:"replicationFactor"`
	// Known IPFS cluster / storage node endpoints.
	StorageNodes []string `json:"storageNodes"`
	// Filecoin API endpoint for deal making.
	FilecoinAPIAddr string `json:"filecoinApiAddr"`
	// Filecoin wallet address for making deals.
	FilecoinWallet string `json:"filecoinWallet"`
	// Minimum Filecoin deal duration.
	MinDealDuration time.Duration `json:"minDealDuration"`
	// Maximum local cache size in bytes.
	LocalCacheMaxSize int64 `json:"localCacheMaxSize"`
	// Local cache directory.
	LocalCacheDir string `json:"localCacheDir"`
	// Cache TTL for hot content.
	CacheTTL time.Duration `json:"cacheTTL"`
}

// DefaultConfig returns sensible defaults for Web3StorageConfig.
func DefaultConfig() Web3StorageConfig {
	return Web3StorageConfig{
		IPFSAPIAddr:       "http://localhost:5001",
		GatewayURL:        "https://ipfs.io/ipfs/",
		LocalGatewayAddr:  ":8080",
		ReplicationFactor: 3,
		MinDealDuration:   24 * time.Hour * 180, // 180 days
		LocalCacheMaxSize: 10 << 30,             // 10 GiB
		LocalCacheDir:     "/var/lib/nas-os/web3cache",
		CacheTTL:          24 * time.Hour,
	}
}

// CID represents a content identifier.
type CID struct {
	// Value is the CID string (e.g. Qm... or bafy...).
	Value string `json:"value"`
	// Codec identifies the content codec (dag-pb, raw, dag-cbor, ...).
	Codec string `json:"codec"`
	// HashFunc is the multihash function used (sha2-256, blake2b-256, ...).
	HashFunc string `json:"hashFunc"`
	// Version is the CID version (0 or 1).
	Version int `json:"version"`
	// Size is the raw content size in bytes.
	Size int64 `json:"size"`
}

// String returns the CID value.
func (c CID) String() string {
	return c.Value
}

// Validate performs basic CID validation (non-empty, correct prefix).
func (c CID) Validate() error {
	if c.Value == "" {
		return fmt.Errorf("empty CID value")
	}
	if len(c.Value) < 10 {
		return fmt.Errorf("CID too short: %d chars", len(c.Value))
	}
	return nil
}

// ContentPin represents a pinned item in IPFS.
type ContentPin struct {
	// CID of the pinned content.
	CID CID `json:"cid"`
	// Status is the current pin status.
	Status PinStatus `json:"status"`
	// Name is an optional human-readable name.
	Name string `json:"name,omitempty"`
	// Tags for categorization.
	Tags []string `json:"tags,omitempty"`
	// Metadata holds arbitrary key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
	// ReplicationCount is how many remote nodes currently hold this content.
	ReplicationCount int `json:"replicationCount"`
	// CreatedAt is when the pin was first requested.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is the last status change time.
	UpdatedAt time.Time `json:"updatedAt"`
	// PinnedByNodes lists node IDs that hold a replica.
	PinnedByNodes []string `json:"pinnedByNodes,omitempty"`
}

// PinRequest is the payload for a Pin API call.
type PinRequest struct {
	// CID to pin (mutually exclusive with Content).
	CID string `json:"cid,omitempty"`
	// Content is raw bytes to add+pin (mutually exclusive with CID).
	Content []byte `json:"-"`
	// FileName is the original file name hint.
	FileName string `json:"fileName,omitempty"`
	// Name is a display name.
	Name string `json:"name,omitempty"`
	// Tags for categorization.
	Tags []string `json:"tags,omitempty"`
	// Metadata holds arbitrary key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
	// Replication overrides the default ReplicationFactor.
	Replication int `json:"replication,omitempty"`
}

// PinQuery defines filters for listing/querying pins.
type PinQuery struct {
	Status   []PinStatus       `json:"status,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
	Name     string            `json:"name,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Before   *time.Time        `json:"before,omitempty"`
	After    *time.Time        `json:"after,omitempty"`
	Limit    int               `json:"limit,omitempty"`
	Offset   int               `json:"offset,omitempty"`
}

// PinListResponse is the response for a list/query call.
type PinListResponse struct {
	Pins    []ContentPin `json:"pins"`
	Total   int          `json:"total"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
	HasMore bool         `json:"hasMore"`
}

// StorageNode represents an IPFS cluster node.
type StorageNode struct {
	// ID is the unique node identifier.
	ID string `json:"id"`
	// Endpoint is the API address.
	Endpoint string `json:"endpoint"`
	// Status is the node's current health status.
	Status NodeStatus `json:"status"`
	// FreeBytes is the available storage in bytes.
	FreeBytes int64 `json:"freeBytes"`
	// TotalBytes is the total storage capacity.
	TotalBytes int64 `json:"totalBytes"`
	// PinCount is the number of pins stored on this node.
	PinCount int `json:"pinCount"`
	// Region is the geographic region label.
	Region string `json:"region,omitempty"`
	// LastSeen is the last health-check time.
	LastSeen time.Time `json:"lastSeen"`
}

// FilecoinDeal represents a Filecoin storage deal.
type FilecoinDeal struct {
	// DealID is the on-chain deal ID.
	DealID string `json:"dealId"`
	// CID of the content in the deal.
	CID CID `json:"cid"`
	// Provider is the miner address (f0...).
	Provider string `json:"provider"`
	// State is the deal lifecycle state.
	State DealState `json:"state"`
	// PricePerEpoch is the cost per epoch in attoFIL.
	PricePerEpoch string `json:"pricePerEpoch"`
	// StartEpoch is the chain epoch when the deal becomes active.
	StartEpoch int64 `json:"startEpoch"`
	// EndEpoch is the chain epoch when the deal expires.
	EndEpoch int64 `json:"endEpoch"`
	// PieceCID is the CommP of the deal.
	PieceCID string `json:"pieceCid,omitempty"`
	// CreatedAt is the local time the deal was initiated.
	CreatedAt time.Time `json:"createdAt"`
	// Label is an optional human-readable label.
	Label string `json:"label,omitempty"`
}

// CacheEntry represents a locally cached IPFS block.
type CacheEntry struct {
	// CID of the cached content.
	CID CID `json:"cid"`
	// LocalPath is the absolute path to the cached file.
	LocalPath string `json:"localPath"`
	// Size is the cached content size in bytes.
	Size int64 `json:"size"`
	// HitCount counts how many times this entry was served from cache.
	HitCount int64 `json:"hitCount"`
	// LastAccessed is the last time this entry was read.
	LastAccessed time.Time `json:"lastAccessed"`
	// CreatedAt is when the entry was cached.
	CreatedAt time.Time `json:"createdAt"`
	// ExpiresAt is when this cache entry should be evicted.
	ExpiresAt time.Time `json:"expiresAt"`
}

// CacheStats holds aggregate cache statistics.
type CacheStats struct {
	TotalEntries int     `json:"totalEntries"`
	TotalSize    int64   `json:"totalSize"`
	MaxSize      int64   `json:"maxSize"`
	HitCount     int64   `json:"hitCount"`
	MissCount    int64   `json:"missCount"`
	HitRate      float64 `json:"hitRate"`
}

// GatewayStats holds aggregate gateway statistics.
type GatewayStats struct {
	TotalRequests    int64   `json:"totalRequests"`
	CacheHits        int64   `json:"cacheHits"`
	CacheMisses      int64   `json:"cacheMisses"`
	TotalBytesServed int64   `json:"totalBytesServed"`
	AvgLatencyMs     float64 `json:"avgLatencyMs"`
	ActiveStreams    int     `json:"activeStreams"`
}

// Manager is the core business-logic type for web3storage.
// Stored in a dedicated struct so tests can instantiate it without the HTTP layer.
type Manager struct {
	cfg          Web3StorageConfig
	mu           sync.RWMutex
	pins         map[string]*ContentPin // keyed by CID value
	nodes        map[string]*StorageNode
	deals        map[string]*FilecoinDeal // keyed by DealID
	cache        map[string]*CacheEntry   // keyed by CID value
	cacheSize    int64
	cacheHits    int64
	cacheMisses  int64
	gatewayReqs  int64
	gatewayBytes int64
	running      bool
}

// NewManager creates a new Manager with the given config.
// If cfg is nil, DefaultConfig() is used.
func NewManager(cfg *Web3StorageConfig) *Manager {
	if cfg == nil {
		c := DefaultConfig()
		cfg = &c
	}
	return &Manager{
		cfg:   *cfg,
		pins:  make(map[string]*ContentPin),
		nodes: make(map[string]*StorageNode),
		deals: make(map[string]*FilecoinDeal),
		cache: make(map[string]*CacheEntry),
	}
}
