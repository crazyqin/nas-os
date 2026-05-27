package pxe

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles PXE HTTP API requests
type Handler struct {
	manager *Manager
}

// NewHandler creates a new PXE handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers PXE API routes under the given router group
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	pxe := r.Group("/pxe")
	{
		pxe.GET("/config", h.HandleGetConfig)
		pxe.PUT("/config", h.HandleUpdateConfig)

		pxe.GET("/clients", h.HandleListClients)
		pxe.GET("/clients/:mac", h.HandleGetClient)
		pxe.PUT("/clients/:mac", h.HandleUpdateClient)

		pxe.POST("/images", h.HandleAddImage)
		pxe.DELETE("/images/:id", h.HandleRemoveImage)
		pxe.GET("/images", h.HandleListImages)

		pxe.PUT("/boot-menu", h.HandleSetBootMenu)

		pxe.GET("/stats", h.HandleGetStats)

		pxe.POST("/start", h.HandleStart)
		pxe.POST("/stop", h.HandleStop)
	}
}

// HandleGetConfig returns the current PXE configuration
func (h *Handler) HandleGetConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, cfg)
}

// HandleUpdateConfig partially updates PXE configuration
func (h *Handler) HandleUpdateConfig(c *gin.Context) {
	var req CreatePXEConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		return
	}

	if err := h.manager.UpdateConfig(req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.manager.GetConfig())
}

// HandleListClients returns all PXE clients
func (h *Handler) HandleListClients(c *gin.Context) {
	clients := h.manager.ListClients()
	c.JSON(http.StatusOK, clients)
}

// HandleGetClient returns a single client by MAC address
func (h *Handler) HandleGetClient(c *gin.Context) {
	mac := c.Param("mac")
	client, err := h.manager.GetClientByMAC(mac)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, client)
}

// HandleUpdateClient updates a PXE client
func (h *Handler) HandleUpdateClient(c *gin.Context) {
	mac := c.Param("mac")
	var req UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		return
	}

	if err := h.manager.UpdateClient(mac, req); err != nil {
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		}
		return
	}

	client, _ := h.manager.GetClientByMAC(mac)
	c.JSON(http.StatusOK, client)
}

// HandleAddImage adds a new boot image
func (h *Handler) HandleAddImage(c *gin.Context) {
	var img PXEImage
	if err := c.ShouldBindJSON(&img); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		return
	}

	if err := h.manager.AddBootImage(img); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "conflict", Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, img)
}

// HandleRemoveImage removes a boot image by ID
func (h *Handler) HandleRemoveImage(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveBootImage(id); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Success: true})
}

// HandleListImages returns all registered boot images
func (h *Handler) HandleListImages(c *gin.Context) {
	images := h.manager.ListImages()
	c.JSON(http.StatusOK, images)
}

// HandleSetBootMenu replaces the boot menu
func (h *Handler) HandleSetBootMenu(c *gin.Context) {
	var menu []BootMenuItem
	if err := c.ShouldBindJSON(&menu); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		return
	}

	if err := h.manager.SetBootMenu(menu); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{Success: true, Data: menu})
}

// HandleGetStats returns PXE service statistics
func (h *Handler) HandleGetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// HandleStart starts the PXE services
func (h *Handler) HandleStart(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "conflict", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Success: true})
}

// HandleStop stops the PXE services
func (h *Handler) HandleStop(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "conflict", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Success: true})
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 14 && msg[:14] == "client not fou" || len(msg) > 13 && msg[:13] == "image not fou"
}
