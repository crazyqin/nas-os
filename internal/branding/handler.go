package branding

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 品牌管理 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	brandGroup := api.Group("/branding")
	{
		// 品牌配置CRUD
		brandGroup.GET("", h.listBrands)
		brandGroup.POST("", h.createBrand)
		brandGroup.GET("/active", h.getActiveBrand)
		brandGroup.PUT("/active/:id", h.setActiveBrand)
		brandGroup.GET("/:id", h.getBrand)
		brandGroup.PUT("/:id", h.updateBrand)
		brandGroup.DELETE("/:id", h.deleteBrand)

		// 主题切换
		brandGroup.PUT("/:id/theme", h.setTheme)

		// Logo管理
		brandGroup.PUT("/:id/logo", h.updateLogo)

		// 登录页面定制
		brandGroup.PUT("/:id/login-screen", h.updateLoginScreen)

		// 自定义CSS
		brandGroup.PUT("/:id/custom-css", h.updateCustomCSS)

		// 字体配置
		brandGroup.PUT("/:id/fonts", h.updateFonts)

		// 导入导出
		brandGroup.GET("/:id/export", h.exportBrand)
		brandGroup.POST("/import", h.importBrand)
		brandGroup.GET("/export-all", h.exportAll)
	}
}

func (h *Handlers) listBrands(c *gin.Context) {
	brands := h.manager.List()
	c.JSON(http.StatusOK, gin.H{"brands": brands, "total": len(brands)})
}

func (h *Handlers) getBrand(c *gin.Context) {
	b, err := h.manager.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, b)
}

func (h *Handlers) getActiveBrand(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetActive())
}

func (h *Handlers) createBrand(c *gin.Context) {
	var cfg BrandConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.Create(&cfg); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

func (h *Handlers) updateBrand(c *gin.Context) {
	var cfg BrandConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.Update(c.Param("id"), &cfg); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handlers) deleteBrand(c *gin.Context) {
	if err := h.manager.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handlers) setActiveBrand(c *gin.Context) {
	if err := h.manager.SetActive(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "active brand set"})
}

func (h *Handlers) setTheme(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.SetTheme(c.Param("id"), req.Mode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "theme updated", "mode": req.Mode})
}

func (h *Handlers) updateLogo(c *gin.Context) {
	var logo Logo
	if err := c.ShouldBindJSON(&logo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.UpdateLogo(c.Param("id"), logo); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logo updated"})
}

func (h *Handlers) updateLoginScreen(c *gin.Context) {
	var ls LoginScreen
	if err := c.ShouldBindJSON(&ls); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.UpdateLoginScreen(c.Param("id"), ls); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "login screen updated"})
}

func (h *Handlers) updateCustomCSS(c *gin.Context) {
	var css CustomCSS
	if err := c.ShouldBindJSON(&css); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.UpdateCustomCSS(c.Param("id"), css); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "custom CSS updated"})
}

func (h *Handlers) updateFonts(c *gin.Context) {
	var fonts Fonts
	if err := c.ShouldBindJSON(&fonts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.UpdateFonts(c.Param("id"), fonts); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "fonts updated"})
}

func (h *Handlers) exportBrand(c *gin.Context) {
	data, err := h.manager.Export(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func (h *Handlers) importBrand(c *gin.Context) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	cfg, err := h.manager.Import(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cfg)
}

func (h *Handlers) exportAll(c *gin.Context) {
	data, err := h.manager.ExportAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}
