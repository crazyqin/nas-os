// Package dataverifier 提供数据完整性校验引擎
package dataverifier

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash/crc32"
	"sync"
	"time"
)

// Manager 数据完整性校验管理器.
type Manager struct {
	mu        sync.RWMutex
	jobs      map[string]*VerifyJob
	results   map[string]*VerifyResult
	checksums map[string]*ChecksumEntry // path -> checksum
	stats     VerifyStats
}

// NewManager 创建校验管理器.
func NewManager() *Manager {
	return &Manager{
		jobs:      make(map[string]*VerifyJob),
		results:   make(map[string]*VerifyResult),
		checksums: make(map[string]*ChecksumEntry),
	}
}

// CreateJob 创建校验任务.
func (m *Manager) CreateJob(req CreateJobRequest) (*VerifyJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 校验算法
	alg := req.Algorithm
	if alg == "" {
		alg = AlgorithmSHA256
	}
	if !isValidAlgorithm(alg) {
		return nil, fmt.Errorf("不支持的算法: %s", alg)
	}

	// 生成ID
	id := fmt.Sprintf("verify-%d", time.Now().UnixNano())

	job := &VerifyJob{
		ID:        id,
		Name:      req.Name,
		Paths:     req.Paths,
		Algorithm: alg,
		Schedule:  req.Schedule,
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.jobs[id] = job
	m.stats.TotalJobs++

	return job, nil
}

// GetJob 获取校验任务.
func (m *Manager) GetJob(id string) (*VerifyJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs 列出所有校验任务.
func (m *Manager) ListJobs() []*VerifyJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*VerifyJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// DeleteJob 删除校验任务.
func (m *Manager) DeleteJob(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	if job.Status == JobStatusRunning {
		return ErrJobRunning
	}

	delete(m.jobs, id)
	m.stats.TotalJobs--
	return nil
}

// RunJob 执行校验任务.
func (m *Manager) RunJob(id string) (*VerifyResult, error) {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return nil, ErrJobNotFound
	}
	if job.Status == JobStatusRunning {
		m.mu.Unlock()
		return nil, ErrJobRunning
	}

	job.Status = JobStatusRunning
	now := time.Now()
	job.LastRun = &now
	m.mu.Unlock()

	// 模拟校验过程
	result := &VerifyResult{
		JobID:     id,
		StartTime: now,
	}

	m.mu.Lock()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	job.Status = JobStatusCompleted
	job.UpdatedAt = time.Now()
	job.FileCount = result.TotalFiles
	job.ErrorCount = result.FailedFiles
	m.results[id] = result
	m.stats.TotalFiles += result.TotalFiles
	m.stats.TotalErrors += result.FailedFiles
	m.stats.LastVerifyTime = &result.EndTime
	m.mu.Unlock()

	return result, nil
}

// GetResult 获取校验结果.
func (m *Manager) GetResult(jobID string) (*VerifyResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.results[jobID]
	if !ok {
		return nil, ErrJobNotFound
	}
	return result, nil
}

// GetStats 获取校验统计.
func (m *Manager) GetStats() VerifyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// StoreChecksum 存储校验和.
func (m *Manager) StoreChecksum(path string, alg VerifyAlgorithm, hash string, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checksums[path] = &ChecksumEntry{
		Path:      path,
		Algorithm: alg,
		Hash:      hash,
		Size:      size,
		UpdatedAt: time.Now(),
	}
}

// VerifyChecksum 校验文件.
func (m *Manager) VerifyChecksum(path string, expectedHash string, alg VerifyAlgorithm) (bool, error) {
	m.mu.RLock()
	entry, ok := m.checksums[path]
	m.mu.RUnlock()

	if !ok {
		return false, nil
	}

	return entry.Hash == expectedHash, nil
}

// ComputeHash 计算哈希值.
func ComputeHash(data []byte, alg VerifyAlgorithm) string {
	switch alg {
	case AlgorithmCRC32:
		return fmt.Sprintf("%08x", crc32.ChecksumIEEE(data))
	case AlgorithmSHA256:
		return fmt.Sprintf("%x", sha256.Sum256(data))
	case AlgorithmSHA512:
		return fmt.Sprintf("%x", sha512.Sum512(data))
	case AlgorithmXXHASH, AlgorithmBLAKE3:
		// 降级到SHA256
		return fmt.Sprintf("%x", sha256.Sum256(data))
	default:
		return fmt.Sprintf("%x", sha256.Sum256(data))
	}
}

func isValidAlgorithm(alg VerifyAlgorithm) bool {
	switch alg {
	case AlgorithmCRC32, AlgorithmSHA256, AlgorithmSHA512, AlgorithmXXHASH, AlgorithmBLAKE3:
		return true
	default:
		return false
	}
}
