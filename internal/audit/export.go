// Package audit 提供审计日志导出功能
// 支持CSV、JSON、YAML格式导出
package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// Exporter 审计日志导出器.
type Exporter struct {
	manager *Manager
}

// NewExporter 创建导出器.
func NewExporter(manager *Manager) *Exporter {
	return &Exporter{
		manager: manager,
	}
}

// ExportRequest 导出请求.
type ExportRequest struct {
	Format            ExportFormat `json:"format"`               // 导出格式
	StartTime         time.Time    `json:"start_time"`           // 开始时间
	EndTime           time.Time    `json:"end_time"`             // 结束时间
	Categories        []Category   `json:"categories,omitempty"` // 分类过滤
	Levels            []Level      `json:"levels,omitempty"`     // 级别过滤
	UserID            string       `json:"user_id,omitempty"`    // 用户ID过滤
	IncludeSignatures bool         `json:"include_signatures"`   // 是否包含签名
	IncludeDetails    bool         `json:"include_details"`      // 是否包含详细信息
	Compress          bool         `json:"compress"`             // 是否压缩
	Timezone          string       `json:"timezone,omitempty"`   // 时区
}

// ExportResult 导出结果.
type ExportResult struct {
	Data        []byte       `json:"-"`            // 导出数据
	Format      ExportFormat `json:"format"`       // 导出格式
	Count       int          `json:"count"`        // 记录数
	Size        int64        `json:"size"`         // 文件大小
	Filename    string       `json:"filename"`     // 文件名
	ContentType string       `json:"content_type"` // 内容类型
}

// Export 执行导出.
func (e *Exporter) Export(req ExportRequest) (*ExportResult, error) {
	// 设置默认时间范围
	if req.StartTime.IsZero() {
		req.StartTime = time.Now().Add(-24 * time.Hour)
	}
	if req.EndTime.IsZero() {
		req.EndTime = time.Now()
	}

	// 查询日志
	opts := QueryOptions{
		StartTime: &req.StartTime,
		EndTime:   &req.EndTime,
		UserID:    req.UserID,
		Limit:     100000, // 导出最大数量
	}

	result, err := e.manager.Query(opts)
	if err != nil {
		return nil, err
	}

	// 应用分类过滤
	if len(req.Categories) > 0 {
		filtered := make([]*Entry, 0)
		for _, entry := range result.Entries {
			for _, cat := range req.Categories {
				if entry.Category == cat {
					filtered = append(filtered, entry)
					break
				}
			}
		}
		result.Entries = filtered
	}

	// 应用级别过滤
	if len(req.Levels) > 0 {
		filtered := make([]*Entry, 0)
		for _, entry := range result.Entries {
			for _, level := range req.Levels {
				if entry.Level == level {
					filtered = append(filtered, entry)
					break
				}
			}
		}
		result.Entries = filtered
	}

	// 如果不包含签名，清除签名字段
	if !req.IncludeSignatures {
		for _, entry := range result.Entries {
			entry.Signature = ""
		}
	}

	// 如果不包含详细信息，清除Details字段
	if !req.IncludeDetails {
		for _, entry := range result.Entries {
			entry.Details = nil
		}
	}

	// 根据格式导出
	var data []byte
	var contentType, filename string

	switch req.Format {
	case ExportJSON:
		data, err = e.exportJSON(result.Entries)
		contentType = "application/json"
		filename = e.generateFilename("json", req.StartTime, req.EndTime)
	case ExportCSV:
		data = e.exportCSV(result.Entries)
		contentType = "text/csv"
		filename = e.generateFilename("csv", req.StartTime, req.EndTime)
	case ExportYAML:
		data, err = e.exportYAML(result.Entries)
		contentType = "application/x-yaml"
		filename = e.generateFilename("yaml", req.StartTime, req.EndTime)
	default:
		data, err = e.exportJSON(result.Entries)
		contentType = "application/json"
		filename = e.generateFilename("json", req.StartTime, req.EndTime)
	}

	if err != nil {
		return nil, err
	}

	return &ExportResult{
		Data:        data,
		Format:      req.Format,
		Count:       len(result.Entries),
		Size:        int64(len(data)),
		Filename:    filename,
		ContentType: contentType,
	}, nil
}

