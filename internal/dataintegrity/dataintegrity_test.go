package dataintegrity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ========== Mock Store ==========

type mockStore struct {
	mu           sync.RWMutex
	checksums    map[string]*FileChecksum
	checks       []*IntegrityCheck
	history      map[string][]*IntegrityCheck
	jobs         map[int64]*IntegrityJob
	nextChecksum int64
	nextCheck    int64
}

func newMockStore() *mockStore {
	return &mockStore{
		checksums: make(map[string]*FileChecksum),
		history:   make(map[string][]*IntegrityCheck),
		jobs:      make(map[int64]*IntegrityJob),
	}
}

func (s *mockStore) checksumKey(filePath string, algo Algorithm) string {
	return filePath + "::" + string(algo)
}

func (s *mockStore) SaveChecksum(cs *FileChecksum) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cs.ID == 0 {
		s.nextChecksum++
		cs.ID = s.nextChecksum
	}
	s.checksums[s.checksumKey(cs.FilePath, cs.Algorithm)] = cs
	return nil
}

func (s *mockStore) GetChecksum(filePath string, algo Algorithm) (*FileChecksum, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cs, ok := s.checksums[s.checksumKey(filePath, algo)]
	if !ok {
		return nil, ErrChecksumNotFound
	}
	return cs, nil
}

func (s *mockStore) ListChecksums(pathPrefix string, algo Algorithm) ([]*FileChecksum, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*FileChecksum
	for _, cs := range s.checksums {
		if algo != "" && cs.Algorithm != algo {
			continue
		}
		if pathPrefix != "" && !strings.HasPrefix(cs.FilePath, pathPrefix) {
			continue
		}
		result = append(result, cs)
	}
	return result, nil
}

func (s *mockStore) DeleteChecksum(filePath string, algo Algorithm) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checksums, s.checksumKey(filePath, algo))
	return nil
}

func (s *mockStore) SaveIntegrityCheck(check *IntegrityCheck) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextCheck++
	check.ID = s.nextCheck
	s.checks = append(s.checks, check)
	s.history[check.FilePath] = append(s.history[check.FilePath], check)
	return nil
}

func (s *mockStore) GetIntegrityHistory(filePath string, limit int) ([]*IntegrityCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h := s.history[filePath]
	if limit > 0 && limit < len(h) {
		return h[len(h)-limit:], nil
	}
	return h, nil
}

func (s *mockStore) SaveJob(job *IntegrityJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *mockStore) GetJob(jobID int64) (*IntegrityJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return nil, ErrIntegrityJobNotFound
	}
	return job, nil
}

func (s *mockStore) ListJobs(limit int) ([]*IntegrityJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*IntegrityJob
	for _, j := range s.jobs {
		result = append(result, j)
	}
	return result, nil
}

// ========== 测试辅助函数 ==========

func setupManager(t *testing.T) (*Manager, *mockStore) {
	t.Helper()
	store := newMockStore()
	m := NewManager(store, zap.NewNop())
	m.Start()
	t.Cleanup(func() { m.Stop() })
	return m, store
}

func createTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	return path
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

// ========== 算法测试 ==========

func TestIsSupportedAlgorithm(t *testing.T) {
	tests := []struct {
		algo     Algorithm
		expected bool
	}{
		{AlgorithmMD5, true},
		{AlgorithmSHA256, true},
		{AlgorithmSHA512, true},
		{AlgorithmBLAKE2b, true},
		{AlgorithmCRC32, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.algo), func(t *testing.T) {
			if got := IsSupportedAlgorithm(tt.algo); got != tt.expected {
				t.Errorf("IsSupportedAlgorithm(%q) = %v, want %v", tt.algo, got, tt.expected)
			}
		})
	}
}

// ========== Manager 生命周期测试 ==========

func TestManagerStartStop(t *testing.T) {
	store := newMockStore()
	m := NewManager(store, nil) // 测试 logger nil 时的默认行为

	if m.IsRunning() {
		t.Error("新创建的 manager 不应该正在运行")
	}

	m.Start()
	if !m.IsRunning() {
		t.Error("Start 后 manager 应该正在运行")
	}

	// 重复 Start 不应 panic
	m.Start()

	m.Stop()
	if m.IsRunning() {
		t.Error("Stop 后 manager 不应该正在运行")
	}

	// 重复 Stop 不应 panic
	m.Stop()
}

