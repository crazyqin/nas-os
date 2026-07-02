// Package diskhealthai2 - 维护建议引擎、磁盘组管理、Service 接口
package diskhealth

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ============================================================
// MaintenanceAdvisor - 维护建议引擎
// ============================================================

// MaintenanceAdvisor 维护建议引擎.
type MaintenanceAdvisor struct {
	mu        sync.RWMutex
	analyzer  *SMARTAnalyzer
	scoreSys  *HealthScoreSystem
	predictor *FailurePredictor
	groups    *DiskGroupManager
}

// NewMaintenanceAdvisor 创建维护建议引擎.
func NewMaintenanceAdvisor(analyzer *SMARTAnalyzer, scoreSys *HealthScoreSystem, predictor *FailurePredictor, groups *DiskGroupManager) *MaintenanceAdvisor {
	return &MaintenanceAdvisor{
		analyzer:  analyzer,
		scoreSys:  scoreSys,
		predictor: predictor,
		groups:    groups,
	}
}

// GenerateAdvice 生成维护建议.
func (m *MaintenanceAdvisor) GenerateAdvice() ([]MaintenanceAdvice, error) {
	devices := m.analyzer.GetDevices()
	if len(devices) == 0 {
		return nil, fmt.Errorf("无磁盘数据")
	}

	var advices []MaintenanceAdvice
	adviceID := 0

	for _, device := range devices {
		score, err := m.scoreSys.Calculate(device)
		if err != nil {
			continue
		}

		prediction, err := m.predictor.Predict(device)
		if err != nil {
			continue
		}

		data, err := m.analyzer.GetLatestData(device)
		if err != nil {
			continue
		}

		// 基于评分生成建议
		if score.Score < 30 {
			adviceID++
			advices = append(advices, MaintenanceAdvice{
				ID:            fmt.Sprintf("ADV-%d", adviceID),
				Device:        device,
				Title:         "立即更换磁盘",
				Description:   fmt.Sprintf("磁盘 %s 健康评分 %.1f（等级 %s），已进入危险区域。建议立即备份数据并更换磁盘。", device, score.Score, score.Grade),
				Priority:      PriorityUrgent,
				Category:      "更换",
				EstimatedCost: estimateReplacementCost(data),
				Urgency:       "立即执行",
				CreatedAt:     time.Now(),
			})
		} else if score.Score < 50 {
			adviceID++
			advices = append(advices, MaintenanceAdvice{
				ID:            fmt.Sprintf("ADV-%d", adviceID),
				Device:        device,
				Title:         "计划更换磁盘",
				Description:   fmt.Sprintf("磁盘 %s 健康评分 %.1f（等级 %s），建议在 30 天内安排更换。", device, score.Score, score.Grade),
				Priority:      PriorityHigh,
				Category:      "更换",
				EstimatedCost: estimateReplacementCost(data),
				Urgency:       "30天内",
				CreatedAt:     time.Now(),
			})
		}

		// 基于故障预测生成建议
		if prediction.FailureProbability > 0.5 {
			adviceID++
			advices = append(advices, MaintenanceAdvice{
				ID:            fmt.Sprintf("ADV-%d", adviceID),
				Device:        device,
				Title:         "故障概率较高",
				Description:   fmt.Sprintf("磁盘 %s 故障概率 %.1f%%，预计剩余 %d 天。建议加强监控并准备备用磁盘。", device, prediction.FailureProbability*100, prediction.EstimatedLifeDays),
				Priority:      PriorityHigh,
				Category:      "监控",
				EstimatedCost: 0,
				Urgency:       "尽快",
				CreatedAt:     time.Now(),
			})
		}

		// 基于 SMART 属性生成具体建议
		reallocated := getAttributeValue(data, SMARTIDReallocatedSectorCt)
		if reallocated > 0 {
			adviceID++
			advices = append(advices, MaintenanceAdvice{
				ID:            fmt.Sprintf("ADV-%d", adviceID),
				Device:        device,
				Title:         "坏扇区检测",
				Description:   fmt.Sprintf("磁盘 %s 检测到 %d 个重映射扇区。建议运行磁盘自检（smartctl -t long）确认状态。", device, reallocated),
				Priority:      PriorityMedium,
				Category:      "检测",
				EstimatedCost: 0,
				Urgency:       "一周内",
				CreatedAt:     time.Now(),
			})
		}

		temp := getAttributeValue(data, SMARTIDTemperatureCelsius)
		if temp > 55 {
			adviceID++
			advices = append(advices, MaintenanceAdvice{
				ID:            fmt.Sprintf("ADV-%d", adviceID),
				Device:        device,
				Title:         "温度过高",
				Description:   fmt.Sprintf("磁盘 %s 当前温度 %d℃，超过安全阈值（55℃）。建议检查散热系统。", device, temp),
				Priority:      PriorityMedium,
				Category:      "散热",
				EstimatedCost: 200, // 风扇更换成本
				Urgency:       "尽快",
				CreatedAt:     time.Now(),
			})
		}

		if data.IsSSD {
			wear := getAttributeValue(data, SMARTIDWearLevelingCount)
			if wear < 20 {
				adviceID++
				advices = append(advices, MaintenanceAdvice{
					ID:            fmt.Sprintf("ADV-%d", adviceID),
					Device:        device,
					Title:         "SSD 磨损严重",
					Description:   fmt.Sprintf("SSD %s 剩余寿命仅 %d%%，建议尽快更换。", device, wear),
					Priority:      PriorityUrgent,
					Category:      "更换",
					EstimatedCost: estimateReplacementCost(data),
					Urgency:       "立即",
					CreatedAt:     time.Now(),
				})
			}
		}
	}

	// 按优先级排序
	sort.Slice(advices, func(i, j int) bool {
		return advices[i].Priority < advices[j].Priority
	})

	return advices, nil
}

