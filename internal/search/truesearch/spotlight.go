// Package truesearch 实现全文搜索引擎 (TrueSearch Phase 2)
// 本文件实现 macOS Spotlight 集成，支持 SMB Spotlight 协议，
// 使 macOS 客户端可通过 Spotlight 直接搜索 NAS 上的文件。
package truesearch

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ─── SMB Spotlight 协议支持 ───────────────────────────────────
//
// macOS Spotlight over SMB 使用以下协议流程：
// 1. 客户端通过 SMB 连接到 NAS 共享
// 2. 客户端发送 Spotlight 搜索请求（基于 Spotlight RPC 或 AFP over SMB）
// 3. NAS 端解析搜索查询，在本地索引中执行搜索
// 4. 将结果转换为 Spotlight 兼容的元数据格式返回
//
// TrueNAS 26 的 TrueSearch 实现了类似的 Spotlight 网关，
// 将 macOS Spotlight 查询转换为后端全文搜索引擎的查询。

// SpotlightServer 提供 macOS Spotlight 搜索的 HTTP/RPC 端点。
// 可集成到 SMB 服务中，处理来自 macOS 客户端的 Spotlight 搜索请求。
type SpotlightServer struct {
	engine     *Engine
	logger     *zap.Logger
	mu         sync.RWMutex
	sharePaths map[string]string // 共享名 → 本地路径映射
}

// NewSpotlightServer 创建 Spotlight 服务器。
func NewSpotlightServer(engine *Engine, logger *zap.Logger) *SpotlightServer {
	return &SpotlightServer{
		engine:     engine,
		logger:     logger,
		sharePaths: make(map[string]string),
	}
}

// RegisterShare 注册 SMB 共享路径映射。
// shareName 是 SMB 共享名，localPath 是对应的本地文件系统路径。
func (s *SpotlightServer) RegisterShare(shareName, localPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sharePaths[shareName] = localPath
}

// UnregisterShare 取消注册 SMB 共享。
func (s *SpotlightServer) UnregisterShare(shareName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sharePaths, shareName)
}

// GetSharePath 获取共享名对应的本地路径。
func (s *SpotlightServer) GetSharePath(shareName string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, ok := s.sharePaths[shareName]
	return path, ok
}

// ListShares 列出所有注册的共享。
func (s *SpotlightServer) ListShares() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string, len(s.sharePaths))
	for k, v := range s.sharePaths {
		result[k] = v
	}
	return result
}

// ─── Spotlight 查询解析 ───────────────────────────────────────

// SpotlightQuery macOS Spotlight 查询请求。
type SpotlightQuery struct {
	Query      string `json:"query" xml:"query"`           // Spotlight 查询字符串
	ShareName  string `json:"shareName" xml:"shareName"`   // SMB 共享名
	Path       string `json:"path,omitempty" xml:"path"`   // 搜索路径限制
	MaxResults int    `json:"maxResults" xml:"maxResults"` // 最大结果数
	SortBy     string `json:"sortBy,omitempty" xml:"sortBy"` // 排序字段
}

// SpotlightResult Spotlight 搜索结果项。
type SpotlightResult struct {
	Path         string            `json:"path" xml:"path"`
	FileName     string            `json:"fileName" xml:"fileName"`
	FileSize     int64             `json:"fileSize" xml:"fileSize"`
	Modification time.Time         `json:"modification" xml:"modification"`
	Score        float64           `json:"score" xml:"score"`
	Snippet      string            `json:"snippet,omitempty" xml:"snippet"`
	Kind         string            `json:"kind" xml:"kind"`
	Metadata     map[string]string `json:"metadata,omitempty" xml:"metadata"`
}

// SpotlightResponse Spotlight 搜索响应。
type SpotlightResponse struct {
	Results []SpotlightResult `json:"results" xml:"results"`
	Total   int               `json:"total" xml:"total"`
	TookMs  int64             `json:"tookMs" xml:"tookMs"`
	Error   string            `json:"error,omitempty" xml:"error,omitempty"`
}

// ─── Spotlight 查询转换 ───────────────────────────────────────

// spotlightQueryMap 将 Spotlight 查询语法关键字映射到 TrueSearch 查询。
// macOS Spotlight 使用 kMDItemxxx 格式的属性查询，需要转换为全文搜索。
var spotlightQueryMap = map[string]string{
	"kMDItemDisplayName":              "name",
	"kMDItemTextContent":              "content",
	"kMDItemFSName":                   "name",
	"kMDItemContentType":              "ext",
	"kMDItemFSSize":                   "size",
	"kMDItemFSCreationDate":           "modTime",
	"kMDItemContentModificationDate":  "modTime",
}

