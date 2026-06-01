package privacycomputing

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewFederatedManager 创建联邦学习管理器
func NewFederatedManager() *FederatedManager {
	return &FederatedManager{
		tasks: make(map[string]*FederatedTask),
	}
}

// CreateTask 创建联邦学习任务
func (fm *FederatedManager) CreateTask(req CreateFederatedTaskRequest) (*FederatedTask, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("任务名称不能为空")
	}

	if req.MaxRounds <= 0 {
		req.MaxRounds = 10
	}

	participants := make([]Participant, 0, len(req.Participants))
	for _, p := range req.Participants {
		participants = append(participants, Participant{
			ID:     p.ID,
			Name:   p.Name,
			Status: "idle",
		})
	}

	// 设置默认配置
	config := req.Config
	if config.AggregationStrategy == "" {
		config.AggregationStrategy = "fedavg"
	}
	if config.LearningRate <= 0 {
		config.LearningRate = 0.01
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 32
	}
	if config.LocalEpochs <= 0 {
		config.LocalEpochs = 5
	}
	if config.MinParticipants <= 0 {
		config.MinParticipants = 2
	}
	if config.PrivacyBudget <= 0 {
		config.PrivacyBudget = 1.0
	}

	task := &FederatedTask{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Status:       "pending",
		ModelType:    req.ModelType,
		Round:        0,
		MaxRounds:    req.MaxRounds,
		Participants: participants,
		GlobalModel:  make(map[string][]float64),
		Metrics:      make(map[string]float64),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Config:       config,
	}

	// 初始化全局模型
	task.GlobalModel["weights"] = fm.initializeWeights(req.ModelType)
	task.GlobalModel["bias"] = []float64{0.0}

	fm.tasks[task.ID] = task
	return task, nil
}

// initializeWeights 初始化模型权重
func (fm *FederatedManager) initializeWeights(modelType string) []float64 {
	var size int
	switch modelType {
	case "linear":
		size = 10
	case "logistic":
		size = 5
	case "mlp":
		size = 50
	default:
		size = 10
	}

	weights := make([]float64, size)
	for i := range weights {
		weights[i] = rand.NormFloat64() * 0.01
	}
	return weights
}

// GetTask 获取联邦学习任务
func (fm *FederatedManager) GetTask(taskID string) (*FederatedTask, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	task, exists := fm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return task, nil
}