// exportJSON 导出为JSON格式.
func (e *Exporter) exportJSON(entries []*Entry) ([]byte, error) {
	export := struct {
		ExportedAt time.Time `json:"exported_at"`
		Count      int       `json:"count"`
		Entries    []*Entry  `json:"entries"`
	}{
		ExportedAt: time.Now(),
		Count:      len(entries),
		Entries:    entries,
	}

	return json.MarshalIndent(export, "", "  ")
}

// exportCSV 导出为CSV格式.
func (e *Exporter) exportCSV(entries []*Entry) []byte {
	var buf strings.Builder

	// 写入表头
	buf.WriteString("ID,Timestamp,Level,Category,Event,UserID,Username,IP,UserAgent,Resource,Action,Status,Message,Details\n")

	// 写入数据行
	for _, entry := range entries {
		row := []string{
			escapeCSVField(entry.ID),
			entry.Timestamp.Format(time.RFC3339),
			string(entry.Level),
			string(entry.Category),
			escapeCSVField(entry.Event),
			escapeCSVField(entry.UserID),
			escapeCSVField(entry.Username),
			escapeCSVField(entry.IP),
			escapeCSVField(entry.UserAgent),
			escapeCSVField(entry.Resource),
			escapeCSVField(entry.Action),
			string(entry.Status),
			escapeCSVField(entry.Message),
			escapeCSVField(detailsToString(entry.Details)),
		}
		buf.WriteString(strings.Join(row, ",") + "\n")
	}

	return []byte(buf.String())
}

// exportYAML 导出为YAML格式.
func (e *Exporter) exportYAML(entries []*Entry) ([]byte, error) {
	export := struct {
		ExportedAt time.Time `yaml:"exported_at"`
		Count      int       `yaml:"count"`
		Entries    []*Entry  `yaml:"entries"`
	}{
		ExportedAt: time.Now(),
		Count:      len(entries),
		Entries:    entries,
	}

	return yaml.Marshal(export)
}

// escapeCSVField 转义CSV字段.
func escapeCSVField(field string) string {
	if field == "" {
		return ""
	}
	// 如果包含逗号、引号或换行，需要用引号包裹
	if strings.Contains(field, ",") || strings.Contains(field, "\"") || strings.Contains(field, "\n") {
		return "\"" + strings.ReplaceAll(field, "\"", "\"\"") + "\""
	}
	return field
}

// detailsToString 将Details转换为字符串.
func detailsToString(details map[string]interface{}) string {
	if len(details) == 0 {
		return ""
	}

	var parts []string
	for k, v := range details {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, "; ")
}

// generateFilename 生成文件名.
func (e *Exporter) generateFilename(format string, start, end time.Time) string {
	return fmt.Sprintf("audit-export-%s-%s.%s",
		start.Format("20060102"),
		end.Format("20060102"),
		format,
	)
}

// ========== Watch/Ignore List 导出 ==========

// WatchListExporter Watch/Ignore List导出器.
type WatchListExporter struct {
	manager *WatchListManager
}

// NewWatchListExporter 创建Watch/Ignore List导出器.
func NewWatchListExporter(manager *WatchListManager) *WatchListExporter {
	return &WatchListExporter{
		manager: manager,
	}
}

// WatchListExportRequest Watch/Ignore List导出请求.
type WatchListExportRequest struct {
	Format ExportFormat `json:"format"` // 导出格式
	Type   ListType     `json:"type"`   // 列表类型 (watch/ignore/all)
}

// WatchListExportResult 导出结果.
type WatchListExportResult struct {
	Data        []byte       `json:"-"`
	Format      ExportFormat `json:"format"`
	WatchCount  int          `json:"watch_count"`
	IgnoreCount int          `json:"ignore_count"`
	Size        int64        `json:"size"`
	Filename    string       `json:"filename"`
	ContentType string       `json:"content_type"`
}

