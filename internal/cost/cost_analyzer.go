// Package cost provides storage cost analysis functionality.
package cost

import (
	"sync"
	"time"
)

// UserCostStats represents cost statistics for a user.
type UserCostStats struct {
	UserID          string    `json:"user_id"`
	StorageUsed     int64     `json:"storage_used"`      // bytes
	StorageQuota    int64     `json:"storage_quota"`     // bytes
	MonthlyCost     float64   `json:"monthly_cost"`      // CNY
	CostPerGB       float64   `json:"cost_per_gb"`       // CNY/GB
	EfficiencyScore float64   `json:"efficiency_score"`  // 0-100
	LastUpdated     time.Time `json:"last_updated"`
	Trend           CostTrend `json:"trend"`
}

// CostTrend represents cost trend over time.
type CostTrend struct {
	LastMonthCost float64 `json:"last_month_cost"`
	ChangePercent float64 `json:"change_percent"`
	GrowthRate    float64 `json:"growth_rate"` // monthly growth rate
}

// DirectoryCostStats represents cost statistics for a directory.
type DirectoryCostStats struct {
	Path            string    `json:"path"`
	StorageUsed     int64     `json:"storage_used"`
	FileCount       int       `json:"file_count"`
	MonthlyCost     float64   `json:"monthly_cost"`
	AvgFileSize     float64   `json:"avg_file_size"`
	EfficiencyScore float64   `json:"efficiency_score"`
	LastUpdated     time.Time `json:"last_updated"`
}

// StorageEfficiencyReport represents storage efficiency analysis.
type StorageEfficiencyReport struct {
	GeneratedAt       time.Time           `json:"generated_at"`
	TotalStorage      int64               `json:"total_storage"`
	UsedStorage       int64               `json:"used_storage"`
	AvailableStorage  int64               `json:"available_storage"`
	UtilizationPercent float64            `json:"utilization_percent"`
	EfficiencyScore   float64             `json:"efficiency_score"`
	UserStats         []UserCostStats     `json:"user_stats"`
	DirectoryStats    []DirectoryCostStats `json:"directory_stats"`
	SavingsPotential  float64             `json:"savings_potential"` // CNY
	Suggestions       []SavingsSuggestion `json:"suggestions"`
}

// SavingsSuggestion represents cost saving suggestion.
type SavingsSuggestion struct {
	Type        string  `json:"type"`        // archive, dedup, tier, cleanup
	Description string  `json:"description"`
	PotentialSaving float64 `json:"potential_saving"` // CNY
	Priority    int     `json:"priority"`    // 1=high, 2=medium, 3=low
	UserID      string  `json:"user_id"`
	Path        string  `json:"path"`
}

// CostAnalyzer analyzes storage costs.
type CostAnalyzer struct {
	mu            sync.RWMutex
	userStats     map[string]*UserCostStats
	directoryStats map[string]*DirectoryCostStats
	config        *CostConfig
}

// CostConfig holds cost analysis configuration.
type CostConfig struct {
	CostPerGBMonthly float64 `json:"cost_per_gb_monthly"` // CNY/GB/month
	SsdPremiumFactor float64 `json:"ssd_premium_factor"`  // SSD cost multiplier
	HddArchiveFactor float64 `json:"hdd_archive_factor"`  // HDD cost discount
	DedupSavingRate  float64 `json:"dedup_saving_rate"`   // dedup savings percent
	ArchiveThreshold int64   `json:"archive_threshold"`   // bytes
}

// NewCostAnalyzer creates a new cost analyzer.
func NewCostAnalyzer(cfg *CostConfig) *CostAnalyzer {
	if cfg == nil {
		cfg = DefaultCostConfig()
	}
	return &CostAnalyzer{
		userStats:      make(map[string]*UserCostStats),
		directoryStats: make(map[string]*DirectoryCostStats),
		config:         cfg,
	}
}

// DefaultCostConfig returns default cost configuration.
func DefaultCostConfig() *CostConfig {
	return &CostConfig{
		CostPerGBMonthly: 0.69,  // ¥69 per 100GB per month = ¥0.69/GB
		SsdPremiumFactor: 3.0,   // SSD costs 3x more
		HddArchiveFactor: 0.3,   // HDD archive costs 30%
		DedupSavingRate:  0.15,  // 15% savings from dedup
		ArchiveThreshold: 100 * 1024 * 1024 * 1024, // 100GB
	}
}

