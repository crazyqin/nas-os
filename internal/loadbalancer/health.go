// Package loadbalancer - 后端健康检查实现
package loadbalancer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	config   HealthCheckConfig
	backends []*Backend
	client   *http.Client
	stopCh   chan struct{}
	results  chan HealthCheckResult

	// 自定义探针
	customProbes map[string]HealthProbeFunc

	mu sync.RWMutex
}

// HealthProbeFunc 自定义健康探针函数
type HealthProbeFunc func(ctx context.Context, backend *Backend) error

// NewHealthChecker 创建健康检查器
func NewHealthChecker(config HealthCheckConfig) *HealthChecker {
	return &HealthChecker{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		stopCh:       make(chan struct{}),
		results:      make(chan HealthCheckResult, 100),
		customProbes: make(map[string]HealthProbeFunc),
	}
}

// SetBackends 设置要检查的后端列表
func (hc *HealthChecker) SetBackends(backends []*Backend) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.backends = backends
}

// RegisterProbe 注册自定义探针
func (hc *HealthChecker) RegisterProbe(name string, probe HealthProbeFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.customProbes[name] = probe
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	if !hc.config.Enabled {
		return
	}

	go hc.run()
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// Results 获取检查结果通道
func (hc *HealthChecker) Results() <-chan HealthCheckResult {
	return hc.results
}

// run 运行健康检查循环
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	// 立即执行一次
	hc.checkAll()

	for {
		select {
		case <-ticker.C:
			hc.checkAll()
		case <-hc.stopCh:
			return
		}
	}
}

// checkAll 检查所有后端
func (hc *HealthChecker) checkAll() {
	hc.mu.RLock()
	backends := hc.backends
	hc.mu.RUnlock()

	var wg sync.WaitGroup
	for _, backend := range backends {
		wg.Add(1)
		go func(b *Backend) {
			defer wg.Done()
			result := hc.check(b)
			hc.updateBackend(b, result)
			select {
			case hc.results <- result:
			default:
				// 通道满，丢弃旧结果
			}
		}(backend)
	}
	wg.Wait()
}

// check 执行单个后端检查
func (hc *HealthChecker) check(backend *Backend) HealthCheckResult {
	start := time.Now()

	var err error
	switch hc.config.Type {
	case HealthCheckHTTP:
		err = hc.checkHTTP(backend)
	case HealthCheckTCP:
		err = hc.checkTCP(backend)
	case HealthCheckCustom:
		err = hc.checkCustom(backend)
	default:
		err = hc.checkHTTP(backend)
	}

	latency := time.Since(start)

	result := HealthCheckResult{
		BackendID: backend.ID,
		Healthy:   err == nil,
		Latency:   latency,
		Timestamp: time.Now(),
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result
}

// checkHTTP HTTP健康检查
func (hc *HealthChecker) checkHTTP(backend *Backend) error {
	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	url := fmt.Sprintf("%s%s", backend.URL, hc.config.Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	// 添加自定义请求头
	for k, v := range hc.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != hc.config.ExpectedStatus {
		return fmt.Errorf("unexpected status code: %d, expected: %d", resp.StatusCode, hc.config.ExpectedStatus)
	}

	return nil
}

// checkTCP TCP健康检查
func (hc *HealthChecker) checkTCP(backend *Backend) error {
	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	// 从URL中提取host:port
	host := backend.URL
	if len(host) > 7 && host[:7] == "http://" {
		host = host[7:]
	} else if len(host) > 8 && host[:8] == "https://" {
		host = host[8:]
	}

	// 移除路径
	for i, c := range host {
		if c == '/' {
			host = host[:i]
			break
		}
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return fmt.Errorf("tcp connect failed: %w", err)
	}
	conn.Close()

	return nil
}

// checkCustom 自定义探针检查
func (hc *HealthChecker) checkCustom(backend *Backend) error {
	hc.mu.RLock()
	probe, exists := hc.customProbes[backend.ID]
	hc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no custom probe registered for backend %s", backend.ID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	return probe(ctx, backend)
}

// updateBackend 更新后端健康状态
func (hc *HealthChecker) updateBackend(backend *Backend, result HealthCheckResult) {
	backend.mu.Lock()
	defer backend.mu.Unlock()

	if result.Healthy {
		// 成功计数 (简化实现，使用元数据存储连续成功次数)
		if backend.Metadata == nil {
			backend.Metadata = make(map[string]string)
		}
		successes := parseCounter(backend.Metadata["health_successes"]) + 1
		backend.Metadata["health_successes"] = fmt.Sprintf("%d", successes)
		backend.Metadata["health_failures"] = "0"

		if successes >= hc.config.HealthyThreshold {
			backend.IsHealthy = true
		}
	} else {
		// 失败计数
		if backend.Metadata == nil {
			backend.Metadata = make(map[string]string)
		}
		failures := parseCounter(backend.Metadata["health_failures"]) + 1
		backend.Metadata["health_failures"] = fmt.Sprintf("%d", failures)
		backend.Metadata["health_successes"] = "0"

		if failures >= hc.config.UnhealthyThreshold {
			backend.IsHealthy = false
		}
	}

	backend.LastCheck = time.Now()
}

// parseCounter 解析计数器
func parseCounter(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

// CheckNow 立即检查指定后端
func (hc *HealthChecker) CheckNow(backend *Backend) HealthCheckResult {
	result := hc.check(backend)
	hc.updateBackend(backend, result)
	return result
}

// CheckAllNow 立即检查所有后端
func (hc *HealthChecker) CheckAllNow() []HealthCheckResult {
	hc.mu.RLock()
	backends := hc.backends
	hc.mu.RUnlock()

	results := make([]HealthCheckResult, 0, len(backends))
	for _, backend := range backends {
		result := hc.check(backend)
		hc.updateBackend(backend, result)
		results = append(results, result)
	}

	return results
}