func TestSetDefaultAlgorithm(t *testing.T) {
	m, _ := setupManager(t)

	if err := m.SetDefaultAlgorithm(AlgorithmMD5); err != nil {
		t.Fatalf("设置 MD5 为默认算法失败: %v", err)
	}
	if m.GetDefaultAlgorithm() != AlgorithmMD5 {
		t.Errorf("期望默认算法为 md5, 实际为 %s", m.GetDefaultAlgorithm())
	}

	if err := m.SetDefaultAlgorithm(AlgorithmSHA256); err != nil {
		t.Fatalf("设置 SHA256 为默认算法失败: %v", err)
	}

	if err := m.SetDefaultAlgorithm("invalid"); err != ErrInvalidAlgorithm {
		t.Errorf("设置无效算法应返回 ErrInvalidAlgorithm, 实际: %v", err)
	}
}

// ========== 校验和计算测试 ==========

func TestCalculateChecksum(t *testing.T) {
	dir := t.TempDir()
	content := "hello world test data"
	path := createTempFile(t, dir, "test.txt", content)

	algorithms := []Algorithm{AlgorithmMD5, AlgorithmSHA256, AlgorithmSHA512, AlgorithmBLAKE2b, AlgorithmCRC32}

	for _, algo := range algorithms {
		t.Run(string(algo), func(t *testing.T) {
			m, _ := setupManager(t)
			ctx := context.Background()

			cs, err := m.CalculateChecksum(ctx, path, algo)
			if err != nil {
				t.Fatalf("计算校验和失败: %v", err)
			}

			if cs.FilePath != path {
				t.Errorf("期望路径 %s, 实际 %s", path, cs.FilePath)
			}
			if cs.Algorithm != algo {
				t.Errorf("期望算法 %s, 实际 %s", algo, cs.Algorithm)
			}
			if cs.Checksum == "" {
				t.Error("校验和不应为空")
			}
			if cs.FileSize != int64(len(content)) {
				t.Errorf("期望文件大小 %d, 实际 %d", len(content), cs.FileSize)
			}

			// 重新计算应得到相同结果
			cs2, err := m.CalculateChecksum(ctx, path, algo)
			if err != nil {
				t.Fatalf("重新计算校验和失败: %v", err)
			}
			if cs.Checksum != cs2.Checksum {
				t.Errorf("两次计算结果不同: %s vs %s", cs.Checksum, cs2.Checksum)
			}
		})
	}
}

func TestCalculateChecksumErrors(t *testing.T) {
	m, _ := setupManager(t)
	ctx := context.Background()

	// 空路径
	_, err := m.CalculateChecksum(ctx, "", AlgorithmSHA256)
	if err != ErrPathRequired {
		t.Errorf("空路径应返回 ErrPathRequired, 实际: %v", err)
	}

	// 不存在的文件
	_, err = m.CalculateChecksum(ctx, "/nonexistent/file.txt", AlgorithmSHA256)
	if err != ErrFileNotFound {
		t.Errorf("不存在的文件应返回 ErrFileNotFound, 实际: %v", err)
	}

	// 无效算法
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "data")
	_, err = m.CalculateChecksum(ctx, path, "invalid_algo")
	if err != ErrInvalidAlgorithm {
		t.Errorf("无效算法应返回 ErrInvalidAlgorithm, 实际: %v", err)
	}

	// 目录
	_, err = m.CalculateChecksum(ctx, dir, AlgorithmSHA256)
	if err == nil {
		t.Error("对目录计算校验和应返回错误")
	}
}

func TestCalculateChecksumDefaultAlgorithm(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "data")
	ctx := context.Background()

	// 不指定算法时使用默认算法
	cs, err := m.CalculateChecksum(ctx, path, "")
	if err != nil {
		t.Fatalf("使用默认算法计算校验和失败: %v", err)
	}
	if cs.Algorithm != AlgorithmSHA256 {
		t.Errorf("期望默认算法 SHA256, 实际 %s", cs.Algorithm)
	}
}

