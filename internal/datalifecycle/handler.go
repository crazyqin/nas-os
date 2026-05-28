// Package datalifecycle 数据生命周期管理模块 REST API 处理器
package datalifecycle

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 数据生命周期模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	dl := r.Group("/data-lifecycle")
	{
		// 生命周期策略
		dl.POST("/policies", h.createPolicy)
		dl.GET("/policies", h.listPolicies)
		dl.GET("/policies/:id", h.getPolicy)
		dl.PUT("/policies/:id", h.updatePolicy)
		dl.DELETE("/policies/:id", h.deletePolicy)

		// 策略评估与执行
		dl.POST("/policies/:id/evaluate", h.evaluatePolicy)

		// 保留策略
		dl.POST("/retention-policies", h.createRetentionPolicy)
		dl.GET("/retention-policies", h.listRetentionPolicies)
		dl.GET("/retention-policies/:id", h.getRetentionPolicy)
		dl.DELETE("/retention-policies/:id", h.deleteRetentionPolicy)
		dl.POST("/retention-policies/:id/enforce", h.enforceRetentionPolicy)

		// 数据项管理
		dl.POST("/items", h.registerDataItem)
		dl.GET("/items", h.listDataItems)
		dl.GET("/items/:id", h.getDataItem)

		// 数据血缘
		dl.POST("/lineage", h.createLineage)
		dl.GET("/lineage/:id", h.getLineage)
		dl.GET("/lineage/graph", h.getLineageGraph)
		dl.GET("/lineage/by-path", h.getLineageByPath)
		dl.DELETE("/lineage/:id", h.deleteLineage)

		// 成本优化
		dl.POST("/cost/analyze", h.analyzeCosts)
		dl.GET("/cost/suggestions", h.listCostSuggestions)

		// 审计日志
		dl.GET("/audit", h.listAuditEvents)

		// 迁移记录
		dl.GET("/migrations", h.getMigrations)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ========== 生命周期策略 API ==========

// createPolicy 创建生命周期策略.
func (h *Handlers) createPolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	policy, err := h.manager.CreatePolicy(req)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrInvalidTier || err == ErrInvalidAction {
			httpStatus = http.StatusBadRequest
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// listPolicies 列出生命周期策略.
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(policies),
			"policies": policies,
		},
	})
}

// getPolicy 获取策略详情.
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// updatePolicy 更新策略.
func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	policy, err := h.manager.UpdatePolicy(id, req)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrPolicyNotFound {
			httpStatus = http.StatusNotFound
		} else if err == ErrInvalidTier || err == ErrInvalidAction {
			httpStatus = http.StatusBadRequest
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// deletePolicy 删除策略.
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
	})
}

// ========== 策略评估 API ==========

// evaluatePolicy 评估策略.
func (h *Handlers) evaluatePolicy(c *gin.Context) {
	policyID := c.Param("id")
	dryRun := c.Query("dry_run") == "true"

	result, err := h.manager.EvaluatePolicy(c.Request.Context(), policyID, dryRun)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrPolicyNotFound {
			httpStatus = http.StatusNotFound
		} else if err == ErrPolicyDisabled {
			httpStatus = http.StatusBadRequest
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// ========== 保留策略 API ==========

// createRetentionPolicy 创建保留策略.
func (h *Handlers) createRetentionPolicy(c *gin.Context) {
	var req CreateRetentionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	policy, err := h.manager.CreateRetentionPolicy(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// listRetentionPolicies 列出保留策略.
func (h *Handlers) listRetentionPolicies(c *gin.Context) {
	policies := h.manager.ListRetentionPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(policies),
			"policies": policies,
		},
	})
}

// getRetentionPolicy 获取保留策略.
func (h *Handlers) getRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetRetentionPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// deleteRetentionPolicy 删除保留策略.
func (h *Handlers) deleteRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRetentionPolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
	})
}

// enforceRetentionPolicy 执行保留策略.
func (h *Handlers) enforceRetentionPolicy(c *gin.Context) {
	id := c.Param("id")
	removed, err := h.manager.EnforceRetentionPolicy(c.Request.Context(), id)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrPolicyNotFound {
			httpStatus = http.StatusNotFound
		} else if err == ErrPolicyDisabled {
			httpStatus = http.StatusBadRequest
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"removed_count": len(removed),
			"removed_paths": removed,
		},
	})
}

// ========== 数据项 API ==========

// registerDataItem 注册数据项.
func (h *Handlers) registerDataItem(c *gin.Context) {
	var item DataItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.RegisterDataItem(&item); err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrPathRequired {
			httpStatus = http.StatusBadRequest
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "success",
		Data:    item,
	})
}

// listDataItems 列出数据项.
func (h *Handlers) listDataItems(c *gin.Context) {
	pathPrefix := c.Query("path_prefix")
	tier := Tier(c.Query("tier"))

	items := h.manager.ListDataItems(pathPrefix, tier)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(items),
			"items": items,
		},
	})
}

// getDataItem 获取数据项.
func (h *Handlers) getDataItem(c *gin.Context) {
	id := c.Param("id")
	item, err := h.manager.GetDataItem(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    item,
	})
}

// ========== 数据血缘 API ==========

// createLineage 创建血缘记录.
func (h *Handlers) createLineage(c *gin.Context) {
	var req CreateLineageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	lineage, err := h.manager.CreateLineage(req)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrPathRequired {
			httpStatus = http.StatusBadRequest
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "success",
		Data:    lineage,
	})
}

// getLineage 获取血缘记录.
func (h *Handlers) getLineage(c *gin.Context) {
	id := c.Param("id")
	lineage, err := h.manager.GetLineage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    lineage,
	})
}

// getLineageGraph 获取血缘关系图.
func (h *Handlers) getLineageGraph(c *gin.Context) {
	filePath := c.Query("file_path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "必须指定 file_path 参数",
		})
		return
	}

	graph := h.manager.GetLineageGraph(filePath)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    graph,
	})
}

// getLineageByPath 按路径获取血缘记录.
func (h *Handlers) getLineageByPath(c *gin.Context) {
	filePath := c.Query("file_path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "必须指定 file_path 参数",
		})
		return
	}

	lineages := h.manager.GetLineageByPath(filePath)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(lineages),
			"lineages": lineages,
		},
	})
}

// deleteLineage 删除血缘记录.
func (h *Handlers) deleteLineage(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteLineage(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
	})
}

// ========== 成本优化 API ==========

// analyzeCosts 分析成本.
func (h *Handlers) analyzeCosts(c *gin.Context) {
	summary := h.manager.AnalyzeCosts(c.Request.Context())
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    summary,
	})
}

// listCostSuggestions 列出成本优化建议.
func (h *Handlers) listCostSuggestions(c *gin.Context) {
	suggestions := h.manager.ListCostSuggestions()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":       len(suggestions),
			"suggestions": suggestions,
		},
	})
}

// ========== 审计日志 API ==========

// listAuditEvents 列出审计事件.
func (h *Handlers) listAuditEvents(c *gin.Context) {
	eventType := EventType(c.Query("type"))
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	events := h.manager.ListAuditEvents(eventType, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// ========== 迁移记录 API ==========

// getMigrations 获取迁移记录.
func (h *Handlers) getMigrations(c *gin.Context) {
	policyID := c.Query("policy_id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	migrations := h.manager.GetMigrations(policyID, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(migrations),
			"migrations": migrations,
		},
	})
}
