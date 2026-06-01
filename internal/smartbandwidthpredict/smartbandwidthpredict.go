package smartbandwidthpredict

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Engine 智能带宽预测引擎
type Engine struct {
	mu          sync.RWMutex
	config      *Config
	logger      *zap.Logger
	collector   *Collector
	predictor   *Predictor
	scheduler   *Scheduler
	running     bool
	stopCh      chan struct{}
	predictions []*BandwidthPrediction
	schedules   []*SchedulePlan
	policies    map[string]*QoSPolicy
}

// NewEngine 创建智能带宽预测引擎
func NewEngine(config *Config, logger *zap.Logger) (*Engine, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	e := &Engine{
		config:      config,
		logger:      logger,
		stopCh:      make(chan struct{}),
		predictions: make([]*BandwidthPrediction, 0),
		schedules:   make([]*SchedulePlan, 0),
		policies:    make(map[string]*QoSPolicy),
	}

	e.collector = NewCollector(config, logger)
	e.predictor = NewPredictor(config, logger)
	e.scheduler = NewScheduler(config, logger)

	return e, nil
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("引擎已经在运行")
	}

	e.logger.Info("启动智能带宽预测引擎",
		zap.Float64("total_bandwidth_mbps", e.config.TotalBandwidthMbps),
		zap.Duration("collect_interval", e.config.CollectInterval),
		zap.Int("prediction_window", e.config.PredictionWindow),
	)

	// 启动采集器
	if err := e.collector.Start(); err != nil {
		return fmt.Errorf("启动采集器失败: %w", err)
	}

	e.running = true

	// 启动预测循环
	go e.predictionLoop()

	e.logger.Info("智能带宽预测引擎启动成功")
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.logger.Info("停止智能带宽预测引擎")

	close(e.stopCh)
	e.collector.Stop()
	e.running = false

	e.logger.Info("智能带宽预测引擎已停止")
}

// IsRunning 检查引擎是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// RecordTraffic 记录流量数据
func (e *Engine) RecordTraffic(sample *TrafficSample) error {
	if sample == nil {
		return fmt.Errorf("采样数据不能为空")
	}

	if sample.Timestamp.IsZero() {
		sample.Timestamp = time.Now()
	}

	return e.collector.Record(sample)
}

// PredictBandwidth 预测带宽
func (e *Engine) PredictBandwidth(horizonMinutes int) (*BandwidthPrediction, error) {
	if horizonMinutes <= 0 {
		horizonMinutes = e.config.PredictionHorizon
	}

	samples := e.collector.GetSamples()
	if len(samples) < e.config.PredictionWindow {
		return nil, fmt.Errorf("采样数据不足: 需要 %d, 当前 %d", e.config.PredictionWindow, len(samples))
	}

	prediction, err := e.predictor.Predict(samples, horizonMinutes)
	if err != nil {
		return nil, fmt.Errorf("预测失败: %w", err)
	}

	e.mu.Lock()
	e.predictions = append(e.predictions, prediction)
	e.mu.Unlock()

	e.logger.Debug("带宽预测完成",
		zap.Float64("predicted_mbps", prediction.PredictedMbps),
		zap.Float64("confidence", prediction.Confidence),
		zap.String("trend", string(prediction.Trend)),
	)

	return prediction, nil
}

// CreateSchedule 创建调度计划
func (e *Engine) CreateSchedule(tasks []*ScheduleTask) (*SchedulePlan, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("任务列表不能为空")
	}

	// 获取当前预测
	currentPrediction, err := e.PredictBandwidth(e.config.PredictionHorizon)
	if err != nil {
		e.logger.Warn("无法获取带宽预测，使用默认值", zap.Error(err))
		currentPrediction = &BandwidthPrediction{
			PredictedMbps: e.config.TotalBandwidthMbps * 0.7,
			LowerBound:    e.config.TotalBandwidthMbps * 0.5,
			UpperBound:    e.config.TotalBandwidthMbps * 0.9,
			Confidence:    0.5,
			Trend:         TrendStable,
		}
	}

	plan, err := e.scheduler.CreatePlan(tasks, currentPrediction, e.policies)
	if err != nil {
		return nil, fmt.Errorf("创建调度计划失败: %w", err)
	}

	e.mu.Lock()
	e.schedules = append(e.schedules, plan)
	e.mu.Unlock()

	e.logger.Info("调度计划创建成功",
		zap.String("plan_id", plan.ID),
		zap.Int("task_count", len(plan.Tasks)),
		zap.Float64("total_mbps", plan.TotalMbps),
	)

	return plan, nil
}