func TestCalculateChecksumBatch(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	createTempFile(t, dir, "a.txt", "content a")
	createTempFile(t, dir, "b.txt", "content b")
	createTempFile(t, dir, "c.txt", "content c")
	ctx := context.Background()

	results, err := m.CalculateChecksumBatch(ctx, dir, AlgorithmSHA256, false)
	if err != nil {
		t.Fatalf("批量计算校验和失败: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("期望 3 个结果, 实际 %d", len(results))
	}
}

func TestCalculateChecksumBatchEmptyPath(t *testing.T) {
	m, _ := setupManager(t)
	ctx := context.Background()

	_, err := m.CalculateChecksumBatch(ctx, "", AlgorithmSHA256, false)
	if err != ErrPathRequired {
		t.Errorf("空路径应返回 ErrPathRequired, 实际: %v", err)
	}
}

// ========== 文件验证测试 ==========

func TestVerifyFileIntact(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "important data")
	ctx := context.Background()

	// 先计算校验和
	_, err := m.CalculateChecksum(ctx, path, AlgorithmSHA256)
	if err != nil {
		t.Fatalf("计算校验和失败: %v", err)
	}

	// 验证文件（未修改应完整）
	check, err := m.VerifyFile(ctx, path)
	if err != nil {
		t.Fatalf("验证文件失败: %v", err)
	}
	if check.Status != StatusIntact {
		t.Errorf("期望状态 intact, 实际 %s", check.Status)
	}
}

func TestVerifyFileCorrupted(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "original data")
	ctx := context.Background()

	// 计算校验和
	_, err := m.CalculateChecksum(ctx, path, AlgorithmSHA256)
	if err != nil {
		t.Fatalf("计算校验和失败: %v", err)
	}

	// 修改文件内容（模拟损坏）
	if err := os.WriteFile(path, []byte("corrupted data!!"), 0644); err != nil {
		t.Fatalf("修改文件失败: %v", err)
	}

	// 验证应检测到损坏
	check, err := m.VerifyFile(ctx, path)
	if err != nil {
		t.Fatalf("验证文件失败: %v", err)
	}
	if check.Status != StatusCorrupted {
		t.Errorf("期望状态 corrupted, 实际 %s", check.Status)
	}
	if check.ExpectedHash == check.ActualHash {
		t.Error("损坏文件的预期和实际校验和不应相同")
	}
}

func TestVerifyFileNoChecksum(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "data")
	ctx := context.Background()

	// 未计算过校验和的文件应返回 unknown
	check, err := m.VerifyFile(ctx, path)
	if err != ErrChecksumNotFound {
		t.Errorf("应返回 ErrChecksumNotFound, 实际: %v", err)
	}
	if check.Status != StatusUnknown {
		t.Errorf("期望状态 unknown, 实际 %s", check.Status)
	}
}

func TestVerifyFileEmptyPath(t *testing.T) {
	m, _ := setupManager(t)
	ctx := context.Background()

	_, err := m.VerifyFile(ctx, "")
	if err != ErrPathRequired {
		t.Errorf("空路径应返回 ErrPathRequired, 实际: %v", err)
	}
}

func TestVerifyDirectory(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	p1 := createTempFile(t, dir, "a.txt", "data a")
	p2 := createTempFile(t, dir, "b.txt", "data b")
	createTempFile(t, dir, "c.txt", "data c")
	ctx := context.Background()

	// 只对 a 和 b 计算校验和
	_, _ = m.CalculateChecksum(ctx, p1, AlgorithmSHA256)
	_, _ = m.CalculateChecksum(ctx, p2, AlgorithmSHA256)

	checks, err := m.VerifyDirectory(ctx, dir, false)
	if err != nil {
		t.Fatalf("验证目录失败: %v", err)
	}
	if len(checks) != 3 {
		t.Errorf("期望 3 个结果, 实际 %d", len(checks))
	}

	// 统计状态
	intact, unknown := 0, 0
	for _, ch := range checks {
		switch ch.Status {
		case StatusIntact:
			intact++
		case StatusUnknown:
			unknown++
		}
	}
	if intact != 2 {
		t.Errorf("期望 2 个 intact, 实际 %d", intact)
	}
	if unknown != 1 {
		t.Errorf("期望 1 个 unknown, 实际 %d", unknown)
	}
}

