// Package hybridcloud provides hybrid cloud intelligent tiered storage capabilities.
// It combines local and cloud storage into a unified pool with automatic data tiering,
// on-demand caching, multi-cloud support, and cost optimization.
package hybridcloud

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Data tier levels
const (
	TierHot  = "hot"  // Local storage - frequently accessed
	TierWarm = "warm" // Recently accessed, may be local or cloud
	TierCold = "cold" // Cloud storage - rarely accessed
)

// Cloud provider types
const (
	ProviderS3     = "s3"
	ProviderOSS    = "oss"     // Alibaba Cloud OSS
	ProviderCOS    = "cos"     // Tencent Cloud COS
	ProviderAzure  = "azure"   // Azure Blob Storage
)

// Sync states
const (
	SyncStateIdle       = "idle"
	SyncStateSyncing    = "syncing"
	SyncStateCompleted  = "completed"
	SyncStateFailed     = "failed"
	SyncStatePaused     = "paused"
)

// Pool status
const (
	PoolStatusActive  = "active"
	PoolStatusDegraded = "degraded"
	PoolStatusOffline = "offline"
)

// Core types

// Manager manages hybrid cloud storage pools
type Manager struct {
	mu             sync.RWMutex
	pools          map[string]*StoragePool
	providers      map[string]CloudProvider
	syncTasks      map[string]*SyncTask
	cache          map[string]*CacheEntry
	tierPolicies   map[string]*TierPolicy
	config         *Config
	encryptionKey  []byte
	stats          *ManagerStats
}

// StoragePool represents a hybrid storage pool
type StoragePool struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	LocalPath    string                 `json:"local_path"`
	CloudBucket  string                 `json:"cloud_bucket"`
	Provider     string                 `json:"provider"`
	Status       string                 `json:"status"`
	TotalSize    int64                  `json:"total_size"`
	UsedSize     int64                  `json:"used_size"`
	LocalSize    int64                  `json:"local_size"`
	CloudSize    int64                  `json:"cloud_size"`
	Files        map[string]*FileEntry  `json:"files"`
	TierPolicy   string                 `json:"tier_policy"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Metadata     map[string]string      `json:"metadata"`
}

// FileEntry represents a file in the storage pool
type FileEntry struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Tier         string    `json:"tier"`
	CloudKey     string    `json:"cloud_key,omitempty"`
	LocalPath    string    `json:"local_path,omitempty"`
	Checksum     string    `json:"checksum"`
	Encrypted    bool      `json:"encrypted"`
	LastAccess   time.Time `json:"last_access"`
	LastModified time.Time `json:"last_modified"`
	SyncState    string    `json:"sync_state"`
	AccessCount  int64     `json:"access_count"`
}

// CloudProvider defines the interface for cloud storage providers
type CloudProvider interface {
	// Name returns the provider name
	Name() string
	// Type returns the provider type (s3, oss, cos, azure)
	Type() string
	// Upload uploads data to cloud storage
	Upload(ctx context.Context, key string, data []byte, opts *UploadOptions) error
	// Download downloads data from cloud storage
	Download(ctx context.Context, key string) ([]byte, error)
	// Delete deletes data from cloud storage
	Delete(ctx context.Context, key string) error
	// Exists checks if a key exists in cloud storage
	Exists(ctx context.Context, key string) (bool, error)
	// List lists objects with the given prefix
	List(ctx context.Context, prefix string) ([]CloudObject, error)
	// GetInfo returns provider information and status
	GetInfo(ctx context.Context) (*ProviderInfo, error)
	// EstimateStorageCost estimates monthly storage cost
	EstimateStorageCost(sizeBytes int64) float64
	// EstimateTransferCost estimates transfer cost
	EstimateTransferCost(bytes int64) float64
}

// TierPolicy defines data tiering rules
type TierPolicy struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	HotThreshold    time.Duration `json:"hot_threshold"`    // Access within this duration = hot
	WarmThreshold   time.Duration `json:"warm_threshold"`   // Access within this duration = warm
	MinLocalSize    int64         `json:"min_local_size"`   // Minimum local storage to keep
	MaxLocalSize    int64         `json:"max_local_size"`   // Maximum local storage allowed
	AutoTierEnabled bool          `json:"auto_tier_enabled"`
	ScheduleCron    string        `json:"schedule_cron"`    // Cron expression for tier evaluation
	CreatedAt       time.Time     `json:"created_at"`
}

// SyncTask represents a synchronization task
type SyncTask struct {
	ID           string        `json:"id"`
	PoolID       string        `json:"pool_id"`
	Direction    string        `json:"direction"` // "up" (local->cloud) or "down" (cloud->local)
	State        string        `json:"state"`
	Progress     float64       `json:"progress"` // 0.0 - 1.0
	TotalBytes   int64         `json:"total_bytes"`
	SyncedBytes  int64         `json:"synced_bytes"`
	TotalFiles   int           `json:"total_files"`
	SyncedFiles  int           `json:"synced_files"`
	ErrorMessage string        `json:"error_message,omitempty"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