// kindMap 文件扩展名到 macOS kind 类型映射。
var kindMap = map[string]string{
	".txt":  "Plain Text",
	".md":   "Markdown",
	".pdf":  "PDF Document",
	".docx": "Microsoft Word",
	".json": "JSON Data",
	".yaml": "YAML Config",
	".yml":  "YAML Config",
	".xml":  "XML Data",
	".csv":  "CSV Data",
	".log":  "Log File",
	".go":   "Go Source",
	".py":   "Python Source",
	".js":   "JavaScript Source",
	".ts":   "TypeScript Source",
	".rs":   "Rust Source",
	".java": "Java Source",
	".c":    "C Source",
	".cpp":  "C++ Source",
	".h":    "Header File",
	".sh":   "Shell Script",
	".conf": "Config File",
	".cfg":  "Config File",
	".ini":  "INI Config",
}

// ParseSpotlightQuery 解析 macOS Spotlight 查询字符串。
// 将 kMDItemxxx = "value" 格式转换为 TrueSearch 查询。
// 不识别的属性会被提取为通用搜索词。
func ParseSpotlightQuery(query string) SearchRequest {
	req := SearchRequest{
		MaxResults: 50,
	}

	// Spotlight 查询格式示例:
	// kMDItemTextContent == "search term"c
	// kMDItemDisplayName == "filename"
	// "plain text search"

	// 按双引号分割，提取搜索词
	terms := extractQuotedStrings(query)
	if len(terms) == 0 {
		// 没有引号，直接使用原始查询
		req.Query = strings.TrimSpace(query)
		return req
	}

	// 检查是否有属性限定符
	for attr, field := range spotlightQueryMap {
		if strings.Contains(query, attr) {
			// 这是一个属性限定查询
			for _, term := range terms {
				switch field {
				case "name":
					if req.Query == "" {
						req.Query = term
					}
				case "ext":
					req.Types = append(req.Types, term)
				default:
					// 内容搜索
					if req.Query == "" {
						req.Query = term
					}
				}
			}
			return req
		}
	}

	// 没有属性限定，使用第一个引号内容作为查询
	req.Query = terms[0]
	return req
}

// extractQuotedStrings 从字符串中提取所有双引号内的内容。
func extractQuotedStrings(s string) []string {
	var result []string
	inQuote := false
	var current strings.Builder

	for _, ch := range s {
		if ch == '"' {
			if inQuote {
				result = append(result, current.String())
				current.Reset()
			}
			inQuote = !inQuote
		} else if inQuote {
			current.WriteRune(ch)
		}
	}

	return result
}

// getKind 根据文件扩展名返回 macOS Spotlight kind 类型。
func getKind(ext string) string {
	ext = strings.ToLower(ext)
	if kind, ok := kindMap[ext]; ok {
		return kind
	}
	return "Document"
}

// toSpotlightResult 将 TrueSearch 搜索结果转换为 Spotlight 结果。
func (s *SpotlightServer) toSpotlightResult(result SearchResult, shareName string) SpotlightResult {
	localPath := result.Path
	sharePath, ok := s.GetSharePath(shareName)
	if ok {
		// 将本地路径转换为 SMB 路径
		localPath = toSMBPath(localPath, sharePath, shareName)
	}

	kind := getKind(filepath.Ext(result.Path))

	metadata := map[string]string{
		"kMDItemDisplayName": result.Name,
		"kMDItemFSName":      result.Name,
		"kMDItemFSSize":      fmt.Sprintf("%d", result.Size),
		"kMDItemKind":        kind,
	}

	if result.ModTime != "" {
		metadata["kMDItemContentModificationDate"] = result.ModTime
	}

	return SpotlightResult{
		Path:     localPath,
		FileName: result.Name,
		FileSize: result.Size,
		Score:    result.Score,
		Snippet:  result.Snippet,
		Kind:     kind,
		Metadata: metadata,
	}
}

// toSMBPath 将本地文件系统路径转换为 SMB 路径格式。
// 例如: /mnt/pool/data/file.txt → smb://nas/share/file.txt
func toSMBPath(localPath, sharePath, shareName string) string {
	rel := strings.TrimPrefix(localPath, sharePath)
	rel = strings.TrimPrefix(rel, "/")
	return fmt.Sprintf("smb://nas/%s/%s", shareName, rel)
}

