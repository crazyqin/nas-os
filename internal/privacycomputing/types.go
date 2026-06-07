package privacycomputing

import (
	"sync"
	"time"
)

// PrivacyEngine 隐私计算引擎核心结构
type PrivacyEngine struct {
	mu              sync.RWMutex
	federatedMgr    *FederatedManager
	mpcMgr          *MPCManager
	differentialMgr *DifferentialManager
	maskingMgr      *MaskingManager
}

// FederatedTask 联邦学习任务
type FederatedTask struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Status       string               `json:"status"` // pending, training, aggregating, completed, failed
	ModelType    string               `json:"model_type"`
	Round        int                  `json:"round"`
	MaxRounds    int                  `json:"max_rounds"`
	Participants []Participant        `json:"participants"`
	GlobalModel  map[string][]float64 `json:"global_model"`
	Metrics      map[string]float64   `json:"metrics"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
	Config       FederatedConfig      `json:"config"`
}

// Participant 联邦学习参与方
type Participant struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"` // idle, training, submitting, completed
	DataSize      int       `json:"data_size"`
	LocalLoss     float64   `json:"local_loss"`
	LocalAccuracy float64   `json:"local_accuracy"`
	LastUpdate    time.Time `json:"last_update"`
}

// FederatedConfig 联邦学习配置
type FederatedConfig struct {
	AggregationStrategy string  `json:"aggregation_strategy"` // fedavg, fedprox, scaffold
	LearningRate        float64 `json:"learning_rate"`
	BatchSize           int     `json:"batch_size"`
	LocalEpochs         int     `json:"local_epochs"`
	MinParticipants     int     `json:"min_participants"`
	PrivacyBudget       float64 `json:"privacy_budget"`
	SecureAggregation   bool    `json:"secure_aggregation"`
}

// MPCProtocol 安全多方计算协议
type MPCProtocol struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`   // secret_sharing, garbled_circuit, homomorphic
	Status       string           `json:"status"` // idle, computing, completed, failed
	Participants []MPCParticipant `json:"participants"`
	Computation  string           `json:"computation"`
	Result       interface{}      `json:"result,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	CompletedAt  *time.Time       `json:"completed_at,omitempty"`
}

// MPCParticipant MPC参与方
type MPCParticipant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"` // dealer, compute, verifier
	Status    string `json:"status"`
	DataShare []byte `json:"data_share,omitempty"`
}

// SecretShare 秘密份额
type SecretShare struct {
	Index int    `json:"index"`
	Value []byte `json:"value"`
	Proof []byte `json:"proof,omitempty"`
	Party string `json:"party"`
}

// DifferentialPrivacyConfig 差分隐私配置
type DifferentialPrivacyConfig struct {
	Epsilon         float64 `json:"epsilon"`
	Delta           float64 `json:"delta"`
	Mechanism       string  `json:"mechanism"` // laplace, gaussian, exponential
	Sensitivity     float64 `json:"sensitivity"`
	NoiseMultiplier float64 `json:"noise_multiplier"`
	ClippingNorm    float64 `json:"clipping_norm"`
}

// PrivacyBudget 隐私预算
type PrivacyBudget struct {
	TotalEpsilon     float64    `json:"total_epsilon"`
	UsedEpsilon      float64    `json:"used_epsilon"`
	RemainingEpsilon float64    `json:"remaining_epsilon"`
	TotalDelta       float64    `json:"total_delta"`
	UsedDelta        float64    `json:"used_delta"`
	Queries          []QueryLog `json:"queries"`
	LastUpdated      time.Time  `json:"last_updated"`
}

// QueryLog 查询日志
type QueryLog struct {
	ID        string    `json:"id"`
	QueryType string    `json:"query_type"`
	Epsilon   float64   `json:"epsilon"`
	Delta     float64   `json:"delta"`
	Timestamp time.Time `json:"timestamp"`
}

// DataMaskRule 数据脱敏规则
type DataMaskRule struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"` // regex, column, row, cell
	Pattern   string                 `json:"pattern,omitempty"`
	Strategy  string                 `json:"strategy"` // mask, partial, hash, tokenize, pseudonymize
	Config    map[string]interface{} `json:"config"`
	Enabled   bool                   `json:"enabled"`
	CreatedAt time.Time              `json:"created_at"`
}

// MaskResult 脱敏结果
type MaskResult struct {
	OriginalCount int          `json:"original_count"`
	MaskedCount   int          `json:"masked_count"`
	RulesApplied  []string     `json:"rules_applied"`
	Details       []MaskDetail `json:"details"`
	ProcessedAt   time.Time    `json:"processed_at"`
}

// MaskDetail 脱敏详情
type MaskDetail struct {
	Field    string `json:"field"`
	Rule     string `json:"rule"`
	Original string `json:"original"`
	Masked   string `json:"masked"`
}

// FederatedManager 联邦学习管理器
type FederatedManager struct {
	mu    sync.RWMutex
	tasks map[string]*FederatedTask
}

// MPCManager 安全多方计算管理器
type MPCManager struct {
	mu        sync.RWMutex
	protocols map[string]*MPCProtocol
}

// DifferentialManager 差分隐私管理器
type DifferentialManager struct {
	mu     sync.RWMutex
	budget *PrivacyBudget
	config DifferentialPrivacyConfig
}

// MaskingManager 数据脱敏管理器
type MaskingManager struct {
	mu    sync.RWMutex
	rules map[string]*DataMaskRule
}

// API Request/Response Types

// CreateFederatedTaskRequest 创建联邦学习任务请求
type CreateFederatedTaskRequest struct {
	Name         string               `json:"name"`
	ModelType    string               `json:"model_type"`
	MaxRounds    int                  `json:"max_rounds"`
	Participants []ParticipantRequest `json:"participants"`
	Config       FederatedConfig      `json:"config"`
}

// ParticipantRequest 参与方请求
type ParticipantRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateMPCProtocolRequest 创建MPC协议请求
type CreateMPCProtocolRequest struct {
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	Participants []MPCParticipantReq `json:"participants"`
	Computation  string              `json:"computation"`
}

// MPCParticipantReq MPC参与方请求
type MPCParticipantReq struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// AddNoiseRequest 添加噪声请求
type AddNoiseRequest struct {
	Data      []float64                 `json:"data"`
	Config    DifferentialPrivacyConfig `json:"config"`
	QueryType string                    `json:"query_type"`
}

// AddNoiseResponse 添加噪声响应
type AddNoiseResponse struct {
	NoisyData   []float64 `json:"noisy_data"`
	NoiseScale  float64   `json:"noise_scale"`
	EpsilonUsed float64   `json:"epsilon_used"`
	PrivacyLoss float64   `json:"privacy_loss"`
}

// CreateMaskRuleRequest 创建脱敏规则请求
type CreateMaskRuleRequest struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Pattern  string                 `json:"pattern,omitempty"`
	Strategy string                 `json:"strategy"`
	Config   map[string]interface{} `json:"config"`
}

// ApplyMaskRequest 应用脱敏请求
type ApplyMaskRequest struct {
	Content string   `json:"content"`
	RuleIDs []string `json:"rule_ids,omitempty"`
}

// ApplyTableMaskRequest 表格脱敏请求
type ApplyTableMaskRequest struct {
	Table string                   `json:"table"`
	Data  []map[string]interface{} `json:"data"`
	Rules map[string]string        `json:"rules"` // column -> rule_id
}
