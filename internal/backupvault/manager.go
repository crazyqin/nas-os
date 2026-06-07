// Package backupvault 提供备份保险库核心管理逻辑
package backupvault

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 备份保险库管理器
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *BackupVaultConfig
	vaults   map[string]*Vault
	jobs     map[string]*BackupJob
	stats    map[string]*DedupStats
	tests    map[string]*RestoreTest
	policies map[string]*SLAPolicy
}

// NewManager 创建备份保险库管理器
func NewManager(logger *zap.Logger, config *BackupVaultConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultBackupVaultConfig()
	}

	m := &Manager{
		logger:   logger,
		config:   config,
		vaults:   make(map[string]*Vault),
		jobs:     make(map[string]*BackupJob),
		stats:    make(map[string]*DedupStats),
		tests:    make(map[string]*RestoreTest),
		policies: make(map[string]*SLAPolicy),
	}

	// 初始化默认保险库
	m.initDefaultVaults()
	// 初始化默认 SLA 策略
	m.initDefaultPolicies()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDefaultVaults 初始化默认保险库
func (m *Manager) initDefaultVaults() {
	defaultVaults := []*Vault{
		{
			ID:            "vault-local",
			Name:          "本地保险库",
			Description:   "本地存储主保险库",
			Status:        VaultStatusActive,
			Location:      "本地机房",
			CapacityBytes: 10 * 1024 * 1024 * 1024 * 1024, // 10TB
			UsedBytes:     2 * 1024 * 1024 * 1024 * 1024,  // 2TB
			Encryption:    EncryptionAES256,
			RetentionDays: 30,
			IsPrimary:     true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            "vault-offsite",
			Name:          "异地保险库",
			Description:   "异地容灾备份保险库",
			Status:        VaultStatusActive,
			Location:      "异地数据中心",
			RemoteURL:     "https://backup.example.com",
			CapacityBytes: 5 * 1024 * 1024 * 1024 * 1024, // 5TB
			UsedBytes:     500 * 1024 * 1024 * 1024,      // 500GB
			Encryption:    EncryptionAES256,
			RetentionDays: 90,
			IsPrimary:     false,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            "vault-cloud",
			Name:          "云保险库",
			Description:   "云存储备份保险库",
			Status:        VaultStatusActive,
			Location:      "云端",
			RemoteURL:     "s3://backup-bucket",
			CapacityBytes: 20 * 1024 * 1024 * 1024 * 1024, // 20TB
			UsedBytes:     1 * 1024 * 1024 * 1024 * 1024,  // 1TB
			Encryption:    EncryptionAES256,
			RetentionDays: 365,
			IsPrimary:     false,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	for _, v := range defaultVaults {
		m.vaults[v.ID] = v
	}
}

// initDefaultPolicies 初始化默认 SLA 策略
func (m *Manager) initDefaultPolicies() {
	defaultPolicies := []*SLAPolicy{
		{
			ID:                 "sla-bronze",
			Name:               "铜级 SLA",
			Description:        "基础备份保护",
			Level:              SLALevelBronze,
			RTOTarget:          480,  // 8 小时
			RPOTarget:          1440, // 24 小时
			BackupFrequency:    "daily",
			RetentionDays:      7,
			MinCopies:          1,
			GeoRedundancy:      false,
			EncryptionRequired: false,
			TestFrequency:      "yearly",
			IsActive:           true,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
		{
			ID:                 "sla-silver",
			Name:               "银级 SLA",
			Description:        "标准备份保护",
			Level:              SLALevelSilver,
			RTOTarget:          240, // 4 小时
			RPOTarget:          60,  // 1 小时
			BackupFrequency:    "daily",
			RetentionDays:      30,
			MinCopies:          2,
			GeoRedundancy:      false,
			EncryptionRequired: true,
			TestFrequency:      "quarterly",
			IsActive:           true,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
		{
			ID:                 "sla-gold",
			Name:               "金级 SLA",
			Description:        "高级备份保护",
			Level:              SLALevelGold,
			RTOTarget:          60, // 1 小时
			RPOTarget:          15, // 15 分钟
			BackupFrequency:    "hourly",
			RetentionDays:      90,
			MinCopies:          3,
			GeoRedundancy:      true,
			EncryptionRequired: true,
			TestFrequency:      "monthly",
			IsActive:           true,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
		{
			ID:                 "sla-platinum",
			Name:               "铂级 SLA",
			Description:        "顶级备份保护",
			Level:              SLALevelPlatinum,
			RTOTarget:          15, // 15 分钟
			RPOTarget:          5,  // 5 分钟
			BackupFrequency:    "hourly",
			RetentionDays:      365,
			MinCopies:          4,
			GeoRedundancy:      true,
			EncryptionRequired: true,
			TestFrequency:      "monthly",
			IsActive:           true,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
	}

	for _, p := range defaultPolicies {
		m.policies[p.ID] = p
	}
}

// ListVaults 列出所有保险库
func (m *Manager) ListVaults() []*Vault {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vaults := make([]*Vault, 0, len(m.vaults))
	for _, v := range m.vaults {
		vaults = append(vaults, v)
	}
	return vaults
}

// GetVault 获取保险库
func (m *Manager) GetVault(id string) (*Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vault, ok := m.vaults[id]
	if !ok {
		return nil, fmt.Errorf("vault not found: %s", id)
	}
	return vault, nil
}

// CreateJob 创建备份任务
func (m *Manager) CreateJob(job *BackupJob) (*BackupJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("backup vault is disabled")
	}

	// 验证保险库存在
	if _, ok := m.vaults[job.VaultID]; !ok {
		return nil, fmt.Errorf("vault not found: %s", job.VaultID)
	}

	if job.ID == "" {
		job.ID = generateID()
	}

	job.Status = JobStatusPending
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()

	m.jobs[job.ID] = job
	m.logger.Info("backup job created",
		zap.String("id", job.ID),
		zap.String("name", job.Name),
		zap.String("vault_id", job.VaultID))

	return job, nil
}

// GetJob 获取备份任务
func (m *Manager) GetJob(id string) (*BackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	return job, nil
}

// ListJobs 列出所有备份任务
func (m *Manager) ListJobs() []*BackupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// GetDedupStats 获取去重统计
func (m *Manager) GetDedupStats(vaultID string) (*DedupStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 验证保险库存在
	if _, ok := m.vaults[vaultID]; !ok {
		return nil, fmt.Errorf("vault not found: %s", vaultID)
	}

	// 检查缓存
	if stat, ok := m.stats[vaultID]; ok {
		return stat, nil
	}

	// 生成模拟数据
	stat := m.generateDedupStats(vaultID)
	m.stats[vaultID] = stat

	return stat, nil
}

// RunRestoreTest 运行恢复演练
func (m *Manager) RunRestoreTest(test *RestoreTest) (*RestoreTest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("backup vault is disabled")
	}

	// 验证保险库存在
	if _, ok := m.vaults[test.VaultID]; !ok {
		return nil, fmt.Errorf("vault not found: %s", test.VaultID)
	}

	if test.ID == "" {
		test.ID = generateID()
	}

	test.Status = TestStatusRunning
	test.StartedAt = &time.Time{}
	*test.StartedAt = time.Now()
	test.CreatedAt = time.Now()

	// 模拟恢复演练
	test = m.simulateRestoreTest(test)

	m.tests[test.ID] = test
	m.logger.Info("restore test started",
		zap.String("id", test.ID),
		zap.String("vault_id", test.VaultID))

	return test, nil
}

// SetSLA 设置 SLA 策略
func (m *Manager) SetSLA(policy *SLAPolicy) (*SLAPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("backup vault is disabled")
	}

	if policy.ID == "" {
		policy.ID = generateID()
	}

	policy.IsActive = true
	policy.UpdatedAt = time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}

	m.policies[policy.ID] = policy
	m.logger.Info("SLA policy set",
		zap.String("id", policy.ID),
		zap.String("name", policy.Name),
		zap.String("level", string(policy.Level)))

	return policy, nil
}

// GetSLA 获取 SLA 策略
func (m *Manager) GetSLA(id string) (*SLAPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("SLA policy not found: %s", id)
	}
	return policy, nil
}

// ListSLAs 列出所有 SLA 策略
func (m *Manager) ListSLAs() []*SLAPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*SLAPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// generateDedupStats 生成去重统计
func (m *Manager) generateDedupStats(vaultID string) *DedupStats {
	totalChunks := int64(100000)
	uniqueChunks := int64(75000)
	duplicateChunks := totalChunks - uniqueChunks
	totalBytes := int64(500 * 1024 * 1024 * 1024)        // 500GB
	deduplicatedBytes := int64(375 * 1024 * 1024 * 1024) // 375GB
	savedBytes := totalBytes - deduplicatedBytes

	return &DedupStats{
		ID:                generateID(),
		VaultID:           vaultID,
		TotalChunks:       totalChunks,
		UniqueChunks:      uniqueChunks,
		DuplicateChunks:   duplicateChunks,
		TotalBytes:        totalBytes,
		DeduplicatedBytes: deduplicatedBytes,
		SavedBytes:        savedBytes,
		DedupRatio:        float64(totalChunks) / float64(uniqueChunks),
		SpaceSavings:      float64(savedBytes) / float64(totalBytes) * 100,
		CompressionRatio:  1.33,
		UpdatedAt:         time.Now(),
	}
}

// simulateRestoreTest 模拟恢复演练
func (m *Manager) simulateRestoreTest(test *RestoreTest) *RestoreTest {
	// 模拟恢复过程
	test.TotalBytes = 100 * 1024 * 1024 * 1024 // 100GB
	test.RestoredBytes = test.TotalBytes
	test.RTOActual = 45 // 45 分钟
	test.RTOTarget = 60 // 目标 60 分钟
	test.RPOActual = 10 // 10 分钟
	test.RPOTarget = 15 // 目标 15 分钟
	test.IsSuccessful = true
	test.VerifiedFiles = 5000
	test.CorruptFiles = 0

	completedAt := time.Now()
	test.CompletedAt = &completedAt
	test.Status = TestStatusSuccess

	return test
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *BackupVaultConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *BackupVaultConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetTest 获取恢复演练
func (m *Manager) GetTest(id string) (*RestoreTest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	test, ok := m.tests[id]
	if !ok {
		return nil, fmt.Errorf("restore test not found: %s", id)
	}
	return test, nil
}

// ListTests 列出所有恢复演练
func (m *Manager) ListTests() []*RestoreTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tests := make([]*RestoreTest, 0, len(m.tests))
	for _, t := range m.tests {
		tests = append(tests, t)
	}
	return tests
}