// Search 执行 Spotlight 搜索。
func (s *SpotlightServer) Search(spotlightReq SpotlightQuery) (*SpotlightResponse, error) {
	start := time.Now()

	if spotlightReq.Query == "" {
		return &SpotlightResponse{
			Error: "查询不能为空",
		}, nil
	}

	// 解析 Spotlight 查询
	searchReq := ParseSpotlightQuery(spotlightReq.Query)
	if spotlightReq.MaxResults > 0 {
		searchReq.MaxResults = spotlightReq.MaxResults
	}

	// 限制搜索路径到共享目录
	if spotlightReq.ShareName != "" {
		sharePath, ok := s.GetSharePath(spotlightReq.ShareName)
		if !ok {
			return &SpotlightResponse{
				Error: fmt.Sprintf("未知的共享: %s", spotlightReq.ShareName),
			}, nil
		}
		if searchReq.Path == "" {
			searchReq.Path = sharePath
		}
	}

	// 执行搜索
	resp, err := s.engine.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("spotlight search: %w", err)
	}

	// 转换结果
	results := make([]SpotlightResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, s.toSpotlightResult(r, spotlightReq.ShareName))
	}

	return &SpotlightResponse{
		Results: results,
		Total:   resp.Total,
		TookMs:  time.Since(start).Milliseconds(),
	}, nil
}

// ─── Spotlight XML RPC 协议 ───────────────────────────────────

// spotlightXMLEnvelope Spotlight XML RPC 响应信封。
type spotlightXMLEnvelope struct {
	XMLName xml.Name           `xml:"methodResponse"`
	Params  spotlightXMLParams `xml:"params"`
}

type spotlightXMLParams struct {
	Param spotlightXMLParam `xml:"param"`
}

type spotlightXMLParam struct {
	Value spotlightXMLValue `xml:"value"`
}

type spotlightXMLValue struct {
	Array  *spotlightXMLArray `xml:"array,omitempty"`
	String string             `xml:"string,omitempty"`
}

type spotlightXMLArray struct {
	Data spotlightXMLData `xml:"data"`
}

type spotlightXMLData struct {
	Values []spotlightXMLValue `xml:"value"`
}

// SearchXML 处理 Spotlight XML RPC 搜索请求。
// 返回 XML 格式的搜索结果，兼容 macOS Spotlight 协议。
func (s *SpotlightServer) SearchXML(query string, shareName string) (string, error) {
	req := SpotlightQuery{
		Query:      query,
		ShareName:  shareName,
		MaxResults: 50,
	}

	resp, err := s.Search(req)
	if err != nil {
		return "", err
	}

	// 构建 XML 响应
	envelope := spotlightXMLEnvelope{}
	array := &spotlightXMLArray{}
	for _, r := range resp.Results {
		val := spotlightXMLValue{
			String: fmt.Sprintf("%s\t%s\t%d\t%s\t%f",
				r.Path, r.FileName, r.FileSize, r.Kind, r.Score),
		}
		array.Data.Values = append(array.Data.Values, val)
	}
	envelope.Params.Param.Value.Array = array

	data, err := xml.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal XML: %w", err)
	}

	return xml.Header + string(data), nil
}

// ─── Spotlight HTTP Handler ───────────────────────────────────

// SpotlightHandler Spotlight HTTP handler，可注册到 Web 服务器。
type SpotlightHandler struct {
	server *SpotlightServer
	logger *zap.Logger
}

// NewSpotlightHandler 创建 Spotlight HTTP handler。
func NewSpotlightHandler(server *SpotlightServer, logger *zap.Logger) *SpotlightHandler {
	return &SpotlightHandler{
		server: server,
		logger: logger,
	}
}

// RegisterRoutesGin 注册 Spotlight 路由到 gin 路由组。
func (h *SpotlightHandler) RegisterRoutesGin(r *gin.RouterGroup) {
	sp := r.Group("/spotlight")
	{
		sp.GET("", h.handleGinGet)
		sp.POST("", h.handleGinPost)
		sp.GET("/meta", h.handleGinMeta)
		sp.GET("/attributes", h.handleGinAttributes)
	}
}

// handleGinGet 处理 gin GET 请求。
func (h *SpotlightHandler) handleGinGet(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少查询参数 q"})
		return
	}

	share := c.Query("share")
	format := c.Query("format")

	if format == "xml" {
		result, err := h.server.SearchXML(q, share)
		if err != nil {
			h.logger.Error("spotlight XML search failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "搜索失败"})
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(result))
		return
	}

	req := SpotlightQuery{
		Query:     q,
		ShareName: share,
	}
	if maxStr := c.Query("max"); maxStr != "" {
		var max int
		if _, err := fmt.Sscanf(maxStr, "%d", &max); err == nil && max > 0 {
			req.MaxResults = max
		}
	}

	resp, err := h.server.Search(req)
	if err != nil {
		h.logger.Error("spotlight search failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "搜索失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp})
}

// handleGinPost 处理 gin POST 请求。
func (h *SpotlightHandler) handleGinPost(c *gin.Context) {
	var req SpotlightQuery
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的请求参数: " + err.Error()})
		return
	}

	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "query 不能为空"})
		return
	}

	resp, err := h.server.Search(req)
	if err != nil {
		h.logger.Error("spotlight search failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "搜索失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp})
}

