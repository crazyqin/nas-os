// Package speedtest 提供网络测速管理核心业务逻辑
package speedtest

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 网络测速管理器.
type Manager struct {
	servers map[string]*TestServer
	results map[string]*TestResult
	history []*TestResult
	mu      sync.RWMutex
}

// NewManager 创建网络测速管理器.
func NewManager() *Manager {
	m := &Manager{
		servers: make(map[string]*TestServer),
		results: make(map[string]*TestResult),
		history: make([]*TestResult, 0),
	}

	// 添加默认服务器
	defaultServers := []TestServer{
		{
			ID:       uuid.New().String(),
			Name:     "北京电信",
			URL:      "https://speedtest.bjtelecom.example.com",
			Location: "北京",
			Distance: 0,
		},
		{
			ID:       uuid.New().String(),
			Name:     "上海联通",
			URL:      "https://speedtest.shunicom.example.com",
			Location: "上海",
			Distance: 1000,
		},
		{
			ID:       uuid.New().String(),
			Name:     "广州移动",
			URL:      "https://speedtest.gzmobile.example.com",
			Location: "广州",
			Distance: 1800,
		},
		{
			ID:       uuid.New().String(),
			Name:     "东京 AWS",
			URL:      "https://speedtest-tokyo.aws.example.com",
			Location: "东京",
			Distance: 2100,
		},
		{
			ID:       uuid.New().String(),
			Name:     "新加坡 DigitalOcean",
			URL:      "https://speedtest-sgp.do.example.com",
			Location: "新加坡",
			Distance: 3500,
		},
	}

	for i := range defaultServers {
		m.servers[defaultServers[i].ID] = &defaultServers[i]
	}

	return m
}

// ========== 测试管理 ==========

// RunTest 运行完整测速（下载 + 上传 + 延迟）.
func (m *Manager) RunTest(serverID string) (*TestResult, error) {
	server, err := m.GetBestServer()
	if err != nil {
		return nil, err
	}

	if serverID != "" {
		m.mu.RLock()
		s, ok := m.servers[serverID]
		m.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("server %q not found", serverID)
		}
		server = s
	}

	// 模拟测速过程
	download := m.simulateDownload(server)
	upload := m.simulateUpload(server)
	latency, jitter := m.simulateLatency(server)
	packetLoss := m.simulatePacketLoss(server)

	result := &TestResult{
		ID:            uuid.New().String(),
		ServerName:    server.Name,
		ServerURL:     server.URL,
		DownloadSpeed: download,
		UploadSpeed:   upload,
		Latency:       latency,
		Jitter:        jitter,
		PacketLoss:    packetLoss,
		Timestamp:     time.Now(),
	}

	m.mu.Lock()
	m.results[result.ID] = result
	m.history = append(m.history, result)
	m.mu.Unlock()

	return result, nil
}

// RunDownloadTest 仅测试下载速度.
func (m *Manager) RunDownloadTest(serverID string) (*TestResult, error) {
	server, err := m.GetBestServer()
	if err != nil {
		return nil, err
	}

	if serverID != "" {
		m.mu.RLock()
		s, ok := m.servers[serverID]
		m.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("server %q not found", serverID)
		}
		server = s
	}

	download := m.simulateDownload(server)

	result := &TestResult{
		ID:            uuid.New().String(),
		ServerName:    server.Name,
		ServerURL:     server.URL,
		DownloadSpeed: download,
		UploadSpeed:   0,
		Timestamp:     time.Now(),
	}

	m.mu.Lock()
	m.results[result.ID] = result
	m.history = append(m.history, result)
	m.mu.Unlock()

	return result, nil
}

// RunUploadTest 仅测试上传速度.
func (m *Manager) RunUploadTest(serverID string) (*TestResult, error) {
	server, err := m.GetBestServer()
	if err != nil {
		return nil, err
	}

	if serverID != "" {
		m.mu.RLock()
		s, ok := m.servers[serverID]
		m.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("server %q not found", serverID)
		}
		server = s
	}

	upload := m.simulateUpload(server)

	result := &TestResult{
		ID:            uuid.New().String(),
		ServerName:    server.Name,
		ServerURL:     server.URL,
		DownloadSpeed: 0,
		UploadSpeed:   upload,
		Timestamp:     time.Now(),
	}

	m.mu.Lock()
	m.results[result.ID] = result
	m.history = append(m.history, result)
	m.mu.Unlock()

	return result, nil
}

// RunLatencyTest 仅测试延迟.
func (m *Manager) RunLatencyTest(serverID string) (*TestResult, error) {
	server, err := m.GetBestServer()
	if err != nil {
		return nil, err
	}

	if serverID != "" {
		m.mu.RLock()
		s, ok := m.servers[serverID]
		m.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("server %q not found", serverID)
		}
		server = s
	}

	latency, jitter := m.simulateLatency(server)
	packetLoss := m.simulatePacketLoss(server)

	result := &TestResult{
		ID:         uuid.New().String(),
		ServerName: server.Name,
		ServerURL:  server.URL,
		Latency:    latency,
		Jitter:     jitter,
		PacketLoss: packetLoss,
		Timestamp:  time.Now(),
	}

	m.mu.Lock()
	m.results[result.ID] = result
	m.history = append(m.history, result)
	m.mu.Unlock()

	return result, nil
}

