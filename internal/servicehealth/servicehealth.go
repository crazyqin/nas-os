// Package servicehealth - 服务健康度评分系统
// 提供服务发现、健康检查、健康评分（0-100）、SLA 管理等功能
package servicehealth

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ============================================================
// 服务状态枚举
// ============================================================

// ServiceStatus 服务状态
type ServiceStatus string

const (
	StatusHealthy  ServiceStatus = "healthy"  // 正常
	StatusWarning  ServiceStatus = "warning"  // 警告
	StatusCritical ServiceStatus = "critical" // 故障
	StatusUnknown  ServiceStatus = "unknown"  // 未知
)

// ============================================================
// 健康检查类型
// ============================================================

// CheckType 健康检查类型
type CheckType string

const (
	CheckHTTP    CheckType = "http"    // HTTP 健康检查
	CheckTCP     CheckType = "tcp"     // TCP 端口检查
	CheckProcess CheckType = "process" // 进程存活检查
	CheckScript  CheckType = "script"  // 自定义脚本检查
)

// ============================================================
// 配置类型
// ============================================================

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name         string            `json:"name"`         // 服务名称
	DisplayName  string            `json:"display_name"` // 显示名称
	CheckType    CheckType         `json:"check_type"`   // 检查类型
	CheckURL     string            `json:"check_url"`    // HTTP 检查 URL
	CheckPort    int               `json:"check_port"`   // TCP 检查端口
	CheckHost    string            `json:"check_host"`   // 检查主机
	ProcessName  string            `json:"process_name"` // 进程名称
	ScriptPath   string            `json:"script_path"`  // 自定义脚本路径
	ScriptArgs   []string          `json:"script_args"`  // 脚本参数
	Interval     time.Duration     `json:"interval"`     // 检查间隔
	Timeout      time.Duration     `json:"timeout"`      // 检查超时
	Enabled      bool              `json:"enabled"`      // 是否启用
	Tags         map[string]string `json:"tags"`         // 标签
	Dependencies []string          `json:"dependencies"` // 依赖服务列表
}

// HealthCheckConfig 健康检查全局配置
type HealthCheckConfig struct {
	DefaultInterval time.Duration `json:"default_interval"` // 默认检查间隔，默认 30s
	DefaultTimeout  time.Duration `json:"default_timeout"`  // 默认超时，5s
	MaxHistoryLen   int           `json:"max_history_len"`  // 最大历史记录数，默认 1000
	WorkerCount     int           `json:"worker_count"`     // 并发检查工作线程数，默认 5
}

// DefaultHealthCheckConfig 默认健康检查配置
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		DefaultInterval: 30 * time.Second,
		DefaultTimeout:  5 * time.Second,
		MaxHistoryLen:   1000,
		WorkerCount:     5,
	}
}

// ============================================================
// 健康检查结果
// ============================================================

// HealthCheckResult 单次健康检查结果
type HealthCheckResult struct {
	ServiceName  string        `json:"service_name"`  // 服务名称
	CheckType    CheckType     `json:"check_type"`    // 检查类型
	Status       ServiceStatus `json:"status"`        // 检查状态
	StatusCode   int           `json:"status_code"`   // HTTP 状态码
	ResponseTime time.Duration `json:"response_time"` // 响应时间
	Message      string        `json:"message"`       // 附加消息
	Timestamp    time.Time     `json:"timestamp"`     // 检查时间
}

// ============================================================
// 服务健康状态
// ============================================================