// Export 导出Watch/Ignore List.
func (e *WatchListExporter) Export(req WatchListExportRequest) (*WatchListExportResult, error) {
	watchEntries := e.manager.ListWatchEntries(WatchListFilter{})
	ignoreEntries := e.manager.ListIgnoreEntries(IgnoreListFilter{})

	var data []byte
	var err error
	var contentType, filename string

	switch req.Format {
	case ExportJSON:
		data, err = e.exportJSON(watchEntries, ignoreEntries, req.Type)
		contentType = "application/json"
		filename = "watch-ign-list.json"
	case ExportCSV:
		data = e.exportCSV(watchEntries, ignoreEntries, req.Type)
		contentType = "text/csv"
		filename = "watch-ign-list.csv"
	case ExportYAML:
		data, err = e.exportYAML(watchEntries, ignoreEntries, req.Type)
		contentType = "application/x-yaml"
		filename = "watch-ign-list.yaml"
	default:
		data, err = e.exportJSON(watchEntries, ignoreEntries, req.Type)
		contentType = "application/json"
		filename = "watch-ign-list.json"
	}

	if err != nil {
		return nil, err
	}

	return &WatchListExportResult{
		Data:        data,
		Format:      req.Format,
		WatchCount:  len(watchEntries),
		IgnoreCount: len(ignoreEntries),
		Size:        int64(len(data)),
		Filename:    filename,
		ContentType: contentType,
	}, nil
}

// exportJSON 导出为JSON.
func (e *WatchListExporter) exportJSON(watchEntries []*WatchListEntry, ignoreEntries []*IgnoreListEntry, listType ListType) ([]byte, error) {
	export := struct {
		ExportedAt    time.Time          `json:"exported_at"`
		WatchEntries  []*WatchListEntry  `json:"watch_entries,omitempty"`
		IgnoreEntries []*IgnoreListEntry `json:"ignore_entries,omitempty"`
	}{
		ExportedAt: time.Now(),
	}

	if listType == ListTypeWatch || listType == "" {
		export.WatchEntries = watchEntries
	}
	if listType == ListTypeIgnore || listType == "" {
		export.IgnoreEntries = ignoreEntries
	}

	return json.MarshalIndent(export, "", "  ")
}

// exportCSV 导出为CSV.
func (e *WatchListExporter) exportCSV(watchEntries []*WatchListEntry, ignoreEntries []*IgnoreListEntry, listType ListType) []byte {
	var buf strings.Builder

	// Watch List
	if listType == ListTypeWatch || listType == "" {
		buf.WriteString("=== Watch List ===\n")
		buf.WriteString("ID,Path,Pattern,Operations,Recursive,Enabled,Description,CreatedAt,CreatedBy,Tags\n")
		for _, entry := range watchEntries {
			ops := make([]string, len(entry.Operations))
			for i, op := range entry.Operations {
				ops[i] = string(op)
			}
			row := []string{
				entry.ID,
				escapeCSVField(entry.Path),
				escapeCSVField(entry.Pattern),
				escapeCSVField(strings.Join(ops, ";")),
				strconv.FormatBool(entry.Recursive),
				strconv.FormatBool(entry.Enabled),
				escapeCSVField(entry.Description),
				entry.CreatedAt.Format(time.RFC3339),
				escapeCSVField(entry.CreatedBy),
				escapeCSVField(strings.Join(entry.Tags, ";")),
			}
			buf.WriteString(strings.Join(row, ",") + "\n")
		}
	}

	// Ignore List
	if listType == ListTypeIgnore || listType == "" {
		if listType == "" {
			buf.WriteString("\n")
		}
		buf.WriteString("=== Ignore List ===\n")
		buf.WriteString("ID,Path,Pattern,Reason,Enabled,Description,CreatedAt,CreatedBy,ExpiresAt,Tags\n")
		for _, entry := range ignoreEntries {
			expiresAt := ""
			if entry.ExpiresAt != nil {
				expiresAt = entry.ExpiresAt.Format(time.RFC3339)
			}
			row := []string{
				entry.ID,
				escapeCSVField(entry.Path),
				escapeCSVField(entry.Pattern),
				escapeCSVField(entry.Reason),
				strconv.FormatBool(entry.Enabled),
				escapeCSVField(entry.Description),
				entry.CreatedAt.Format(time.RFC3339),
				escapeCSVField(entry.CreatedBy),
				expiresAt,
				escapeCSVField(strings.Join(entry.Tags, ";")),
			}
			buf.WriteString(strings.Join(row, ",") + "\n")
		}
	}

	return []byte(buf.String())
}

// exportYAML 导出为YAML.
func (e *WatchListExporter) exportYAML(watchEntries []*WatchListEntry, ignoreEntries []*IgnoreListEntry, listType ListType) ([]byte, error) {
	export := struct {
		ExportedAt    time.Time          `yaml:"exported_at"`
		WatchEntries  []*WatchListEntry  `yaml:"watch_entries,omitempty"`
		IgnoreEntries []*IgnoreListEntry `yaml:"ignore_entries,omitempty"`
	}{
		ExportedAt: time.Now(),
	}

	if listType == ListTypeWatch || listType == "" {
		export.WatchEntries = watchEntries
	}
	if listType == ListTypeIgnore || listType == "" {
		export.IgnoreEntries = ignoreEntries
	}

	return yaml.Marshal(export)
}