// CacheEntry represents a cached cloud object
type CacheEntry struct {
	Key          string    `json:"key"`
	LocalPath    string    `json:"local_path"`
	Size         int64     `json:"size"`
	LastAccess   time.Time `json:"last_access"`
	HitCount     int64     `json:"hit_count"`
	ExpiresAt    time.Time `json:"expires_at"`
	Pinned       bool      `json:"pinned"` // Prevent eviction
}

// CostEstimate represents cost estimation results
type CostEstimate struct {
	StorageCostMonthly   float64            `json:"storage_cost_monthly"`
	TransferCostMonthly  float64            `json:"transfer_cost_monthly"`
	TotalCostMonthly     float64            `json:"total_cost_monthly"`
	CostByTier           map[string]float64 `json:"cost_by_tier"`
	CostByProvider       map[string]float64 `json:"cost_by_provider"`
	SavingsEstimate      float64            `json:"savings_estimate"`
	OptimizationTips     []string           `json:"optimization_tips"`
	CalculatedAt         time.Time          `json:"calculated_at"`
}

// Config contains hybrid cloud configuration
type Config struct {
	DefaultProvider    string        `json:"default_provider"`
	EncryptionEnabled  bool          `json:"encryption_enabled"`
	EncryptionKey      string        `json:"encryption_key"`
	CacheDir           string        `json:"cache_dir"`
	MaxCacheSize       int64         `json:"max_cache_size"`
	CacheTTL           time.Duration `json:"cache_ttl"`
	ConcurrentSyncs    int           `json:"concurrent_syncs"`
	SyncInterval       time.Duration `json:"sync_interval"`
	BandwidthLimit     int64         `json:"bandwidth_limit"` // bytes per second
	RetryAttempts      int           `json:"retry_attempts"`
	RetryDelay         time.Duration `json:"retry_delay"`
	Providers          []ProviderConfig `json:"providers"`
}

// ProviderConfig contains provider-specific configuration
type ProviderConfig struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Endpoint    string            `json:"endpoint"`
	Region      string            `json:"region"`
	Bucket      string            `json:"bucket"`
	AccessKey   string            `json:"access_key"`
	SecretKey   string            `json:"secret_key"`
	Options     map[string]string `json:"options"`
}

