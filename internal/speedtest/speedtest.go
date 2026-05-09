package speedtest

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// TestResult 测试结果
type TestResult struct {
	TestType      string        `json:"test_type"`
	DownloadSpeed float64       `json:"download_speed"`  // Mbps
	UploadSpeed   float64       `json:"upload_speed"`    // Mbps
	Latency       float64       `json:"latency"`         // ms
	DiskReadSpeed float64       `json:"disk_read_speed"` // MB/s
	DiskWriteSpeed float64      `json:"disk_write_speed"` // MB/s
	Timestamp     time.Time     `json:"timestamp"`
	Duration      time.Duration `json:"duration"`
	Device        string        `json:"device"`
	Error         string        `json:"error,omitempty"`
}

// TestConfig 测试配置
type TestConfig struct {
	ServerURL  string `json:"server_url"`
	Duration   int    `json:"duration"`     // seconds
	Parallel   int    `json:"parallel"`
	TestFileSize int  `json:"test_file_size"` // MB
	TargetDisk string `json:"target_disk"`
}

// Manager 速度测试管理器
type Manager struct {
	mu         sync.RWMutex
	config     TestConfig
	history    []TestResult
	maxHistory int
}

// NewManager 创建新的速度测试管理器
func NewManager(cfg *TestConfig) *Manager {
	m := &Manager{
		maxHistory: 50,
		history:    make([]TestResult, 0, 50),
	}
	if cfg != nil {
		m.config = *cfg
	}
	return m
}

// RunNetworkTest 执行网络测速测试（模拟数据）
func (m *Manager) RunNetworkTest() (*TestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	start := time.Now()
	duration := time.Duration(m.config.Duration) * time.Second
	if duration == 0 {
		duration = 5 * time.Second
	}

	// 模拟测速延迟
	time.Sleep(duration)

	result := &TestResult{
		TestType:      "network",
		DownloadSpeed: 100 + rand.Float64()*400,  // 100-500 Mbps
		UploadSpeed:   50 + rand.Float64()*200,   // 50-250 Mbps
		Latency:       5 + rand.Float64()*30,      // 5-35 ms
		Timestamp:     start,
		Duration:      time.Since(start),
		Device:        m.config.TargetDisk,
	}

	m.addToHistory(result)
	log.Printf("✅ 网络测速完成: 下载 %.2f Mbps, 上传 %.2f Mbps, 延迟 %.2f ms",
		result.DownloadSpeed, result.UploadSpeed, result.Latency)

	return result, nil
}

// RunDiskTest 执行磁盘测速测试（模拟数据）
func (m *Manager) RunDiskTest(targetPath string) (*TestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	start := time.Now()
	duration := time.Duration(m.config.Duration) * time.Second
	if duration == 0 {
		duration = 3 * time.Second
	}

	// 模拟测速延迟
	time.Sleep(duration)

	result := &TestResult{
		TestType:       "disk",
		DiskReadSpeed:  200 + rand.Float64()*800,  // 200-1000 MB/s
		DiskWriteSpeed: 100 + rand.Float64()*500,  // 100-600 MB/s
		Timestamp:      start,
		Duration:       time.Since(start),
		Device:         targetPath,
	}

	m.addToHistory(result)
	log.Printf("✅ 磁盘测速完成: 读取 %.2f MB/s, 写入 %.2f MB/s",
		result.DiskReadSpeed, result.DiskWriteSpeed)

	return result, nil
}

// RunFullTest 执行综合测速测试（网络+磁盘）
func (m *Manager) RunFullTest(targetPath string) (*TestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	start := time.Now()
	duration := time.Duration(m.config.Duration) * time.Second
	if duration == 0 {
		duration = 8 * time.Second
	}

	// 模拟测速延迟
	time.Sleep(duration)

	result := &TestResult{
		TestType:       "full",
		DownloadSpeed:  100 + rand.Float64()*400,
		UploadSpeed:    50 + rand.Float64()*200,
		Latency:        5 + rand.Float64()*30,
		DiskReadSpeed:  200 + rand.Float64()*800,
		DiskWriteSpeed: 100 + rand.Float64()*500,
		Timestamp:      start,
		Duration:       time.Since(start),
		Device:         targetPath,
	}

	m.addToHistory(result)
	log.Printf("✅ 综合测速完成: 网络 下载 %.2f Mbps / 上传 %.2f Mbps | 磁盘 读取 %.2f MB/s / 写入 %.2f MB/s",
		result.DownloadSpeed, result.UploadSpeed, result.DiskReadSpeed, result.DiskWriteSpeed)

	return result, nil
}

// GetHistory 获取历史测试记录
func (m *Manager) GetHistory(limit int) []TestResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// 返回最新的记录（倒序）
	result := make([]TestResult, limit)
	for i := 0; i < limit; i++ {
		result[i] = m.history[len(m.history)-1-i]
	}
	return result
}

// ClearHistory 清空历史记录
func (m *Manager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = make([]TestResult, 0, m.maxHistory)
}

// GetLatestResult 获取最新的测试结果
func (m *Manager) GetLatestResult() *TestResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.history) == 0 {
		return nil
	}

	latest := m.history[len(m.history)-1]
	return &latest
}

// addToHistory 添加结果到历史记录（需要持有写锁）
func (m *Manager) addToHistory(result *TestResult) {
	if len(m.history) >= m.maxHistory {
		// 移除最旧的记录
		m.history = m.history[1:]
	}
	m.history = append(m.history, *result)
}

func init() {
	rand.Seed(time.Now().UnixNano())
	fmt.Println("SpeedTest 模块已初始化")
}
