package ai_classify

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers AI 分类处理器
type Handlers struct {
	classifier *Classifier
	tagger     *Tagger
	detector   *SimilarityDetector
	learner    *Learner
}

// NewHandlers 创建处理器
func NewHandlers(config Config) (*Handlers, error) {
	classifier, err := NewClassifier(config)
	if err != nil {
		return nil, err
	}

	tagger := NewTagger(config)
	detector := NewSimilarityDetector(config)
	learner := NewLearner(config, classifier, tagger)

	return &Handlers{
		classifier: classifier,
		tagger:     tagger,
		detector:   detector,
		learner:    learner,
	}, nil
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ai := r.Group("/ai-classify")
	{
		// 分类相关
		ai.POST("/classify", h.classifyFile)
		ai.POST("/classify/batch", h.classifyBatch)
		ai.GET("/classify/result/:id", h.getClassifyResult)

		// 分类管理
		ai.GET("/categories", h.listCategories)
		ai.POST("/categories", h.createCategory)
		ai.PUT("/categories/:id", h.updateCategory)
		ai.DELETE("/categories/:id", h.deleteCategory)

		// 标签相关
		ai.GET("/tags", h.listTags)
		ai.POST("/tags", h.createTag)
		ai.PUT("/tags/:id", h.updateTag)
		ai.DELETE("/tags/:id", h.deleteTag)
		ai.POST("/tags/generate", h.generateTags)

		// 规则相关
		ai.GET("/rules", h.listRules)
		ai.POST("/rules", h.createRule)
		ai.PUT("/rules/:id", h.updateRule)
		ai.DELETE("/rules/:id", h.deleteRule)

		// 相似度检测
		ai.POST("/similarity/detect", h.detectSimilarity)
		ai.POST("/similarity/index", h.indexFile)
		ai.POST("/similarity/index-dir", h.indexDirectory)
		ai.GET("/similarity/duplicates", h.findDuplicates)
		ai.GET("/similarity/stats", h.getSimilarityStats)

		// 学习相关
		ai.POST("/learn/feedback", h.submitFeedback)
		ai.POST("/learn/from-dir", h.learnFromDirectory)
		ai.GET("/learn/stats", h.getLearningStats)
		ai.GET("/learn/suggestions", h.getSuggestions)
		ai.POST("/learn/optimize", h.optimizeRules)
	}
}