// UploadOptions contains options for uploading to cloud
type UploadOptions struct {
	ContentType  string            `json:"content_type,omitempty"`
	Encrypted    bool              `json:"encrypted,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	StorageClass string            `json:"storage_class,omitempty"`
}

// CloudObject represents an object in cloud storage
type CloudObject struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
	StorageClass string    `json:"storage_class"`
}

// ProviderInfo contains provider status information
type ProviderInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Available    bool   `json:"available"`
	TotalStorage int64  `json:"total_storage"`
	UsedStorage  int64  `json:"used_storage"`
	Region       string `json:"region"`
}

// ManagerStats contains manager statistics
type ManagerStats struct {
	TotalPools     int       `json:"total_pools"`
	TotalFiles     int       `json:"total_files"`
	TotalSize      int64     `json:"total_size"`
	LocalSize      int64     `json:"local_size"`
	CloudSize      int64     `json:"cloud_size"`
	CacheHits      int64     `json:"cache_hits"`
	CacheMisses    int64     `json:"cache_misses"`
	LastSyncTime   time.Time `json:"last_sync_time"`
}

// Errors
var (
	ErrPoolNotFound       = errors.New("pool not found")
	ErrProviderNotFound   = errors.New("provider not found")
	ErrFileNotFound       = errors.New("file not found")
	ErrSyncInProgress     = errors.New("sync already in progress")
	ErrCacheFull          = errors.New("cache is full")
	ErrEncryptionFailed   = errors.New("encryption failed")
	ErrDecryptionFailed   = errors.New("decryption failed")
	ErrInvalidConfig      = errors.New("invalid configuration")
	ErrPolicyNotFound     = errors.New("tier policy not found")
	ErrTaskNotFound       = errors.New("sync task not found")
	ErrProviderExists     = errors.New("provider already exists")
	ErrPoolExists         = errors.New("pool already exists")
	ErrInvalidTier        = errors.New("invalid tier level")
	ErrCloudUnavailable   = errors.New("cloud provider unavailable")
)

// NewManager creates a new hybrid cloud manager
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	m := &Manager{
		pools:        make(map[string]*StoragePool),
		providers:    make(map[string]CloudProvider),
		syncTasks:    make(map[string]*SyncTask),
		cache:        make(map[string]*CacheEntry),
		tierPolicies: make(map[string]*TierPolicy),
		config:       config,
		stats:        &ManagerStats{},
	}

	if config.EncryptionEnabled && config.EncryptionKey != "" {
		key, err := hex.DecodeString(config.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("invalid encryption key: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("encryption key must be 32 bytes for AES-256")
		}
		m.encryptionKey = key
	}

	return m, nil
}

// RegisterProvider registers a cloud provider
func (m *Manager) RegisterProvider(provider CloudProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := provider.Name()
	if _, exists := m.providers[name]; exists {
		return ErrProviderExists
	}

	m.providers[name] = provider
	return nil
}

// ListCloudProviders returns all registered cloud providers
func (m *Manager) ListCloudProviders() []CloudProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]CloudProvider, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetProvider returns a specific cloud provider by name
func (m *Manager) GetProvider(name string) (CloudProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, exists := m.providers[name]
	if !exists {
		return nil, ErrProviderNotFound
	}
	return provider, nil
}

// CreatePool creates a new hybrid storage pool
func (m *Manager) CreatePool(ctx context.Context, poolConfig *StoragePool) (*StoragePool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if poolConfig.Name == "" {
		return nil, fmt.Errorf("pool name is required")
	}

	// Check provider exists
	provider, exists := m.providers[poolConfig.Provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	// Verify provider is available
	info, err := provider.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("provider unavailable: %w", err)
	}
	if !info.Available {
		return nil, ErrCloudUnavailable
	}

	// Generate ID if not provided
	if poolConfig.ID == "" {
		poolConfig.ID = generateID("pool")
	}

	// Check for duplicate
	if _, exists := m.pools[poolConfig.ID]; exists {
		return nil, ErrPoolExists
	}

	pool := &StoragePool{
		ID:         poolConfig.ID,
		Name:       poolConfig.Name,
		LocalPath:  poolConfig.LocalPath,
		CloudBucket: poolConfig.CloudBucket,
		Provider:   poolConfig.Provider,
		Status:     PoolStatusActive,
		Files:      make(map[string]*FileEntry),
		TierPolicy: poolConfig.TierPolicy,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Metadata:   poolConfig.Metadata,
	}

	if pool.Metadata == nil {
		pool.Metadata = make(map[string]string)
	}

	m.pools[pool.ID] = pool
	m.stats.TotalPools++

	return pool, nil
}

// GetPool returns a storage pool by ID
func (m *Manager) GetPool(poolID string) (*StoragePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

// ListPools returns all storage pools
func (m *Manager) ListPools() []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*StoragePool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	return pools
}

// SetTierPolicy sets a tier policy for a pool
func (m *Manager) SetTierPolicy(poolID string, policy *TierPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return ErrPoolNotFound
	}

	if policy.ID == "" {
		policy.ID = generateID("policy")
	}
	policy.CreatedAt = time.Now()

	m.tierPolicies[policy.ID] = policy
	pool.TierPolicy = policy.ID
	pool.UpdatedAt = time.Now()

	return nil
}

// GetTierPolicy returns a tier policy by ID
func (m *Manager) GetTierPolicy(policyID string) (*TierPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.tierPolicies[policyID]
	if !exists {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

// SyncToCloud syncs files from local to cloud
func (m *Manager) SyncToCloud(ctx context.Context, poolID string, filePaths []string) (*SyncTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, ErrPoolNotFound
	}

	provider, exists := m.providers[pool.Provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	// Check if sync is already in progress for this pool
	for _, task := range m.syncTasks {
		if task.PoolID == poolID && task.State == SyncStateSyncing {
			return nil, ErrSyncInProgress
		}
	}

	task := &SyncTask{
		ID:        generateID("sync"),
		PoolID:    poolID,
		Direction: "up",
		State:     SyncStateSyncing,
		StartedAt: timePtr(time.Now()),
		CreatedAt: time.Now(),
	}

	// Calculate total bytes
	for _, path := range filePaths {
		if entry, exists := pool.Files[path]; exists {
			task.TotalBytes += entry.Size
			task.TotalFiles++
		}
	}

	m.syncTasks[task.ID] = task

	// Start sync in background
	go m.executeSyncToCloud(ctx, task, pool, provider, filePaths)

	return task, nil
}

// executeSyncToCloud performs the actual sync operation
func (m *Manager) executeSyncToCloud(ctx context.Context, task *SyncTask, pool *StoragePool, provider CloudProvider, filePaths []string) {
	for _, path := range filePaths {
		m.mu.RLock()
		entry, exists := pool.Files[path]
		m.mu.RUnlock()

		if !exists {
			continue
		}

		// Prepare data for upload
		data := []byte(path) // In real implementation, read from localPath

		// Encrypt if enabled
		if m.config.EncryptionEnabled && len(m.encryptionKey) > 0 {
			encrypted, err := m.encrypt(data)
			if err != nil {
				task.State = SyncStateFailed
				task.ErrorMessage = fmt.Sprintf("encryption failed for %s: %v", path, err)
				completedAt := time.Now()
				task.CompletedAt = &completedAt
				return
			}
			data = encrypted
			entry.Encrypted = true
		}

		opts := &UploadOptions{
			ContentType: "application/octet-stream",
		}

		if err := provider.Upload(ctx, entry.CloudKey, data, opts); err != nil {
			task.State = SyncStateFailed
			task.ErrorMessage = fmt.Sprintf("upload failed for %s: %v", path, err)
			completedAt := time.Now()
			task.CompletedAt = &completedAt
			return
		}

		m.mu.Lock()
		entry.SyncState = SyncStateCompleted
		entry.LastModified = time.Now()
		task.SyncedBytes += entry.Size
		task.SyncedFiles++
		task.Progress = float64(task.SyncedBytes) / float64(task.TotalBytes)
		pool.UpdatedAt = time.Now()
		m.mu.Unlock()
	}

	completedAt := time.Now()
	task.State = SyncStateCompleted
	task.CompletedAt = &completedAt
	task.Progress = 1.0

	m.mu.Lock()
	m.stats.LastSyncTime = completedAt
	m.mu.Unlock()
}

// FetchFromCloud fetches a file from cloud to local cache
func (m *Manager) FetchFromCloud(ctx context.Context, poolID string, filePath string) (*CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, ErrPoolNotFound
	}

	entry, exists := pool.Files[filePath]
	if !exists {
		return nil, ErrFileNotFound
	}

	provider, exists := m.providers[pool.Provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	// Check cache first
	if cached, ok := m.cache[entry.CloudKey]; ok {
		if time.Now().Before(cached.ExpiresAt) {
			cached.LastAccess = time.Now()
			cached.HitCount++
			m.stats.CacheHits++
			return cached, nil
		}
		// Cache expired, remove it
		delete(m.cache, entry.CloudKey)
	}

	m.stats.CacheMisses++

	// Download from cloud
	data, err := provider.Download(ctx, entry.CloudKey)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Decrypt if needed
	if entry.Encrypted && len(m.encryptionKey) > 0 {
		decrypted, err := m.decrypt(data)
		if err != nil {
			return nil, ErrDecryptionFailed
		}
		data = decrypted
	}

	// Check cache size limit
	cacheSize := m.getCacheSize()
	if cacheSize+entry.Size > m.config.MaxCacheSize {
		m.evictCache(entry.Size)
	}

	// Create cache entry
	cacheEntry := &CacheEntry{
		Key:        entry.CloudKey,
		LocalPath:  fmt.Sprintf("%s/%s", m.config.CacheDir, entry.CloudKey),
		Size:       entry.Size,
		LastAccess: time.Now(),
		HitCount:   1,
		ExpiresAt:  time.Now().Add(m.config.CacheTTL),
	}

	m.cache[entry.CloudKey] = cacheEntry
	entry.Tier = TierHot
	entry.LastAccess = time.Now()
	entry.AccessCount++

	return cacheEntry, nil
}

// EstimateCost estimates the cost of storage and transfers
func (m *Manager) EstimateCost(poolID string) (*CostEstimate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, ErrPoolNotFound
	}

	provider, exists := m.providers[pool.Provider]
	if !exists {
		return nil, ErrProviderNotFound
	}

	estimate := &CostEstimate{
		CostByTier:     make(map[string]float64),
		CostByProvider: make(map[string]float64),
		OptimizationTips: []string{},
		CalculatedAt:    time.Now(),
	}

	// Calculate costs by tier
	var hotSize, warmSize, coldSize int64
	for _, entry := range pool.Files {
		switch entry.Tier {
		case TierHot:
			hotSize += entry.Size
		case TierWarm:
			warmSize += entry.Size
		case TierCold:
			coldSize += entry.Size
		}
	}

	// Hot tier: local storage (no cloud cost)
	estimate.CostByTier[TierHot] = 0
	// Warm tier: partial cloud storage
	estimate.CostByTier[TierWarm] = provider.EstimateStorageCost(warmSize) * 0.5
	// Cold tier: full cloud storage
	estimate.CostByTier[TierCold] = provider.EstimateStorageCost(coldSize)

	// Transfer costs (estimated monthly)
	estimatedTransfer := pool.UsedSize / 10 // Assume 10% transfer per month
	estimate.TransferCostMonthly = provider.EstimateTransferCost(estimatedTransfer)

	// Total storage cost
	estimate.StorageCostMonthly = 0
	for _, cost := range estimate.CostByTier {
		estimate.StorageCostMonthly += cost
	}

	estimate.CostByProvider[pool.Provider] = estimate.StorageCostMonthly + estimate.TransferCostMonthly
	estimate.TotalCostMonthly = estimate.StorageCostMonthly + estimate.TransferCostMonthly

	// Optimization tips
	if coldSize > hotSize*2 {
		estimate.OptimizationTips = append(estimate.OptimizationTips,
			"Consider archiving cold data to cheaper storage class")
		estimate.SavingsEstimate = estimate.CostByTier[TierCold] * 0.3
	}

	if len(estimate.OptimizationTips) == 0 {
		estimate.OptimizationTips = append(estimate.OptimizationTips,
			"Storage allocation is well optimized")
	}

	return estimate, nil
}

// EvaluateTiers evaluates and updates file tiers based on policy
func (m *Manager) EvaluateTiers(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return ErrPoolNotFound
	}

	if pool.TierPolicy == "" {
		return nil
	}

	policy, exists := m.tierPolicies[pool.TierPolicy]
	if !exists {
		return ErrPolicyNotFound
	}

	if !policy.AutoTierEnabled {
		return nil
	}

	now := time.Now()

	for _, entry := range pool.Files {
		timeSinceAccess := now.Sub(entry.LastAccess)

		var newTier string
		switch {
		case timeSinceAccess <= policy.HotThreshold:
			newTier = TierHot
		case timeSinceAccess <= policy.WarmThreshold:
			newTier = TierWarm
		default:
			newTier = TierCold
		}

		if entry.Tier != newTier {
			entry.Tier = newTier
			entry.LastModified = now
		}
	}

	pool.UpdatedAt = now
	return nil
}

// GetSyncTask returns a sync task by ID
func (m *Manager) GetSyncTask(taskID string) (*SyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.syncTasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListSyncTasks returns all sync tasks for a pool
func (m *Manager) ListSyncTasks(poolID string) []*SyncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*SyncTask, 0)
	for _, task := range m.syncTasks {
		if poolID == "" || task.PoolID == poolID {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetStats returns manager statistics
func (m *Manager) GetStats() *ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	for _, pool := range m.pools {
		stats.TotalFiles += len(pool.Files)
		stats.TotalSize += pool.UsedSize
		stats.LocalSize += pool.LocalSize
		stats.CloudSize += pool.CloudSize
	}
	return &stats
}

// PinCache pins a cache entry to prevent eviction
func (m *Manager) PinCache(key string, pinned bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.cache[key]
	if !exists {
		return fmt.Errorf("cache entry not found: %s", key)
	}

	entry.Pinned = pinned
	return nil
}

// ClearCache clears all cache entries
func (m *Manager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache = make(map[string]*CacheEntry)
}

// getCacheSize returns total cache size (must be called with lock held)
func (m *Manager) getCacheSize() int64 {
	var total int64
	for _, entry := range m.cache {
		total += entry.Size
	}
	return total
}

// evictCache removes cache entries to make space (must be called with lock held)
func (m *Manager) evictCache(neededSize int64) {
	// Sort by last access time, remove oldest non-pinned entries
	type cacheItem struct {
		key  string
		time time.Time
		size int64
	}

	items := make([]cacheItem, 0, len(m.cache))
	for key, entry := range m.cache {
		if !entry.Pinned {
			items = append(items, cacheItem{key: key, time: entry.LastAccess, size: entry.Size})
		}
	}

	// Simple bubble sort by time (oldest first)
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].time.Before(items[i].time) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	var freed int64
	for _, item := range items {
		if freed >= neededSize {
			break
		}
		delete(m.cache, item.key)
		freed += item.size
	}
}

// encrypt encrypts data using AES-256-GCM
func (m *Manager) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return nil, ErrEncryptionFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrEncryptionFailed
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, ErrEncryptionFailed
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// decrypt decrypts data using AES-256-GCM
func (m *Manager) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrDecryptionFailed
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// generateID generates a unique ID with prefix
func generateID(prefix string) string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

// timePtr returns a pointer to a time value
func timePtr(t time.Time) *time.Time {
	return &t
}