// UpdateUserStats updates cost statistics for a user.
func (a *CostAnalyzer) UpdateUserStats(userID string, storageUsed int64, quota int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	stats := a.userStats[userID]
	if stats == nil {
		stats = &UserCostStats{
			UserID:      userID,
			LastUpdated: now,
		}
		a.userStats[userID] = stats
	}

	// Calculate cost
	gbUsed := float64(storageUsed) / (1024 * 1024 * 1024)
	monthlyCost := gbUsed * a.config.CostPerGBMonthly

	// Calculate previous month for trend
	prevStats := a.userStats[userID]
	oldCost := 0.0
	if prevStats != nil && prevStats.MonthlyCost > 0 {
		oldCost = prevStats.MonthlyCost
	}

	changePercent := 0.0
	if oldCost > 0 {
		changePercent = (monthlyCost - oldCost) / oldCost * 100
	}

	// Calculate efficiency score (0-100)
	efficiency := calculateEfficiencyScore(storageUsed, quota)

	stats.StorageUsed = storageUsed
	stats.StorageQuota = quota
	stats.MonthlyCost = monthlyCost
	stats.CostPerGB = a.config.CostPerGBMonthly
	stats.EfficiencyScore = efficiency
	stats.LastUpdated = now
	stats.Trend = CostTrend{
		LastMonthCost: oldCost,
		ChangePercent: changePercent,
		GrowthRate:    changePercent / 30, // daily rate approximation
	}
}

// UpdateDirectoryStats updates cost statistics for a directory.
func (a *CostAnalyzer) UpdateDirectoryStats(path string, storageUsed int64, fileCount int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	stats := a.directoryStats[path]
	if stats == nil {
		stats = &DirectoryCostStats{
			Path:        path,
			LastUpdated: now,
		}
		a.directoryStats[path] = stats
	}

	gbUsed := float64(storageUsed) / (1024 * 1024 * 1024)
	monthlyCost := gbUsed * a.config.CostPerGBMonthly

	avgFileSize := 0.0
	if fileCount > 0 {
		avgFileSize = float64(storageUsed) / float64(fileCount)
	}

	efficiency := calculateDirectoryEfficiency(storageUsed, fileCount, avgFileSize)

	stats.StorageUsed = storageUsed
	stats.FileCount = fileCount
	stats.MonthlyCost = monthlyCost
	stats.AvgFileSize = avgFileSize
	stats.EfficiencyScore = efficiency
	stats.LastUpdated = now
}

// GetUserStats returns cost statistics for a user.
func (a *CostAnalyzer) GetUserStats(userID string) *UserCostStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.userStats[userID]
}

// GetAllUserStats returns all user cost statistics.
func (a *CostAnalyzer) GetAllUserStats() []UserCostStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]UserCostStats, 0)
	for _, stats := range a.userStats {
		result = append(result, *stats)
	}
	return result
}

// GetDirectoryStats returns cost statistics for a directory.
func (a *CostAnalyzer) GetDirectoryStats(path string) *DirectoryCostStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.directoryStats[path]
}

// GenerateReport generates a comprehensive efficiency report.
func (a *CostAnalyzer) GenerateReport(totalStorage int64) *StorageEfficiencyReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	now := time.Now()
	report := &StorageEfficiencyReport{
		GeneratedAt: now,
		TotalStorage: totalStorage,
		UserStats:   a.getAllUserStatsCopy(),
		DirectoryStats: a.getAllDirectoryStatsCopy(),
		Suggestions: make([]SavingsSuggestion, 0),
	}

	// Calculate totals
	var usedTotal int64
	var totalMonthlyCost float64
	for _, stats := range a.userStats {
		usedTotal += stats.StorageUsed
		totalMonthlyCost += stats.MonthlyCost
	}

	report.UsedStorage = usedTotal
	report.AvailableStorage = totalStorage - usedTotal
	if totalStorage > 0 {
		report.UtilizationPercent = float64(usedTotal) / float64(totalStorage) * 100
	}

	// Calculate overall efficiency
	report.EfficiencyScore = calculateOverallEfficiency(a.userStats)

	// Generate savings suggestions
	report.Suggestions = a.generateSuggestions()
	report.SavingsPotential = a.calculateTotalPotentialSavings()

	return report
}

