// Package cost provides cloud storage cost comparison and analysis.
package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CloudProvider represents a cloud storage provider.
type CloudProvider string

const (
	ProviderAliyunOSS  CloudProvider = "aliyun_oss"
	ProviderTencentCOS CloudProvider = "tencent_cos"
	ProviderAWSS3      CloudProvider = "aws_s3"
	ProviderAzureBlob  CloudProvider = "azure_blob"
	ProviderGCS        CloudProvider = "google_cloud_storage"
	ProviderOneDrive   CloudProvider = "onedrive"
)

// PricingTier represents storage pricing tier.
type PricingTier struct {
	Name           string  `json:"name"`             // e.g., "Standard", "Low-Frequency"
	StoragePrice   float64 `json:"storage_price"`    // CNY/GB/month
	DownloadPrice  float64 `json:"download_price"`   // CNY/GB
	UploadPrice    float64 `json:"upload_price"`     // CNY/GB (usually free)
	MinStorageDays int     `json:"min_storage_days"` // Minimum storage period
	Description    string  `json:"description"`
}

// CloudPricing represents pricing data for a cloud provider.
type CloudPricing struct {
	Provider    CloudProvider `json:"provider"`
	DisplayName string        `json:"display_name"`
	Region      string        `json:"region"` // e.g., "cn-east-1", "us-west-2"
	Tiers       []PricingTier `json:"tiers"`
	LastUpdated time.Time     `json:"last_updated"`
	Currency    string        `json:"currency"` // "CNY" or "USD"
}

// CostComparison represents a cost comparison result.
type CostComparison struct {
	GeneratedAt         time.Time      `json:"generated_at"`
	StorageSizeGB       float64        `json:"storage_size_gb"`
	MonthlyDownloadGB   float64        `json:"monthly_download_gb"`
	StorageDurationDays int            `json:"storage_duration_days"`
	Providers           []ProviderCost `json:"providers"`
	BestValue           *ProviderCost  `json:"best_value"`
	LocalCost           float64        `json:"local_cost"` // Local NAS storage cost
	Savings             float64        `json:"savings"`    // Savings vs cheapest cloud
}

// ProviderCost represents cost for a specific provider.
type ProviderCost struct {
	Provider            CloudProvider `json:"provider"`
	DisplayName         string        `json:"display_name"`
	TierName            string        `json:"tier_name"`
	MonthlyStorageCost  float64       `json:"monthly_storage_cost"`
	MonthlyTransferCost float64       `json:"monthly_transfer_cost"`
	TotalMonthlyCost    float64       `json:"total_month_cost"`
	TotalYearlyCost     float64       `json:"total_year_cost"`
	RecommendedTier     bool          `json:"recommended"`
}

// MigrationCost represents migration cost estimate.
type MigrationCost struct {
	DataSizeGB        float64       `json:"data_size_gb"`
	SourceProvider    CloudProvider `json:"source_provider"`
	TargetProvider    CloudProvider `json:"target_provider"`
	UploadCost        float64       `json:"upload_cost"`
	DownloadCost      float64       `json:"download_cost"`
	TransferTimeEst   time.Duration `json:"transfer_time_est"`
	TransferSpeedMbps int           `json:"transfer_speed_mbps"` // Assumed speed
	BreakEvenMonths   int           `json:"break_even_months"`   // Months to recover migration cost
}

// CostAnalyzerConfig holds cost analyzer configuration.
type CostAnalyzerConfig struct {
	LocalStorageCostPerGB     float64 `json:"local_storage_cost_per_gb"` // CNY/GB/month (hardware amortization)
	DefaultTransferSpeedMbps  int     `json:"default_transfer_speed_mbps"`
	PricingUpdateIntervalDays int     `json:"pricing_update_interval_days"`
	DataDir                   string  `json:"data_dir"`
}

// CloudCostAnalyzer provides cloud cost comparison analysis.
type CloudCostAnalyzer struct {
	mu         sync.RWMutex
	pricing    map[CloudProvider]*CloudPricing
	config     *CostAnalyzerConfig
	logger     *zap.Logger
	configPath string
}

