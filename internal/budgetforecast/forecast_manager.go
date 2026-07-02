// Package budgetforecast - 预算预测管理器
// 基于历史数据预测未来存储和计算成本
package budgetforecast

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ForecastManager 预算预测管理器.
type ForecastManager struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	engine  *ForecastEngine
	models  map[string]*ForecastModel
	configs map[string]*BudgetConfig
	trends  map[string]*CostTrend
	exports map[string]*ExportResponse
	alerts  []BudgetAlert
}

// NewForecastManager 创建预算预测管理器.
func NewForecastManager(logger *zap.Logger, engine *ForecastEngine) *ForecastManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	fm := &ForecastManager{
		logger:  logger,
		engine:  engine,
		models:  make(map[string]*ForecastModel),
		configs: make(map[string]*BudgetConfig),
		trends:  make(map[string]*CostTrend),
		exports: make(map[string]*ExportResponse),
		alerts:  make([]BudgetAlert, 0),
	}

	// 初始化默认预测模型
	fm.initDefaultModels()
	// 初始化默认预算配置
	fm.initDefaultConfigs()

	return fm
}

// initDefaultModels 初始化默认预测模型.
func (fm *ForecastManager) initDefaultModels() {
	fm.models["model-linear"] = &ForecastModel{
		ID:   "model-linear",
		Name: "线性回归模型",
		Type: "linear",
		Parameters: map[string]float64{
			"slope":     0,
			"intercept": 0,
		},
		Accuracy:    0.85,
		LastTrained: time.Now(),
		IsActive:    true,
		Description: "基于最小二乘法的线性回归预测，适合稳定增长的趋势",
	}

	fm.models["model-exponential"] = &ForecastModel{
		ID:   "model-exponential",
		Name: "指数平滑模型",
		Type: "exponential",
		Parameters: map[string]float64{
			"alpha": 0.3,
			"beta":  0.1,
		},
		Accuracy:    0.80,
		LastTrained: time.Now(),
		IsActive:    true,
		Description: "指数平滑预测模型，适合有季节性波动的数据",
	}

	fm.models["model-polynomial"] = &ForecastModel{
		ID:   "model-polynomial",
		Name: "多项式回归模型",
		Type: "polynomial",
		Parameters: map[string]float64{
			"degree": 2,
		},
		Accuracy:    0.75,
		LastTrained: time.Now(),
		IsActive:    true,
		Description: "多项式回归预测，适合非线性增长趋势",
	}
}

// initDefaultConfigs 初始化默认预算配置.
func (fm *ForecastManager) initDefaultConfigs() {
	fm.configs["config-default"] = &BudgetConfig{
		ID:            "config-default",
		Name:          "默认预算配置",
		MonthlyBudget: 5000,
		YearlyBudget:  60000,
		Currency:      "CNY",
		AlertThresholds: []AlertThreshold{
			{Percentage: 80, Severity: "info", Enabled: true},
			{Percentage: 90, Severity: "warning", Enabled: true},
			{Percentage: 100, Severity: "critical", Enabled: true},
		},
		StartDate: time.Now().AddDate(-1, 0, 0),
		EndDate:   time.Now().AddDate(1, 0, 0),
		IsActive:  true,
	}
}

// GenerateForecast 生成预测.
func (fm *ForecastManager) GenerateForecast(months int, modelType string) (*ForecastResult, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// 获取预测模型
	model, ok := fm.models[fmt.Sprintf("model-%s", modelType)]
	if !ok {
		model = fm.models["model-linear"] // 默认使用线性模型
	}

	// 使用引擎生成预测
	result := fm.engine.Forecast(months)

	// 根据模型类型调整预测结果
	if model.Type == "exponential" {
		result = fm.applyExponentialSmoothing(result, model)
	}

	fm.logger.Info("Forecast generated",
		zap.Int("months", months),
		zap.String("model", model.Name),
		zap.Int("forecastPoints", len(result.Forecast)),
	)

	return &result, nil
}

// applyExponentialSmoothing 应用指数平滑.
func (fm *ForecastManager) applyExponentialSmoothing(result ForecastResult, model *ForecastModel) ForecastResult {
	alpha := model.Parameters["alpha"]
	beta := model.Parameters["beta"]

	if len(result.Forecast) == 0 {
		return result
	}

	// 简单的指数平滑调整
	for i := range result.Forecast {
		// 降低置信度
		result.Forecast[i].Confidence *= (1 - alpha*0.1)
		// 略微调整预测值
		adjustment := 1.0 + beta*float64(i)*0.01
		result.Forecast[i].PredictedGB *= adjustment
		result.Forecast[i].PredictedCost *= adjustment
	}

	return result
}

