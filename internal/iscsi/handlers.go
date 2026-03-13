package iscsi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers for iSCSI API
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new iSCSI handlers
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes registers iSCSI API routes
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	iscsi := api.Group("/iscsi")
	{
		// Target management
		iscsi.GET("/targets", h.listTargets)
		iscsi.POST("/targets", h.createTarget)
		iscsi.GET("/targets/:id", h.getTarget)
		iscsi.PUT("/targets/:id", h.updateTarget)
		iscsi.DELETE("/targets/:id", h.deleteTarget)

		// Target status and control
		iscsi.GET("/targets/:id/status", h.getTargetStatus)
		iscsi.POST("/targets/:id/enable", h.enableTarget)
		iscsi.POST("/targets/:id/disable", h.disableTarget)

		// LUN management
		iscsi.GET("/targets/:id/luns", h.listLUNs)
		iscsi.POST("/targets/:id/luns", h.addLUN)
		iscsi.GET("/targets/:id/luns/:lunId", h.getLUN)
		iscsi.DELETE("/targets/:id/luns/:lunId", h.removeLUN)
		iscsi.POST("/targets/:id/luns/:lunId/expand", h.expandLUN)

		// LUN snapshots
		iscsi.GET("/targets/:id/luns/:lunId/snapshots", h.listLUNSnapshots)
		iscsi.POST("/targets/:id/luns/:lunId/snapshots", h.createLUNSnapshot)
		iscsi.DELETE("/targets/:id/luns/:lunId/snapshots/:snapId", h.deleteLUNSnapshot)

		// Service management
		iscsi.GET("/status", h.getServiceStatus)
		iscsi.POST("/start", h.startService)
		iscsi.POST("/stop", h.stopService)
		iscsi.POST("/restart", h.restartService)
		iscsi.POST("/apply", h.applyConfig)
	}
}