// ========== 任务测试 ==========

func TestCreateJob(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()

	job, err := m.CreateJob(CreateJobRequest{
		Name:      "测试任务",
		Path:      dir,
		Algorithm: AlgorithmSHA256,
		Recursive: false,
	})
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if job.Name != "测试任务" {
		t.Errorf("期望任务名 '测试任务', 实际 '%s'", job.Name)
	}
	if job.State != JobStatePending {
		t.Errorf("期望状态 pending, 实际 %s", job.State)
	}
	if job.Algorithm != AlgorithmSHA256 {
		t.Errorf("期望算法 sha256, 实际 %s", job.Algorithm)
	}
}

func TestCreateJobEmptyPath(t *testing.T) {
	m, _ := setupManager(t)

	_, err := m.CreateJob(CreateJobRequest{Name: "test", Path: ""})
	if err != ErrPathRequired {
		t.Errorf("空路径应返回 ErrPathRequired, 实际: %v", err)
	}
}

func TestCreateJobInvalidAlgorithm(t *testing.T) {
	m, _ := setupManager(t)

	_, err := m.CreateJob(CreateJobRequest{Name: "test", Path: "/tmp", Algorithm: "bad"})
	if err != ErrInvalidAlgorithm {
		t.Errorf("无效算法应返回 ErrInvalidAlgorithm, 实际: %v", err)
	}
}

func TestGetJobNotFound(t *testing.T) {
	m, _ := setupManager(t)

	_, err := m.GetJob(999)
	if err != ErrIntegrityJobNotFound {
		t.Errorf("不存在的任务应返回 ErrIntegrityJobNotFound, 实际: %v", err)
	}
}

func TestListJobs(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()

	_, _ = m.CreateJob(CreateJobRequest{Name: "job1", Path: dir})
	_, _ = m.CreateJob(CreateJobRequest{Name: "job2", Path: dir})

	jobs := m.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("期望 2 个任务, 实际 %d", len(jobs))
	}
}

func TestStartAndCompleteJob(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	createTempFile(t, dir, "f1.txt", "data1")
	createTempFile(t, dir, "f2.txt", "data2")

	ctx := context.Background()
	// 先计算校验和
	_, _ = m.CalculateChecksum(ctx, filepath.Join(dir, "f1.txt"), AlgorithmSHA256)
	_, _ = m.CalculateChecksum(ctx, filepath.Join(dir, "f2.txt"), AlgorithmSHA256)

	job, err := m.CreateJob(CreateJobRequest{Name: "test", Path: dir})
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	if err := m.StartJob(job.ID); err != nil {
		t.Fatalf("启动任务失败: %v", err)
	}

	// 等待完成
	deadline := time.After(5 * time.Second)
	for {
		j, _ := m.GetJob(job.ID)
		if j.State == JobStateCompleted || j.State == JobStateFailed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("任务超时未完成")
		case <-time.After(50 * time.Millisecond):
		}
	}

	j, _ := m.GetJob(job.ID)
	if j.State != JobStateCompleted {
		t.Errorf("期望任务完成, 实际状态 %s", j.State)
	}
	if j.TotalFiles != 2 {
		t.Errorf("期望总文件数 2, 实际 %d", j.TotalFiles)
	}
	if j.Intact != 2 {
		t.Errorf("期望完整文件 2, 实际 %d", j.Intact)
	}
	if j.Progress != 1.0 {
		t.Errorf("期望进度 1.0, 实际 %f", j.Progress)
	}
}

func TestStartJobNotFound(t *testing.T) {
	m, _ := setupManager(t)

	if err := m.StartJob(999); err != ErrIntegrityJobNotFound {
		t.Errorf("不存在的任务应返回 ErrIntegrityJobNotFound, 实际: %v", err)
	}
}

func TestCancelJobNotRunning(t *testing.T) {
	m, _ := setupManager(t)

	if err := m.CancelJob(999); err != ErrJobNotRunning {
		t.Errorf("未运行的任务应返回 ErrJobNotRunning, 实际: %v", err)
	}
}

func TestCreateJobDefaultName(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()

	job, err := m.CreateJob(CreateJobRequest{Path: dir})
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if job.Name == "" {
		t.Error("默认任务名不应为空")
	}
}

