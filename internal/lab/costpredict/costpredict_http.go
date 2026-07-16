// costpredict_http.go - HTTP handlers for cost prediction API using standard library.
package costpredict

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Handlers HTTP处理器（使用标准库，兼容已有测试）.
type Handlers struct {
	predictor *Predictor
	mux       *http.ServeMux
}

// NewHandlers 创建HTTP处理器.
func NewHandlers(p *Predictor) *Handlers {
	h := &Handlers{
		predictor: p,
		mux:       http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

// ServeHTTP 实现 http.Handler 接口.
func (h *Handlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handlers) registerRoutes() {
	h.mux.HandleFunc("/api/v1/costpredict/records", h.handleRecords)
	h.mux.HandleFunc("/api/v1/costpredict/records/batch", h.handleRecordsBatch)
	h.mux.HandleFunc("/api/v1/costpredict/predict", h.handlePredict)
	h.mux.HandleFunc("/api/v1/costpredict/predict/department", h.handlePredictDepartment)
	h.mux.HandleFunc("/api/v1/costpredict/predict/project", h.handlePredictProject)
	h.mux.HandleFunc("/api/v1/costpredict/predict/storage-type", h.handlePredictStorageType)
	h.mux.HandleFunc("/api/v1/costpredict/capacity/forecast", h.handleCapacityForecast)
	h.mux.HandleFunc("/api/v1/costpredict/suggestions", h.handleSuggestions)
	h.mux.HandleFunc("/api/v1/costpredict/alerts", h.handleAlerts)
	h.mux.HandleFunc("/api/v1/costpredict/reports/", h.handleReports)
	h.mux.HandleFunc("/api/v1/costpredict/reports", h.handleReports)
	h.mux.HandleFunc("/api/v1/costpredict/budget", h.handleBudget)
	h.mux.HandleFunc("/api/v1/costpredict/currencies", h.handleCurrencies)
	h.mux.HandleFunc("/api/v1/costpredict/convert", h.handleConvert)
}

func jsonResp(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	jsonResp(w, code, map[string]string{"error": msg})
}

// handleRecords 处理 /records 路由.
func (h *Handlers) handleRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var record CostRecord
		if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
			jsonErr(w, http.StatusBadRequest, "无效的请求体")
			return
		}
		h.predictor.AddRecord(record)
		jsonResp(w, http.StatusCreated, map[string]string{"message": "成本记录添加成功"})
	case http.MethodGet:
		records := h.predictor.GetRecords()
		jsonResp(w, http.StatusOK, map[string]interface{}{"records": records, "total": len(records)})
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleRecordsBatch 处理批量添加记录.
func (h *Handlers) handleRecordsBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	// 尝试解析包装格式 {"records": [...]}
	var wrappedReq struct {
		Records []CostRecord `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&wrappedReq); err == nil && len(wrappedReq.Records) > 0 {
		h.predictor.AddRecords(wrappedReq.Records)
		jsonResp(w, http.StatusCreated, map[string]interface{}{"message": "批量添加成功", "count": len(wrappedReq.Records)})
		return
	}
	// 尝试解析直接数组格式 [...]
	var records []CostRecord
	if err := json.NewDecoder(r.Body).Decode(&records); err != nil {
		jsonErr(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	h.predictor.AddRecords(records)
	jsonResp(w, http.StatusCreated, map[string]interface{}{"message": "批量添加成功", "count": len(records)})
}

// handlePredict 处理预测请求.
func (h *Handlers) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	periods := 3
	if p := r.URL.Query().Get("periods"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			periods = v
		}
	}
	results, err := h.predictor.PredictCost(periods)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, results)
}

// handlePredictDepartment 处理按部门预测.
func (h *Handlers) handlePredictDepartment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	dept := r.URL.Query().Get("department")
	if dept == "" {
		jsonErr(w, http.StatusBadRequest, "缺少 department 参数")
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
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, results)
}

// handlePredictProject 处理按项目预测.
func (h *Handlers) handlePredictProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	project := r.URL.Query().Get("project")
	if project == "" {
		jsonErr(w, http.StatusBadRequest, "缺少 project 参数")
		return
	}
	results, err := h.predictor.PredictCostByProject(project, 3)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, results)
}

// handlePredictStorageType 处理按存储类型预测.
func (h *Handlers) handlePredictStorageType(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	stType := r.URL.Query().Get("type")
	if stType == "" {
		jsonErr(w, http.StatusBadRequest, "缺少 type 参数")
		return
	}
	results, err := h.predictor.PredictCostByStorageType(StorageType(stType), 3)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, results)
}

// handleCapacityForecast 处理容量预测.
func (h *Handlers) handleCapacityForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	months := 6
	if m := r.URL.Query().Get("months"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			months = v
		}
	}
	results, err := h.predictor.PredictCapacityGrowth(months)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, results)
}

// handleSuggestions 处理优化建议.
func (h *Handlers) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	suggestions := h.predictor.GenerateOptimizationSuggestions()
	jsonResp(w, http.StatusOK, suggestions)
}

// handleAlerts 处理预算告警.
func (h *Handlers) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	alerts, err := h.predictor.CheckBudgetAlerts(3)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, alerts)
}

// handleReports 处理报告.
func (h *Handlers) handleReports(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/costpredict/reports")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodPost:
		var req struct {
			ReportType string   `json:"report_type"`
			Currency   Currency `json:"currency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, http.StatusBadRequest, "无效的请求体")
			return
		}
		report, err := h.predictor.GenerateReport(req.ReportType, req.Currency)
		if err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResp(w, http.StatusCreated, report)
	case http.MethodGet:
		if path == "" {
			reports := h.predictor.ListReports()
			jsonResp(w, http.StatusOK, reports)
			return
		}
		report, err := h.predictor.GetReport(path)
		if err != nil {
			jsonErr(w, http.StatusNotFound, fmt.Sprintf("报告不存在: %s", path))
			return
		}
		jsonResp(w, http.StatusOK, report)
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleBudget 处理预算设置.
func (h *Handlers) handleBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	var req struct {
		Department string  `json:"department"`
		Amount     float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Department == "" {
		jsonErr(w, http.StatusBadRequest, "缺少 department 参数")
		return
	}
	h.predictor.SetBudgetLimit(req.Department, req.Amount)
	jsonResp(w, http.StatusOK, map[string]string{"message": "预算设置成功"})
}

// handleCurrencies 处理币种列表.
func (h *Handlers) handleCurrencies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		currencies := h.predictor.ListCurrencies()
		jsonResp(w, http.StatusOK, currencies)
	case http.MethodPost:
		var rate CurrencyRate
		if err := json.NewDecoder(r.Body).Decode(&rate); err != nil {
			jsonErr(w, http.StatusBadRequest, "无效的请求体")
			return
		}
		h.predictor.SetCurrencyRate(rate)
		jsonResp(w, http.StatusOK, map[string]string{"message": "汇率设置成功"})
	default:
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleConvert 处理币种转换.
func (h *Handlers) handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	amountStr := r.URL.Query().Get("amount")
	currency := r.URL.Query().Get("currency")
	if amountStr == "" || currency == "" {
		jsonErr(w, http.StatusBadRequest, "缺少 amount 或 currency 参数")
		return
	}
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "无效的金额")
		return
	}
	result, err := h.predictor.ConvertCost(amount, Currency(currency))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]interface{}{
		"original":  amount,
		"currency":  currency,
		"converted": result,
	})
}