// NewCloudCostAnalyzer creates a new cost analyzer.
func NewCloudCostAnalyzer(configPath string, logger *zap.Logger) (*CloudCostAnalyzer, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &CostAnalyzerConfig{
		LocalStorageCostPerGB:     0.05, // ~60 CNY/TB/year amortized
		DefaultTransferSpeedMbps:  100,  // 100 Mbps upload
		PricingUpdateIntervalDays: 30,
		DataDir:                   "/var/lib/nas-os/cost",
	}

	a := &CloudCostAnalyzer{
		pricing:    make(map[CloudProvider]*CloudPricing),
		config:     config,
		logger:     logger,
		configPath: configPath,
	}

	// Initialize default pricing data
	a.initDefaultPricing()

	if err := a.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return a, nil
}

// initDefaultPricing initializes default cloud pricing data (2026 estimates).
func (a *CloudCostAnalyzer) initDefaultPricing() {
	// Aliyun OSS pricing (CNY)
	a.pricing[ProviderAliyunOSS] = &CloudPricing{
		Provider:    ProviderAliyunOSS,
		DisplayName: "阿里云 OSS",
		Region:      "cn-east-1",
		Currency:    "CNY",
		LastUpdated: time.Now(),
		Tiers: []PricingTier{
			{Name: "标准存储", StoragePrice: 0.12, DownloadPrice: 0.50, UploadPrice: 0, MinStorageDays: 0, Description: "适合频繁访问"},
			{Name: "低频存储", StoragePrice: 0.08, DownloadPrice: 0.80, UploadPrice: 0, MinStorageDays: 30, Description: "适合月访问<12次"},
			{Name: "归档存储", StoragePrice: 0.033, DownloadPrice: 2.00, UploadPrice: 0, MinStorageDays: 60, Description: "适合长期归档"},
		},
	}

	// Tencent COS pricing (CNY)
	a.pricing[ProviderTencentCOS] = &CloudPricing{
		Provider:    ProviderTencentCOS,
		DisplayName: "腾讯云 COS",
		Region:      "ap-guangzhou",
		Currency:    "CNY",
		LastUpdated: time.Now(),
		Tiers: []PricingTier{
			{Name: "标准存储", StoragePrice: 0.118, DownloadPrice: 0.50, UploadPrice: 0, MinStorageDays: 0},
			{Name: "低频存储", StoragePrice: 0.08, DownloadPrice: 0.80, UploadPrice: 0, MinStorageDays: 30},
			{Name: "归档存储", StoragePrice: 0.033, DownloadPrice: 2.00, UploadPrice: 0, MinStorageDays: 60},
		},
	}

	// AWS S3 pricing (USD, converted to CNY at ~7.2)
	a.pricing[ProviderAWSS3] = &CloudPricing{
		Provider:    ProviderAWSS3,
		DisplayName: "AWS S3",
		Region:      "us-west-2",
		Currency:    "CNY",
		LastUpdated: time.Now(),
		Tiers: []PricingTier{
			{Name: "S3 Standard", StoragePrice: 0.1728, DownloadPrice: 7.20, UploadPrice: 0, MinStorageDays: 0}, // $0.023 * 7.2
			{Name: "S3 Standard-IA", StoragePrice: 0.072, DownloadPrice: 7.92, UploadPrice: 0, MinStorageDays: 30},
			{Name: "S3 Glacier", StoragePrice: 0.0288, DownloadPrice: 28.8, UploadPrice: 0, MinStorageDays: 90},
		},
	}

	// Azure Blob (CNY)
	a.pricing[ProviderAzureBlob] = &CloudPricing{
		Provider:    ProviderAzureBlob,
		DisplayName: "Azure Blob Storage",
		Region:      "eastasia",
		Currency:    "CNY",
		LastUpdated: time.Now(),
		Tiers: []PricingTier{
			{Name: "Hot", StoragePrice: 0.1536, DownloadPrice: 7.20, UploadPrice: 0},
			{Name: "Cool", StoragePrice: 0.072, DownloadPrice: 7.92, UploadPrice: 0, MinStorageDays: 30},
			{Name: "Archive", StoragePrice: 0.0144, DownloadPrice: 28.8, UploadPrice: 0, MinStorageDays: 180},
		},
	}

	// Google Cloud Storage (CNY)
	a.pricing[ProviderGCS] = &CloudPricing{
		Provider:    ProviderGCS,
		DisplayName: "Google Cloud Storage",
		Region:      "asia-east1",
		Currency:    "CNY",
		LastUpdated: time.Now(),
		Tiers: []PricingTier{
			{Name: "Standard", StoragePrice: 0.1728, DownloadPrice: 7.20, UploadPrice: 0},
			{Name: "Nearline", StoragePrice: 0.072, DownloadPrice: 7.92, UploadPrice: 0, MinStorageDays: 30},
			{Name: "Coldline", StoragePrice: 0.036, DownloadPrice: 28.8, UploadPrice: 0, MinStorageDays: 90},
		},
	}
}

