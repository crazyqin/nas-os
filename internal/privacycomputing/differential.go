package privacycomputing

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// NewDifferentialManager 创建差分隐私管理器
func NewDifferentialManager() *DifferentialManager {
	return &DifferentialManager{
		budget: &PrivacyBudget{
			TotalEpsilon:     1.0,
			UsedEpsilon:      0,
			RemainingEpsilon: 1.0,
			TotalDelta:       1e-5,
			UsedDelta:        0,
			Queries:          make([]QueryLog, 0),
			LastUpdated:      time.Now(),
		},
		config: DifferentialPrivacyConfig{
			Epsilon:         1.0,
			Delta:           1e-5,
			Mechanism:       "laplace",
			Sensitivity:     1.0,
			NoiseMultiplier: 1.0,
			ClippingNorm:    1.0,
		},
	}
}

// SetConfig 设置差分隐私配置
func (dm *DifferentialManager) SetConfig(config DifferentialPrivacyConfig) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if config.Epsilon <= 0 {
		return fmt.Errorf("epsilon必须大于0")
	}
	if config.Delta < 0 {
		return fmt.Errorf("delta必须大于等于0")
	}
	if config.Sensitivity <= 0 {
		return fmt.Errorf("sensitivity必须大于0")
	}

	validMechanisms := map[string]bool{
		"laplace":    true,
		"gaussian":   true,
		"exponential": true,
	}
	if !validMechanisms[config.Mechanism] {
		return fmt.Errorf("不支持的机制: %s", config.Mechanism)
	}

	dm.config = config
	return nil
}

// GetConfig 获取差分隐私配置
func (dm *DifferentialManager) GetConfig() DifferentialPrivacyConfig {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.config
}

// SetBudget 设置隐私预算
func (dm *DifferentialManager) SetBudget(epsilon, delta float64) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if epsilon <= 0 {
		return fmt.Errorf("epsilon必须大于0")
	}
	if delta < 0 {
		return fmt.Errorf("delta必须大于等于0")
	}

	dm.budget.TotalEpsilon = epsilon
	dm.budget.RemainingEpsilon = epsilon - dm.budget.UsedEpsilon
	dm.budget.TotalDelta = delta
	dm.budget.LastUpdated = time.Now()

	return nil
}

// GetBudget 获取隐私预算
func (dm *DifferentialManager) GetBudget() *PrivacyBudget {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// 返回副本
	budgetCopy := *dm.budget
	budgetCopy.Queries = make([]QueryLog, len(dm.budget.Queries))
	copy(budgetCopy.Queries, dm.budget.Queries)
	return &budgetCopy
}

// AddNoise 添加差分隐私噪声
func (dm *DifferentialManager) AddNoise(req AddNoiseRequest) (*AddNoiseResponse, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if len(req.Data) == 0 {
		return nil, fmt.Errorf("数据不能为空")
	}

	// 使用请求中的配置或默认配置
	config := req.Config
	if config.Epsilon == 0 {
		config = dm.config
	}

	// 检查隐私预算
	epsilonUsed := config.Epsilon
	if dm.budget.RemainingEpsilon < epsilonUsed {
		return nil, fmt.Errorf("隐私预算不足，剩余: %.4f, 需要: %.4f", dm.budget.RemainingEpsilon, epsilonUsed)
	}

	// 根据机制添加噪声
	var noisyData []float64
	var noiseScale float64

	switch config.Mechanism {
	case "laplace":
		noisyData, noiseScale = dm.addLaplaceNoise(req.Data, config)
	case "gaussian":
		noisyData, noiseScale = dm.addGaussianNoise(req.Data, config)
	case "exponential":
		noisyData, noiseScale = dm.addExponentialNoise(req.Data, config)
	default:
		return nil, fmt.Errorf("不支持的机制: %s", config.Mechanism)
	}

	// 更新隐私预算
	dm.budget.UsedEpsilon += epsilonUsed
	dm.budget.RemainingEpsilon = dm.budget.TotalEpsilon - dm.budget.UsedEpsilon
	dm.budget.LastUpdated = time.Now()

	// 记录查询日志
	queryLog := QueryLog{
		ID:        uuid.New().String(),
		QueryType: req.QueryType,
		Epsilon:   epsilonUsed,
		Delta:     config.Delta,
		Timestamp: time.Now(),
	}
	dm.budget.Queries = append(dm.budget.Queries, queryLog)

	return &AddNoiseResponse{
		NoisyData:   noisyData,
		NoiseScale:  noiseScale,
		EpsilonUsed: epsilonUsed,
		PrivacyLoss: epsilonUsed,
	}, nil
}

// addLaplaceNoise 添加拉普拉斯噪声
func (dm *DifferentialManager) addLaplaceNoise(data []float64, config DifferentialPrivacyConfig) ([]float64, float64) {
	// 噪声尺度 = sensitivity / epsilon
	scale := config.Sensitivity / config.Epsilon
	if config.NoiseMultiplier > 0 {
		scale *= config.NoiseMultiplier
	}

	noisyData := make([]float64, len(data))
	for i, v := range data {
		// 拉普拉斯分布采样
		noise := sampleLaplace(0, scale)
		noisyData[i] = v + noise
	}

	return noisyData, scale
}