// SetAlerts 设置预算告警.
func (fm *ForecastManager) SetAlerts(configID string, thresholds []AlertThreshold) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	config, ok := fm.configs[configID]
	if !ok {
		return fmt.Errorf("budget config not found: %s", configID)
	}

	config.AlertThresholds = thresholds

	// 重新生成告警
	fm.regenerateAlerts(config)

	fm.logger.Info("Alerts updated",
		zap.String("configID", configID),
		zap.Int("thresholds", len(thresholds)),
	)

	return nil
}

// regenerateAlerts 重新生成告警.
func (fm *ForecastManager) regenerateAlerts(config *BudgetConfig) {
	fm.alerts = make([]BudgetAlert, 0)

	// 获取当前预测
	result := fm.engine.Forecast(12)

	for _, threshold := range config.AlertThresholds {
		if !threshold.Enabled {
			continue
		}

		budgetLimit := config.MonthlyBudget * threshold.Percentage / 100

		// 找到预测成本超过阈值的时间点
		for _, point := range result.Forecast {
			if point.PredictedCost >= budgetLimit {
				fm.alerts = append(fm.alerts, BudgetAlert{
					Threshold:     budgetLimit,
					PredictedDate: point.Date,
					Severity:      threshold.Severity,
				})
				break
			}
		}
	}
}

// GetTrends 获取成本趋势.
func (fm *ForecastManager) GetTrends(resourceType string, period string, startDate, endDate time.Time) (*CostTrend, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	trendKey := fmt.Sprintf("%s-%s", resourceType, period)

	// 如果已有缓存的趋势数据
	if trend, ok := fm.trends[trendKey]; ok {
		// 检查时间范围
		if !startDate.IsZero() && trend.StartDate.Before(startDate) {
			// 需要重新生成
		} else {
			return trend, nil
		}
	}

	// 生成新的趋势数据
	trend := fm.generateTrendData(resourceType, period, startDate, endDate)

	// 缓存
	fm.trends[trendKey] = trend

	fm.logger.Info("Trend generated",
		zap.String("resourceType", resourceType),
		zap.String("period", period),
		zap.Int("dataPoints", len(trend.DataPoints)),
	)

	return trend, nil
}

// generateTrendData 生成趋势数据.
func (fm *ForecastManager) generateTrendData(resourceType, period string, startDate, endDate time.Time) *CostTrend {
	// 使用引擎的历史数据
	result := fm.engine.Forecast(0)

	dataPoints := make([]TrendDataPoint, 0)

	// 从历史数据生成趋势点
	for _, snap := range result.HistoryPoints {
		if !startDate.IsZero() && snap.Date.Before(startDate) {
			continue
		}
		if !endDate.IsZero() && snap.Date.After(endDate) {
			continue
		}

		costPerTB := snap.CostPerTB
		if costPerTB == 0 {
			costPerTB = 100
		}
		usedTB := float64(snap.UsedBytes) / (1024 * 1024 * 1024 * 1024)

		dataPoints = append(dataPoints, TrendDataPoint{
			Date:     snap.Date,
			Value:    usedTB * costPerTB,
			Unit:     "CNY",
			IsActual: true,
		})
	}

	// 按日期排序
	sort.Slice(dataPoints, func(i, j int) bool {
		return dataPoints[i].Date.Before(dataPoints[j].Date)
	})

	// 计算趋势
	trendDirection := "stable"
	growthRate := 0.0
	if len(dataPoints) >= 2 {
		first := dataPoints[0].Value
		last := dataPoints[len(dataPoints)-1].Value
		if first > 0 {
			growthRate = ((last - first) / first) * 100
		}
		if growthRate > 5 {
			trendDirection = "increasing"
		} else if growthRate < -5 {
			trendDirection = "decreasing"
		}
	}

	return &CostTrend{
		ID:           fmt.Sprintf("trend-%s-%s", resourceType, period),
		ResourceType: resourceType,
		Period:       period,
		StartDate:    startDate,
		EndDate:      endDate,
		DataPoints:   dataPoints,
		Trend:        trendDirection,
		GrowthRate:   math.Round(growthRate*100) / 100,
	}
}