// CompareCosts compares storage costs across cloud providers.
func (a *CloudCostAnalyzer) CompareCosts(ctx context.Context, storageGB, monthlyDownloadGB, durationDays int) (*CostComparison, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	comparison := &CostComparison{
		GeneratedAt:         time.Now(),
		StorageSizeGB:       float64(storageGB),
		MonthlyDownloadGB:   float64(monthlyDownloadGB),
		StorageDurationDays: durationDays,
		Providers:           []ProviderCost{},
	}

	// Calculate local storage cost
	comparison.LocalCost = float64(storageGB) * a.config.LocalStorageCostPerGB * float64(durationDays) / 30

	// Calculate costs for each provider
	for provider, pricing := range a.pricing {
		for _, tier := range pricing.Tiers {
			// Check minimum storage days constraint
			if tier.MinStorageDays > 0 && durationDays < tier.MinStorageDays {
				continue
			}

			monthlyStorage := float64(storageGB) * tier.StoragePrice
			monthlyTransfer := float64(monthlyDownloadGB) * tier.DownloadPrice
			totalMonthly := monthlyStorage + monthlyTransfer
			totalYearly := totalMonthly * 12

			providerCost := ProviderCost{
				Provider:            provider,
				DisplayName:         pricing.DisplayName,
				TierName:            tier.Name,
				MonthlyStorageCost:  monthlyStorage,
				MonthlyTransferCost: monthlyTransfer,
				TotalMonthlyCost:    totalMonthly,
				TotalYearlyCost:     totalYearly,
				RecommendedTier:     false,
			}

			// Recommend cheapest tier per provider
			if len(comparison.Providers) == 0 ||
				provider != comparison.Providers[len(comparison.Providers)-1].Provider {
				providerCost.RecommendedTier = true
			}

			comparison.Providers = append(comparison.Providers, providerCost)
		}
	}

	// Find best value
	if len(comparison.Providers) > 0 {
		bestIdx := 0
		for i, pc := range comparison.Providers {
			if pc.TotalMonthlyCost < comparison.Providers[bestIdx].TotalMonthlyCost {
				bestIdx = i
			}
		}
		comparison.BestValue = &comparison.Providers[bestIdx]
		comparison.Savings = comparison.LocalCost - comparison.BestValue.TotalMonthlyCost*float64(durationDays)/30
	}

	a.logger.Info("Cost comparison completed",
		zap.Float64("storage_gb", float64(storageGB)),
		zap.Int("providers", len(comparison.Providers)))

	return comparison, nil
}

// EstimateMigrationCost estimates migration cost between providers.
func (a *CloudCostAnalyzer) EstimateMigrationCost(ctx context.Context, dataSizeGB float64, source, target CloudProvider) (*MigrationCost, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	sourcePricing := a.pricing[source]
	targetPricing := a.pricing[target]

	if sourcePricing == nil || targetPricing == nil {
		return nil, fmt.Errorf("pricing data not available for provider")
	}

	// Get standard tier pricing (assume migration uses standard tier)
	sourceDownloadPrice := 0.50 // Default
	targetUploadPrice := 0.0    // Usually free

	for _, tier := range sourcePricing.Tiers {
		if tier.Name == "标准存储" || tier.Name == "Standard" || tier.Name == "Hot" {
			sourceDownloadPrice = tier.DownloadPrice
			break
		}
	}

	migration := &MigrationCost{
		DataSizeGB:        dataSizeGB,
		SourceProvider:    source,
		TargetProvider:    target,
		DownloadCost:      dataSizeGB * sourceDownloadPrice,
		UploadCost:        dataSizeGB * targetUploadPrice,
		TransferSpeedMbps: a.config.DefaultTransferSpeedMbps,
	}

	// Estimate transfer time
	// Mbps = Megabits per second, convert to MB/s = Mbps / 8
	bytesPerSecond := float64(a.config.DefaultTransferSpeedMbps) * 1000000 / 8
	totalBytes := dataSizeGB * 1000000000
	seconds := totalBytes / bytesPerSecond
	migration.TransferTimeEst = time.Duration(seconds) * time.Second

	// Calculate break-even point
	// Assume target has cheaper storage
	targetStoragePrice := 0.12
	for _, tier := range targetPricing.Tiers {
		if tier.StoragePrice < targetStoragePrice {
			targetStoragePrice = tier.StoragePrice
		}
	}
	sourceStoragePrice := 0.12
	for _, tier := range sourcePricing.Tiers {
		if tier.StoragePrice < sourceStoragePrice {
			sourceStoragePrice = tier.StoragePrice
		}
	}

	monthlySavings := (sourceStoragePrice - targetStoragePrice) * dataSizeGB
	if monthlySavings > 0 && migration.DownloadCost > 0 {
		migration.BreakEvenMonths = int(migration.DownloadCost / monthlySavings)
	}

	a.logger.Info("Migration cost estimated",
		zap.Float64("data_size_gb", dataSizeGB),
		zap.String("source", string(source)),
		zap.String("target", string(target)),
		zap.Float64("cost", migration.DownloadCost+migration.UploadCost))

	return migration, nil
}

