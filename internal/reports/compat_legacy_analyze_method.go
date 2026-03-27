package reports

// Analyze 为旧报表测试保留兼容签名。
func (a *CostAnalyzer) Analyze(volumeMetrics []StorageMetrics, userUsages []UserStorageUsage, history []CostTrendDataPoint, period ReportPeriod) *LegacyCostAnalysisReport {
	return a.AnalyzeLegacy(volumeMetrics, userUsages, history, period)
}