// ========== 导出API处理器 ==========

// ExportHandlers 导出API处理器.
type ExportHandlers struct {
	auditExporter     *Exporter
	watchListExporter *WatchListExporter
}

// NewExportHandlers 创建导出处理器.
func NewExportHandlers(auditExporter *Exporter, watchListExporter *WatchListExporter) *ExportHandlers {
	return &ExportHandlers{
		auditExporter:     auditExporter,
		watchListExporter: watchListExporter,
	}
}

// RegisterRoutes 注册导出路由.
func (h *ExportHandlers) RegisterRoutes(api *gin.RouterGroup) {
	export := api.Group("/audit/export")
	{
		export.POST("", h.ExportAuditLogs)
		export.POST("/list", h.ExportWatchList)
	}
}

// ExportAuditLogsRequest 导出审计日志请求.
type ExportAuditLogsRequest struct {
	Format            ExportFormat `json:"format"`
	StartTime         string       `json:"start_time,omitempty"`
	EndTime           string       `json:"end_time,omitempty"`
	Categories        []Category   `json:"categories,omitempty"`
	Levels            []Level      `json:"levels,omitempty"`
	UserID            string       `json:"user_id,omitempty"`
	IncludeSignatures bool         `json:"include_signatures"`
	IncludeDetails    bool         `json:"include_details"`
}

// ExportAuditLogs 导出审计日志
// @Summary 导出审计日志
// @Description 导出指定时间范围的审计日志（支持JSON/CSV/YAML格式）
// @Tags audit-export
// @Accept json
// @Produce octet-stream
// @Param request body ExportAuditLogsRequest true "导出参数"
// @Success 200 {file} file
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /audit/export [post]
// @Security BearerAuth.
func (h *ExportHandlers) ExportAuditLogs(c *gin.Context) {
	var req ExportAuditLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 设置默认格式
	if req.Format == "" {
		req.Format = ExportJSON
	}

	// 解析时间
	var startTime, endTime time.Time
	var err error

	if req.StartTime != "" {
		startTime, err = time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的开始时间格式"))
			return
		}
	}
	if req.EndTime != "" {
		endTime, err = time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的结束时间格式"))
			return
		}
	}

	exportReq := ExportRequest{
		Format:            req.Format,
		StartTime:         startTime,
		EndTime:           endTime,
		Categories:        req.Categories,
		Levels:            req.Levels,
		UserID:            req.UserID,
		IncludeSignatures: req.IncludeSignatures,
		IncludeDetails:    req.IncludeDetails,
	}

	result, err := h.auditExporter.Export(exportReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, "导出失败"))
		return
	}

	c.Header("Content-Type", result.ContentType)
	c.Header("Content-Disposition", "attachment; filename="+result.Filename)
	c.Data(http.StatusOK, result.ContentType, result.Data)
}

// ExportWatchListRequest 导出Watch/Ignore List请求.
type ExportWatchListRequest struct {
	Format ExportFormat `json:"format"`
	Type   ListType     `json:"type"` // watch/ignore/all
}

// ExportWatchList 导出Watch/Ignore List
// @Summary 导出监控/忽略列表
// @Description 导出监控列表或忽略列表（支持JSON/CSV/YAML格式）
// @Tags audit-export
// @Accept json
// @Produce octet-stream
// @Param request body ExportWatchListRequest true "导出参数"
// @Success 200 {file} file
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /audit/export/list [post]
// @Security BearerAuth.
func (h *ExportHandlers) ExportWatchList(c *gin.Context) {
	var req ExportWatchListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 设置默认值
	if req.Format == "" {
		req.Format = ExportJSON
	}
	if req.Type == "" {
		req.Type = "" // 导出所有
	}

	result, err := h.watchListExporter.Export(WatchListExportRequest(req))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, "导出失败"))
		return
	}

	c.Header("Content-Type", result.ContentType)
	c.Header("Content-Disposition", "attachment; filename="+result.Filename)
	c.Data(http.StatusOK, result.ContentType, result.Data)
}
