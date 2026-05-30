package nasbenchmark

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// BenchmarkType represents the type of benchmark
type BenchmarkType string

const (
	BenchmarkSequentialRead  BenchmarkType = "sequential_read"
	BenchmarkSequentialWrite BenchmarkType = "sequential_write"
	BenchmarkRandomRead      BenchmarkType = "random_read"
	BenchmarkRandomWrite     BenchmarkType = "random_write"
	BenchmarkMixed           BenchmarkType = "mixed"
	BenchmarkNetwork         BenchmarkType = "network"
	BenchmarkCPU             BenchmarkType = "cpu"
	BenchmarkMemory          BenchmarkType = "memory"
)

// BenchmarkResult represents a benchmark result
type BenchmarkResult struct {
	ID          string        `json:"id"`
	Type        BenchmarkType `json:"type"`
	Path        string        `json:"path"`
	BlockSize   int           `json:"block_size_kb"`
	FileSize    int           `json:"file_size_mb"`
	Threads     int           `json:"threads"`
	Duration    time.Duration `json:"duration"`
	Throughput  float64       `json:"throughput_mbps"`
	IOPS        float64       `json:"iops"`
	Latency     LatencyStats  `json:"latency"`
	Timestamp   time.Time     `json:"timestamp"`
	Status      string        `json:"status"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
}

// LatencyStats represents latency statistics
type LatencyStats struct {
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	Avg    time.Duration `json:"avg"`
	P50    time.Duration `json:"p50"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
}

// BenchmarkConfig represents benchmark configuration
type BenchmarkConfig struct {
	Type      BenchmarkType `json:"type"`
	Path      string        `json:"path"`
	BlockSize int           `json:"block_size_kb"`
	FileSize  int           `json:"file_size_mb"`
	Threads   int           `json:"threads"`
	Duration  time.Duration `json:"duration"`
	Count     int           `json:"count"`
}

