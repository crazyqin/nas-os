package diskbench

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BenchmarkManager 磁盘性能测试管理器
type BenchmarkManager struct {
	mu      sync.RWMutex
	results []*BenchResult
	running map[string]*BenchResult
	tmpDir  string
}

// BenchResult 测试结果
type BenchResult struct {
	ID          string        `json:"id"`
	TargetPath  string        `json:"target_path"`
	Status      BenchStatus   `json:"status"`
	SeqReadMBps float64       `json:"seq_read_mbps"`
	SeqWriteMBps float64      `json:"seq_write_mbps"`
	RandomReadIOPS float64    `json:"random_read_iops"`
	RandomWriteIOPS float64   `json:"random_write_iops"`
	FileSizeMB  int           `json:"file_size_mb"`
	BlockSizeKB int           `json:"block_size_kb"`
	LatencyAvg  time.Duration `json:"latency_avg"`
	LatencyP99  time.Duration `json:"latency_p99"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
}

type BenchStatus string

const (
	StatusPending   BenchStatus = "pending"
	StatusRunning   BenchStatus = "running"
	StatusCompleted BenchStatus = "completed"
	StatusFailed    BenchStatus = "failed"
)

// NewBenchmarkManager 创建管理器
func NewBenchmarkManager(tmpDir string) *BenchmarkManager {
	if tmpDir == "" {
		tmpDir = "/tmp/nas-bench"
	}
	os.MkdirAll(tmpDir, 0755)
	return &BenchmarkManager{
		results: make([]*BenchResult, 0),
		running: make(map[string]*BenchResult),
		tmpDir:  tmpDir,
	}
}

// RunBenchmark 启动性能测试
func (m *BenchmarkManager) RunBenchmark(targetPath string, fileSizeMB int) (*BenchResult, error) {
	if fileSizeMB <= 0 {
		fileSizeMB = 256
	}
	if fileSizeMB > 4096 {
		return nil, fmt.Errorf("文件大小不能超过 4096MB")
	}

	result := &BenchResult{
		ID:         fmt.Sprintf("bench-%d", time.Now().UnixNano()),
		TargetPath: targetPath,
		Status:     StatusPending,
		FileSizeMB: fileSizeMB,
		BlockSizeKB: 64,
		StartedAt:  time.Now(),
	}

	m.mu.Lock()
	m.running[result.ID] = result
	m.mu.Unlock()

	go m.runBenchmark(result)
	return result, nil
}

func (m *BenchmarkManager) runBenchmark(result *BenchResult) {
	result.Status = StatusRunning

	testFile := filepath.Join(result.TargetPath, fmt.Sprintf(".bench-%d.tmp", time.Now().UnixNano()))
	defer os.Remove(testFile)

	// 顺序写测试
	writeSpeed, err := m.benchSequentialWrite(testFile, result.FileSizeMB, result.BlockSizeKB)
	if err != nil {
		result.Status = StatusFailed
		result.ErrorMsg = fmt.Sprintf("顺序写测试失败: %v", err)
		m.finish(result)
		return
	}
	result.SeqWriteMBps = writeSpeed

	// 顺序读测试
	readSpeed, err := m.benchSequentialRead(testFile, result.BlockSizeKB)
	if err != nil {
		result.Status = StatusFailed
		result.ErrorMsg = fmt.Sprintf("顺序读测试失败: %v", err)
		m.finish(result)
		return
	}
	result.SeqReadMBps = readSpeed

	// 随机读写测试 (4K block)
	rRead, rWrite, latAvg, latP99 := m.benchRandomIO(testFile)
	result.RandomReadIOPS = rRead
	result.RandomWriteIOPS = rWrite
	result.LatencyAvg = latAvg
	result.LatencyP99 = latP99

	now := time.Now()
	result.CompletedAt = &now
	result.Status = StatusCompleted
	m.finish(result)
}

func (m *BenchmarkManager) finish(result *BenchResult) {
	m.mu.Lock()
	delete(m.running, result.ID)
	m.results = append(m.results, result)
	m.mu.Unlock()
}

func (m *BenchmarkManager) benchSequentialWrite(path string, sizeMB, blockKB int) (float64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	blockSize := blockKB * 1024
	buf := make([]byte, blockSize)
	rand.Read(buf)
	totalBytes := int64(sizeMB) * 1024 * 1024

	start := time.Now()
	written := int64(0)
	for written < totalBytes {
		toWrite := blockSize
		if int64(toWrite) > totalBytes-written {
			toWrite = int(totalBytes - written)
		}
		n, err := f.Write(buf[:toWrite])
		if err != nil {
			return 0, err
		}
		written += int64(n)
	}
	f.Sync()
	elapsed := time.Since(start).Seconds()

	return float64(written) / 1024 / 1024 / elapsed, nil
}

func (m *BenchmarkManager) benchSequentialRead(path string, blockKB int) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	blockSize := blockKB * 1024
	buf := make([]byte, blockSize)
	start := time.Now()
	totalRead := int64(0)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			totalRead += int64(n)
		}
		if err != nil {
			break
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		return 0, nil
	}
	return float64(totalRead) / 1024 / 1024 / elapsed, nil
}

func (m *BenchmarkManager) benchRandomIO(path string) (readIOPS, writeIOPS float64, latAvg, latP99 time.Duration) {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return 0, 0, 0, 0
	}
	defer f.Close()

	info, _ := f.Stat()
	fileSize := info.Size()
	if fileSize == 0 {
		return 0, 0, 0, 0
	}

	blockSize := 4096
	buf := make([]byte, blockSize)
	rand.Read(buf)
	ops := 1000
	latencies := make([]time.Duration, 0, ops*2)

	// 随机写
	writeStart := time.Now()
	for i := 0; i < ops; i++ {
		offset := rand.Int63n(fileSize-int64(blockSize)) &^ (int64(blockSize) - 1)
		t := time.Now()
		f.WriteAt(buf, offset)
		latencies = append(latencies, time.Since(t))
	}
	f.Sync()
	writeElapsed := time.Since(writeStart).Seconds()
	if writeElapsed > 0 {
		writeIOPS = float64(ops) / writeElapsed
	}

	// 随机读
	readStart := time.Now()
	for i := 0; i < ops; i++ {
		offset := rand.Int63n(fileSize-int64(blockSize)) &^ (int64(blockSize) - 1)
		t := time.Now()
		f.ReadAt(buf, offset)
		latencies = append(latencies, time.Since(t))
	}
	readElapsed := time.Since(readStart).Seconds()
	if readElapsed > 0 {
		readIOPS = float64(ops) / readElapsed
	}

	// 延迟统计
	if len(latencies) > 0 {
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		latAvg = total / time.Duration(len(latencies))

		// P99
		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		p99Idx := int(float64(len(sorted)) * 0.99)
		if p99Idx >= len(sorted) {
			p99Idx = len(sorted) - 1
		}
		latP99 = sorted[p99Idx]
	}

	return
}

// GetResult 获取测试结果
func (m *BenchmarkManager) GetResult(id string) (*BenchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if r, ok := m.running[id]; ok {
		return r, nil
	}
	for _, r := range m.results {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("测试结果 %s 不存在", id)
}

// ListResults 列出所有结果
func (m *BenchmarkManager) ListResults() []*BenchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]*BenchResult, 0, len(m.results)+len(m.running))
	for _, r := range m.running {
		all = append(all, r)
	}
	all = append(all, m.results...)
	return all
}
