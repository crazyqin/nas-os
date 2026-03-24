// Package enhanced provides SMB audit API handlers
package enhanced

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// SMAuditAPIHandler SMB审计API处理器
type SMAuditAPIHandler struct {
	manager *SMAuditManager
}

// NewSMAuditAPIHandler 创建SMB审计API处理器
func NewSMAuditAPIHandler(manager *SMAuditManager) *SMAuditAPIHandler {
	return &SMAuditAPIHandler{
		manager: manager,
	}
}

// RegisterRoutes 注册API路由
func (h *SMAuditAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/audit/smb/config", h.handleConfig)
	mux.HandleFunc("/api/audit/smb/events", h.handleEvents)
	mux.HandleFunc("/api/audit/smb/statistics", h.handleStatistics)
	mux.HandleFunc("/api/audit/smb/levels", h.handleLevels)
	mux.HandleFunc("/api/audit/smb/export", h.handleExport)
}

// handleConfig 处理配置请求
// GET: 获取当前配置
// PUT: 更新配置
func (h *SMAuditAPIHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPut:
		h.updateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getConfig 获取当前配置
func (h *SMAuditAPIHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	config := h.manager.GetConfig()
	h.respondJSON(w, http.StatusOK, config)
}

// updateConfig 更新配置
func (h *SMAuditAPIHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var config SMAuditConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.respondError(w, http.StatusBadRequest, "无效的配置格式: "+err.Error())
		return
	}

	// 验证配置
	if err := h.validateConfig(&config); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.manager.UpdateConfig(config)
	h.respondJSON(w, http.StatusOK, map[string]string{
		"message": "配置已更新",
	})
}

// validateConfig 验证配置
func (h *SMAuditAPIHandler) validateConfig(config *SMAuditConfig) error {
	// 验证审计级别
	validLevels := map[SMAuditLevel]bool{
		SMAuditLevelNone:     true,
		SMAuditLevelMinimal:  true,
		SMAuditLevelStandard: true,
		SMAuditLevelDetailed: true,
		SMAuditLevelFull:     true,
	}

	if !validLevels[config.Level] {
		return &ValidationError{Field: "level", Message: "无效的审计级别"}
	}

	// 验证数值范围
	if config.MaxLogAgeDays < 0 || config.MaxLogAgeDays > 365 {
		return &ValidationError{Field: "max_log_age_days", Message: "日志保留天数必须在0-365之间"}
	}

	if config.MaxLogSizeMB < 1 || config.MaxLogSizeMB > 10240 {
		return &ValidationError{Field: "max_log_size_mb", Message: "日志大小限制必须在1-10240MB之间"}
	}

	if config.MaxContentSize < 0 || config.MaxContentSize > 65536 {
		return &ValidationError{Field: "max_content_size", Message: "内容摘要大小必须在0-65536字节之间"}
	}

	return nil
}

// handleEvents 处理事件查询请求
func (h *SMAuditAPIHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析查询参数
	opts := h.parseQueryOptions(r)

	events, total := h.manager.QueryEvents(opts)

	response := map[string]interface{}{
		"events": events,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	}

	h.respondJSON(w, http.StatusOK, response)
}

// parseQueryOptions 解析查询参数
func (h *SMAuditAPIHandler) parseQueryOptions(r *http.Request) SMAuditQueryOptions {
	query := r.URL.Query()

	opts := SMAuditQueryOptions{
		Limit:     100,
		Offset:    0,
		SessionID: query.Get("session_id"),
		ShareName: query.Get("share_name"),
		Username:  query.Get("username"),
		ClientIP:  query.Get("client_ip"),
		FilePath:  query.Get("file_path"),
		Status:    query.Get("status"),
		Operation: SMBFileOperation(query.Get("operation")),
	}

	if limit, err := strconv.Atoi(query.Get("limit")); err == nil && limit > 0 {
		opts.Limit = limit
		if opts.Limit > 1000 {
			opts.Limit = 1000 // 最大限制
		}
	}

	if offset, err := strconv.Atoi(query.Get("offset")); err == nil && offset >= 0 {
		opts.Offset = offset
	}

	if startTime := query.Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			opts.StartTime = &t
		}
	}

	if endTime := query.Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			opts.EndTime = &t
		}
	}

	return opts
}