// BenchmarkSuite represents a benchmark suite result
type BenchmarkSuite struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Results   []BenchmarkResult `json:"results"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Summary   SuiteSummary      `json:"summary"`
}

// SuiteSummary represents a summary of benchmark suite
type SuiteSummary struct {
	TotalTests     int     `json:"total_tests"`
	PassedTests    int     `json:"passed_tests"`
	FailedTests    int     `json:"failed_tests"`
	AvgThroughput  float64 `json:"avg_throughput_mbps"`
	MaxThroughput  float64 `json:"max_throughput_mbps"`
	MinThroughput  float64 `json:"min_throughput_mbps"`
	TotalDuration  time.Duration `json:"total_duration"`
}

// Manager manages benchmarks
type Manager struct {
	mu      sync.RWMutex
	results map[string]*BenchmarkResult
	suites  map[string]*BenchmarkSuite
}

// NewManager creates a new benchmark manager
func NewManager() *Manager {
	return &Manager{
		results: make(map[string]*BenchmarkResult),
		suites:  make(map[string]*BenchmarkSuite),
	}
}

// RunBenchmark runs a single benchmark
func (m *Manager) RunBenchmark(config BenchmarkConfig) (*BenchmarkResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &BenchmarkResult{
		ID:        fmt.Sprintf("bench-%d", time.Now().UnixNano()),
		Type:      config.Type,
		Path:      config.Path,
		BlockSize: config.BlockSize,
		FileSize:  config.FileSize,
		Threads:   config.Threads,
		Timestamp: time.Now(),
		Status:    "running",
	}

	// Simulate benchmark execution
	start := time.Now()
	time.Sleep(time.Duration(100+rand.Intn(500)) * time.Millisecond)
	duration := time.Since(start)

	result.Duration = duration
	result.Status = "completed"

	// Generate simulated results based on benchmark type
	switch config.Type {
	case BenchmarkSequentialRead:
		result.Throughput = 500 + rand.Float64()*1500
		result.IOPS = result.Throughput * 1024 / float64(config.BlockSize)
	case BenchmarkSequentialWrite:
		result.Throughput = 400 + rand.Float64()*1200
		result.IOPS = result.Throughput * 1024 / float64(config.BlockSize)
	case BenchmarkRandomRead:
		result.Throughput = 100 + rand.Float64()*400
		result.IOPS = result.Throughput * 1024 / float64(config.BlockSize) * 10
	case BenchmarkRandomWrite:
		result.Throughput = 80 + rand.Float64()*300
		result.IOPS = result.Throughput * 1024 / float64(config.BlockSize) * 10
	case BenchmarkMixed:
		result.Throughput = 200 + rand.Float64()*800
		result.IOPS = result.Throughput * 1024 / float64(config.BlockSize) * 5
	case BenchmarkNetwork:
		result.Throughput = 100 + rand.Float64()*900
		result.IOPS = 0
	case BenchmarkCPU:
		result.Throughput = float64(1000 + rand.Intn(5000))
		result.IOPS = result.Throughput * 1000
	case BenchmarkMemory:
		result.Throughput = float64(5000 + rand.Intn(15000))
		result.IOPS = result.Throughput * 1000
	}

	// Generate latency stats
	result.Latency = LatencyStats{
		Min: time.Duration(rand.Intn(100)) * time.Microsecond,
		Max: time.Duration(100+rand.Intn(900)) * time.Microsecond,
		Avg: time.Duration(50+rand.Intn(200)) * time.Microsecond,
		P50: time.Duration(40+rand.Intn(160)) * time.Microsecond,
		P95: time.Duration(80+rand.Intn(400)) * time.Microsecond,
		P99: time.Duration(100+rand.Intn(800)) * time.Microsecond,
	}

	m.results[result.ID] = result

	return result, nil
}

// RunSuite runs a benchmark suite
func (m *Manager) RunSuite(name string, configs []BenchmarkConfig) (*BenchmarkSuite, error) {
	suite := &BenchmarkSuite{
		ID:        fmt.Sprintf("suite-%d", time.Now().UnixNano()),
		Name:      name,
		Results:   make([]BenchmarkResult, 0),
		StartTime: time.Now(),
	}

	for _, config := range configs {
		result, err := m.RunBenchmark(config)
		if err != nil {
			suite.Summary.FailedTests++
			continue
		}

		suite.Results = append(suite.Results, *result)
		suite.Summary.PassedTests++

		if result.Throughput > suite.Summary.MaxThroughput {
			suite.Summary.MaxThroughput = result.Throughput
		}
		if suite.Summary.MinThroughput == 0 || result.Throughput < suite.Summary.MinThroughput {
			suite.Summary.MinThroughput = result.Throughput
		}
		suite.Summary.AvgThroughput += result.Throughput
	}

	suite.EndTime = time.Now()
	suite.Summary.TotalTests = len(configs)
	suite.Summary.TotalDuration = suite.EndTime.Sub(suite.StartTime)

	if suite.Summary.PassedTests > 0 {
		suite.Summary.AvgThroughput /= float64(suite.Summary.PassedTests)
	}

	m.mu.Lock()
	m.suites[suite.ID] = suite
	m.mu.Unlock()

	return suite, nil
}

// GetResult returns a benchmark result by ID
func (m *Manager) GetResult(id string) (*BenchmarkResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.results[id]
	if !ok {
		return nil, fmt.Errorf("benchmark result not found: %s", id)
	}

	return result, nil
}

// ListResults lists all benchmark results
func (m *Manager) ListResults() []*BenchmarkResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*BenchmarkResult, 0, len(m.results))
	for _, r := range m.results {
		results = append(results, r)
	}

	return results
}

// GetSuite returns a benchmark suite by ID
func (m *Manager) GetSuite(id string) (*BenchmarkSuite, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suite, ok := m.suites[id]
	if !ok {
		return nil, fmt.Errorf("benchmark suite not found: %s", id)
	}

	return suite, nil
}

// ListSuites lists all benchmark suites
func (m *Manager) ListSuites() []*BenchmarkSuite {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suites := make([]*BenchmarkSuite, 0, len(m.suites))
	for _, s := range m.suites {
		suites = append(suites, s)
	}

	return suites
}

// CompareResults compares two benchmark results
func (m *Manager) CompareResults(id1, id2 string) (map[string]interface{}, error) {
	r1, err := m.GetResult(id1)
	if err != nil {
		return nil, err
	}

	r2, err := m.GetResult(id2)
	if err != nil {
		return nil, err
	}

	comparison := map[string]interface{}{
		"result1": map[string]interface{}{
			"id":         r1.ID,
			"type":       r1.Type,
			"throughput":  r1.Throughput,
			"iops":       r1.IOPS,
		},
		"result2": map[string]interface{}{
			"id":         r2.ID,
			"type":       r2.Type,
			"throughput":  r2.Throughput,
			"iops":       r2.IOPS,
		},
		"throughput_diff_pct": ((r2.Throughput - r1.Throughput) / r1.Throughput) * 100,
		"iops_diff_pct": func() float64 {
			if r1.IOPS == 0 {
				return 0
			}
			return ((r2.IOPS - r1.IOPS) / r1.IOPS) * 100
		}(),
	}

	return comparison, nil
}
