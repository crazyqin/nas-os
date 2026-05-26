package hybridcloud

import (
	"context"
	"testing"
	"time"
)

// MockCloudProvider implements CloudProvider for testing
type MockCloudProvider struct {
	name        string
	providerType string
	available   bool
	uploadErr   error
	downloadErr error
	deleteErr   error
	existsErr   error
	listErr     error
	storageCost float64
	transferCost float64
	objects     map[string][]byte
}

func NewMockProvider(name, providerType string) *MockCloudProvider {
	return &MockCloudProvider{
		name:         name,
		providerType: providerType,
		available:    true,
		objects:      make(map[string][]byte),
		storageCost:  0.023, // per GB per month
		transferCost: 0.09,  // per GB
	}
}

func (m *MockCloudProvider) Name() string          { return m.name }
func (m *MockCloudProvider) Type() string           { return m.providerType }

func (m *MockCloudProvider) Upload(ctx context.Context, key string, data []byte, opts *UploadOptions) error {
	if m.uploadErr != nil {
		return m.uploadErr
	}
	m.objects[key] = data
	return nil
}

func (m *MockCloudProvider) Download(ctx context.Context, key string) ([]byte, error) {
	if m.downloadErr != nil {
		return nil, m.downloadErr
	}
	data, exists := m.objects[key]
	if !exists {
		return nil, ErrFileNotFound
	}
	return data, nil
}

func (m *MockCloudProvider) Delete(ctx context.Context, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.objects, key)
	return nil
}

func (m *MockCloudProvider) Exists(ctx context.Context, key string) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, exists := m.objects[key]
	return exists, nil
}

func (m *MockCloudProvider) List(ctx context.Context, prefix string) ([]CloudObject, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	objects := make([]CloudObject, 0)
	for key := range m.objects {
		objects = append(objects, CloudObject{
			Key:  key,
			Size: int64(len(m.objects[key])),
		})
	}
	return objects, nil
}

func (m *MockCloudProvider) GetInfo(ctx context.Context) (*ProviderInfo, error) {
	return &ProviderInfo{
		Name:      m.name,
		Type:      m.providerType,
		Available: m.available,
		Region:    "us-east-1",
	}, nil
}

func (m *MockCloudProvider) EstimateStorageCost(sizeBytes int64) float64 {
	sizeGB := float64(sizeBytes) / (1024 * 1024 * 1024)
	return sizeGB * m.storageCost
}

func (m *MockCloudProvider) EstimateTransferCost(bytes int64) float64 {
	sizeGB := float64(bytes) / (1024 * 1024 * 1024)
	return sizeGB * m.transferCost
}

// Test helpers