// handleGinMeta 处理元数据同步请求。
func (h *SpotlightHandler) handleGinMeta(c *gin.Context) {
	share := c.Query("share")
	path := c.Query("path")
	if share == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少 share 或 path 参数"})
		return
	}

	meta, err := h.server.SyncMetadata(share, path)
	if err != nil {
		h.logger.Error("sync metadata failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "元数据同步失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": meta})
}

// handleGinAttributes 返回支持的 Spotlight 属性列表。
func (h *SpotlightHandler) handleGinAttributes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"attributes": SupportedSpotlightAttributes(),
			"kinds":      SupportedKinds(),
		},
	})
}

// HandleHTTP 处理原生 HTTP 请求。
// 支持 GET /spotlight?q=...&share=... 和 POST /spotlight 两种方式。
func (h *SpotlightHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handlePost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet 处理 GET 请求。
func (h *SpotlightHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "Missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	share := r.URL.Query().Get("share")
	format := r.URL.Query().Get("format")

	if format == "xml" {
		result, err := h.server.SearchXML(q, share)
		if err != nil {
			h.logger.Error("spotlight XML search failed", zap.Error(err))
			http.Error(w, "Search failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = io.WriteString(w, result)
		return
	}

	req := SpotlightQuery{
		Query:     q,
		ShareName: share,
	}
	if maxStr := r.URL.Query().Get("max"); maxStr != "" {
		var max int
		if _, err := fmt.Sscanf(maxStr, "%d", &max); err == nil && max > 0 {
			req.MaxResults = max
		}
	}

	resp, err := h.server.Search(req)
	if err != nil {
		h.logger.Error("spotlight search failed", zap.Error(err))
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	writeJSON(w, resp)
}

// handlePost 处理 POST 请求。
func (h *SpotlightHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read body failed", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req SpotlightQuery
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	resp, err := h.server.Search(req)
	if err != nil {
		h.logger.Error("spotlight search failed", zap.Error(err))
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	writeJSON(w, resp)
}

// writeJSON 将对象序列化为 JSON 并写入 ResponseWriter。
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "JSON encode failed", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

// ─── Spotlight 属性同步 ───────────────────────────────────────

// SpotlightMetadata 同步 Spotlight 元数据属性。
// 当索引文件时，将 TrueSearch 的元数据映射到 macOS Spotlight 属性，
// 使 macOS Finder 的 "获取信息" 和 Spotlight 搜索能正确显示文件属性。
type SpotlightMetadata struct {
	ShareName string            `json:"shareName"`
	Path      string            `json:"path"`
	Props     map[string]string `json:"props"`
}

// SyncMetadata 同步文件的 Spotlight 元数据。
// 将 TrueSearch 索引信息转换为 macOS Spotlight 属性。
func (s *SpotlightServer) SyncMetadata(shareName, localPath string) (*SpotlightMetadata, error) {
	sharePath, ok := s.GetSharePath(shareName)
	if !ok {
		return nil, fmt.Errorf("unknown share: %s", shareName)
	}

	props := make(map[string]string)

	// 优先从文件系统获取元数据
	info, err := os.Stat(localPath)
	if err == nil {
		name := filepath.Base(localPath)
		props["kMDItemDisplayName"] = name
		props["kMDItemFSName"] = name
		props["kMDItemFSSize"] = fmt.Sprintf("%d", info.Size())
		props["kMDItemKind"] = getKind(filepath.Ext(localPath))
		props["kMDItemContentModificationDate"] = info.ModTime().Format(time.RFC3339)
	}

	// 尝试从搜索索引获取额外元数据
	resp, err := s.engine.Search(SearchRequest{
		Query:      filepath.Base(localPath),
		Path:       sharePath,
		MaxResults: 1,
	})
	if err == nil && len(resp.Results) > 0 {
		r := resp.Results[0]
		if r.Snippet != "" {
			props["kMDItemTextContent"] = r.Snippet
		}
		if r.ModTime != "" {
			props["kMDItemContentModificationDate"] = r.ModTime
		}
	}

	return &SpotlightMetadata{
		ShareName: shareName,
		Path:      toSMBPath(localPath, sharePath, shareName),
		Props:     props,
	}, nil
}

// ─── Spotlight 搜索属性列表 ───────────────────────────────────

// SupportedSpotlightAttributes 返回支持的 Spotlight 属性列表。
// macOS 客户端可据此了解 NAS 支持哪些 Spotlight 搜索属性。
func SupportedSpotlightAttributes() []string {
	attrs := make([]string, 0, len(spotlightQueryMap))
	for k := range spotlightQueryMap {
		attrs = append(attrs, k)
	}
	return attrs
}

// SupportedKinds 返回支持的文件类型列表。
func SupportedKinds() []string {
	kinds := make(map[string]bool)
	for _, k := range kindMap {
		kinds[k] = true
	}
	result := make([]string, 0, len(kinds))
	for k := range kinds {
		result = append(result, k)
	}
	return result
}
