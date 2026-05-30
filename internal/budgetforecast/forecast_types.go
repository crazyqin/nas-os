// Package budgetforecast - 预算预测类型定义
// 基于历史数据预测未来存储和计算成本
package budgetforecast

import (
	"time"
)

// ForecastModel 预测模型
type ForecastModel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // linear, exponential, polynomial
	Parameters  map[string]float64 `json:"parameters"`
	Accuracy    float64   `json:"accuracy"`    // 模型准确度 0-1
	LastTrained time.Time `json:"last_trained"`
	IsActive    bool      `json:"is_active"`
	Description string    `json:"description"`
}

// CostTrend 成本趋势
type CostTrend struct {
	ID          string    `json:"id"`
	ResourceType string   `json:"resource_type"` // storage, compute, network
	Period      string    `json:"period"`        // daily, weekly, monthly
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	DataPoints  []TrendDataPoint `json:"data_points"`
	Trend       string    `json:"trend"`        // increasing, decreasing, stable
	GrowthRate  float64   `json:"growth_rate"`  // 增长率 (%)
	Forecast    []TrendDataPoint `json:"forecast,omitempty"`
}

// TrendDataPoint 趋势数据点
type TrendDataPoint struct {
	Date     time.Time `json:"date"`
	Value    float64   `json:"value"`
	Unit     string    `json:"unit"`
	IsActual bool      `json:"is_actual"` // true=实际值, false=预测值
}

// BudgetConfig 预算配置
type BudgetConfig struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	MonthlyBudget float64   `json:"monthly_budget"`
	YearlyBudget  float64   `json:"yearly_budget"`
	Currency      string    `json:"currency"`
	AlertThresholds []AlertThreshold `json:"alert_thresholds"`
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	IsActive      bool      `json:"is_active"`
}

// AlertThreshold 告警阈值
type AlertThreshold struct {
	Percentage float64 `json:"percentage"` // 80, 90, 100
	Severity   string  `json:"severity"`   // info, warning, critical
	Enabled    bool    `json:"enabled"`
}

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
	ExportPDF  ExportFormat = "pdf"
	ExportXLSX ExportFormat = "xlsx"
)

// ExportRequest 导出请求
type ExportRequest struct {
	Format    ExportFormat `json:"format"`
	StartDate time.Time    `json:"start_date"`
	EndDate   time.Time    `json:"end_date"`
	ResourceType string    `json:"resource_type,omitempty"`
	IncludeForecast bool   `json:"include_forecast"`
}

// ExportResponse 导出响应
type ExportResponse struct {
	ID          string    `json:"id"`
	Format      ExportFormat `json:"format"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	DownloadURL string    `json:"download_url"`
	GeneratedAt time.Time `json:"generated_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}