// ========== 修复建议测试 ==========

func TestGetRepairSuggestionsIntact(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "data")
	ctx := context.Background()

	_, _ = m.CalculateChecksum(ctx, path, AlgorithmSHA256)

	sug, err := m.GetRepairSuggestions(ctx, path)
	if err != nil {
		t.Fatalf("获取修复建议失败: %v", err)
	}
	if sug.Status != StatusIntact {
		t.Errorf("完整文件的状态应为 intact, 实际 %s", sug.Status)
	}
}

func TestGetRepairSuggestionsEmptyPath(t *testing.T) {
	m, _ := setupManager(t)
	ctx := context.Background()

	_, err := m.GetRepairSuggestions(ctx, "")
	if err != ErrPathRequired {
		t.Errorf("空路径应返回 ErrPathRequired, 实际: %v", err)
	}
}

func TestGetRepairSuggestionsWithSnapshot(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "original")
	ctx := context.Background()

	_, _ = m.CalculateChecksum(ctx, path, AlgorithmSHA256)

	// 损坏文件
	os.WriteFile(path, []byte("corrupted"), 0644)

	// 设置快照提供者
	m.SetSnapshotProvider(&mockSnapshotProvider{
		snapshots: []*SnapshotInfo{
			{ID: "snap-1", Name: "daily", CreatedAt: time.Now(), Size: 100},
		},
	})

	sug, err := m.GetRepairSuggestions(ctx, path)
	if err != nil {
		t.Fatalf("获取修复建议失败: %v", err)
	}
	if sug.Status != StatusCorrupted {
		t.Errorf("损坏文件的状态应为 corrupted, 实际 %s", sug.Status)
	}
	if sug.Strategy != StrategySnapshotRestore {
		t.Errorf("期望策略 snapshot_restore, 实际 %s", sug.Strategy)
	}
	if len(sug.Sources) != 1 {
		t.Errorf("期望 1 个修复来源, 实际 %d", len(sug.Sources))
	}
}

func TestGetRepairSuggestionsWithReplica(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "original")
	ctx := context.Background()

	_, _ = m.CalculateChecksum(ctx, path, AlgorithmSHA256)
	os.WriteFile(path, []byte("corrupted"), 0644)

	// 只设置副本提供者（无快照）
	m.SetReplicationProvider(&mockReplicationProvider{
		replicas: []*ReplicaInfo{
			{ID: "rep-1", Status: "healthy", ModTime: time.Now(), Size: 100},
		},
	})

	sug, err := m.GetRepairSuggestions(ctx, path)
	if err != nil {
		t.Fatalf("获取修复建议失败: %v", err)
	}
	if sug.Strategy != StrategyReplicaRestore {
		t.Errorf("期望策略 replica_restore, 实际 %s", sug.Strategy)
	}
}

// ========== Mock 提供者 ==========

type mockSnapshotProvider struct {
	snapshots []*SnapshotInfo
}

func (p *mockSnapshotProvider) ListSnapshots(path string) ([]*SnapshotInfo, error) {
	return p.snapshots, nil
}

func (p *mockSnapshotProvider) RestoreFromSnapshot(snapshotID string, filePath string) error {
	return nil
}

type mockReplicationProvider struct {
	replicas []*ReplicaInfo
}

func (p *mockReplicationProvider) ListReplicas(filePath string) ([]*ReplicaInfo, error) {
	return p.replicas, nil
}

func (p *mockReplicationProvider) RestoreFromReplica(replicaID string, targetPath string) error {
	return nil
}

// ========== 历史和记录测试 ==========

func TestGetFileHistory(t *testing.T) {
	m, _ := setupManager(t)
	dir := t.TempDir()
	path := createTempFile(t, dir, "test.txt", "data")
	ctx := context.Background()

	// 多次验证产生历史
	_, _ = m.CalculateChecksum(ctx, path, AlgorithmSHA256)
	_, _ = m.VerifyFile(ctx, path)
	_, _ = m.VerifyFile(ctx, path)

	history, err := m.GetFileHistory(path, 10)
	if err != nil {
		t.Fatalf("获取历史失败: %v", err)
	}
	if len(history)
