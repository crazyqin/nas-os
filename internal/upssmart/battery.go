// Package upssmart 提供 UPS 智能管理功能
package upssmart

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// BatteryCondition 电池状况
type BatteryCondition string

const (
	ConditionExcellent BatteryCondition = "excellent" // 优秀
	ConditionGood      BatteryCondition = "good"      // 良好
	ConditionFair      BatteryCondition = "fair"      // 一般
	ConditionPoor      BatteryCondition = "poor"      // 差
	ConditionReplace   BatteryCondition = "replace"   // 需要更换
)

// BatteryTestResult 电池测试结果
type BatteryTestResult string

const (
	TestPassed  BatteryTestResult = "passed"  // 通过
	TestFailed  BatteryTestResult = "failed"  // 失败
	TestWarning BatteryTestResult = "warning" // 警告
	TestPending BatteryTestResult = "pending" // 待测试
)

// BatteryHealth 电池健康数据
type BatteryHealth struct {
	UPSID              string            `json:"ups_id"`              // UPS 设备 ID
	InstalledDate      time.Time         `json:"installed_date"`      // 安装日期
	AgeMonths          int               `json:"age_months"`          // 使用月数
	CycleCount         int               `json:"cycle_count"`         // 充放电次数
	DesignCapacity     int               `json:"design_capacity"`     // 设计容量 mAh
	CurrentCapacity    int               `json:"current_capacity"`    // 当前容量 mAh
	CapacityPercent    float64           `json:"capacity_percent"`    // 容量百分比
	Condition          BatteryCondition  `json:"condition"`           // 电池状况
	HealthScore        int               `json:"health_score"`        // 健康评分 0-100
	LastTestDate       time.Time         `json:"last_test_date"`      // 上次测试日期
	LastTestResult     BatteryTestResult `json:"last_test_result"`    // 上次测试结果
	EstimatedLifeLeft  time.Duration     `json:"estimated_life_left"` // 预计剩余寿命
	ReplaceRecommended bool              `json:"replace_recommended"` // 建议更换
	ReplaceDeadline    time.Time         `json:"replace_deadline"`    // 建议更换日期
	TemperatureAvg     float64           `json:"temperature_avg"`     // 平均温度
	DischargeDepthAvg  float64           `json:"discharge_depth_avg"` // 平均放电深度
	LastUpdated        time.Time         `json:"last_updated"`        // 最后更新时间
}

// BatteryHealthConfig 电池健康管理配置
type BatteryHealthConfig struct {
	ReplacementAgeMonths  int           `json:"replacement_age_months"`  // 更换年龄阈值（月）
	ReplacementCycleCount int           `json:"replacement_cycle_count"` // 更换充放电次数阈值
	CapacityThreshold     float64       `json:"capacity_threshold"`      // 容量衰减阈值（百分比）
	TestInterval          time.Duration `json:"test_interval"`           // 测试间隔
	AlertOnReplace        bool          `json:"alert_on_replace"`        // 更换时发送告警
	HealthyTempMin        float64       `json:"healthy_temp_min"`        // 健康温度最小值
	HealthyTempMax        float64       `json:"healthy_temp_max"`        // 健康温度最大值
}

// DefaultBatteryHealthConfig 返回默认配置
func DefaultBatteryHealthConfig() BatteryHealthConfig {
	return BatteryHealthConfig{
		ReplacementAgeMonths:  36,                 // 3年
		ReplacementCycleCount: 500,                // 500次充放电
		CapacityThreshold:     80.0,               // 容量低于80%建议更换
		TestInterval:          7 * 24 * time.Hour, // 每周测试一次
		AlertOnReplace:        true,
		HealthyTempMin:        15.0, // 最低15°C
		HealthyTempMax:        30.0, // 最高30°C
	}
}

// BatteryManager 电池健康管理器
type BatteryManager struct {
	mu         sync.RWMutex
	config     BatteryHealthConfig
	upsManager *UPSManager
	healthMap  map[string]*BatteryHealth // UPS ID -> 健康数据
	testQueue  chan string               // 待测试队列
	stopCh     chan struct{}
	running    bool
	onAlert    func(string, *BatteryHealth) // 告警回调
}

