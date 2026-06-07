package federatedlearn

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeOnline      NodeStatus = "online"
	NodeOffline     NodeStatus = "offline"
	NodeTraining    NodeStatus = "training"
	NodeAggregating NodeStatus = "aggregating"
)

// ModelType 模型类型
type ModelType string

const (
	ModelClassification ModelType = "classification"
	ModelRegression     ModelType = "regression"
	ModelClustering     ModelType = "clustering"
	ModelAnomaly        ModelType = "anomaly_detection"
)

// AggregationStrategy 聚合策略
type AggregationStrategy string

const (
	StrategyFedAvg  AggregationStrategy = "fedavg"  // 联邦平均
	StrategyFedProx AggregationStrategy = "fedprox" // 近端联邦
	StrategyFedNova AggregationStrategy = "fednova" // 新星联邦
)

// FederatedNode 联邦学习节点
type FederatedNode struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Address      string     `json:"address"`
	Status       NodeStatus `json:"status"`
	DataSize     int        `json:"data_size"`
	ComputePower float64    `json:"compute_power"` // FLOPS
	LastSeen     time.Time  `json:"last_seen"`
	CreatedAt    time.Time  `json:"created_at"`
	Tags         []string   `json:"tags"`
}

// GlobalModel 全局模型
type GlobalModel struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Type      ModelType           `json:"type"`
	Version   int                 `json:"version"`
	Weights   []float64           `json:"weights"`
	Metrics   *ModelMetrics       `json:"metrics"`
	Strategy  AggregationStrategy `json:"strategy"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// LocalModel 本地模型
type LocalModel struct {
	ID        string        `json:"id"`
	NodeID    string        `json:"node_id"`
	GlobalID  string        `json:"global_id"`
	Version   int           `json:"version"`
	Weights   []float64     `json:"weights"`
	Metrics   *ModelMetrics `json:"metrics"`
	DataSize  int           `json:"data_size"`
	Epochs    int           `json:"epochs"`
	CreatedAt time.Time     `json:"created_at"`
}

// ModelMetrics 模型指标
type ModelMetrics struct {
	Loss      float64 `json:"loss"`
	Accuracy  float64 `json:"accuracy"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1Score   float64 `json:"f1_score"`
	RMSE      float64 `json:"rmse,omitempty"`
	R2        float64 `json:"r2,omitempty"`
}