// ExportReport 导出报告.
func (fm *ForecastManager) ExportReport(req ExportRequest) (*ExportResponse, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 生成预测数据
	result := fm.engine.Forecast(12)

	// 生成趋势数据
	trend := fm.generateTrendData(req.ResourceType, "monthly", req.StartDate, req.EndDate)

	// 根据格式导出
	var fileContent []byte
	var fileName string
	var err error

	switch req.Format {
	case ExportJSON:
		fileContent, err = fm.exportJSON(result, trend, req.IncludeForecast)
		fileName = fmt.Sprintf("budget_forecast_%s.json", time.Now().Format("20060102_150405"))
	case ExportCSV:
		fileContent, err = fm.exportCSV(result, trend)
		fileName = fmt.Sprintf("budget_forecast_%s.csv", time.Now().Format("20060102_150405"))
	default:
		return nil, fmt.Errorf("unsupported export format: %s", req.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("export failed: %w", err)
	}

	exportID := fmt.Sprintf("export-%d", time.Now().UnixNano())
	export := &ExportResponse{
		ID:          exportID,
		Format:      req.Format,
		FileName:    fileName,
		FileSize:    int64(len(fileContent)),
		DownloadURL: fmt.Sprintf("/api/v1/budget/forecast/download/%s", exportID),
		GeneratedAt: time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	// 缓存导出结果
	fm.exports[exportID] = export

	fm.logger.Info("Report exported",
		zap.String("exportID", exportID),
		zap.String("format", string(req.Format)),
		zap.Int64("fileSize", export.FileSize),
	)

	return export, nil
}

// exportJSON 导出JSON格式.
func (fm *ForecastManager) exportJSON(result ForecastResult, trend *CostTrend, includeForecast bool) ([]byte, error) {
	data := map[string]interface{}{
		"forecast_result": result,
		"cost_trend":      trend,
		"generated_at":    time.Now(),
	}

	if !includeForecast {
		delete(data, "forecast_result")
	}

	return json.MarshalIndent(data, "", "  ")
}

// exportCSV 导出CSV格式.
func (fm *ForecastManager) exportCSV(result ForecastResult, trend *CostTrend) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)

	// 写入标题
	writer.Write([]string{"日期", "类型", "存储量(GB)", "成本(CNY)", "置信度"})

	// 写入历史数据
	for _, point := range trend.DataPoints {
		writer.Write([]string{
			point.Date.Format("2006-01-02"),
			"实际",
			"",
			fmt.Sprintf("%.2f", point.Value),
			"",
		})
	}

	// 写入预测数据
	for _, point := range result.Forecast {
		writer.Write([]string{
			point.Date.Format("2006-01-02"),
			"预测",
			fmt.Sprintf("%.2f", point.PredictedGB),
			fmt.Sprintf("%.2f", point.PredictedCost),
			fmt.Sprintf("%.2f%%", point.Confidence*100),
		})
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// GetAlerts 获取预算告警.
func (fm *ForecastManager) GetAlerts() []BudgetAlert {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.alerts
}

// GetModels 获取预测模型列表.
func (fm *ForecastManager) GetModels() []*ForecastModel {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	models := make([]*ForecastModel, 0, len(fm.models))
	for _, m := range fm.models {
		models = append(models, m)
	}
	return models
}

// GetConfigs 获取预算配置列表.
func (fm *ForecastManager) GetConfigs() []*BudgetConfig {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	configs := make([]*BudgetConfig, 0, len(fm.configs))
	for _, c := range fm.configs {
		configs = append(configs, c)
	}
	return configs
}

// UpdateConfig 更新预算配置.
func (fm *ForecastManager) UpdateConfig(configID string, config *BudgetConfig) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.configs[configID]; !ok {
		return fmt.Errorf("budget config not found: %s", configID)
	}

	config.ID = configID
	fm.configs[configID] = config

	// 重新生成告警
	fm.regenerateAlerts(config)

	fm.logger.Info("Budget config updated",
		zap.String("configID", configID),
	)

	return nil
}

// GetExport 获取导出结果.
func (fm *ForecastManager) GetExport(exportID string) (*ExportResponse, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	export, ok := fm.exports[exportID]
	if !ok {
		return nil, fmt.Errorf("export not found: %s", exportID)
	}

	// 检查是否过期
	if time.Now().After(export.ExpiresAt) {
		return nil, fmt.Errorf("export has expired")
	}

	return export, nil
}