// NewBatteryManager 创建电池管理器
func NewBatteryManager(upsManager *UPSManager, config BatteryHealthConfig) *BatteryManager {
	return &BatteryManager{
		config:     config,
		upsManager: upsManager,
		healthMap:  make(map[string]*BatteryHealth),
		testQueue:  make(chan string, 100),
		stopCh:     make(chan struct{}),
	}
}

// RegisterAlertCallback 注册告警回调
func (bm *BatteryManager) RegisterAlertCallback(fn func(string, *BatteryHealth)) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.onAlert = fn
}

// InitBatteryHealth 初始化电池健康数据
func (bm *BatteryManager) InitBatteryHealth(upsID string, installedDate time.Time, designCapacity int) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if _, exists := bm.healthMap[upsID]; exists {
		return
	}

	ageMonths := int(time.Since(installedDate).Hours() / 24 / 30)

	bm.healthMap[upsID] = &BatteryHealth{
		UPSID:           upsID,
		InstalledDate:   installedDate,
		AgeMonths:       ageMonths,
		DesignCapacity:  designCapacity,
		CurrentCapacity: designCapacity,
		CapacityPercent: 100.0,
		Condition:       ConditionExcellent,
		HealthScore:     100,
		LastTestResult:  TestPending,
		LastUpdated:     time.Now(),
	}

	log.Printf("✅ 初始化电池健康数据: %s (安装于 %s)", upsID, installedDate.Format("2006-01-02"))
}

// Start 启动电池管理器
func (bm *BatteryManager) Start() {
	bm.mu.Lock()
	if bm.running {
		bm.mu.Unlock()
		return
	}
	bm.running = true
	bm.mu.Unlock()

	go bm.monitorLoop()
	go bm.testLoop()

	log.Println("✅ 电池健康管理器已启动")
}

// Stop 停止电池管理器
func (bm *BatteryManager) Stop() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.running {
		return
	}

	close(bm.stopCh)
	bm.running = false
	log.Println("电池健康管理器已停止")
}

// GetBatteryHealth 获取电池健康数据
func (bm *BatteryManager) GetBatteryHealth(upsID string) (*BatteryHealth, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	health, exists := bm.healthMap[upsID]
	if !exists {
		return nil, fmt.Errorf("未找到 UPS %s 的电池健康数据", upsID)
	}

	return health, nil
}

// GetAllBatteryHealth 获取所有电池健康数据
func (bm *BatteryManager) GetAllBatteryHealth() []*BatteryHealth {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make([]*BatteryHealth, 0, len(bm.healthMap))
	for _, h := range bm.healthMap {
		result = append(result, h)
	}
	return result
}

// TriggerBatteryTest 触发电池测试
func (bm *BatteryManager) TriggerBatteryTest(upsID string) error {
	bm.mu.RLock()
	_, exists := bm.healthMap[upsID]
	bm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("未找到 UPS %s 的电池健康数据", upsID)
	}

	select {
	case bm.testQueue <- upsID:
		log.Printf("✅ UPS %s 的电池测试已加入队列", upsID)
		return nil
	default:
		return fmt.Errorf("测试队列已满")
	}
}

// monitorLoop 监控循环
func (bm *BatteryManager) monitorLoop() {
	ticker := time.NewTicker(1 * time.Hour) // 每小时更新一次
	defer ticker.Stop()

	for {
		select {
		case <-bm.stopCh:
			return
		case <-ticker.C:
			bm.updateAllHealth()
		}
	}
}

// testLoop 测试循环
func (bm *BatteryManager) testLoop() {
	for {
		select {
		case <-bm.stopCh:
			return
		case upsID := <-bm.testQueue:
			bm.runBatteryTest(upsID)
		}
	}
}

