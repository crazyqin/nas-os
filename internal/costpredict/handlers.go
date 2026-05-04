package costpredict

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handlers 存储成本预测 HTTP 处理器.
type Handlers struct {
	predictor *Predictor
}

// NewHandlers 创建成本预测 HTTP 处理器.
func NewHandlers(predictor *Predictor) *Handlers {
	return &Handlers{predictor: predictor}
}

// ServeHTTP 实现 http.Handler 接口，路由分发.
func (h *Handlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/costpredict")
	path = strings.TrimPrefix(path, "/")

	// 路由匹配
	switch {
	case path == "records" && r.Method == http.MethodPost:
		h.addRecord(w, r)
	case path == "records" && r.Method == http.MethodGet:
		h.listRecords(w, r)
	case path == "records/batch" && r.Method == http.MethodPost:
		h.addRecordsBatch(w, r)
	case path == "predict" && r.Method == http.MethodGet:
		h.predict(w, r)
	case path == "predict/department" && r.Method == http.MethodGet:
		h.predictByDepartment(w, r)
	case path == "predict/project" && r.Method == http.MethodGet:
		h.predictByProject(w, r)
	case path == "predict/storage-type" && r.Method == http.MethodGet:
		h.predictByStorageType(w, r)
	case path == "capacity/forecast" && r.Method == http.MethodGet:
		h.capacityForecast(w, r)
	case path == "suggestions" && r.Method == http.MethodGet:
		h.optimizationSuggestions(w, r)
	case path == "alerts" && r.Method == http.MethodGet:
		h.budgetAlerts(w, r)
	case path == "reports" && r.Method == http.MethodPost:
		h.generateReport(w, r)
	case path == "reports" && r.Method == http.MethodGet:
		h.listReports(w, r)
	case strings.HasPrefix(path, "reports/") && r.Method == http.MethodGet:
		h.getReport(w, r)
	case path == "budget" && r.Method == http.MethodPost:
		h.setBudget(w, r)
	case path == "currencies" && r.Method == http.MethodGet:
		h.listCurrencies(w, r)
	case path == "currencies" && r.Method == http.MethodPost:
		h.setCurrency(w, r)
	case path == "convert" && r.Method == http.MethodGet:
		h.convertCost(w, r)
	case path == "trend/report" && r.Method == http.MethodGet:
		h.trendReport(w, r)
	case path == "trend/moving-averages" && r.Method == http.MethodGet:
		h.movingAverages(w, r)
	case path == "trend/seasonality" && r.Method == http.MethodGet:
		h.seasonality(w, r)
	case path == "trend/anomalies" && r.Method == http.MethodGet:
		h.anomalies(w, r)
	case path == "trend/accuracy" && r.Method == http.MethodGet:
		h.predictionAccuracy(w, r)
	default:
		http.NotFound(w, r)
	}
}

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// addRecord 添加单条成本记录.
func (h *Handlers) addRecord(w http.ResponseWriter, r *http.Request) {
	var rec CostRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if rec.Time.IsZero() {
		rec.Time = time.Now()
	}
	h.predictor.AddRecord(rec)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "成本记录添加成功",
		"record":  rec,
	})
}

// addRecordsBatch 批量添加成本记录.
func (h *Handlers) addRecordsBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Records []CostRecord `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	for i := range req.Records {
		if req.Records[i].Time.IsZero() {
			req.Records[i].Time = time.Now()
		}
	}
	h.predictor.AddRecords(req.Records)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "批量添加成功",
		"count":   len(req.Records),
	})
}

// listRecords 列出所有记录.
func (h *Handlers) listRecords(w http.ResponseWriter, r *http.Request) {
	records := h.predictor.GetRecords()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": records,
		"total":   len(records),
	})
}

// predict 通用预测.
func (h *Handlers) predict(w http.ResponseWriter, r *http.Request) {
	periods := 3
	if p := r.URL.Query().Get("periods"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			periods = v
		}
	}
	results, err := h.predictor.PredictCost(periods)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"predictions": results,
		"periods":     periods,
	})
}

// predictByDepartment 按部门预测.
func (h *Handlers) predictByDepartment(w http.ResponseWriter, r *http.Request) {
	dept := r.URL.Query().Get("department")
	if dept == "" {
		writeError(w, http.StatusBadRequest, "缺少department参数")
		return
	}
	periods := 3
	if p := r.URL.Query().Get("periods"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			periods = v
		}
	}
	results, err := h.predictor.PredictCostByDepartment(dept, periods)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"department":  dept,
		"predictions": results,
		"periods":     periods,
	})
}

// predictByProject 按项目预测.
func (h *Handlers) predictByProject(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeError(w, http.StatusBadRequest, "缺少project参数")
		return
	}
	periods := 3
	if p := r.URL.Query().Get("periods"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			periods = v
		}
	}
	results, err := h.predictor.PredictCostByProject(project, periods)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":     project,
		"predictions": results,
		"periods":     periods,
	})
}

// predictByStorageType 按存储类型预测.
func (h *Handlers) predictByStorageType(w http.ResponseWriter, r *http.Request) {
	st := StorageType(r.URL.Query().Get("type"))
	if st == "" {
		writeError(w, http.StatusBadRequest, "缺少type参数")
		return
	}
	periods := 3
	if p := r.URL.Query().Get("periods"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			periods = v
		}
	}
	results, err := h.predictor.PredictCostByStorageType(st, periods)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"storage_type": st,
		"predictions":  results,
		"periods":      periods,
	})
}

// capacityForecast 容量增长预测.
func (h *Handlers) capacityForecast(w http.ResponseWriter, r *http.Request) {
	months := 6
	if m := r.URL.Query().Get("months"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			months = v
		}
	}
	forecasts, err := h.predictor.PredictCapacityGrowth(months)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"forecasts": forecasts,
		"months":    months,
	})
}

// optimizationSuggestions 优化建议.
func (h *Handlers) optimizationSuggestions(w http.ResponseWriter, r *http.Request) {
	suggestions := h.predictor.GenerateOptimizationSuggestions()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
		"total":       len(suggestions),
	})
}

// budgetAlerts 预算告警.
func (h *Handlers) budgetAlerts(w http.ResponseWriter, r *http.Request) {
	periods := 3
	if p := r.URL.Query().Get("periods"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			periods = v
		}
	}
	alerts, err := h.predictor.CheckBudgetAlerts(periods)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts":  alerts,
		"total":   len(alerts),
		"periods": periods,
	})
}

// generateReport 生成报告.
func (h *Handlers) generateReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReportType string   `json:"report_type"`
		Currency   Currency `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if req.Currency == "" {
		req.Currency = CNY
	}
	report, err := h.predictor.GenerateReport(req.ReportType, req.Currency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "报告生成成功",
		"report":  report,
	})
}