// UpdatePricing updates pricing data for a provider.
func (a *CloudCostAnalyzer) UpdatePricing(ctx context.Context, provider CloudProvider, pricing *CloudPricing) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	pricing.LastUpdated = time.Now()
	a.pricing[provider] = pricing

	a.logger.Info("Updated pricing data", zap.String("provider", string(provider)))
	return a.saveConfig()
}

// GetPricing returns current pricing data.
func (a *CloudCostAnalyzer) GetPricing(ctx context.Context) map[CloudProvider]*CloudPricing {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[CloudProvider]*CloudPricing)
	for k, v := range a.pricing {
		result[k] = v
	}
	return result
}

// GetProviderPricing returns pricing for a specific provider.
func (a *CloudCostAnalyzer) GetProviderPricing(ctx context.Context, provider CloudProvider) (*CloudPricing, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	pricing, exists := a.pricing[provider]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", provider)
	}
	return pricing, nil
}

// GetCostDashboard returns cost dashboard summary.
func (a *CloudCostAnalyzer) GetCostDashboard(ctx context.Context, storageGB float64) map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Compare local vs cloud
	localMonthly := storageGB * a.config.LocalStorageCostPerGB

	cloudOptions := []map[string]interface{}{}
	for _, pricing := range a.pricing {
		cheapestTier := pricing.Tiers[0]
		for _, tier := range pricing.Tiers {
			if tier.StoragePrice < cheapestTier.StoragePrice {
				cheapestTier = tier
			}
		}
		cloudMonthly := storageGB * cheapestTier.StoragePrice
		cloudOptions = append(cloudOptions, map[string]interface{}{
			"provider":     pricing.DisplayName,
			"tier":         cheapestTier.Name,
			"monthly_cost": cloudMonthly,
			"vs_local":     localMonthly - cloudMonthly,
			"currency":     pricing.Currency,
		})
	}

	return map[string]interface{}{
		"storage_gb":         storageGB,
		"local_monthly_cost": localMonthly,
		"cloud_options":      cloudOptions,
		"pricing_updated":    a.getLastPricingUpdate(),
	}
}

// getLastPricingUpdate returns the most recent pricing update time.
func (a *CloudCostAnalyzer) getLastPricingUpdate() time.Time {
	latest := time.Time{}
	for _, pricing := range a.pricing {
		if pricing.LastUpdated.After(latest) {
			latest = pricing.LastUpdated
		}
	}
	return latest
}

// loadConfig loads cost analyzer configuration.
func (a *CloudCostAnalyzer) loadConfig() error {
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Config  *CostAnalyzerConfig             `json:"config"`
		Pricing map[CloudProvider]*CloudPricing `json:"pricing"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Config != nil {
		a.config = cfg.Config
	}
	for k, v := range cfg.Pricing {
		a.pricing[k] = v
	}

	return nil
}

// saveConfig saves cost analyzer configuration.
func (a *CloudCostAnalyzer) saveConfig() error {
	cfg := struct {
		Config  *CostAnalyzerConfig             `json:"config"`
		Pricing map[CloudProvider]*CloudPricing `json:"pricing"`
	}{
		Config:  a.config,
		Pricing: a.pricing,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(a.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(a.configPath, data, 0644)
}