// updateAllHealth 更新所有电池健康数据
func (bm *BatteryManager) updateAllHealth() {
	bm.mu.RLock()
	upsIDs := make([]string, 0, len(bm.healthMap))
	for id := range bm.healthMap {
		upsIDs = append(upsIDs, id)
	}
	bm.mu.RUnlock()

	for _, id := range upsIDs {
		bm.updateHealth(id)
	}
}

// updateHealth 更新单个电池健康数据
func (bm *BatteryManager) updateHealth(upsID string) {
	// 获取 UPS 状态
	device, err := bm.upsManager.GetDevice(upsID)
	if err != nil {
		return
	}

	bm.mu.Lock()
	health, exists := bm.healthMap[upsID]
	if !exists {
		bm.mu.Unlock()
		return
	}

	// 更新温度平均值（指数移动平均）
	if health.TemperatureAvg == 0 {
		health.TemperatureAvg = device.Status.Temperature
	} else {
		health.TemperatureAvg = health.TemperatureAvg*0.9 + device.Status.Temperature*0.1
	}

	// 更新健康评分
	health.HealthScore = bm.calculateHealthScore(health, device.Status)

	// 更新电池状况
	health.Condition = bm.determineCondition(health)

	// 更新更换建议
	health.ReplaceRecommended = bm.shouldReplace(health)
	if health.ReplaceRecommended {
		health.ReplaceDeadline = bm.calculateReplaceDeadline(health)
	}

	// 更新年龄
	health.AgeMonths = int(time.Since(health.InstalledDate).Hours() / 24 / 30)

	// 更新预计剩余寿命
	health.EstimatedLifeLeft = bm.estimateRemainingLife(health)

	health.LastUpdated = time.Now()
	bm.mu.Unlock()

	// 检查是否需要告警
	bm.checkAlerts(upsID, health)
}

// calculateHealthScore 计算健康评分
func (bm *BatteryManager) calculateHealthScore(health *BatteryHealth, status UPSStatus) int {
	score := 100.0

	// 容量衰减因子
	if health.CapacityPercent < 100 {
		score -= (100 - health.CapacityPercent) * 0.5
	}

	// 年龄因子（每增加1年扣2分）
	ageYears := float64(health.AgeMonths) / 12.0
	score -= ageYears * 2

	// 充放电次数因子（每100次扣1分）
	score -= float64(health.CycleCount) / 100.0

	// 温度因子
	bm.mu.RLock()
	config := bm.config
	bm.mu.RUnlock()

	if status.Temperature < config.HealthyTempMin {
		score -= (config.HealthyTempMin - status.Temperature) * 2
	} else if status.Temperature > config.HealthyTempMax {
		score -= (status.Temperature - config.HealthyTempMax) * 2
	}

	// 测试结果因子
	if health.LastTestResult == TestFailed {
		score -= 20
	} else if health.LastTestResult == TestWarning {
		score -= 10
	}

	return int(math.Max(0, math.Min(100, score)))
}

// determineCondition 确定电池状况
func (bm *BatteryManager) determineCondition(health *BatteryHealth) BatteryCondition {
	score := health.HealthScore

	switch {
	case score >= 90:
		return ConditionExcellent
	case score >= 75:
		return ConditionGood
	case score >= 60:
		return ConditionFair
	case score >= 40:
		return ConditionPoor
	default:
		return ConditionReplace
	}
}

// shouldReplace 判断是否需要更换
func (bm *BatteryManager) shouldReplace(health *BatteryHealth) bool {
	bm.mu.RLock()
	config := bm.config
	bm.mu.RUnlock()

	// 年龄超过阈值
	if health.AgeMonths >= config.ReplacementAgeMonths {
		return true
	}

	// 充放电次数超过阈值
	if health.CycleCount >= config.ReplacementCycleCount {
		return true
	}

	// 容量衰减超过阈值
	if health.CapacityPercent < config.CapacityThreshold {
		return true
	}

	// 测试失败
	if health.LastTestResult == TestFailed {
		return true
	}

	// 健康评分过低
	if health.HealthScore < 40 {
		return true
	}

	return false
}