// TrainingTask 训练任务
type TrainingTask struct {
	ID           string    `json:"id"`
	GlobalID     string    `json:"global_id"`
	NodeID       string    `json:"node_id"`
	Status       string    `json:"status"` // pending, running, completed, failed
	Epochs       int       `json:"epochs"`
	BatchSize    int       `json:"batch_size"`
	LearningRate float64   `json:"learning_rate"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Error        string    `json:"error,omitempty"`
}

// TrainingRound 训练轮次
type TrainingRound struct {
	ID                 string        `json:"id"`
	GlobalID           string        `json:"global_id"`
	Round              int           `json:"round"`
	Status             string        `json:"status"` // collecting, aggregating, completed
	ParticipatingNodes []string      `json:"participating_nodes"`
	LocalModels        []LocalModel  `json:"local_models"`
	GlobalMetrics      *ModelMetrics `json:"global_metrics"`
	StartedAt          time.Time     `json:"started_at"`
	CompletedAt        time.Time     `json:"completed_at"`
}

// Service 联邦学习服务
type Service struct {
	nodes  map[string]*FederatedNode
	models map[string]*GlobalModel
	locals map[string]*LocalModel
	tasks  map[string]*TrainingTask
	rounds map[string]*TrainingRound
	mu     sync.RWMutex
}

// NewService 创建服务
func NewService() *Service {
	return &Service{
		nodes:  make(map[string]*FederatedNode),
		models: make(map[string]*GlobalModel),
		locals: make(map[string]*LocalModel),
		tasks:  make(map[string]*TrainingTask),
		rounds: make(map[string]*TrainingRound),
	}
}

// RegisterNode 注册节点
func (s *Service) RegisterNode(ctx context.Context, node *FederatedNode) (*FederatedNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node.ID = fmt.Sprintf("node_%d", time.Now().UnixNano())
	node.Status = NodeOnline
	node.LastSeen = time.Now()
	node.CreatedAt = time.Now()

	s.nodes[node.ID] = node
	return node, nil
}

// GetNode 获取节点
func (s *Service) GetNode(id string) (*FederatedNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return node, nil
}

// ListNodes 列出节点
func (s *Service) ListNodes(status NodeStatus) []*FederatedNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*FederatedNode
	for _, node := range s.nodes {
		if status == "" || node.Status == status {
			result = append(result, node)
		}
	}
	return result
}

// CreateModel 创建全局模型
func (s *Service) CreateModel(ctx context.Context, model *GlobalModel) (*GlobalModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	model.ID = fmt.Sprintf("model_%d", time.Now().UnixNano())
	model.Version = 1
	model.CreatedAt = time.Now()
	model.UpdatedAt = time.Now()

	// 初始化权重
	if len(model.Weights) == 0 {
		model.Weights = make([]float64, 10)
		for i := range model.Weights {
			model.Weights[i] = rand.NormFloat64()
		}
	}

	model.Metrics = &ModelMetrics{}
	s.models[model.ID] = model

	return model, nil
}

// GetModel 获取模型
func (s *Service) GetModel(id string) (*GlobalModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	model, ok := s.models[id]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", id)
	}
	return model, nil
}

// ListModels 列出模型
func (s *Service) ListModels(modelType ModelType) []*GlobalModel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*GlobalModel
	for _, model := range s.models {
		if modelType == "" || model.Type == modelType {
			result = append(result, model)
		}
	}
	return result
}

// StartTraining 启动训练
func (s *Service) StartTraining(ctx context.Context, globalID string, epochs int, batchSize int, learningRate float64) (*TrainingRound, error) {
	s.mu.Lock()
	model, ok := s.models[globalID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("model not found: %s", globalID)
	}

	// 选择参与节点
	var participatingNodes []string
	for _, node := range s.nodes {
		if node.Status == NodeOnline {
			participatingNodes = append(participatingNodes, node.ID)
			node.Status = NodeTraining
		}
	}

	if len(participatingNodes) == 0 {
		s.mu.Unlock()
		return nil, fmt.Errorf("no available nodes")
	}

	round := &TrainingRound{
		ID:                 fmt.Sprintf("round_%d", time.Now().UnixNano()),
		GlobalID:           globalID,
		Round:              model.Version,
		Status:             "collecting",
		ParticipatingNodes: participatingNodes,
		LocalModels:        make([]LocalModel, 0),
		StartedAt:          time.Now(),
	}
	s.rounds[round.ID] = round
	s.mu.Unlock()

	// 异步执行训练
	go s.executeTrainingRound(round, epochs, batchSize, learningRate)

	return round, nil
}

// executeTrainingRound 执行训练轮次
func (s *Service) executeTrainingRound(round *TrainingRound, epochs int, batchSize int, learningRate float64) {
	var wg sync.WaitGroup
	localModels := make(chan LocalModel, len(round.ParticipatingNodes))

	// 分发训练任务到各节点
	for _, nodeID := range round.ParticipatingNodes {
		wg.Add(1)
		go func(nID string) {
			defer wg.Done()
			localModel := s.trainLocalModel(nID, round.GlobalID, epochs, batchSize, learningRate)
			if localModel != nil {
				localModels <- *localModel
			}
		}(nodeID)
	}

	// 等待所有节点完成
	go func() {
		wg.Wait()
		close(localModels)
	}()

	// 收集本地模型
	for model := range localModels {
		s.mu.Lock()
		round.LocalModels = append(round.LocalModels, model)
		s.mu.Unlock()
	}

	// 聚合模型
	s.mu.Lock()
	round.Status = "aggregating"
	s.mu.Unlock()

	s.aggregateModels(round)
}

// trainLocalModel 本地训练
func (s *Service) trainLocalModel(nodeID string, globalID string, epochs int, batchSize int, learningRate float64) *LocalModel {
	s.mu.RLock()
	node, ok := s.nodes[nodeID]
	model, modelOk := s.models[globalID]
	if !ok || !modelOk {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	// 模拟训练过程
	time.Sleep(time.Duration(epochs) * time.Second)

	// 模拟训练结果
	weights := make([]float64, len(model.Weights))
	for i := range weights {
		weights[i] = model.Weights[i] + rand.NormFloat64()*learningRate
	}

	localModel := &LocalModel{
		ID:        fmt.Sprintf("local_%d_%s", time.Now().UnixNano(), nodeID),
		NodeID:    nodeID,
		GlobalID:  globalID,
		Version:   model.Version,
		Weights:   weights,
		DataSize:  node.DataSize,
		Epochs:    epochs,
		CreatedAt: time.Now(),
		Metrics: &ModelMetrics{
			Loss:     0.5 + rand.Float64()*0.3,
			Accuracy: 0.7 + rand.Float64()*0.2,
		},
	}

	s.mu.Lock()
	s.locals[localModel.ID] = localModel
	node.Status = NodeOnline
	s.mu.Unlock()

	return localModel
}

// aggregateModels 聚合模型
func (s *Service) aggregateModels(round *TrainingRound) {
	s.mu.Lock()
	defer s.mu.Unlock()

	model, ok := s.models[round.GlobalID]
	if !ok || len(round.LocalModels) == 0 {
		return
	}

	// 联邦平均算法
	newWeights := make([]float64, len(model.Weights))
	totalDataSize := 0

	for _, local := range round.LocalModels {
		totalDataSize += local.DataSize
	}

	for _, local := range round.LocalModels {
		weight := float64(local.DataSize) / float64(totalDataSize)
		for i := range newWeights {
			if i < len(local.Weights) {
				newWeights[i] += local.Weights[i] * weight
			}
		}
	}

	model.Weights = newWeights
	model.Version++
	model.UpdatedAt = time.Now()

	// 计算聚合后的指标
	avgLoss := 0.0
	avgAccuracy := 0.0
	for _, local := range round.LocalModels {
		if local.Metrics != nil {
			avgLoss += local.Metrics.Loss
			avgAccuracy += local.Metrics.Accuracy
		}
	}
	avgLoss /= float64(len(round.LocalModels))
	avgAccuracy /= float64(len(round.LocalModels))

	model.Metrics = &ModelMetrics{
		Loss:     avgLoss,
		Accuracy: avgAccuracy,
	}

	round.GlobalMetrics = model.Metrics
	round.Status = "completed"
	round.CompletedAt = time.Now()
}

// GetRound 获取训练轮次
func (s *Service) GetRound(id string) (*TrainingRound, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	round, ok := s.rounds[id]
	if !ok {
		return nil, fmt.Errorf("round not found: %s", id)
	}
	return round, nil
}

// ListRounds 列出训练轮次
func (s *Service) ListRounds(globalID string) []*TrainingRound {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TrainingRound
	for _, round := range s.rounds {
		if globalID == "" || round.GlobalID == globalID {
			result = append(result, round)
		}
	}
	return result
}

// Predict 预测
func (s *Service) Predict(ctx context.Context, modelID string, input []float64) (map[string]interface{}, error) {
	s.mu.RLock()
	model, ok := s.models[modelID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	s.mu.RUnlock()

	// 简化的预测逻辑
	output := make([]float64, len(input))
	for i, x := range input {
		if i < len(model.Weights) {
			output[i] = x*model.Weights[i] + rand.NormFloat64()*0.1
		}
	}

	// 计算预测置信度
	confidence := 0.5 + model.Metrics.Accuracy*0.5

	return map[string]interface{}{
		"output":        output,
		"confidence":    confidence,
		"model_version": model.Version,
	}, nil
}

// GetStatistics 获取统计信息
func (s *Service) GetStatistics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	onlineNodes := 0
	totalDataSize := 0
	for _, node := range s.nodes {
		if node.Status == NodeOnline {
			onlineNodes++
		}
		totalDataSize += node.DataSize
	}

	return map[string]interface{}{
		"total_nodes":     len(s.nodes),
		"online_nodes":    onlineNodes,
		"total_models":    len(s.models),
		"total_rounds":    len(s.rounds),
		"total_data_size": totalDataSize,
	}
}
