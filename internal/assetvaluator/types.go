package assetvaluator

import (
	"sync"
	"time"
)

// AssetType 资产类型.
type AssetType string

const (
	AssetPhoto    AssetType = "photo"
	AssetVideo    AssetType = "video"
	AssetDocument AssetType = "document"
	AssetMusic    AssetType = "music"
	AssetSoftware AssetType = "software"
	AssetCrypto   AssetType = "crypto"
	AssetNFT      AssetType = "nft"
	AssetOther    AssetType = "other"
)

// ValuationMethod 估值方法.
type ValuationMethod string

const (
	ValuationMarket    ValuationMethod = "market"
	ValuationCost      ValuationMethod = "cost"
	ValuationIncome    ValuationMethod = "income"
	ValuationSentiment ValuationMethod = "sentiment"
)

// DigitalAsset 数字资产.
type DigitalAsset struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       AssetType              `json:"type"`
	Path       string                 `json:"path"`
	SizeBytes  int64                  `json:"size_bytes"`
	CreatedAt  time.Time              `json:"created_at"`
	ModifiedAt time.Time              `json:"modified_at"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AssetValuation 资产估值.
type AssetValuation struct {
	ID         string          `json:"id"`
	AssetID    string          `json:"asset_id"`
	Method     ValuationMethod `json:"method"`
	ValueUSD   float64         `json:"value_usd"`
	Confidence float64         `json:"confidence"`
	Factors    []string        `json:"factors"`
	ValuedAt   time.Time       `json:"valued_at"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty"`
}

// InsurancePolicy 保险策略.
type InsurancePolicy struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	AssetTypes    []AssetType `json:"asset_types"`
	TotalValueUSD float64     `json:"total_value_usd"`
	CoverageUSD   float64     `json:"coverage_usd"`
	PremiumUSD    float64     `json:"premium_usd"`
	StartDate     time.Time   `json:"start_date"`
	EndDate       time.Time   `json:"end_date"`
	Status        string      `json:"status"`
}

// EstatePlan 遗产规划.
type EstatePlan struct {
	ID            string        `json:"id"`
	OwnerID       string        `json:"owner_id"`
	Beneficiaries []Beneficiary `json:"beneficiaries"`
	Assets        []string      `json:"assets"`
	TotalValue    float64       `json:"total_value"`
	Status        string        `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// Beneficiary 受益人.
type Beneficiary struct {
	UserID     string      `json:"user_id"`
	Name       string      `json:"name"`
	Share      float64     `json:"share"` // 0.0 to 1.0
	AssetTypes []AssetType `json:"asset_types,omitempty"`
}

// ValuationReport 估值报告.
type ValuationReport struct {
	ID            string           `json:"id"`
	TotalAssets   int              `json:"total_assets"`
	TotalValueUSD float64          `json:"total_value_usd"`
	ByType        []TypeValuation  `json:"by_type"`
	TopAssets     []AssetValuation `json:"top_assets"`
	Trend         string           `json:"trend"`
	GeneratedAt   time.Time        `json:"generated_at"`
}

// TypeValuation 类型估值.
type TypeValuation struct {
	Type     AssetType `json:"type"`
	Count    int       `json:"count"`
	ValueUSD float64   `json:"value_usd"`
	Percent  float64   `json:"percent"`
}

// ValuationFactors 估值因子配置.
type ValuationFactors struct {
	SentimentWeight float64 `json:"sentiment_weight"`
	RarityWeight    float64 `json:"rarity_weight"`
	AgeWeight       float64 `json:"age_weight"`
	QualityWeight   float64 `json:"quality_weight"`
	MarketWeight    float64 `json:"market_weight"`
}

// Manager 估值管理器.
type Manager struct {
	mu         sync.RWMutex
	assets     map[string]*DigitalAsset
	valuations map[string]*AssetValuation
	policies   []*InsurancePolicy
	estates    []*EstatePlan
	factors    *ValuationFactors
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		assets:     make(map[string]*DigitalAsset),
		valuations: make(map[string]*AssetValuation),
		policies:   make([]*InsurancePolicy, 0),
		estates:    make([]*EstatePlan, 0),
		factors: &ValuationFactors{
			SentimentWeight: 0.3,
			RarityWeight:    0.2,
			AgeWeight:       0.1,
			QualityWeight:   0.2,
			MarketWeight:    0.2,
		},
	}
}

// RegisterAsset 注册资产.
func (m *Manager) RegisterAsset(asset *DigitalAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	asset.CreatedAt = time.Now()
	m.assets[asset.ID] = asset
	return nil
}

// GetValue 获取资产估值.
func (m *Manager) GetValue(assetID string) (*AssetValuation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.valuations[assetID]
	return v, ok
}

// AddValuation 添加估值.
func (m *Manager) AddValuation(valuation *AssetValuation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	valuation.ValuedAt = time.Now()
	m.valuations[valuation.AssetID] = valuation
}

// CreatePolicy 创建保险策略.
func (m *Manager) CreatePolicy(policy *InsurancePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies = append(m.policies, policy)
}

// CreateEstatePlan 创建遗产规划.
func (m *Manager) CreateEstatePlan(plan *EstatePlan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()
	m.estates = append(m.estates, plan)
}

// GenerateReport 生成估值报告.
func (m *Manager) GenerateReport() *ValuationReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	report := &ValuationReport{
		ByType:      make([]TypeValuation, 0),
		TopAssets:   make([]AssetValuation, 0),
		GeneratedAt: time.Now(),
	}
	typeCount := make(map[AssetType]*TypeValuation)
	for _, v := range m.valuations {
		report.TotalAssets++
		report.TotalValueUSD += v.ValueUSD
		asset := m.assets[v.AssetID]
		if asset != nil {
			t, ok := typeCount[asset.Type]
			if !ok {
				t = &TypeValuation{Type: asset.Type}
				typeCount[asset.Type] = t
			}
			t.Count++
			t.ValueUSD += v.ValueUSD
		}
		report.TopAssets = append(report.TopAssets, *v)
	}
	for _, t := range typeCount {
		if report.TotalValueUSD > 0 {
			t.Percent = t.ValueUSD / report.TotalValueUSD * 100
		}
		report.ByType = append(report.ByType, *t)
	}
	return report
}

// GetStats 获取统计.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	totalValue := 0.0
	for _, v := range m.valuations {
		totalValue += v.ValueUSD
	}
	return map[string]interface{}{
		"total_assets":    len(m.assets),
		"total_valued":    len(m.valuations),
		"total_value_usd": totalValue,
		"active_policies": len(m.policies),
		"estate_plans":    len(m.estates),
	}
}