// ApplyQoS 应用QoS策略
func (e *Engine) ApplyQoS(policy *QoSPolicy) error {
	if policy == nil {
		return fmt.Errorf("QoS策略不能为空")
	}

	if policy.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	if policy.Priority < 1 || policy.Priority > 10 {
		return fmt.Errorf("优先级必须在1-10之间")
	}

	if policy.MinMbps < 0 {
		return fmt.Errorf("最小带宽不能为负数")
	}

	if policy.MaxMbps <= 0 {
		return fmt.Errorf("最大带宽必须大于0")
	}

	if policy.MinMbps > policy.MaxMbps {
		return fmt.Errorf("最小带宽不能大于最大带宽")
	}

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("qos_%d", time.Now().UnixNano())
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	policy.Enabled = true

	e.mu.Lock()
	e.policies[policy.ID] = policy
	e.mu.Unlock()

	e.logger.Info("QoS策略应用成功",
		zap.String("policy_id", policy.ID),
		zap.String("name", policy.Name),
		zap.Int("priority", policy.Priority),
		zap.Float64("min_mbps", policy.MinMbps),
		zap.Float64("max_mbps", policy.MaxMbps),
	)

	return nil
}

// GetPredictions 获取所有预测结果
func (e *Engine) GetPredictions() []*BandwidthPrediction {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*BandwidthPrediction, len(e.predictions))
	copy(result, e.predictions)
	return result
}

// GetSchedules 获取所有调度计划
func (e *Engine) GetSchedules() []*SchedulePlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*SchedulePlan, len(e.schedules))
	copy(result, e.schedules)
	return result
}

// GetQoSPolicies 获取所有QoS策略
func (e *Engine) GetQoSPolicies() map[string]*QoSPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*QoSPolicy)
	for k, v := range e.policies {
		result[k] = v
	}
	return result
}

// GetConfig 获取配置
func (e *Engine) GetConfig() *Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// UpdateConfig 更新配置
func (e *Engine) UpdateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.config = config
	e.collector.UpdateConfig(config)
	e.predictor.UpdateConfig(config)
	e.scheduler.UpdateConfig(config)

	e.logger.Info("配置更新成功")
	return nil
}

// GetCollector 获取采集器
func (e *Engine) GetCollector() *Collector {
	return e.collector
}

// GetPredictor 获取预测器
func (e *Engine) GetPredictor() *Predictor {
	return e.predictor
}

// GetScheduler 获取调度器
func (e *Engine) GetScheduler() *Scheduler {
	return e.scheduler
}

// predictionLoop 预测循环
func (e *Engine) predictionLoop() {
	ticker := time.NewTicker(e.config.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.runPrediction()
		}
	}
}

// runPrediction 执行预测
func (e *Engine) runPrediction() {
	samples := e.collector.GetSamples()
	if len(samples) < e.config.PredictionWindow {
		e.logger.Debug("采样数据不足，跳过预测",
			zap.Int("current", len(samples)),
			zap.Int("required", e.config.PredictionWindow),
		)
		return
	}

	prediction, err := e.predictor.Predict(samples, e.config.PredictionHorizon)
	if err != nil {
		e.logger.Error("预测失败", zap.Error(err))
		return
	}

	e.mu.Lock()
	e.predictions = append(e.predictions, prediction)
	e.mu.Unlock()

	// 检测异常
	if e.predictor.IsAnomaly(samples) {
		e.logger.Warn("检测到异常流量",
			zap.Float64("predicted_mbps", prediction.PredictedMbps),
			zap.Float64("confidence", prediction.Confidence),
		)
	}
}