// GetAdviceForDevice 获取单设备建议.
func (m *MaintenanceAdvisor) GetAdviceForDevice(device string) ([]MaintenanceAdvice, error) {
	allAdvices, err := m.GenerateAdvice()
	if err != nil {
		return nil, err
	}

	var deviceAdvices []MaintenanceAdvice
	for _, advice := range allAdvices {
		if advice.Device == device {
			deviceAdvices = append(deviceAdvices, advice)
		}
	}
	return deviceAdvices, nil
}

// estimateReplacementCost 估算更换成本.
func estimateReplacementCost(data *SMARTData) float64 {
	capacityTB := float64(data.CapacityBytes) / 1e12

	if data.IsSSD {
		// SSD: 约 400-800 元/TB
		return capacityTB * 600
	}

	// HDD: 约 100-200 元/TB
	return capacityTB * 150
}

// ============================================================
// DiskGroupManager - 磁盘组管理
// ============================================================

// DiskGroupManager 磁盘组管理.
type DiskGroupManager struct {
	mu       sync.RWMutex
	analyzer *SMARTAnalyzer
	scoreSys *HealthScoreSystem
	groups   map[string]*DiskGroup
}

// NewDiskGroupManager 创建磁盘组管理器.
func NewDiskGroupManager(analyzer *SMARTAnalyzer, scoreSys *HealthScoreSystem) *DiskGroupManager {
	return &DiskGroupManager{
		analyzer: analyzer,
		scoreSys: scoreSys,
		groups:   make(map[string]*DiskGroup),
	}
}

// CreateGroup 创建磁盘组.
func (g *DiskGroupManager) CreateGroup(id, name, groupType string, disks []string) *DiskGroup {
	g.mu.Lock()
	defer g.mu.Unlock()

	group := &DiskGroup{
		ID:        id,
		Name:      name,
		Type:      groupType,
		Disks:     disks,
		CreatedAt: time.Now(),
	}
	g.groups[id] = group
	return group
}

// GetGroup 获取磁盘组.
func (g *DiskGroupManager) GetGroup(id string) (*DiskGroup, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	group, ok := g.groups[id]
	if !ok {
		return nil, fmt.Errorf("磁盘组 %s 不存在", id)
	}
	return group, nil
}

// ListGroups 列出所有磁盘组.
func (g *DiskGroupManager) ListGroups() []*DiskGroup {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var groups []*DiskGroup
	for _, group := range g.groups {
		groups = append(groups, group)
	}
	return groups
}

// EvaluateGroup 评估磁盘组健康状态.
func (g *DiskGroupManager) EvaluateGroup(id string) (*DiskGroup, error) {
	group, err := g.GetGroup(id)
	if err != nil {
		return nil, err
	}

	var totalScore float64
	var minScore float64 = 100
	var weakestDisk string
	var count int

	for _, device := range group.Disks {
		score, err := g.scoreSys.Calculate(device)
		if err != nil {
			continue
		}

		totalScore += score.Score
		count++

		if score.Score < minScore {
			minScore = score.Score
			weakestDisk = device
		}
	}

	if count == 0 {
		return group, nil
	}

	group.GroupScore = totalScore / float64(count)
	group.GroupGrade = scoreToGrade(group.GroupScore)
	group.GroupStatus = scoreToStatus(group.GroupScore)
	group.WeakestDisk = weakestDisk
	group.PriorityDisk = weakestDisk

	return group, nil
}

// EvaluateAllGroups 评估所有磁盘组.
func (g *DiskGroupManager) EvaluateAllGroups() []*DiskGroup {
	groups := g.ListGroups()

	var results []*DiskGroup
	for _, group := range groups {
		evaluated, err := g.EvaluateGroup(group.ID)
		if err != nil {
			continue
		}
		results = append(results, evaluated)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].GroupScore < results[j].GroupScore
	})

	return results
}

// RemoveGroup 删除磁盘组.
func (g *DiskGroupManager) RemoveGroup(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.groups[id]; !ok {
		return fmt.Errorf("磁盘组 %s 不存在", id)
	}
	delete(g.groups, id)
	return nil
}

// ============================================================
// DiskHealthService - 统一服务接口
// ============================================================

// DiskHealthService 磁盘健康分析统一服务.
type DiskHealthService struct {
	Analyzer  *SMARTAnalyzer
	ScoreSys  *HealthScoreSystem
	Predictor *FailurePredictor
	Advisor   *MaintenanceAdvisor
	GroupMgr  *DiskGroupManager
}

// NewDiskHealthService 创建统一服务.
func NewDiskHealthService(maxHistory int) *DiskHealthService {
	analyzer := NewSMARTAnalyzer(maxHistory)
	scoreSys := NewHealthScoreSystem(analyzer)
	predictor := NewFailurePredictor(analyzer, scoreSys)
	groupMgr := NewDiskGroupManager(analyzer, scoreSys)
	advisor := NewMaintenanceAdvisor(analyzer, scoreSys, predictor, groupMgr)

	return &DiskHealthService{
		Analyzer:  analyzer,
		ScoreSys:  scoreSys,
		Predictor: predictor,
		Advisor:   advisor,
		GroupMgr:  groupMgr,
	}
}

// ScanAllDisk 扫描所有磁盘（触发扫描）.
func (s *DiskHealthService) ScanAllDisk() (string, time.Time) {
	scanID := fmt.Sprintf("SCAN-%d", time.Now().Unix())
	return scanID, time.Now()
}