// handleStatistics 处理统计请求
func (h *SMAuditAPIHandler) handleStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStatistics()
	h.respondJSON(w, http.StatusOK, stats)
}

// handleLevels 处理审计级别列表请求
func (h *SMAuditAPIHandler) handleLevels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	levels := []map[string]interface{}{
		{
			"level":       SMAuditLevelNone,
			"name":        "无审计",
			"description": "不记录任何审计日志",
		},
		{
			"level":       SMAuditLevelMinimal,
			"name":        "最小审计",
			"description": "仅记录会话连接/断开事件",
		},
		{
			"level":       SMAuditLevelStandard,
			"name":        "标准审计",
			"description": "记录会话、文件操作摘要",
		},
		{
			"level":       SMAuditLevelDetailed,
			"name":        "详细审计",
			"description": "记录所有文件操作详情，包括权限变更",
		},
		{
			"level":       SMAuditLevelFull,
			"name":        "完整审计",
			"description": "记录所有操作，包括文件内容摘要（谨慎使用，影响性能）",
		},
	}

	h.respondJSON(w, http.StatusOK, levels)
}

// handleExport 处理导出请求
func (h *SMAuditAPIHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析查询参数
	opts := h.parseQueryOptions(r)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	// 获取所有匹配的事件
	opts.Limit = 10000 // 导出最大数量
	events, _ := h.manager.QueryEvents(opts)

	switch format {
	case "json":
		h.exportJSON(w, events)
	case "csv":
		h.exportCSV(w, events)
	default:
		h.respondError(w, http.StatusBadRequest, "不支持的导出格式: "+format)
	}
}

// exportJSON 导出JSON格式
func (h *SMAuditAPIHandler) exportJSON(w http.ResponseWriter, events []SMAuditEvent) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=smb-audit-export.json")

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(map[string]interface{}{
		"exported_at": time.Now(),
		"count":       len(events),
		"events":      events,
	})
}

// exportCSV 导出CSV格式
func (h *SMAuditAPIHandler) exportCSV(w http.ResponseWriter, events []SMAuditEvent) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=smb-audit-export.csv")

	// CSV头
	w.Write([]byte("timestamp,session_id,username,client_ip,share_name,operation,file_path,status,bytes_read,bytes_written,error_message\n"))

	// 数据行
	for _, event := range events {
		row := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%d,%d,%s\n",
			event.Timestamp.Format(time.RFC3339),
			event.SessionID,
			event.Username,
			event.ClientIP,
			event.ShareName,
			event.Operation,
			event.FilePath,
			event.Status,
			event.BytesRead,
			event.BytesWritten,
			event.ErrorMessage,
		)
		w.Write([]byte(row))
	}
}

// respondJSON 返回JSON响应
func (h *SMAuditAPIHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError 返回错误响应
func (h *SMAuditAPIHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{
		"error": message,
	})
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error 实现error接口
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ========== 审计配置持久化 ==========

// SMAuditConfigManager 审计配置管理器
type SMAuditConfigManager struct {
	configPath string
}

// NewSMAuditConfigManager 创建配置管理器
func NewSMAuditConfigManager(configPath string) *SMAuditConfigManager {
	return &SMAuditConfigManager{
		configPath: configPath,
	}
}

// LoadConfig 加载配置
func (m *SMAuditConfigManager) LoadConfig() (*SMAuditConfig, error) {
	data, err := os.ReadFile(m.configPath)
	if os.IsNotExist(err) {
		// 返回默认配置
		config := DefaultSMAuditConfig()
		return &config, nil
	}
	if err != nil {
		return nil, err
	}

	var config SMAuditConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig 保存配置
func (m *SMAuditConfigManager) SaveConfig(config *SMAuditConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0640)
}