// listReports 列出报告.
func (h *Handlers) listReports(w http.ResponseWriter, r *http.Request) {
	reports := h.predictor.ListReports()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"total":   len(reports),
	})
}

// getReport 获取报告详情.
func (h *Handlers) getReport(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/costpredict/reports/")
	id := strings.TrimPrefix(path, "/")
	report, err := h.predictor.GetReport(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// setBudget 设置预算.
func (h *Handlers) setBudget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Department string  `json:"department"`
		Amount     float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if req.Department == "" {
		writeError(w, http.StatusBadRequest, "缺少department")
		return
	}
	h.predictor.SetBudgetLimit(req.Department, req.Amount)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "预算设置成功",
		"department": req.Department,
		"amount":     req.Amount,
	})
}

// listCurrencies 列出币种.
func (h *Handlers) listCurrencies(w http.ResponseWriter, r *http.Request) {
	currencies := h.predictor.ListCurrencies()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"currencies": currencies,
		"total":      len(currencies),
	})
}

// setCurrency 设置汇率.
func (h *Handlers) setCurrency(w http.ResponseWriter, r *http.Request) {
	var rate CurrencyRate
	if err := json.NewDecoder(r.Body).Decode(&rate); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	h.predictor.SetCurrencyRate(rate)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "汇率设置成功",
		"rate":    rate,
	})
}

// convertCost 币种转换.
func (h *Handlers) convertCost(w http.ResponseWriter, r *http.Request) {
	amountStr := r.URL.Query().Get("amount")
	currency := Currency(r.URL.Query().Get("currency"))
	if amountStr == "" || currency == "" {
		writeError(w, http.StatusBadRequest, "缺少amount或currency参数")
		return
	}
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "amount参数无效")
		return
	}
	converted, err := h.predictor.ConvertCost(amount, currency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"original_cny":  amount,
		"target":        currency,
		"converted":     converted,
	})
}

// ========== 趋势分析端点 ==========

// trendReport 趋势分析报告.
func (h *Handlers) trendReport(w http.ResponseWriter, r *http.Request) {
	periods := 3
	if p := r.URL.Query().Get("periods"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			periods = v
		}
	}
	analyzer := NewTrendAnalyzer(h.predictor)
	report, err := analyzer.GenerateTrendReport(periods)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// movingAverages 移动平均分析.
func (h *Handlers) movingAverages(w http.ResponseWriter, r *http.Request) {
	analyzer := NewTrendAnalyzer(h.predictor)
	results, err := analyzer.AnalyzeMovingAverages()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"moving_averages": results,
		"total":           len(results),
	})
}

// seasonality 季节性检测.
func (h *Handlers) seasonality(w http.ResponseWriter, r *http.Request) {
	analyzer := NewTrendAnalyzer(h.predictor)
	results, err := analyzer.DetectSeasonality()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"seasonality": results,
		"total":       len(results),
	})
}

// anomalies 成本异常检测.
func (h *Handlers) anomalies(w http.ResponseWriter, r *http.Request) {
	threshold := 0.2
	if t := r.URL.Query().Get("threshold"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil && v > 0 {
			threshold = v
		}
	}
	analyzer := NewTrendAnalyzer(h.predictor)
	results, err := analyzer.DetectAnomalies(threshold)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"anomalies": results,
		"total":     len(results),
		"threshold": threshold,
	})
}

// predictionAccuracy 预测精度验证.
func (h *Handlers) predictionAccuracy(w http.ResponseWriter, r *http.Request) {
	trainRatio := 0.7
	if t := r.URL.Query().Get("train_ratio"); t != "" {
		if v, err := strconv.ParseFloat(t, 64); err == nil && v > 0 && v < 1 {
			trainRatio = v
		}
	}
	analyzer := NewTrendAnalyzer(h.predictor)
	results, err := analyzer.ValidatePredictionAccuracy(trainRatio)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"accuracy":    results,
		"train_ratio": trainRatio,
	})
}