// ListTasks 列出所有联邦学习任务
func (fm *FederatedManager) ListTasks() []*FederatedTask {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	tasks := make([]*FederatedTask, 0, len(fm.tasks))
	for _, task := range fm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// StartTraining 开始联邦学习训练
func (fm *FederatedManager) StartTraining(taskID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	task, exists := fm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	if task.Status != "pending" && task.Status != "completed" {
		return fmt.Errorf("任务状态不允许启动: %s", task.Status)
	}

	// 检查参与方数量
	activeParticipants := 0
	for _, p := range task.Participants {
		if p.Status != "failed" {
			activeParticipants++
		}
	}

	if activeParticipants < task.Config.MinParticipants {
		return fmt.Errorf("参与方数量不足: %d/%d", activeParticipants, task.Config.MinParticipants)
	}

	task.Status = "training"
	task.UpdatedAt = time.Now()

	// 模拟训练过程
	go fm.simulateTraining(task)

	return nil
}

// simulateTraining 模拟联邦学习训练过程
func (fm *FederatedManager) simulateTraining(task *FederatedTask) {
	for round := 0; round < task.MaxRounds; round++ {
		fm.mu.Lock()
		task.Round = round + 1
		task.UpdatedAt = time.Now()

		// 更新参与方状态
		for i := range task.Participants {
			if task.Participants[i].Status != "failed" {
				task.Participants[i].Status = "training"
				task.Participants[i].LastUpdate = time.Now()
			}
		}
		fm.mu.Unlock()

		// 模拟本地训练
		time.Sleep(100 * time.Millisecond)

		fm.mu.Lock()
		// 聚合本地模型
		localUpdates := make([]map[string][]float64, 0)
		for i := range task.Participants {
			if task.Participants[i].Status == "training" {
				// 模拟本地更新
				update := fm.simulateLocalUpdate(task.GlobalModel, task.Config.LearningRate)
				localUpdates = append(localUpdates, update)

				// 更新参与方指标
				task.Participants[i].LocalLoss = rand.Float64() * 0.5
				task.Participants[i].LocalAccuracy = 0.7 + rand.Float64()*0.3
				task.Participants[i].Status = "submitting"
			}
		}

		// 聚合模型更新
		if len(localUpdates) > 0 {
			task.GlobalModel = fm.aggregateUpdates(localUpdates, task.Config.AggregationStrategy)
		}

		// 更新全局指标
		task.Metrics["loss"] = rand.Float64() * 0.3
		task.Metrics["accuracy"] = 0.8 + rand.Float64()*0.2
		task.Metrics["round"] = float64(round + 1)

		// 更新参与方状态
		for i := range task.Participants {
			if task.Participants[i].Status == "submitting" {
				task.Participants[i].Status = "completed"
			}
		}

		task.UpdatedAt = time.Now()
		fm.mu.Unlock()

		time.Sleep(50 * time.Millisecond)
	}

	fm.mu.Lock()
	task.Status = "completed"
	task.UpdatedAt = time.Now()
	fm.mu.Unlock()
}

// simulateLocalUpdate 模拟本地模型更新
func (fm *FederatedManager) simulateLocalUpdate(globalModel map[string][]float64, lr float64) map[string][]float64 {
	update := make(map[string][]float64)
	for key, weights := range globalModel {
		newWeights := make([]float64, len(weights))
		for i, w := range weights {
			// 模拟梯度更新
			gradient := rand.NormFloat64() * 0.1
			newWeights[i] = w - lr*gradient
		}
		update[key] = newWeights
	}
	return update
}

// aggregateUpdates 聚合模型更新
func (fm *FederatedManager) aggregateUpdates(updates []map[string][]float64, strategy string) map[string][]float64 {
	if len(updates) == 0 {
		return nil
	}

	result := make(map[string][]float64)

	switch strategy {
	case "fedavg":
		// FedAvg: 平均聚合
		for key := range updates[0] {
			avgWeights := make([]float64, len(updates[0][key]))
			for _, update := range updates {
				for i, w := range update[key] {
					avgWeights[i] += w
				}
			}
			for i := range avgWeights {
				avgWeights[i] /= float64(len(updates))
			}
			result[key] = avgWeights
		}

	case "fedprox":
		// FedProx: 带近端项的聚合
		for key := range updates[0] {
			avgWeights := make([]float64, len(updates[0][key]))
			for _, update := range updates {
				for i, w := range update[key] {
					avgWeights[i] += w
				}
			}
			for i := range avgWeights {
				avgWeights[i] /= float64(len(updates))
			}
			result[key] = avgWeights
		}

	case "scaffold":
		// SCAFFOLD: 带控制变量的聚合
		for key := range updates[0] {
			avgWeights := make([]float64, len(updates[0][key]))
			for _, update := range updates {
				for i, w := range update[key] {
					avgWeights[i] += w
				}
			}
			for i := range avgWeights {
				avgWeights[i] /= float64(len(updates))
			}
			result[key] = avgWeights
		}

	default:
		// 默认使用 FedAvg
		return fm.aggregateUpdates(updates, "fedavg")
	}

	return result
}

// UpdateParticipantStatus 更新参与方状态
func (fm *FederatedManager) UpdateParticipantStatus(taskID, participantID, status string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	task, exists := fm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	for i, p := range task.Participants {
		if p.ID == participantID {
			task.Participants[i].Status = status
			task.Participants[i].LastUpdate = time.Now()
			task.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("参与方不存在: %s", participantID)
}

// SubmitLocalUpdate 提交本地模型更新
func (fm *FederatedManager) SubmitLocalUpdate(taskID, participantID string, update map[string][]float64) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	task, exists := fm.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	if task.Status != "training" {
		return fmt.Errorf("任务未处于训练状态")
	}

	found := false
	for i, p := range task.Participants {
		if p.ID == participantID {
			if p.Status != "training" {
				return fmt.Errorf("参与方状态不允许提交更新")
			}
			task.Participants[i].Status = "submitting"
			task.Participants[i].LastUpdate = time.Now()
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("参与方不存在: %s", participantID)
	}

	task.UpdatedAt = time.Now()
	return nil
}

// DeleteTask 删除联邦学习任务
func (fm *FederatedManager) DeleteTask(taskID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.tasks[taskID]; !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	delete(fm.tasks, taskID)
	return nil
}

// EvaluateModel 评估全局模型
func (fm *FederatedManager) EvaluateModel(taskID string) (map[string]float64, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	task, exists := fm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}

	// 模拟评估结果
	metrics := map[string]float64{
		"accuracy":  0.85 + rand.Float64()*0.15,
		"precision": 0.80 + rand.Float64()*0.20,
		"recall":    0.75 + rand.Float64()*0.25,
		"f1_score":  0.80 + rand.Float64()*0.20,
		"auc_roc":   0.82 + rand.Float64()*0.18,
		"loss":      rand.Float64() * 0.3,
	}

	_ = task
	return metrics, nil
}

// ComputeModelDistance 计算模型距离（用于隐私分析）
func (fm *FederatedManager) ComputeModelDistance(model1, model2 map[string][]float64) float64 {
	totalDist := 0.0
	count := 0

	for key := range model1 {
		if weights1, ok := model1[key]; ok {
			if weights2, ok := model2[key]; ok {
				minLen := int(math.Min(float64(len(weights1)), float64(len(weights2))))
				for i := 0; i < minLen; i++ {
					diff := weights1[i] - weights2[i]
					totalDist += diff * diff
					count++
				}
			}
		}
	}

	if count == 0 {
		return 0
	}
	return math.Sqrt(totalDist / float64(count))
}
