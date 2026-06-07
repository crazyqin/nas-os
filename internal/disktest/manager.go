// Package disktest 提供磁盘测试功能
// 顺序读写、随机 IOPS、SMART 健康、坏块扫描、延迟测试
package disktest

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// TestResult 测试结果
type TestResult struct {
	Device    string        `json:"device"`
	TestType  string        `json:"testType"` // seq_read, seq_write, rand_read, rand_write, latency
	Speed     float64       `json:"speed"`    // MB/s 或 IOPS
	IOPS      float64       `json:"iops"`     // IOPS
	Latency   time.Duration `json:"latency"`  // 平均延迟
	Duration  time.Duration `json:"duration"` // 耗时
	Status    string        `json:"status"`   // completed, failed, cancelled
	Timestamp time.Time     `json:"timestamp"`
}

// TestTask 测试任务
type TestTask struct {
	ID        string      `json:"id"`
	Device    string      `json:"device"`
	TestType  string      `json:"testType"`
	Progress  float64     `json:"progress"` // 0-100
	Status    string      `json:"status"`   // pending, running, completed, failed, cancelled
	Result    *TestResult `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	StartTime time.Time   `json:"startTime"`
	EndTime   time.Time   `json:"endTime,omitempty"`
}

// SMARTData SMART 数据
type SMARTData struct {
	Device          string  `json:"device"`
	Model           string  `json:"model"`
	Temp            float64 `json:"temp"`            // 温度 (°C)
	PowerOnHours    uint64  `json:"powerOnHours"`    // 通电时间
	PowerCycleCount uint64  `json:"powerCycleCount"` // 通电次数
	ReallocSectors  uint64  `json:"reallocSectors"`  // 重分配扇区
	PendingSectors  uint64  `json:"pendingSectors"`  // 待处理扇区
	HealthScore     float64 `json:"healthScore"`     // 健康评分 (0-100)
	Status          string  `json:"status"`          // PASSED, FAILED, WARNING
}

// DiskBenchResult 磁盘基准测试结果
type DiskBenchResult struct {
	Device        string        `json:"device"`
	SeqRead       float64       `json:"seqRead"`  // MB/s
	SeqWrite      float64       `json:"seqWrite"` // MB/s
	RandReadIOPS  float64       `json:"randReadIOPS"`
	RandWriteIOPS float64       `json:"randWriteIOPS"`
	AvgLatency    time.Duration `json:"avgLatency"`
	MaxLatency    time.Duration `json:"maxLatency"`
	Timestamp     time.Time     `json:"timestamp"`
}

// TestConfig 测试配置
type TestConfig struct {
	BlockSize  int           `json:"blockSize"`  // 块大小 (字节)
	TestSize   int64         `json:"testSize"`   // 测试数据大小 (字节)
	QueueDepth int           `json:"queueDepth"` // 队列深度
	Duration   time.Duration `json:"duration"`   // 测试持续时间
}

// ========== Manager ==========

// Manager 磁盘测试管理器
type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*TestTask
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*TestTask),
	}
}

// generateID 生成任务 ID
func (m *Manager) generateID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

// RunBench 运行基准测试
func (m *Manager) RunBench(device string, config *TestConfig) (*DiskBenchResult, error) {
	if device == "" {
		return nil, fmt.Errorf("device is required")
	}
	if config == nil {
		config = &TestConfig{
			BlockSize:  4096,
			TestSize:   1024 * 1024 * 1024,
			QueueDepth: 32,
			Duration:   10 * time.Second,
		}
	}

	// 创建任务
	task := &TestTask{
		ID:        m.generateID(),
		Device:    device,
		TestType:  "bench",
		Status:    "running",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	log.Printf("[磁盘测试] 开始基准测试: %s", device)

	// 模拟测试
	time.Sleep(100 * time.Millisecond)
	task.Progress = 100
	task.Status = "completed"
	task.EndTime = time.Now()

	result := &DiskBenchResult{
		Device:        device,
		SeqRead:       550.0,
		SeqWrite:      520.0,
		RandReadIOPS:  95000,
		RandWriteIOPS: 85000,
		AvgLatency:    50 * time.Microsecond,
		MaxLatency:    500 * time.Microsecond,
		Timestamp:     time.Now(),
	}

	task.Result = &TestResult{
		Device:    device,
		TestType:  "bench",
		Speed:     result.SeqRead,
		IOPS:      result.RandReadIOPS,
		Latency:   result.AvgLatency,
		Duration:  task.EndTime.Sub(task.StartTime),
		Status:    "completed",
		Timestamp: time.Now(),
	}

	log.Printf("[磁盘测试] 基准测试完成: %s", device)
	return result, nil
}

// RunSMART 运行 SMART 检查
func (m *Manager) RunSMART(device string) (*SMARTData, error) {
	if device == "" {
		return nil, fmt.Errorf("device is required")
	}

	log.Printf("[磁盘测试] 检查 SMART 数据: %s", device)

	// 模拟 SMART 数据读取
	data := &SMARTData{
		Device:          device,
		Model:           "Samsung 870 EVO 1TB",
		Temp:            38.0,
		PowerOnHours:    8760,
		PowerCycleCount: 500,
		ReallocSectors:  0,
		PendingSectors:  0,
		HealthScore:     98.5,
		Status:          "PASSED",
	}

	log.Printf("[磁盘测试] SMART 检查完成: %s - %s", device, data.Status)
	return data, nil
}

// RunBadBlocks 运行坏块扫描
func (m *Manager) RunBadBlocks(device string) (*TestTask, error) {
	if device == "" {
		return nil, fmt.Errorf("device is required")
	}

	task := &TestTask{
		ID:        m.generateID(),
		Device:    device,
		TestType:  "badblocks",
		Status:    "running",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	log.Printf("[磁盘测试] 开始坏块扫描: %s", device)

	// 模拟扫描
	time.Sleep(50 * time.Millisecond)
	task.Progress = 100
	task.Status = "completed"
	task.EndTime = time.Now()
	task.Result = &TestResult{
		Device:    device,
		TestType:  "badblocks",
		Status:    "completed",
		Duration:  task.EndTime.Sub(task.StartTime),
		Timestamp: time.Now(),
	}

	log.Printf("[磁盘测试] 坏块扫描完成: %s - 无坏块", device)
	return task, nil
}

// RunLatencyTest 运行延迟测试
func (m *Manager) RunLatencyTest(device string, count int) (*TestResult, error) {
	if device == "" {
		return nil, fmt.Errorf("device is required")
	}
	if count <= 0 {
		count = 1000
	}

	task := &TestTask{
		ID:        m.generateID(),
		Device:    device,
		TestType:  "latency",
		Status:    "running",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	log.Printf("[磁盘测试] 开始延迟测试: %s, 次数 %d", device, count)

	// 模拟延迟测试
	time.Sleep(50 * time.Millisecond)
	task.Progress = 100
	task.Status = "completed"
	task.EndTime = time.Now()

	result := &TestResult{
		Device:    device,
		TestType:  "latency",
		Speed:     0,
		IOPS:      float64(count) / task.EndTime.Sub(task.StartTime).Seconds(),
		Latency:   25 * time.Microsecond,
		Duration:  task.EndTime.Sub(task.StartTime),
		Status:    "completed",
		Timestamp: time.Now(),
	}
	task.Result = result

	log.Printf("[磁盘测试] 延迟测试完成: %s - 平均延迟 %v", device, result.Latency)
	return result, nil
}

// GetTask 获取任务
func (m *Manager) GetTask(taskID string) *TestTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.tasks[taskID]
}

// ListTasks 列出所有任务
func (m *Manager) ListTasks() []TestTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]TestTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, *t)
	}
	return tasks
}

// CancelTask 取消任务
func (m *Manager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != "running" && task.Status != "pending" {
		return fmt.Errorf("task %s is not cancellable (status: %s)", taskID, task.Status)
	}

	task.Status = "cancelled"
	task.EndTime = time.Now()
	log.Printf("[磁盘测试] 取消任务: %s", taskID)
	return nil
}