// addGaussianNoise 添加高斯噪声
func (dm *DifferentialManager) addGaussianNoise(data []float64, config DifferentialPrivacyConfig) ([]float64, float64) {
	// 高斯噪声标准差 = sensitivity * sqrt(2 * ln(1.25/delta)) / epsilon
	delta := config.Delta
	if delta == 0 {
		delta = 1e-5
	}

	sigma := config.Sensitivity * math.Sqrt(2*math.Log(1.25/delta)) / config.Epsilon
	if config.NoiseMultiplier > 0 {
		sigma *= config.NoiseMultiplier
	}

	noisyData := make([]float64, len(data))
	for i, v := range data {
		noise := rand.NormFloat64() * sigma
		noisyData[i] = v + noise
	}

	return noisyData, sigma
}

// addExponentialNoise 添加指数机制噪声
func (dm *DifferentialManager) addExponentialNoise(data []float64, config DifferentialPrivacyConfig) ([]float64, float64) {
	// 指数机制适用于离散值选择
	scale := 2.0 * config.Sensitivity / config.Epsilon
	if config.NoiseMultiplier > 0 {
		scale *= config.NoiseMultiplier
	}

	noisyData := make([]float64, len(data))
	for i, v := range data {
		// 使用Gumbel分布模拟指数机制
		noise := -math.Log(-math.Log(rand.Float64())) * scale
		noisyData[i] = v + noise
	}

	return noisyData, scale
}

// sampleLaplace 从拉普拉斯分布采样
func sampleLaplace(mu, b float64) float64 {
	u := rand.Float64() - 0.5
	if u < 0 {
		return mu + b*math.Log(1+2*u)
	}
	return mu - b*math.Log(1-2*u)
}

// ComputePrivacyLoss 计算隐私损失
func (dm *DifferentialManager) ComputePrivacyLoss(epsilon float64, nQueries int) float64 {
	// 组合定理：多次查询的隐私损失
	// 简单组合：epsilon_total = n * epsilon
	return float64(nQueries) * epsilon
}

// ComputeAdvancedComposition 计算高级组合隐私损失
func (dm *DifferentialManager) ComputeAdvancedComposition(epsilon, delta float64, nQueries int) float64 {
	// 高级组合定理
	// epsilon_total = sqrt(2 * n * log(1/delta)) * epsilon + n * epsilon * (exp(epsilon) - 1)
	term1 := math.Sqrt(2*float64(nQueries)*math.Log(1/delta)) * epsilon
	term2 := float64(nQueries) * epsilon * (math.Exp(epsilon) - 1)
	return term1 + term2
}

// ClipGradient 裁剪梯度（用于隐私保护的梯度下降）
func (dm *DifferentialManager) ClipGradient(gradient []float64, clippingNorm float64) []float64 {
	// 计算梯度范数
	norm := 0.0
	for _, g := range gradient {
		norm += g * g
	}
	norm = math.Sqrt(norm)

	// 如果范数超过阈值，进行裁剪
	if norm > clippingNorm {
		clipped := make([]float64, len(gradient))
		ratio := clippingNorm / norm
		for i, g := range gradient {
			clipped[i] = g * ratio
		}
		return clipped
	}

	return gradient
}

// PrivateMean 计算隐私保护的均值
func (dm *DifferentialManager) PrivateMean(data []float64, epsilon float64) (float64, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("数据不能为空")
	}

	// 计算真实均值
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	trueMean := sum / float64(len(data))

	// 添加噪声
	// 对于均值查询，灵敏度 = (max - min) / n
	// 简化处理，假设灵敏度为1
	sensitivity := 1.0 / float64(len(data))
	scale := sensitivity / epsilon
	noise := sampleLaplace(0, scale)

	return trueMean + noise, nil
}

// PrivateHistogram 计算隐私保护的直方图
func (dm *DifferentialManager) PrivateHistogram(data []int, nBins int, epsilon float64) ([]float64, error) {
	if nBins <= 0 {
		return nil, fmt.Errorf("bins数量必须大于0")
	}

	// 计算真实直方图
	histogram := make([]float64, nBins)
	for _, v := range data {
		if v >= 0 && v < nBins {
			histogram[v]++
		}
	}

	// 对每个bin添加噪声
	// 对于直方图查询，每个bin的灵敏度为1
	scale := 1.0 / epsilon
	for i := range histogram {
		noise := sampleLaplace(0, scale)
		histogram[i] += noise
		// 确保非负
		if histogram[i] < 0 {
			histogram[i] = 0
		}
	}

	return histogram, nil
}

// PrivateCount 计算隐私保护的计数
func (dm *DifferentialManager) PrivateCount(data []bool, epsilon float64) (float64, error) {
	// 计算真实计数
	count := 0
	for _, v := range data {
		if v {
			count++
		}
	}

	// 添加噪声
	// 对于计数查询，灵敏度为1
	scale := 1.0 / epsilon
	noise := sampleLaplace(0, scale)

	return float64(count) + noise, nil
}

// ResetBudget 重置隐私预算
func (dm *DifferentialManager) ResetBudget() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.budget.UsedEpsilon = 0
	dm.budget.RemainingEpsilon = dm.budget.TotalEpsilon
	dm.budget.UsedDelta = 0
	dm.budget.Queries = make([]QueryLog, 0)
	dm.budget.LastUpdated = time.Now()
}

// GetQueryLogs 获取查询日志
func (dm *DifferentialManager) GetQueryLogs() []QueryLog {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	logs := make([]QueryLog, len(dm.budget.Queries))
	copy(logs, dm.budget.Queries)
	return logs
}
