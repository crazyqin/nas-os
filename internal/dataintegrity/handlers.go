// Package dataintegrity 提供 REST API 处理器
package dataintegrity

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 数据完整性模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	di := r.Group("/data-integrity")
	{
		// 校验和管理
		di.POST("/checksum", h.calculateChecksum)
		di.POST("/checksum/batch", h.calculateChecksumBatch)
		di.GET("/checksums", h.listChecksums)
		di.DELETE("/checksum", h.deleteChecksum)

		// 文件验证
		di.POST("/verify", h.verifyFile)
		di.POST("/verify/directory", h.verifyDirectory)

		// 完整性检查任务
		di.POST("/jobs", h.createJob)
		di.GET("/jobs", h.listJobs)
		di.GET("/jobs/:id", h.getJob)
		di.POST("/jobs/:id/start", h.startJob)
		di.POST("/jobs/:id/cancel", h.cancelJob)

		// 修复建议
		di.POST("/repair/suggestions", h.getRepairSuggestions)

		// 完整性报告
		di.POST("/report", h.generateReport)

		// 文件历史
		di.GET("/history", h.getFileHistory)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ========== 校验和 API ==========

// calculateChecksum 计算单个文件的校验和.
func (h *Handlers) calculateChecksum(c *gin.Context) {
	var req CalculateChecksumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	cs, err := h.manager.CalculateChecksum(c.Request.Context(), req.FilePath, req.Algorithm)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrFileNotFound {
			httpStatus = http.StatusNotFound
		} else if err == ErrPathRequired || err == ErrInvalidAlgorithm {
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
		Data:    cs,
	})
}

// calculateChecksumBatch 批量计算目录下文件的校验和.
func (h *Handlers) calculateChecksumBatch(c *gin.Context) {
	var req struct {
		Path      string    `json:"path" binding:"required"`
		Algorithm Algorithm `json:"algorithm"`
		Recursive bool      `json:"recursive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	results, err := h.manager.CalculateChecksumBatch(c.Request.Context(), req.Path, req.Algorithm, req.Recursive)
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

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(results),
			"results": results,
		},
	})
}

// listChecksums 列出校验和记录.
func (h *Handlers) listChecksums(c *gin.Context) {
	pathPrefix := c.Query("path")
	algo := Algorithm(c.Query("algorithm"))

	checksums, err := h.manager.ListChecksums(pathPrefix, algo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(checksums),
			"checksums": checksums,
		},
	})
}

// deleteChecksum 删除校验和记录.
func (h *Handlers) deleteChecksum(c *gin.Context) {
	filePath := c.Query("file_path")
	algo := Algorithm(c.Query("algorithm"))

	if err := h.manager.DeleteChecksum(filePath, algo); err != nil {
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

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
	})
}

// ========== 文件验证 API ==========

// verifyFile 验证单个文件.
func (h *Handlers) verifyFile(c *gin.Context) {
	var req VerifyFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	check, err := h.manager.VerifyFile(c.Request.Context(), req.FilePath)
	if err != nil && err != ErrChecksumNotFound {
		httpStatus := http.StatusInternalServerError
		if err == ErrFileNotFound {
			httpStatus = http.StatusNotFound
		} else if err == ErrPathRequired {
			httpStatus = http.StatusBadRequest
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	httpStatus := http.StatusOK
	if check != nil && check.Status == StatusCorrupted {
		httpStatus = http.StatusConflict // 409 表示数据损坏
	}

	c.JSON(httpStatus, response{
		Code:    0,
		Message: "success",
		Data:    check,
	})
}

// verifyDirectory 验证目录.
func (h *Handlers) verifyDirectory(c *gin.Context) {
	var req struct {
		Path      string `json:"path" binding:"required"`
		Recursive bool   `json:"recursive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	checks, err := h.manager.VerifyDirectory(c.Request.Context(), req.Path, req.Recursive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	// 统计结果
	intact, corrupted, unknown := 0, 0, 0
	for _, ch := range checks {
		switch ch.Status {
		case StatusIntact:
			intact++
		case StatusCorrupted:
			corrupted++
		default:
			unknown++
		}
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(checks),
			"intact":    intact,
			"corrupted": corrupted,
			"unknown":   unknown,
			"checks":    checks,
		},
	})
}

// ========== 任务 API ==========

// createJob 创建完整性检查任务.
func (h *Handlers) createJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	job, err := h.manager.CreateJob(req)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrPathRequired || err == ErrInvalidAlgorithm {
			httpStatus = http.StatusBadRequest
		} else if err == ErrJobAlreadyRunning {
			httpStatus = http.StatusConflict
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
		Data:    job,
	})
}

// listJobs 列出任务.
func (h *Handlers) listJobs(c *gin.Context) {
	jobs := h.manager.ListJobs()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(jobs),
			"jobs":  jobs,
		},
	})
}

// getJob 获取任务详情.
func (h *Handlers) getJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "无效的任务ID",
		})
		return
	}

	job, err := h.manager.GetJob(id)
	if err != nil {
		httpStatus := http.StatusNotFound
		if err == ErrIntegrityJobNotFound {
			httpStatus = http.StatusNotFound
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
		Data:    job,
	})
}

// startJob 启动任务.
func (h *Handlers) startJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "无效的任务ID",
		})
		return
	}

	if err := h.manager.StartJob(id); err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrIntegrityJobNotFound {
			httpStatus = http.StatusNotFound
		} else if err == ErrJobAlreadyRunning {
			httpStatus = http.StatusConflict
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "任务已启动",
	})
}

// cancelJob 取消任务.
func (h *Handlers) cancelJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "无效的任务ID",
		})
		return
	}

	if err := h.manager.CancelJob(id); err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrJobNotRunning {
			httpStatus = http.StatusConflict
		}
		c.JSON(httpStatus, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "任务已取消",
	})
}

// ========== 修复建议 API ==========

// getRepairSuggestions 获取修复建议.
func (h *Handlers) getRepairSuggestions(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	suggestion, err := h.manager.GetRepairSuggestions(c.Request.Context(), req.FilePath)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if err == ErrFileNotFound {
			httpStatus = http.StatusNotFound
		} else if err == ErrPathRequired {
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
		Data:    suggestion,
	})
}

// ========== 报告 API ==========

// generateReport 生成完整性报告.
func (h *Handlers) generateReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	report, err := h.manager.GenerateReport(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// ========== 历史 API ==========

// getFileHistory 获取文件检查历史.
func (h *Handlers) getFileHistory(c *gin.Context) {
	filePath := c.Query("file_path")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 20
	}

	history, err := h.manager.GetFileHistory(filePath, limit)
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

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}