// generateSuggestions generates cost savings suggestions.
func (a *CostAnalyzer) generateSuggestions() []SavingsSuggestion {
	suggestions := make([]SavingsSuggestion, 0)

	// Check for users near quota limit
	for userID, stats := range a.userStats {
		if stats.StorageQuota > 0 {
			utilization := float64(stats.StorageUsed) / float64(stats.StorageQuota)
			if utilization > 0.8 {
				suggestions = append(suggestions, SavingsSuggestion{
					Type:           "cleanup",
					Description:    "用户存储使用率超过80%，建议清理旧文件或升级配额",
					PotentialSaving: stats.MonthlyCost * 0.2,
					Priority:       1,
					UserID:         userID,
				})
			}
		}

		// Check for high growth rate
		if stats.Trend.GrowthRate > 0.5 {
			suggestions = append(suggestions, SavingsSuggestion{
				Type:           "tier",
				Description:    "用户存储增长迅速，建议启用冷数据分层",
				PotentialSaving: stats.MonthlyCost * a.config.HddArchiveFactor,
				Priority:       2,
				UserID:         userID,
			})
		}
	}

	// Check for directories with large files
	for path, stats := range a.directoryStats {
		if stats.StorageUsed > a.config.ArchiveThreshold {
			suggestions = append(suggestions, SavingsSuggestion{
				Type:           "archive",
				Description:    "目录存储超过归档阈值，建议迁移至低成本存储",
				PotentialSaving: stats.MonthlyCost * (1 - a.config.HddArchiveFactor),
				Priority:       2,
				Path:           path,
			})
		}
	}

	// Dedup suggestion
	suggestions = append(suggestions, SavingsSuggestion{
		Type:           "dedup",
		Description:    "启用数据去重可节省约15%存储空间",
		PotentialSaving: a.calculateTotalCost() * a.config.DedupSavingRate,
		Priority:       3,
	})

	return suggestions
}

// calculateTotalCost returns total monthly cost.
func (a *CostAnalyzer) calculateTotalCost() float64 {
	var total float64
	for _, stats := range a.userStats {
		total += stats.MonthlyCost
	}
	return total
}

// calculateTotalPotentialSavings returns total potential savings.
func (a *CostAnalyzer) calculateTotalPotentialSavings() float64 {
	var total float64
	for _, suggestion := range a.generateSuggestions() {
		total += suggestion.PotentialSaving
	}
	return total
}

// Helper functions

func calculateEfficiencyScore(used int64, quota int64) float64 {
	if quota <= 0 {
		return 50.0 // neutral score for unlimited quota
	}
	utilization := float64(used) / float64(quota)
	if utilization < 0.5 {
		return 30.0 + utilization * 20 // low utilization = lower score
	} else if utilization > 0.9 {
		return 100.0 // high utilization = high score
	}
	return 50.0 + utilization * 50
}

func calculateDirectoryEfficiency(storage int64, fileCount int, avgSize float64) float64 {
	// Higher score for larger directories with more files
	gbSize := float64(storage) / (1024 * 1024 * 1024)

	score := 50.0
	if gbSize > 10 {
		score += 20
	}
	if fileCount > 1000 {
		score += 10
	}
	if avgSize > 10*1024*1024 { // avg > 10MB
		score -= 10 // large files might be better archived
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

func calculateOverallEfficiency(userStats map[string]*UserCostStats) float64 {
	if len(userStats) == 0 {
		return 0
	}

	var totalScore float64
	for _, stats := range userStats {
		totalScore += stats.EfficiencyScore
	}
	return totalScore / float64(len(userStats))
}

func (a *CostAnalyzer) getAllUserStatsCopy() []UserCostStats {
	result := make([]UserCostStats, 0)
	for _, stats := range a.userStats {
		result = append(result, *stats)
	}
	return result
}

func (a *CostAnalyzer) getAllDirectoryStatsCopy() []DirectoryCostStats {
	result := make([]DirectoryCostStats, 0)
	for _, stats := range a.directoryStats {
		result = append(result, *stats)
	}
	return result
}