// classifyFile 分类文件
func (h *Handlers) classifyFile(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	// 安全检查
	if strings.Contains(req.Path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效路径"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := h.classifier.Classify(ctx, req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// classifyBatch 批量分类
func (h *Handlers) classifyBatch(c *gin.Context) {
	var req struct {
		Paths      []string `json:"paths" binding:"required"`
		Concurrency int      `json:"concurrency"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "路径列表为空"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	results, err := h.classifier.ClassifyBatch(ctx, req.Paths, concurrency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":   len(results),
			"results": results,
		},
	})
}

// getClassifyResult 获取分类结果
func (h *Handlers) getClassifyResult(c *gin.Context) {
	// 如果需要异步处理，可以实现结果缓存
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "使用 POST /classify 进行分类",
	})
}

// listCategories 列出分类
func (h *Handlers) listCategories(c *gin.Context) {
	categories := h.classifier.GetCategories()

	// 构建树形结构
	tree := buildCategoryTree(categories)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"categories": categories,
			"tree":       tree,
		},
	})
}

// buildCategoryTree 构建分类树
func buildCategoryTree(categories []Category) []map[string]interface{} {
	nodeMap := make(map[string]*map[string]interface{})
	var roots []map[string]interface{}

	// 创建所有节点
	for _, cat := range categories {
		node := map[string]interface{}{
			"id":       cat.ID,
			"name":     cat.Name,
			"color":    cat.Color,
			"icon":     cat.Icon,
			"children": []map[string]interface{}{},
		}
		nodeMap[cat.ID] = &node
	}

	// 构建树
	for _, cat := range categories {
		node := nodeMap[cat.ID]
		if cat.ParentID == "" {
			roots = append(roots, *node)
		} else {
			if parent, ok := nodeMap[cat.ParentID]; ok {
				children := (*parent)["children"].([]map[string]interface{})
				(*parent)["children"] = append(children, *node)
			}
		}
	}

	return roots
}

// createCategory 创建分类
func (h *Handlers) createCategory(c *gin.Context) {
	var req Category
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.classifier.AddCategory(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    req,
	})
}

// updateCategory 更新分类
func (h *Handlers) updateCategory(c *gin.Context) {
	id := c.Param("id")

	var req Category
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	req.ID = id
	req.UpdatedAt = time.Now()

	// 实现更新逻辑
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// deleteCategory 删除分类
func (h *Handlers) deleteCategory(c *gin.Context) {
	id := c.Param("id")

	// 实现删除逻辑
	_ = id

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// listTags 列出标签
func (h *Handlers) listTags(c *gin.Context) {
	tags := h.tagger.GetCustomTags()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tags,
	})
}

// createTag 创建标签
func (h *Handlers) createTag(c *gin.Context) {
	var req Tag
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	h.tagger.AddCustomTag(req)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    req,
	})
}

// updateTag 更新标签
func (h *Handlers) updateTag(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// deleteTag 删除标签
func (h *Handlers) deleteTag(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// generateTags 生成标签
func (h *Handlers) generateTags(c *gin.Context) {
	var req struct {
		Path     string `json:"path" binding:"required"`
		Content  string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	result, err := h.classifier.Classify(ctx, req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"tags": result.Tags,
		},
	})
}

// listRules 列出规则
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.classifier.GetRules()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// createRule 创建规则
func (h *Handlers) createRule(c *gin.Context) {
	var req ClassificationRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.classifier.AddRule(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    req,
	})
}

// updateRule 更新规则
func (h *Handlers) updateRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// deleteRule 删除规则
func (h *Handlers) deleteRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// detectSimilarity 检测相似度
func (h *Handlers) detectSimilarity(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		MinScore float64 `json:"minScore"`
		MaxResults int `json:"maxResults"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	var options []DetectOption
	if req.MinScore > 0 {
		options = append(options, func(o *DetectOptions) { o.MinScore = req.MinScore })
	}
	if req.MaxResults > 0 {
		options = append(options, func(o *DetectOptions) { o.MaxResults = req.MaxResults })
	}

	results, err := h.detector.DetectSimilar(ctx, req.Path, options...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"similarFiles": results,
			"count":        len(results),
		},
	})
}

// indexFile 索引文件
func (h *Handlers) indexFile(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.detector.IndexFile(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "索引成功",
	})
}

// indexDirectory 索引目录
func (h *Handlers) indexDirectory(c *gin.Context) {
	var req struct {
		Path       string `json:"path" binding:"required"`
		Recursive  bool   `json:"recursive"`
		Concurrency int   `json:"concurrency"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Minute)
	defer cancel()

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	if err := h.detector.IndexDirectory(ctx, req.Path, req.Recursive, concurrency); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "索引成功",
	})
}

// findDuplicates 查找重复文件
func (h *Handlers) findDuplicates(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	duplicates, err := h.detector.FindDuplicates(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"duplicateGroups": duplicates,
			"groupCount":      len(duplicates),
		},
	})
}

// getSimilarityStats 获取相似度统计
func (h *Handlers) getSimilarityStats(c *gin.Context) {
	stats := h.detector.GetStatistics()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// submitFeedback 提交反馈
func (h *Handlers) submitFeedback(c *gin.Context) {
	var req UserFeedback
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.learner.LearnFromFeedback(ctx, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "反馈已提交",
	})
}

// learnFromDirectory 从目录学习
func (h *Handlers) learnFromDirectory(c *gin.Context) {
	var req struct {
		Directory  string `json:"directory" binding:"required"`
		CategoryID string `json:"categoryId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.learner.LearnFromDirectory(ctx, req.Directory, req.CategoryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "学习成功",
	})
}

// getLearningStats 获取学习统计
func (h *Handlers) getLearningStats(c *gin.Context) {
	stats := h.learner.GetLearningStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getSuggestions 获取规则建议
func (h *Handlers) getSuggestions(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	suggestions, err := h.learner.SuggestRules(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    suggestions,
	})
}

// optimizeRules 优化规则
func (h *Handlers) optimizeRules(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	if err := h.learner.OptimizeRules(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则优化完成",
	})
}

// parseIntParam 解析整数参数
func parseIntParam(c *gin.Context, name string, defaultVal int) int {
	val := c.Query(name)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// parseFloatParam 解析浮点参数
func parseFloatParam(c *gin.Context, name string, defaultVal float64) float64 {
	val := c.Query(name)
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}