// ========== 服务器管理 ==========

// ListServers 列出所有服务器.
func (m *Manager) ListServers() []*TestServer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]*TestServer, 0, len(m.servers))
	for _, s := range m.servers {
		cp := *s
		servers = append(servers, &cp)
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Distance < servers[j].Distance
	})
	return servers
}

// AddServer 添加自定义服务器.
func (m *Manager) AddServer(req AddServerRequest) *TestServer {
	m.mu.Lock()
	defer m.mu.Unlock()

	server := &TestServer{
		ID:       uuid.New().String(),
		Name:     req.Name,
		URL:      req.URL,
		Location: req.Location,
		Distance: req.Distance,
	}

	m.servers[server.ID] = server
	return server
}

// RemoveServer 移除服务器.
func (m *Manager) RemoveServer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.servers[id]; !ok {
		return fmt.Errorf("server %q not found", id)
	}
	delete(m.servers, id)
	return nil
}

// GetBestServer 获取最佳服务器（距离最近）.
func (m *Manager) GetBestServer() (*TestServer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.servers) == 0 {
		return nil, fmt.Errorf("no servers available")
	}

	var best *TestServer
	for _, s := range m.servers {
		if best == nil || s.Distance < best.Distance {
			best = s
		}
	}

	cp := *best
	return &cp, nil
}

// ========== 历史管理 ==========

// GetHistory 获取测速历史.
func (m *Manager) GetHistory(limit int) []*TestResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*TestResult, limit)
	for i, r := range m.history[start:] {
		cp := *r
		result[i] = &cp
	}
	return result
}

// GetStats 获取统计数据.
func (m *Manager) GetStats() *SpeedStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.history) == 0 {
		return &SpeedStats{
			TestCount: 0,
		}
	}

	var totalDownload, totalUpload, totalLatency float64
	downloadCount, uploadCount, latencyCount := 0, 0, 0

	for _, r := range m.history {
		if r.DownloadSpeed > 0 {
			totalDownload += r.DownloadSpeed
			downloadCount++
		}
		if r.UploadSpeed > 0 {
			totalUpload += r.UploadSpeed
			uploadCount++
		}
		if r.Latency > 0 {
			totalLatency += r.Latency
			latencyCount++
		}
	}

	stats := &SpeedStats{
		TestCount:    len(m.history),
		LastTestTime: m.history[len(m.history)-1].Timestamp,
	}

	if downloadCount > 0 {
		stats.AvgDownload = math.Round(totalDownload/float64(downloadCount)*100) / 100
	}
	if uploadCount > 0 {
		stats.AvgUpload = math.Round(totalUpload/float64(uploadCount)*100) / 100
	}
	if latencyCount > 0 {
		stats.AvgLatency = math.Round(totalLatency/float64(latencyCount)*100) / 100
	}

	return stats
}

// ClearHistory 清除历史记录.
func (m *Manager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.results = make(map[string]*TestResult)
	m.history = make([]*TestResult, 0)
}

// ========== 模拟方法 ==========

func (m *Manager) simulateDownload(server *TestServer) float64 {
	baseSpeed := 500.0 // Mbps
	distanceFactor := 1.0 - (server.Distance / 10000.0)
	if distanceFactor < 0.3 {
		distanceFactor = 0.3
	}
	noise := (rand.Float64() - 0.5) * 100
	speed := baseSpeed*distanceFactor + noise
	if speed < 10 {
		speed = 10 + rand.Float64()*20
	}
	return math.Round(speed*100) / 100
}

func (m *Manager) simulateUpload(server *TestServer) float64 {
	baseSpeed := 200.0 // Mbps
	distanceFactor := 1.0 - (server.Distance / 10000.0)
	if distanceFactor < 0.3 {
		distanceFactor = 0.3
	}
	noise := (rand.Float64() - 0.5) * 50
	speed := baseSpeed*distanceFactor + noise
	if speed < 5 {
		speed = 5 + rand.Float64()*10
	}
	return math.Round(speed*100) / 100
}

func (m *Manager) simulateLatency(server *TestServer) (latency, jitter float64) {
	baseLatency := 1.0 + server.Distance/100.0
	noise := (rand.Float64() - 0.5) * 5
	latency = baseLatency + noise
	if latency < 1 {
		latency = 1
	}
	latency = math.Round(latency*100) / 100

	jitter = math.Round((rand.Float64()*3+0.5)*100) / 100
	return
}

func (m *Manager) simulatePacketLoss(server *TestServer) float64 {
	baseLoss := 0.01 + server.Distance/100000.0
	noise := rand.Float64() * 0.5
	loss := baseLoss + noise
	if loss > 5 {
		loss = 5
	}
	return math.Round(loss*100) / 100
}
