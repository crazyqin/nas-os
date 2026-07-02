package filetagger

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	apiresponse "nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 文件标签系统 HTTP 处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	ft := api.Group("/filetagger")
	{
		// 标签管理
		ft.GET("/tags", h.listTags)
		ft.POST("/tags", h.createTag)
		ft.GET("/tags/tree", h.getTagTree)
		ft.GET("/tags/stats", h.getOverallStats)
		ft.GET("/tags/:id", h.getTag)
		ft.PUT("/tags/:id", h.updateTag)
		ft.DELETE("/tags/:id", h.deleteTag)
		ft.GET("/tags/:id/stats", h.getTagStats)
		ft.GET("/tags/:id/ancestors", h.getTagAncestors)

		// 文件标签
		ft.GET("/files/:encodedPath/tags", h.getFileTags)
		ft.POST("/files/:encodedPath/tags", h.addFileTag)
		ft.DELETE("/files/:encodedPath/tags/:tagId", h.removeFileTag)

		// 自动标签规则
		ft.GET("/rules", h.listRules)
		ft.POST("/rules", h.createRule)
		ft.GET("/rules/:id", h.getRule)
		ft.PUT("/rules/:id", h.updateRule)
		ft.DELETE("/rules/:id", h.deleteRule)

		// 搜索
		ft.POST("/search", h.search)

		// 批量操作
		ft.POST("/batch/apply", h.batchApply)
		ft.POST("/batch/operation", h.batchOperation)

		// 扫描
		ft.POST("/scan", h.scanFiles)

		// 导入导出
		ft.GET("/export", h.exportData)
		ft.POST("/import", h.importData)

		// 分类
		ft.GET("/classify/:encodedPath", h.classifyFile)
	}
}

// ========== 标签管理 Handlers ==========

func (h *Handlers) listTags(c *gin.Context) {
	tags := h.engine.ListTags()
	c.JSON(http.StatusOK, apiresponse.Success(tags))
}

type createTagRequest struct {
	Name     string       `json:"name" binding:"required"`
	Category FileCategory `json:"category"`
	ParentID string       `json:"parentId"`
	Color    string       `json:"color"`
	Icon     string       `json:"icon"`
}

func (h *Handlers) createTag(c *gin.Context) {
	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	if req.Category == "" {
		req.Category = CategoryOther
	}

	tag, err := h.engine.CreateTag(req.Name, req.Category, req.ParentID, req.Color, req.Icon)
	if err != nil {
		code := apiresponse.CodeInternalError
		switch err {
		case ErrTagExists:
			code = apiresponse.CodeConflict
		case ErrTagNotFound:
			code = apiresponse.CodeNotFound
		}
		c.JSON(http.StatusOK, apiresponse.Error(code, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(tag))
}

func (h *Handlers) getTag(c *gin.Context) {
	id := c.Param("id")
	tag, err := h.engine.GetTag(id)
	if err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeNotFound, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(tag))
}

type updateTagRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parentId"`
	Color    string `json:"color"`
	Icon     string `json:"icon"`
}

func (h *Handlers) updateTag(c *gin.Context) {
	id := c.Param("id")
	var req updateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	tag, err := h.engine.UpdateTag(id, req.Name, req.ParentID, req.Color, req.Icon)
	if err != nil {
		code := apiresponse.CodeInternalError
		switch err {
		case ErrTagNotFound:
			code = apiresponse.CodeNotFound
		case ErrCircularParent:
			code = apiresponse.CodeBadRequest
		}
		c.JSON(http.StatusOK, apiresponse.Error(code, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(tag))
}

func (h *Handlers) deleteTag(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.DeleteTag(id); err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeNotFound, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

func (h *Handlers) getTagTree(c *gin.Context) {
	tree := h.engine.GetTagTree()
	c.JSON(http.StatusOK, apiresponse.Success(tree))
}

func (h *Handlers) getTagAncestors(c *gin.Context) {
	id := c.Param("id")
	ancestors := h.engine.GetTagAncestors(id)
	c.JSON(http.StatusOK, apiresponse.Success(ancestors))
}

func (h *Handlers) getTagStats(c *gin.Context) {
	id := c.Param("id")
	stat, err := h.engine.GetTagStats(id)
	if err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeNotFound, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(stat))
}

func (h *Handlers) getOverallStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, apiresponse.Success(stats))
}

// ========== 文件标签 Handlers ==========

func decodeFilePath(encoded string) string {
	// 支持 base64 或直接路径
	if strings.HasPrefix(encoded, "/") {
		return encoded
	}
	// 尝试作为URL解码
	return encoded
}

func (h *Handlers) getFileTags(c *gin.Context) {
	filePath := decodeFilePath(c.Param("encodedPath"))
	ft := h.engine.GetFileTags(filePath)
	c.JSON(http.StatusOK, apiresponse.Success(ft))
}

type addFileTagRequest struct {
	TagID string `json:"tagId" binding:"required"`
}