// calculateReplaceDeadline 计算建议更换日期
func (bm *BatteryManager) calculateReplaceDeadline(health *BatteryHealth) time.Time {
	// 基于健康评分估算剩余时间
	switch {
	case health.HealthScore < 20:
		return time.Now().Add(7 * 24 * time.Hour) // 1周内
	case health.HealthScore < 40:
		return time.Now().Add(30 * 24 * time.Hour) // 1个月内
	case health.HealthScore < 60:
		return time.Now().Add(90 * 24 * time.Hour) // 3个月内
	default:
		return time.Now().Add(180 * 24 * time.Hour) // 6个月内
	}
}

// estimateRemainingLife 估算电池剩余寿命
func (bm *BatteryManager) estimateRemainingLife(health *BatteryHealth) time.Duration {
	// 基于充放电次数和年龄估算
	avgCyclesPerMonth := float64(health.CycleCount) / math.Max(1, float64(health.AgeMonths))

	bm.mu.RLock()
	config := bm.config
	bm.mu.RUnlock()

	remainingCycles := float64(config.ReplacementCycleCount - health.CycleCount)
	if remainingCycles <= 0 {
		return 0
	}

	remainingMonths := remainingCycles / math.Max(1, avgCyclesPerMonth)
	return time.Duration(remainingMonths*30*24) * time.Hour
}

// runBatteryTest 运行电池测试
func (bm *BatteryManager) runBatteryTest(upsID string) {
	log.Printf("🔋 开始电池测试: UPS %s", upsID)

	bm.mu.Lock()
	health, exists := bm.healthMap[upsID]
	if !exists {
		bm.mu.Unlock()
		return
	}

	// 模拟测试过程（实际应发送测试命令并等待结果）
	time.Sleep(5 * time.Second)

	// 模拟测试结果
	testResult := TestPassed
	if health.CapacityPercent < 80 {
		testResult = TestWarning
	}
	if health.CapacityPercent < 60 {
		testResult = TestFailed
	}

	// 更新测试结果
	health.LastTestDate = time.Now()
	health.LastTestResult = testResult

	// 模拟容量衰减（每次测试减少0.1-0.5%）
	capacityLoss := 0.1 + float64(health.CycleCount)*0.001
	health.CapacityPercent = math.Max(0, health.CapacityPercent-capacityLoss)
	health.CurrentCapacity = int(float64(health.DesignCapacity) * health.CapacityPercent / 100)

	// 增加充放电次数
	health.CycleCount++

	bm.mu.Unlock()

	log.Printf("✅ 电池测试完成: UPS %s, 结果: %s, 容量: %.1f%%",
		upsID, testResult, health.CapacityPercent)

	// 重新评估健康状态
	bm.updateHealth(upsID)
}

// checkAlerts 检查告警条件
func (bm *BatteryManager) checkAlerts(upsID string, health *BatteryHealth) {
	bm.mu.RLock()
	alertOnReplace := bm.config.AlertOnReplace
	bm.mu.RUnlock()

	if !alertOnReplace {
		return
	}

	bm.mu.RLock()
	onAlert := bm.onAlert
	bm.mu.RUnlock()

	if onAlert == nil {
		return
	}

	// 检查是否需要更换
	if health.ReplaceRecommended {
		message := fmt.Sprintf("⚠️ UPS %s 电池建议更换 (状况: %s, 健康评分: %d, 建议更换日期: %s)",
			upsID, health.Condition, health.HealthScore,
			health.ReplaceDeadline.Format("2006-01-02"))
		log.Println(message)
		onAlert(message, health)
	}

	// 检查测试失败
	if health.LastTestResult == TestFailed {
		message := fmt.Sprintf("❌ UPS %s 电池测试失败，请立即检查", upsID)
		log.Println(message)
		onAlert(message, health)
	}
}

// String 返回电池管理器摘要
func (bm *BatteryManager) String() string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return fmt.Sprintf("BatteryManager[batteries=%d, running=%v]",
		len(bm.healthMap), bm.running)
}