// ServiceHealth 服务健康状态
type ServiceHealth struct {
	Config       ServiceConfig       `json:"config"`        // 服务配置
	Status       ServiceStatus       `json:"status"`        // 当前状态
	Score        float64             `json:"score"`         // 健康评分 0-100
	Uptime       float64             `json:"uptime"`        // 可用性百分比
	LastCheck    *HealthCheckResult  `json:"last_check"`    // 最后一次检查
	Checks       []HealthCheckResult `json:"checks"`        // 历史检查记录
	TotalChecks  int                 `json:"total_checks"`  // 总检查次数
	FailedChecks int                 `json:"failed_checks"` // 失败次数
	CreatedAt    time.Time           `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time           `json:"updated_at"`    // 更新时间
}

// ============================================================
// 服务健康度管理器
// ============================================================

// ServiceHealthManager 服务健康度管理器
type ServiceHealthManager struct {
	mu sync.RWMutex

	// 配置
	config HealthCheckConfig

	// 服务注册表
	services map[string]*ServiceHealth // 服务名 -> 健康状态

	// HTTP 客户端
	httpClient *http.Client

	// 依赖
	logger *zap.Logger

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServiceHealthManager 创建服务健康度管理器
func NewServiceHealthManager(config HealthCheckConfig, logger *zap.Logger) *ServiceHealthManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceHealthManager{
		config:   config,
		services: make(map[string]*ServiceHealth),
		httpClient: &http.Client{
			Timeout: config.DefaultTimeout,
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// ============================================================
// 服务注册与管理
// ============================================================

// RegisterService 注册服务
func (m *ServiceHealthManager) RegisterService(cfg ServiceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.Name == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	// 设置默认值
	if cfg.Interval <= 0 {
		cfg.Interval = m.config.DefaultInterval
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = m.config.DefaultTimeout
	}
	if cfg.CheckHost == "" {
		cfg.CheckHost = "localhost"
	}

	// 检查是否已注册
	if _, exists := m.services[cfg.Name]; exists {
		return fmt.Errorf("服务 %s 已注册", cfg.Name)
	}

	m.services[cfg.Name] = &ServiceHealth{
		Config:    cfg,
		Status:    StatusUnknown,
		Score:     0,
		Checks:    make([]HealthCheckResult, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.logger.Info("服务已注册",
		zap.String("name", cfg.Name),
		zap.String("type", string(cfg.CheckType)))

	return nil
}

// UnregisterService 注销服务
func (m *ServiceHealthManager) UnregisterService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[name]; !exists {
		return fmt.Errorf("服务 %s 未注册", name)
	}

	delete(m.services, name)
	m.logger.Info("服务已注销", zap.String("name", name))
	return nil
}

// GetService 获取单个服务健康状态
func (m *ServiceHealthManager) GetService(name string) (*ServiceHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.services[name]
	if !exists {
		return nil, fmt.Errorf("服务 %s 未注册", name)
	}

	// 返回副本
	result := *health
	return &result, nil
}

// ListServices 列出所有服务健康状态
func (m *ServiceHealthManager) ListServices() []*ServiceHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ServiceHealth, 0, len(m.services))
	for _, h := range m.services {
		copy := *h
		result = append(result, &copy)
	}
	return result
}

// ============================================================
// 服务发现
// ============================================================

// DiscoverServices 自动发现运行中的服务
// 通过扫描常用端口和进程检测本地服务
func (m *ServiceHealthManager) DiscoverServices(ctx context.Context) []ServiceConfig {
	discovered := make([]ServiceConfig, 0)

	// 常见服务端口映射
	commonPorts := map[int]string{
		80:    "http",
		443:   "https",
		3000:  "grafana",
		5432:  "postgresql",
		6379:  "redis",
		8080:  "api",
		8443:  "api-ssl",
		9090:  "prometheus",
		9100:  "node-exporter",
		2379:  "etcd",
		2380:  "etcd-peer",
		3306:  "mysql",
		8983:  "solr",
		8123:  "clickhouse",
		9200:  "elasticsearch",
		5601:  "kibana",
		8888:  "jupyter",
		19999: "netdata",
		22:    "ssh",
		53:    "dns",
		25:    "smtp",
		143:   "imap",
		993:   "imaps",
		995:   "pop3s",
		111:   "rpcbind",
		2049:  "nfs",
		445:   "smb",
		6667:  "irc",
		1883:  "mqtt",
		8883:  "mqtts",
		51820: "wireguard",
	}

	m.logger.Info("开始服务发现扫描")

	for port, serviceName := range commonPorts {
		select {
		case <-ctx.Done():
			return discovered
		default:
		}

		addr := fmt.Sprintf("localhost:%d", port)
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			cfg := ServiceConfig{
				Name:        serviceName,
				DisplayName: fmt.Sprintf("%s (port %d)", serviceName, port),
				CheckType:   CheckTCP,
				CheckHost:   "localhost",
				CheckPort:   port,
				Interval:    60 * time.Second,
				Timeout:     3 * time.Second,
				Enabled:     false, // 发现的服务默认不启用，需手动开启
				Tags:        map[string]string{"source": "discovery"},
			}
			discovered = append(discovered, cfg)
			m.logger.Info("发现服务",
				zap.String("name", serviceName),
				zap.Int("port", port))
		}
	}

	m.logger.Info("服务发现完成", zap.Int("count", len(discovered)))
	return discovered
}

// ============================================================
// 健康检查执行
// ============================================================

// RunCheck 执行单次健康检查
func (m *ServiceHealthManager) RunCheck(ctx context.Context, name string) (*HealthCheckResult, error) {
	m.mu.RLock()
	health, exists := m.services[name]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("服务 %s 未注册", name)
	}
	cfg := health.Config
	m.mu.RUnlock()

	var result *HealthCheckResult

	switch cfg.CheckType {
	case CheckHTTP:
		result = m.checkHTTP(ctx, cfg)
	case CheckTCP:
		result = m.checkTCP(ctx, cfg)
	case CheckProcess:
		result = m.checkProcess(ctx, cfg)
	case CheckScript:
		result = m.checkScript(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的检查类型: %s", cfg.CheckType)
	}

	// 更新服务状态
	m.updateServiceHealth(name, result)

	return result, nil
}

// RunAllChecks 执行所有服务的健康检查
func (m *ServiceHealthManager) RunAllChecks(ctx context.Context) {
	m.mu.RLock()
	names := make([]string, 0, len(m.services))
	for name, h := range m.services {
		if h.Config.Enabled {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()

	// 使用工作池并发检查
	workerCount := m.config.WorkerCount
	if workerCount <= 0 {
		workerCount = 5
	}

	jobs := make(chan string, len(names))
	var wg sync.WaitGroup

	// 启动工作线程
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					if _, err := m.RunCheck(ctx, name); err != nil {
						m.logger.Error("健康检查失败",
							zap.String("service", name),
							zap.Error(err))
					}
				}
			}
		}()
	}

	// 发送任务
	for _, name := range names {
		jobs <- name
	}
	close(jobs)

	wg.Wait()
}

// checkHTTP HTTP 健康检查
func (m *ServiceHealthManager) checkHTTP(ctx context.Context, cfg ServiceConfig) *HealthCheckResult {
	result := &HealthCheckResult{
		ServiceName: cfg.Name,
		CheckType:   CheckHTTP,
		Timestamp:   time.Now(),
	}

	url := cfg.CheckURL
	if url == "" {
		url = fmt.Sprintf("http://%s:%d/health", cfg.CheckHost, cfg.CheckPort)
	}

	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("创建请求失败: %v", err)
		return result
	}

	start := time.Now()
	resp, err := m.httpClient.Do(req)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("请求失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// 状态码判断
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = StatusHealthy
		result.Message = "HTTP 检查通过"
	} else if resp.StatusCode >= 500 {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("服务端错误: %d", resp.StatusCode)
	} else {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("异常状态码: %d", resp.StatusCode)
	}

	// 响应时间判断
	if result.ResponseTime > 5*time.Second {
		if result.Status == StatusHealthy {
			result.Status = StatusWarning
			result.Message = fmt.Sprintf("响应时间过长: %v", result.ResponseTime)
		}
	}

	return result
}

// checkTCP TCP 端口检查
func (m *ServiceHealthManager) checkTCP(ctx context.Context, cfg ServiceConfig) *HealthCheckResult {
	result := &HealthCheckResult{
		ServiceName: cfg.Name,
		CheckType:   CheckTCP,
		Timestamp:   time.Now(),
	}

	addr := net.JoinHostPort(cfg.CheckHost, fmt.Sprintf("%d", cfg.CheckPort))

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, cfg.Timeout)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("TCP 连接失败: %v", err)
		return result
	}
	conn.Close()

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("TCP 端口 %d 可达", cfg.CheckPort)

	return result
}

// checkProcess 进程存活检查
func (m *ServiceHealthManager) checkProcess(ctx context.Context, cfg ServiceConfig) *HealthCheckResult {
	result := &HealthCheckResult{
		ServiceName: cfg.Name,
		CheckType:   CheckProcess,
		Timestamp:   time.Now(),
	}

	if cfg.ProcessName == "" {
		result.Status = StatusUnknown
		result.Message = "未配置进程名称"
		return result
	}

	start := time.Now()
	// 使用 pgrep 查找进程
	cmd := exec.CommandContext(ctx, "pgrep", "-x", cfg.ProcessName)
	output, err := cmd.Output()
	result.ResponseTime = time.Since(start)

	if err != nil {
		// pgrep 返回 1 表示未找到进程
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("进程 %s 未运行", cfg.ProcessName)
		return result
	}

	if len(output) > 0 {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("进程 %s 运行中", cfg.ProcessName)
	} else {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("进程 %s 未找到", cfg.ProcessName)
	}

	return result
}

// checkScript 自定义脚本检查
func (m *ServiceHealthManager) checkScript(ctx context.Context, cfg ServiceConfig) *HealthCheckResult {
	result := &HealthCheckResult{
		ServiceName: cfg.Name,
		CheckType:   CheckScript,
		Timestamp:   time.Now(),
	}

	if cfg.ScriptPath == "" {
		result.Status = StatusUnknown
		result.Message = "未配置脚本路径"
		return result
	}

	scriptCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	args := append([]string{cfg.ScriptPath}, cfg.ScriptArgs...)
	cmd := exec.CommandContext(scriptCtx, "sh", args...)

	start := time.Now()
	output, err := cmd.CombinedOutput()
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("脚本执行失败: %v, 输出: %s", err, string(output))
		return result
	}

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("脚本检查通过: %s", string(output))

	return result
}

// updateServiceHealth 更新服务健康状态和评分
func (m *ServiceHealthManager) updateServiceHealth(name string, result *HealthCheckResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.services[name]
	if !exists {
		return
	}

	// 更新最后检查
	health.LastCheck = result
	health.UpdatedAt = time.Now()

	// 添加到历史
	health.Checks = append(health.Checks, *result)
	health.TotalChecks++

	if result.Status == StatusCritical {
		health.FailedChecks++
	}

	// 裁剪历史
	if len(health.Checks) > m.config.MaxHistoryLen {
		health.Checks = health.Checks[len(health.Checks)-m.config.MaxHistoryLen:]
	}

	// 更新状态
	health.Status = result.Status

	// 更新可用性
	if health.TotalChecks > 0 {
		health.Uptime = float64(health.TotalChecks-health.FailedChecks) / float64(health.TotalChecks) * 100
	}

	// 计算健康评分
	health.Score = m.calculateHealthScore(health)
}

// ============================================================
// 健康评分算法（0-100）
// ============================================================

// calculateHealthScore 计算健康评分
// 权重：可用性 40%, 性能 30%, 错误率 20%, 资源消耗 10%
func (m *ServiceHealthManager) calculateHealthScore(health *ServiceHealth) float64 {
	if health.TotalChecks == 0 {
		return 0
	}

	// 1. 可用性评分（40%）- 正常运行时间比例
	availabilityScore := health.Uptime

	// 2. 性能评分（30%）- 基于响应时间
	performanceScore := m.calculatePerformanceScore(health)

	// 3. 错误率评分（20%）
	errorRate := float64(health.FailedChecks) / float64(health.TotalChecks) * 100
	errorScore := math.Max(0, 100-errorRate*10) // 每1%错误率扣10分

	// 4. 资源消耗评分（10%）- 基于最近的响应时间趋势
	resourceScore := m.calculateResourceScore(health)

	// 加权计算
	score := availabilityScore*0.4 +
		performanceScore*0.3 +
		errorScore*0.2 +
		resourceScore*0.1

	// 限制在 0-100 范围
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return math.Round(score*100) / 100
}

// calculatePerformanceScore 计算性能评分
func (m *ServiceHealthManager) calculatePerformanceScore(health *ServiceHealth) float64 {
	if len(health.Checks) == 0 {
		return 50 // 无数据时给中间分
	}

	// 取最近10次检查的平均响应时间
	recentCount := 10
	if len(health.Checks) < recentCount {
		recentCount = len(health.Checks)
	}

	var totalResponseTime time.Duration
	recentChecks := health.Checks[len(health.Checks)-recentCount:]
	for _, check := range recentChecks {
		totalResponseTime += check.ResponseTime
	}

	avgResponseMs := float64(totalResponseTime.Milliseconds()) / float64(recentCount)

	// 评分标准：
	// < 100ms  -> 100分
	// 100-500ms -> 80分
	// 500ms-1s -> 60分
	// 1-3s     -> 40分
	// 3-5s     -> 20分
	// > 5s     -> 0分
	switch {
	case avgResponseMs < 100:
		return 100
	case avgResponseMs < 500:
		return 100 - (avgResponseMs-100)/400*20
	case avgResponseMs < 1000:
		return 80 - (avgResponseMs-500)/500*20
	case avgResponseMs < 3000:
		return 60 - (avgResponseMs-1000)/2000*20
	case avgResponseMs < 5000:
		return 40 - (avgResponseMs-3000)/2000*20
	default:
		return 0
	}
}

// calculateResourceScore 计算资源消耗评分
func (m *ServiceHealthManager) calculateResourceScore(health *ServiceHealth) float64 {
	if len(health.Checks) < 2 {
		return 100 // 数据不足，假设正常
	}

	// 检查响应时间是否呈上升趋势（表示资源消耗增加）
	recentCount := 5
	if len(health.Checks) < recentCount {
		recentCount = len(health.Checks)
	}

	recentChecks := health.Checks[len(health.Checks)-recentCount:]

	// 计算响应时间趋势
	var trendSum float64
	for i := 1; i < len(recentChecks); i++ {
		prev := float64(recentChecks[i-1].ResponseTime.Milliseconds())
		curr := float64(recentChecks[i].ResponseTime.Milliseconds())
		if prev > 0 {
			trendSum += (curr - prev) / prev
		}
	}

	avgTrend := trendSum / float64(len(recentChecks)-1)

	// 趋势评分
	// 稳定或下降 -> 100
	// 轻微上升 -> 80
	// 明显上升 -> 50
	// 急剧上升 -> 20
	switch {
	case avgTrend <= 0:
		return 100
	case avgTrend < 0.1:
		return 80
	case avgTrend < 0.3:
		return 50
	default:
		return 20
	}
}

// ============================================================
// 定时检查调度
// ============================================================

// StartPeriodicChecks 启动定时健康检查
func (m *ServiceHealthManager) StartPeriodicChecks() {
	m.logger.Info("启动定时健康检查")

	go func() {
		// 首次立即检查
		m.RunAllChecks(m.ctx)

		// 按最短间隔轮询
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		lastCheck := make(map[string]time.Time)

		for {
			select {
			case <-m.ctx.Done():
				m.logger.Info("定时健康检查已停止")
				return
			case now := <-ticker.C:
				m.mu.RLock()
				for name, health := range m.services {
					if !health.Config.Enabled {
						continue
					}

					last, checked := lastCheck[name]
					if !checked || now.Sub(last) >= health.Config.Interval {
						// 需要执行检查
						go func(n string) {
							if _, err := m.RunCheck(m.ctx, n); err != nil {
								m.logger.Error("定时检查失败",
									zap.String("service", n),
									zap.Error(err))
							}
							m.mu.Lock()
							lastCheck[n] = time.Now()
							m.mu.Unlock()
						}(name)
					}
				}
				m.mu.RUnlock()
			}
		}
	}()
}

// StopPeriodicChecks 停止定时健康检查
func (m *ServiceHealthManager) StopPeriodicChecks() {
	m.cancel()
}

// ============================================================
// 辅助方法
// ============================================================

// UpdateServiceConfig 更新服务配置
func (m *ServiceHealthManager) UpdateServiceConfig(name string, cfg ServiceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.services[name]
	if !exists {
		return fmt.Errorf("服务 %s 未注册", name)
	}

	health.Config = cfg
	health.UpdatedAt = time.Now()
	return nil
}

// EnableService 启用服务检查
func (m *ServiceHealthManager) EnableService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.services[name]
	if !exists {
		return fmt.Errorf("服务 %s 未注册", name)
	}

	health.Config.Enabled = true
	health.UpdatedAt = time.Now()
	return nil
}

// DisableService 禁用服务检查
func (m *ServiceHealthManager) DisableService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.services[name]
	if !exists {
		return fmt.Errorf("服务 %s 未注册", name)
	}

	health.Config.Enabled = false
	health.UpdatedAt = time.Now()
	return nil
}

// GetServiceChecks 获取服务历史检查记录
func (m *ServiceHealthManager) GetServiceChecks(name string, limit int) ([]HealthCheckResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.services[name]
	if !exists {
		return nil, fmt.Errorf("服务 %s 未注册", name)
	}

	if limit <= 0 || limit > len(health.Checks) {
		limit = len(health.Checks)
	}

	start := len(health.Checks) - limit
	result := make([]HealthCheckResult, limit)
	copy(result, health.Checks[start:])
	return result, nil
}

// GetServiceStats 获取服务统计信息
func (m *ServiceHealthManager) GetServiceStats(name string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.services[name]
	if !exists {
		return nil, fmt.Errorf("服务 %s 未注册", name)
	}

	stats := map[string]interface{}{
		"name":           name,
		"status":         health.Status,
		"score":          health.Score,
		"uptime":         health.Uptime,
		"total_checks":   health.TotalChecks,
		"failed_checks":  health.FailedChecks,
		"created_at":     health.CreatedAt,
		"updated_at":     health.UpdatedAt,
		"check_interval": health.Config.Interval.String(),
	}

	if health.LastCheck != nil {
		stats["last_check_time"] = health.LastCheck.Timestamp
		stats["last_response_time"] = health.LastCheck.ResponseTime.String()
	}

	return stats, nil
}