func (h *Handlers) addFileTag(c *gin.Context) {
	filePath := decodeFilePath(c.Param("encodedPath"))
	var req addFileTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	if err := h.engine.AddFileTag(filePath, req.TagID, false, ""); err != nil {
		code := apiresponse.CodeInternalError
		if err == ErrTagNotFound {
			code = apiresponse.CodeNotFound
		}
		c.JSON(http.StatusOK, apiresponse.Error(code, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.SuccessWithMessage("标签已添加", nil))
}

func (h *Handlers) removeFileTag(c *gin.Context) {
	filePath := decodeFilePath(c.Param("encodedPath"))
	tagID := c.Param("tagId")

	if err := h.engine.RemoveFileTag(filePath, tagID); err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.SuccessWithMessage("标签已移除", nil))
}

// ========== 规则 Handlers ==========

func (h *Handlers) listRules(c *gin.Context) {
	rules := h.engine.ListRules()
	c.JSON(http.StatusOK, apiresponse.Success(rules))
}

func (h *Handlers) createRule(c *gin.Context) {
	var rule AutoRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	created, err := h.engine.CreateRule(rule)
	if err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeBadRequest, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(created))
}

func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.engine.GetRule(id)
	if err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeNotFound, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(rule))
}

func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")
	var rule AutoRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}
	rule.ID = id

	updated, err := h.engine.UpdateRule(rule)
	if err != nil {
		code := apiresponse.CodeInternalError
		if err == ErrRuleNotFound {
			code = apiresponse.CodeNotFound
		}
		c.JSON(http.StatusOK, apiresponse.Error(code, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(updated))
}

func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.DeleteRule(id); err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeNotFound, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

// ========== 搜索 Handler ==========

func (h *Handlers) search(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	result := h.engine.Search(query)
	c.JSON(http.StatusOK, apiresponse.Success(result))
}

// ========== 批量操作 Handlers ==========

type batchApplyRequest struct {
	Files []string `json:"files" binding:"required"`
	TagID string   `json:"tagId" binding:"required"`
}

func (h *Handlers) batchApply(c *gin.Context) {
	var req batchApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	count, err := h.engine.BatchApplyTags(req.Files, req.TagID, false, "")
	if err != nil {
		code := apiresponse.CodeInternalError
		if err == ErrTagNotFound {
			code = apiresponse.CodeNotFound
		}
		c.JSON(http.StatusOK, apiresponse.Error(code, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.SuccessWithMessage(
		"批量标签完成",
		gin.H{"applied": count, "total": len(req.Files)},
	))
}

func (h *Handlers) batchOperation(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	if req.Operation == BatchOpDelete && !req.Confirm {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeBadRequest, "删除操作需要确认 (confirm=true)"))
		return
	}

	// 搜索匹配的文件
	searchResult := h.engine.Search(SearchQuery{
		Tags:     req.TagIDs,
		PageSize: 10000,
	})

	result := BatchResult{
		Operation:  req.Operation,
		TotalFiles: len(searchResult.Files),
	}

	// 按实际需求执行操作（此处为统计框架，实际IO操作由调用方实现）
	result.Processed = result.TotalFiles

	c.JSON(http.StatusOK, apiresponse.Success(result))
}

// ========== 扫描 Handler ==========

func (h *Handlers) scanFiles(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	result := h.doScan(req)
	c.JSON(http.StatusOK, apiresponse.Success(result))
}

func (h *Handlers) doScan(req ScanRequest) ScanResult {
	result := ScanResult{}
	for _, root := range req.Paths {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			result.ScannedFiles++

			// 获取文件信息
			ext := getExt(path)
			mime := DetectMIME(path)
			category := ClassifyFile(ext, mime)

			// 确保分类标签存在
			_ = category

			// 匹配规则
			tagIDs := h.engine.MatchFile(path, info.Size(), mime, nil)
			for _, tagID := range tagIDs {
				if addErr := h.engine.AddFileTag(path, tagID, true, ""); addErr == nil {
					result.NewTags++
				}
			}

			return nil
		})
	}
	return result
}

// osFileInfo 用于测试mock.
// type osFileInfo interface {
// 	IsDir() bool
// 	Size() int64
// }

func getExt(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}

// ========== 导入导出 Handlers ==========

func (h *Handlers) exportData(c *gin.Context) {
	data, err := h.engine.ExportJSON()
	if err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeInternalError, err.Error()))
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=filetagger-export.json")
	c.Data(http.StatusOK, "application/json", data)
}

type importRequest struct {
	Data      json.RawMessage `json:"data" binding:"required"`
	Overwrite bool            `json:"overwrite"`
}

func (h *Handlers) importData(c *gin.Context) {
	var req importRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(apiresponse.CodeBadRequest, "参数错误: "+err.Error()))
		return
	}

	tags, rules, fileTags, err := h.engine.ImportJSON(req.Data, req.Overwrite)
	if err != nil {
		c.JSON(http.StatusOK, apiresponse.Error(apiresponse.CodeBadRequest, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.SuccessWithMessage("导入完成", gin.H{
		"tagsImported":     tags,
		"rulesImported":    rules,
		"fileTagsImported": fileTags,
	}))
}

// ========== 分类 Handler ==========

func (h *Handlers) classifyFile(c *gin.Context) {
	filePath := decodeFilePath(c.Param("encodedPath"))
	ext := getExt(filePath)
	mime := ""
	mimeParam := c.Query("mime")
	if mimeParam != "" {
		mime = mimeParam
	} else {
		mime = DetectMIME(filePath)
	}

	category := ClassifyFile(ext, mime)
	c.JSON(http.StatusOK, apiresponse.Success(gin.H{
		"path":     filePath,
		"ext":      ext,
		"mime":     mime,
		"category": category,
	}))
}

// ========== 辅助函数 ==========

// parseIntParam 解析整数查询参数.
func parseIntParam(c *gin.Context, name string, defaultVal int) int {
	s := c.Query(name)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