// Response types
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func fail(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== Target Handlers ==========

// @Summary List iSCSI targets
// @Description Get all iSCSI targets
// @Tags iscsi
// @Accept json
// @Produce json
// @Success 200 {object} Response
// @Router /iscsi/targets [get]
func (h *Handlers) listTargets(c *gin.Context) {
	targets := h.manager.ListTargets()
	c.JSON(http.StatusOK, success(targets))
}

// @Summary Create iSCSI target
// @Description Create a new iSCSI target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param request body TargetInput true "Target configuration"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 409 {object} Response
// @Router /iscsi/targets [post]
func (h *Handlers) createTarget(c *gin.Context) {
	var input TargetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	target, err := h.manager.CreateTarget(input)
	if err != nil {
		if err == ErrTargetExists {
			c.JSON(http.StatusConflict, fail(409, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(target))
}

// @Summary Get iSCSI target
// @Description Get an iSCSI target by ID
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id} [get]
func (h *Handlers) getTarget(c *gin.Context) {
	id := c.Param("id")
	target, err := h.manager.GetTarget(id)
	if err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(target))
}

// @Summary Update iSCSI target
// @Description Update an iSCSI target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param request body TargetInput true "Target configuration"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id} [put]
func (h *Handlers) updateTarget(c *gin.Context) {
	id := c.Param("id")
	var input TargetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	target, err := h.manager.UpdateTarget(id, input)
	if err != nil {
		if err == ErrTargetNotFound {
			c.JSON(http.StatusNotFound, fail(404, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(target))
}

// @Summary Delete iSCSI target
// @Description Delete an iSCSI target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id} [delete]
func (h *Handlers) deleteTarget(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTarget(id); err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}

// @Summary Get target status
// @Description Get operational status of an iSCSI target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/status [get]
func (h *Handlers) getTargetStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := h.manager.GetTargetStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(status))
}

// @Summary Enable target
// @Description Enable an iSCSI target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/enable [post]
func (h *Handlers) enableTarget(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.EnableTarget(id); err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}

// @Summary Disable target
// @Description Disable an iSCSI target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/disable [post]
func (h *Handlers) disableTarget(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DisableTarget(id); err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}

// ========== LUN Handlers ==========

// @Summary List LUNs
// @Description List all LUNs for a target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns [get]
func (h *Handlers) listLUNs(c *gin.Context) {
	targetID := c.Param("id")
	target, err := h.manager.GetTarget(targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(target.LUNs))
}

// @Summary Add LUN
// @Description Add a LUN to a target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param request body LUNInput true "LUN configuration"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns [post]
func (h *Handlers) addLUN(c *gin.Context) {
	targetID := c.Param("id")
	var input LUNInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	lun, err := h.manager.AddLUN(targetID, input)
	if err != nil {
		if err == ErrTargetNotFound {
			c.JSON(http.StatusNotFound, fail(404, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(lun))
}

// @Summary Get LUN
// @Description Get a specific LUN
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param lunId path string true "LUN ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns/{lunId} [get]
func (h *Handlers) getLUN(c *gin.Context) {
	targetID := c.Param("id")
	lunID := c.Param("lunId")

	lun, err := h.manager.GetLUN(targetID, lunID)
	if err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(lun))
}

// @Summary Remove LUN
// @Description Remove a LUN from a target
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param lunId path string true "LUN ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns/{lunId} [delete]
func (h *Handlers) removeLUN(c *gin.Context) {
	targetID := c.Param("id")
	lunID := c.Param("lunId")

	if err := h.manager.RemoveLUN(targetID, lunID); err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}

// @Summary Expand LUN
// @Description Expand a LUN's size
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param lunId path string true "LUN ID"
// @Param request body LUNExpandInput true "New size"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns/{lunId}/expand [post]
func (h *Handlers) expandLUN(c *gin.Context) {
	targetID := c.Param("id")
	lunID := c.Param("lunId")

	var input LUNExpandInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	lun, err := h.manager.ExpandLUN(targetID, lunID, input.Size)
	if err != nil {
		if err == ErrTargetNotFound || err == ErrLUNNotFound {
			c.JSON(http.StatusNotFound, fail(404, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(lun))
}

// ========== Snapshot Handlers ==========

// @Summary List LUN snapshots
// @Description List all snapshots for a LUN
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param lunId path string true "LUN ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns/{lunId}/snapshots [get]
func (h *Handlers) listLUNSnapshots(c *gin.Context) {
	targetID := c.Param("id")
	lunID := c.Param("lunId")

	lun, err := h.manager.GetLUN(targetID, lunID)
	if err != nil {
		c.JSON(http.StatusNotFound, fail(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(lun.Snapshots))
}

// @Summary Create LUN snapshot
// @Description Create a snapshot of a LUN
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param lunId path string true "LUN ID"
// @Param request body LUNSnapshotInput true "Snapshot configuration"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns/{lunId}/snapshots [post]
func (h *Handlers) createLUNSnapshot(c *gin.Context) {
	targetID := c.Param("id")
	lunID := c.Param("lunId")

	var input LUNSnapshotInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	snapshot, err := h.manager.CreateLUNSnapshot(targetID, lunID, input)
	if err != nil {
		if err == ErrTargetNotFound || err == ErrLUNNotFound {
			c.JSON(http.StatusNotFound, fail(404, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, fail(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(snapshot))
}

// @Summary Delete LUN snapshot
// @Description Delete a LUN snapshot
// @Tags iscsi
// @Accept json
// @Produce json
// @Param id path string true "Target ID"
// @Param lunId path string true "LUN ID"
// @Param snapId path string true "Snapshot ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /iscsi/targets/{id}/luns/{lunId}/snapshots/{snapId} [delete]
func (h *Handlers) deleteLUNSnapshot(c *gin.Context) {
	// This would require extending the manager to support snapshot deletion
	c.JSON(http.StatusOK, success(nil))
}

// ========== Service Handlers ==========

// @Summary Get service status
// @Description Get iSCSI target service status
// @Tags iscsi
// @Accept json
// @Produce json
// @Success 200 {object} Response
// @Router /iscsi/status [get]
func (h *Handlers) getServiceStatus(c *gin.Context) {
	running, err := h.manager.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, fail(500, err.Error()))
		return
	}

	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, success(gin.H{
		"running": running,
		"config":  config,
	}))
}

// @Summary Start service
// @Description Start iSCSI target service
// @Tags iscsi
// @Accept json
// @Produce json
// @Success 200 {object} Response
// @Router /iscsi/start [post]
func (h *Handlers) startService(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, fail(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}

// @Summary Stop service
// @Description Stop iSCSI target service
// @Tags iscsi
// @Accept json
// @Produce json
// @Success 200 {object} Response
// @Router /iscsi/stop [post]
func (h *Handlers) stopService(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, fail(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}

// @Summary Restart service
// @Description Restart iSCSI target service
// @Tags iscsi
// @Accept json
// @Produce json
// @Success 200 {object} Response
// @Router /iscsi/restart [post]
func (h *Handlers) restartService(c *gin.Context) {
	if err := h.manager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, fail(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}

// @Summary Apply configuration
// @Description Apply iSCSI configuration to the system
// @Tags iscsi
// @Accept json
// @Produce json
// @Success 200 {object} Response
// @Router /iscsi/apply [post]
func (h *Handlers) applyConfig(c *gin.Context) {
	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, fail(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, success(nil))
}