func setupTestManager(t *testing.T) (*Manager, *MockCloudProvider) {
	t.Helper()

	config := &Config{
		DefaultProvider:   "test-s3",
		EncryptionEnabled: false,
		CacheDir:          "/tmp/test-cache",
		MaxCacheSize:      1024 * 1024 * 100, // 100MB
		CacheTTL:          1 * time.Hour,
		ConcurrentSyncs:   3,
		SyncInterval:      5 * time.Minute,
		RetryAttempts:     3,
		RetryDelay:        1 * time.Second,
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	provider := NewMockProvider("test-s3", ProviderS3)
	if err := manager.RegisterProvider(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	return manager, provider
}

func setupTestPool(t *testing.T, manager *Manager) *StoragePool {
	t.Helper()

	pool, err := manager.CreatePool(context.Background(), &StoragePool{
		Name:        "test-pool",
		LocalPath:   "/data/local",
		CloudBucket: "test-bucket",
		Provider:    "test-s3",
		Metadata:    map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Add test files
	manager.mu.Lock()
	pool.Files["/data/file1.txt"] = &FileEntry{
		Path:       "/data/file1.txt",
		Size:       1024,
		Tier:       TierHot,
		CloudKey:   "file1.txt",
		LocalPath:  "/data/local/file1.txt",
		Checksum:   "abc123",
		LastAccess: time.Now(),
		SyncState:  SyncStateCompleted,
	}
	pool.Files["/data/file2.txt"] = &FileEntry{
		Path:       "/data/file2.txt",
		Size:       2048,
		Tier:       TierCold,
		CloudKey:   "file2.txt",
		LocalPath:  "/data/local/file2.txt",
		Checksum:   "def456",
		LastAccess: time.Now().Add(-30 * 24 * time.Hour),
		SyncState:  SyncStateCompleted,
	}
	manager.mu.Unlock()

	return pool
}

// Tests

func TestNewManager(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			DefaultProvider: "s3",
		}
		manager, err := NewManager(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if manager == nil {
			t.Fatal("manager should not be nil")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		_, err := NewManager(nil)
		if err != ErrInvalidConfig {
			t.Fatalf("expected ErrInvalidConfig, got: %v", err)
		}
	})

	t.Run("with encryption", func(t *testing.T) {
		config := &Config{
			DefaultProvider:   "s3",
			EncryptionEnabled: true,
			EncryptionKey:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}
		manager, err := NewManager(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(manager.encryptionKey) != 32 {
			t.Fatal("encryption key should be 32 bytes")
		}
	})

	t.Run("invalid encryption key length", func(t *testing.T) {
		config := &Config{
			DefaultProvider:   "s3",
			EncryptionEnabled: true,
			EncryptionKey:     "short",
		}
		_, err := NewManager(config)
		if err == nil {
			t.Fatal("expected error for short encryption key")
		}
	})
}

func TestRegisterProvider(t *testing.T) {
	manager, _ := setupTestManager(t)

	t.Run("register new provider", func(t *testing.T) {
		provider := NewMockProvider("test-oss", ProviderOSS)
		err := manager.RegisterProvider(provider)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		providers := manager.ListCloudProviders()
		if len(providers) != 2 {
			t.Fatalf("expected 2 providers, got %d", len(providers))
		}
	})

	t.Run("duplicate provider", func(t *testing.T) {
		provider := NewMockProvider("test-s3", ProviderS3)
		err := manager.RegisterProvider(provider)
		if err != ErrProviderExists {
			t.Fatalf("expected ErrProviderExists, got: %v", err)
		}
	})
}

func TestListCloudProviders(t *testing.T) {
	manager, _ := setupTestManager(t)

	providers := manager.ListCloudProviders()
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Name() != "test-s3" {
		t.Fatalf("expected provider name 'test-s3', got '%s'", providers[0].Name())
	}
}

func TestGetProvider(t *testing.T) {
	manager, _ := setupTestManager(t)

	t.Run("existing provider", func(t *testing.T) {
		provider, err := manager.GetProvider("test-s3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provider.Name() != "test-s3" {
			t.Fatalf("expected 'test-s3', got '%s'", provider.Name())
		}
	})

	t.Run("non-existing provider", func(t *testing.T) {
		_, err := manager.GetProvider("non-existing")
		if err != ErrProviderNotFound {
			t.Fatalf("expected ErrProviderNotFound, got: %v", err)
		}
	})
}

func TestCreatePool(t *testing.T) {
	manager, _ := setupTestManager(t)

	t.Run("create pool", func(t *testing.T) {
		pool, err := manager.CreatePool(context.Background(), &StoragePool{
			Name:        "my-pool",
			LocalPath:   "/data",
			CloudBucket: "bucket1",
			Provider:    "test-s3",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pool.Name != "my-pool" {
			t.Fatalf("expected pool name 'my-pool', got '%s'", pool.Name)
		}
		if pool.Status != PoolStatusActive {
			t.Fatalf("expected status 'active', got '%s'", pool.Status)
		}
	})

	t.Run("duplicate pool ID", func(t *testing.T) {
		_, err := manager.CreatePool(context.Background(), &StoragePool{
			ID:          "fixed-id",
			Name:        "pool1",
			LocalPath:   "/data1",
			CloudBucket: "bucket1",
			Provider:    "test-s3",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = manager.CreatePool(context.Background(), &StoragePool{
			ID:          "fixed-id",
			Name:        "pool2",
			LocalPath:   "/data2",
			CloudBucket: "bucket2",
			Provider:    "test-s3",
		})
		if err != ErrPoolExists {
			t.Fatalf("expected ErrPoolExists, got: %v", err)
		}
	})

	t.Run("non-existing provider", func(t *testing.T) {
		_, err := manager.CreatePool(context.Background(), &StoragePool{
			Name:        "pool-fail",
			LocalPath:   "/data",
			CloudBucket: "bucket",
			Provider:    "non-existing",
		})
		if err != ErrProviderNotFound {
			t.Fatalf("expected ErrProviderNotFound, got: %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := manager.CreatePool(context.Background(), &StoragePool{
			LocalPath:   "/data",
			CloudBucket: "bucket",
			Provider:    "test-s3",
		})
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})
}

func TestGetPool(t *testing.T) {
	manager, _ := setupTestManager(t)
	pool := setupTestPool(t, manager)

	t.Run("existing pool", func(t *testing.T) {
		got, err := manager.GetPool(pool.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != pool.ID {
			t.Fatalf("expected pool ID '%s', got '%s'", pool.ID, got.ID)
		}
	})

	t.Run("non-existing pool", func(t *testing.T) {
		_, err := manager.GetPool("non-existing")
		if err != ErrPoolNotFound {
			t.Fatalf("expected ErrPoolNotFound, got: %v", err)
		}
	})
}

func TestListPools(t *testing.T) {
	manager, _ := setupTestManager(t)
	setupTestPool(t, manager)

	pools := manager.ListPools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
}

func TestSetTierPolicy(t *testing.T) {
	manager, _ := setupTestManager(t)
	pool := setupTestPool(t, manager)

	t.Run("set policy", func(t *testing.T) {
		policy := &TierPolicy{
			Name:            "default-policy",
			HotThreshold:    24 * time.Hour,
			WarmThreshold:   7 * 24 * time.Hour,
			MinLocalSize:    1024 * 1024,
			MaxLocalSize:    1024 * 1024 * 100,
			AutoTierEnabled: true,
		}

		err := manager.SetTierPolicy(pool.ID, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if policy.ID == "" {
			t.Fatal("policy ID should be auto-generated")
		}

		// Verify policy was set
		got, err := manager.GetTierPolicy(policy.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "default-policy" {
			t.Fatalf("expected policy name 'default-policy', got '%s'", got.Name)
		}
	})

	t.Run("non-existing pool", func(t *testing.T) {
		err := manager.SetTierPolicy("non-existing", &TierPolicy{Name: "test"})
		if err != ErrPoolNotFound {
			t.Fatalf("expected ErrPoolNotFound, got: %v", err)
		}
	})
}

func TestGetTierPolicy(t *testing.T) {
	manager, _ := setupTestManager(t)

	t.Run("non-existing policy", func(t *testing.T) {
		_, err := manager.GetTierPolicy("non-existing")
		if err != ErrPolicyNotFound {
			t.Fatalf("expected ErrPolicyNotFound, got: %v", err)
		}
	})
}

func TestSyncToCloud(t *testing.T) {
	manager, provider := setupTestManager(t)
	pool := setupTestPool(t, manager)

	// Pre-load data into provider for testing
	provider.objects["file1.txt"] = []byte("file1 content")
	provider.objects["file2.txt"] = []byte("file2 content")

	t.Run("sync files", func(t *testing.T) {
		task, err := manager.SyncToCloud(context.Background(), pool.ID, []string{"/data/file1.txt"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if task.State != SyncStateSyncing {
			t.Fatalf("expected state 'syncing', got '%s'", task.State)
		}
		if task.Direction != "up" {
			t.Fatalf("expected direction 'up', got '%s'", task.Direction)
		}
		if task.TotalFiles != 1 {
			t.Fatalf("expected 1 file, got %d", task.TotalFiles)
		}
		if task.TotalBytes != 1024 {
			t.Fatalf("expected 1024 bytes, got %d", task.TotalBytes)
		}

		// Wait for sync to complete
		time.Sleep(100 * time.Millisecond)

		// Check task status
		got, err := manager.GetSyncTask(task.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != SyncStateCompleted {
			t.Fatalf("expected state 'completed', got '%s'", got.State)
		}
		if got.Progress != 1.0 {
			t.Fatalf("expected progress 1.0, got %f", got.Progress)
		}
	})

	t.Run("non-existing pool", func(t *testing.T) {
		_, err := manager.SyncToCloud(context.Background(), "non-existing", []string{"/data/file1.txt"})
		if err != ErrPoolNotFound {
			t.Fatalf("expected ErrPoolNotFound, got: %v", err)
		}
	})
}

func TestFetchFromCloud(t *testing.T) {
	manager, provider := setupTestManager(t)
	pool := setupTestPool(t, manager)

	// Pre-load data into provider
	provider.objects["file2.txt"] = []byte("file2 content from cloud")

	t.Run("fetch existing file", func(t *testing.T) {
		entry, err := manager.FetchFromCloud(context.Background(), pool.ID, "/data/file2.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if entry.Key != "file2.txt" {
			t.Fatalf("expected key 'file2.txt', got '%s'", entry.Key)
		}
		if entry.HitCount != 1 {
			t.Fatalf("expected hit count 1, got %d", entry.HitCount)
		}
		if entry.Pinned {
			t.Fatal("entry should not be pinned")
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		// Fetch again - should hit cache
		entry, err := manager.FetchFromCloud(context.Background(), pool.ID, "/data/file2.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.HitCount != 2 {
			t.Fatalf("expected hit count 2, got %d", entry.HitCount)
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		_, err := manager.FetchFromCloud(context.Background(), pool.ID, "/data/non-existing.txt")
		if err != ErrFileNotFound {
			t.Fatalf("expected ErrFileNotFound, got: %v", err)
		}
	})

	t.Run("non-existing pool", func(t *testing.T) {
		_, err := manager.FetchFromCloud(context.Background(), "non-existing", "/data/file2.txt")
		if err != ErrPoolNotFound {
			t.Fatalf("expected ErrPoolNotFound, got: %v", err)
		}
	})
}

func TestEstimateCost(t *testing.T) {
	manager, _ := setupTestManager(t)
	pool := setupTestPool(t, manager)

	t.Run("estimate cost", func(t *testing.T) {
		estimate, err := manager.EstimateCost(pool.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if estimate.TotalCostMonthly < 0 {
			t.Fatal("total cost should not be negative")
		}
		if estimate.CalculatedAt.IsZero() {
			t.Fatal("calculated time should be set")
		}
		if len(estimate.OptimizationTips) == 0 {
			t.Fatal("should have at least one optimization tip")
		}
	})

	t.Run("non-existing pool", func(t *testing.T) {
		_, err := manager.EstimateCost("non-existing")
		if err != ErrPoolNotFound {
			t.Fatalf("expected ErrPoolNotFound, got: %v", err)
		}
	})
}

func TestEvaluateTiers(t *testing.T) {
	manager, _ := setupTestManager(t)
	pool := setupTestPool(t, manager)

	// Set tier policy
	policy := &TierPolicy{
		Name:            "test-policy",
		HotThreshold:    24 * time.Hour,
		WarmThreshold:   7 * 24 * time.Hour,
		AutoTierEnabled: true,
	}
	if err := manager.SetTierPolicy(pool.ID, policy); err != nil {
		t.Fatalf("failed to set policy: %v", err)
	}

	t.Run("evaluate tiers", func(t *testing.T) {
		err := manager.EvaluateTiers(pool.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// file1 was recently accessed, should be hot
		manager.mu.RLock()
		file1 := pool.Files["/data/file1.txt"]
		file2 := pool.Files["/data/file2.txt"]
		manager.mu.RUnlock()

		if file1.Tier != TierHot {
			t.Fatalf("file1 should be hot, got '%s'", file1.Tier)
		}
		if file2.Tier != TierCold {
			t.Fatalf("file2 should be cold, got '%s'", file2.Tier)
		}
	})

	t.Run("no policy", func(t *testing.T) {
		pool2, _ := manager.CreatePool(context.Background(), &StoragePool{
			Name:        "no-policy-pool",
			LocalPath:   "/data2",
			CloudBucket: "bucket2",
			Provider:    "test-s3",
		})
		err := manager.EvaluateTiers(pool2.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("auto tier disabled", func(t *testing.T) {
		pool3, _ := manager.CreatePool(context.Background(), &StoragePool{
			Name:        "disabled-pool",
			LocalPath:   "/data3",
			CloudBucket: "bucket3",
			Provider:    "test-s3",
		})
		policy2 := &TierPolicy{
			Name:            "disabled-policy",
			AutoTierEnabled: false,
		}
		manager.SetTierPolicy(pool3.ID, policy2)

		err := manager.EvaluateTiers(pool3.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGetSyncTask(t *testing.T) {
	manager, provider := setupTestManager(t)
	pool := setupTestPool(t, manager)

	provider.objects["file1.txt"] = []byte("content")

	// Start a sync
	task, _ := manager.SyncToCloud(context.Background(), pool.ID, []string{"/data/file1.txt"})

	t.Run("existing task", func(t *testing.T) {
		got, err := manager.GetSyncTask(task.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != task.ID {
			t.Fatalf("expected task ID '%s', got '%s'", task.ID, got.ID)
		}
	})

	t.Run("non-existing task", func(t *testing.T) {
		_, err := manager.GetSyncTask("non-existing")
		if err != ErrTaskNotFound {
			t.Fatalf("expected ErrTaskNotFound, got: %v", err)
		}
	})
}

func TestListSyncTasks(t *testing.T) {
	manager, provider := setupTestManager(t)
	pool := setupTestPool(t, manager)

	provider.objects["file1.txt"] = []byte("content")

	// Start a sync
	manager.SyncToCloud(context.Background(), pool.ID, []string{"/data/file1.txt"})

	t.Run("list all tasks", func(t *testing.T) {
		tasks := manager.ListSyncTasks("")
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
	})

	t.Run("list tasks by pool", func(t *testing.T) {
		tasks := manager.ListSyncTasks(pool.ID)
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
	})

	t.Run("list tasks for non-existing pool", func(t *testing.T) {
		tasks := manager.ListSyncTasks("non-existing")
		if len(tasks) != 0 {
			t.Fatalf("expected 0 tasks, got %d", len(tasks))
		}
	})
}

func TestGetStats(t *testing.T) {
	manager, _ := setupTestManager(t)
	setupTestPool(t, manager)

	stats := manager.GetStats()
	if stats.TotalPools != 1 {
		t.Fatalf("expected 1 pool, got %d", stats.TotalPools)
	}
	if stats.TotalFiles != 2 {
		t.Fatalf("expected 2 files, got %d", stats.TotalFiles)
	}
}

func TestPinCache(t *testing.T) {
	manager, provider := setupTestManager(t)
	pool := setupTestPool(t, manager)

	provider.objects["file2.txt"] = []byte("content")

	// First fetch to populate cache
	manager.FetchFromCloud(context.Background(), pool.ID, "/data/file2.txt")

	t.Run("pin cache entry", func(t *testing.T) {
		err := manager.PinCache("file2.txt", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unpin cache entry", func(t *testing.T) {
		err := manager.PinCache("file2.txt", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-existing entry", func(t *testing.T) {
		err := manager.PinCache("non-existing", true)
		if err == nil {
			t.Fatal("expected error for non-existing cache entry")
		}
	})
}

func TestClearCache(t *testing.T) {
	manager, provider := setupTestManager(t)
	pool := setupTestPool(t, manager)

	provider.objects["file2.txt"] = []byte("content")

	// Populate cache
	manager.FetchFromCloud(context.Background(), pool.ID, "/data/file2.txt")

	manager.ClearCache()

	stats := manager.GetStats()
	if stats.CacheHits != 0 {
		t.Fatalf("cache should be cleared, hits: %d", stats.CacheHits)
	}
}

func TestEncryption(t *testing.T) {
	config := &Config{
		DefaultProvider:   "test-s3",
		EncryptionEnabled: true,
		EncryptionKey:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Run("encrypt and decrypt", func(t *testing.T) {
		original := []byte("Hello, World!")

		encrypted, err := manager.encrypt(original)
		if err != nil {
			t.Fatalf("encryption failed: %v", err)
		}

		decrypted, err := manager.decrypt(encrypted)
		if err != nil {
			t.Fatalf("decryption failed: %v", err)
		}

		if string(decrypted) != string(original) {
			t.Fatalf("decrypted data doesn't match original: got '%s', want '%s'", decrypted, original)
		}
	})

	t.Run("decrypt invalid data", func(t *testing.T) {
		_, err := manager.decrypt([]byte("invalid"))
		if err != ErrDecryptionFailed {
			t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
		}
	})
}

func TestMultipleProviders(t *testing.T) {
	manager, _ := setupTestManager(t)

	providers := []struct {
		name string
		typ  string
	}{
		{"s3-provider", ProviderS3},
		{"oss-provider", ProviderOSS},
		{"cos-provider", ProviderCOS},
		{"azure-provider", ProviderAzure},
	}

	for _, p := range providers {
		provider := NewMockProvider(p.name, p.typ)
		if err := manager.RegisterProvider(provider); err != nil {
			t.Fatalf("failed to register provider %s: %v", p.name, err)
		}
	}

	allProviders := manager.ListCloudProviders()
	if len(allProviders) != 5 { // 1 from setupTestManager + 4 new
		t.Fatalf("expected 5 providers, got %d", len(allProviders))
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create multiple pools concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer func() { done <- true }()
			_, _ = manager.CreatePool(context.Background(), &StoragePool{
				Name:        "pool-" + string(rune('A'+idx)),
				LocalPath:   "/data",
				CloudBucket: "bucket",
				Provider:    "test-s3",
			})
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	pools := manager.ListPools()
	if len(pools) != 10 {
		t.Fatalf("expected 10 pools, got %d", len(pools))
	}
}

func TestProviderTypes(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		expected string
	}{
		{"S3", ProviderS3, "s3"},
		{"OSS", ProviderOSS, "oss"},
		{"COS", ProviderCOS, "cos"},
		{"Azure", ProviderAzure, "azure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider(tt.name, tt.typ)
			if provider.Type() != tt.expected {
				t.Fatalf("expected type '%s', got '%s'", tt.expected, provider.Type())
			}
		})
	}
}

func TestTierConstants(t *testing.T) {
	if TierHot != "hot" {
		t.Fatalf("TierHot should be 'hot', got '%s'", TierHot)
	}
	if TierWarm != "warm" {
		t.Fatalf("TierWarm should be 'warm', got '%s'", TierWarm)
	}
	if TierCold != "cold" {
		t.Fatalf("TierCold should be 'cold', got '%s'", TierCold)
	}
}

func TestSyncStates(t *testing.T) {
	states := []string{
		SyncStateIdle,
		SyncStateSyncing,
		SyncStateCompleted,
		SyncStateFailed,
		SyncStatePaused,
	}

	expected := []string{"idle", "syncing", "completed", "failed", "paused"}
	for i, state := range states {
		if state != expected[i] {
			t.Fatalf("state[%d] should be '%s', got '%s'", i, expected[i], state)
		}
	}
}

func TestPoolStatuses(t *testing.T) {
	statuses := []string{
		PoolStatusActive,
		PoolStatusDegraded,
		PoolStatusOffline,
	}

	expected := []string{"active", "degraded", "offline"}
	for i, status := range statuses {
		if status != expected[i] {
			t.Fatalf("status[%d] should be '%s', got '%s'", i, expected[i], status)
		}
	}
}

func TestEstimateCostWithDifferentTiers(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Create pool with files in different tiers
	pool, _ := manager.CreatePool(context.Background(), &StoragePool{
		Name:        "tier-pool",
		LocalPath:   "/data",
		CloudBucket: "bucket",
		Provider:    "test-s3",
	})

	manager.mu.Lock()
	pool.Files["/hot-file"] = &FileEntry{
		Path: "/hot-file", Size: 1024 * 1024 * 10, Tier: TierHot,
		LastAccess: time.Now(),
	}
	pool.Files["/warm-file"] = &FileEntry{
		Path: "/warm-file", Size: 1024 * 1024 * 50, Tier: TierWarm,
		LastAccess: time.Now().Add(-3 * 24 * time.Hour),
	}
	pool.Files["/cold-file"] = &FileEntry{
		Path: "/cold-file", Size: 1024 * 1024 * 100, Tier: TierCold,
		LastAccess: time.Now().Add(-30 * 24 * time.Hour),
	}
	manager.mu.Unlock()

	estimate, err := manager.EstimateCost(pool.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hot tier should have no cloud cost
	if estimate.CostByTier[TierHot] != 0 {
		t.Fatalf("hot tier cost should be 0, got %f", estimate.CostByTier[TierHot])
	}

	// Cold tier should have cloud cost
	if estimate.CostByTier[TierCold] <= 0 {
		t.Fatal("cold tier cost should be positive")
	}
}

func BenchmarkCreatePool(b *testing.B) {
	config := &Config{
		DefaultProvider: "test-s3",
	}
	manager, _ := NewManager(config)
	provider := NewMockProvider("test-s3", ProviderS3)
	manager.RegisterProvider(provider)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CreatePool(context.Background(), &StoragePool{
			Name:        "bench-pool",
			LocalPath:   "/data",
			CloudBucket: "bucket",
			Provider:    "test-s3",
		})
	}
}

func BenchmarkFetchFromCloud(b *testing.B) {
	config := &Config{
		DefaultProvider: "test-s3",
		CacheDir:        "/tmp/bench-cache",
		MaxCacheSize:    1024 * 1024 * 100,
		CacheTTL:        1 * time.Hour,
	}
	manager, _ := NewManager(config)
	provider := NewMockProvider("test-s3", ProviderS3)
	manager.RegisterProvider(provider)

	pool, _ := manager.CreatePool(context.Background(), &StoragePool{
		Name:        "bench-pool",
		LocalPath:   "/data",
		CloudBucket: "bucket",
		Provider:    "test-s3",
	})

	manager.mu.Lock()
	pool.Files["/data/file.txt"] = &FileEntry{
		Path:      "/data/file.txt",
		Size:      1024,
		CloudKey:  "file.txt",
		LastAccess: time.Now(),
	}
	manager.mu.Unlock()

	provider.objects["file.txt"] = make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.FetchFromCloud(context.Background(), pool.ID, "/data/file.txt")
	}
}
