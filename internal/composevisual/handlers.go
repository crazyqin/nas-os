package composevisual

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册路由.
func RegisterRoutes(rg *gin.RouterGroup, mgr *Manager) {
	h := &handler{mgr: mgr}

	v1 := rg.Group("/composevisual")
	{
		v1.GET("/projects", h.listProjects)
		v1.POST("/projects", h.createProject)
		v1.GET("/projects/:id", h.getProject)
		v1.PUT("/projects/:id", h.updateProject)
		v1.DELETE("/projects/:id", h.deleteProject)

		v1.POST("/projects/:id/services", h.addService)
		v1.PUT("/projects/:id/services/:sid", h.updateService)
		v1.DELETE("/projects/:id/services/:sid", h.deleteService)

		v1.POST("/projects/:id/connect", h.connectServices)
		v1.POST("/projects/:id/export", h.exportCompose)
		v1.POST("/projects/:id/deploy", h.deploy)
		v1.GET("/projects/:id/topology", h.getTopology)

		v1.GET("/templates", h.listTemplates)
		v1.POST("/templates/:id/instantiate", h.instantiateTemplate)

		v1.POST("/import", h.importCompose)
	}
}

type handler struct {
	mgr *Manager
}

// listProjects 列出项目.
func (h *handler) listProjects(c *gin.Context) {
	projects := h.mgr.ListProjects()
	c.JSON(http.StatusOK, gin.H{"projects": projects, "total": len(projects)})
}

// createProject 创建项目.
func (h *handler) createProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	project := h.mgr.CreateProject(&req)
	c.JSON(http.StatusCreated, project)
}

// getProject 获取项目详情.
func (h *handler) getProject(c *gin.Context) {
	id := c.Param("id")
	project, err := h.mgr.GetProject(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

// updateProject 更新项目.
func (h *handler) updateProject(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	project, err := h.mgr.UpdateProject(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

// deleteProject 删除项目.
func (h *handler) deleteProject(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteProject(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "项目已删除"})
}

// addService 添加服务.
func (h *handler) addService(c *gin.Context) {
	projectID := c.Param("id")
	var req AddServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	service, err := h.mgr.AddService(projectID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, service)
}

// updateService 更新服务.
func (h *handler) updateService(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("sid")
	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	service, err := h.mgr.UpdateService(projectID, serviceName, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, service)
}

// deleteService 删除服务.
func (h *handler) deleteService(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("sid")
	if err := h.mgr.DeleteService(projectID, serviceName); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已删除"})
}

// connectServices 连接服务.
func (h *handler) connectServices(c *gin.Context) {
	projectID := c.Param("id")
	var req ConnectServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.mgr.ConnectServices(projectID, &req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接已建立"})
}

// exportCompose 导出 compose.
func (h *handler) exportCompose(c *gin.Context) {
	projectID := c.Param("id")
	content, err := h.mgr.ExportCompose(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ExportComposeResponse{Content: content})
}

// deploy 部署.
func (h *handler) deploy(c *gin.Context) {
	projectID := c.Param("id")
	result, err := h.mgr.Deploy(projectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// getTopology 获取拓扑图.
func (h *handler) getTopology(c *gin.Context) {
	projectID := c.Param("id")
	topology, startOrder, err := h.mgr.GenerateTopology(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, TopologyResponse{
		Topology:   topology,
		StartOrder: startOrder,
	})
}

// listTemplates 列出模板.
func (h *handler) listTemplates(c *gin.Context) {
	query := c.Query("query")
	category := c.DefaultQuery("category", "all")
	minRating := 0.0
	if v := c.Query("minRating"); v != "" {
		fmt.Sscanf(v, "%f", &minRating)
	}
	sortBy := c.DefaultQuery("sortBy", "rating")
	templates := h.mgr.ListTemplates(query, category, minRating, sortBy)
	c.JSON(http.StatusOK, TemplateSearchResponse{Templates: templates, Total: len(templates)})
}

// instantiateTemplate 从模板创建.
func (h *handler) instantiateTemplate(c *gin.Context) {
	templateID := c.Param("id")
	var req InstantiateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	project, err := h.mgr.InstantiateTemplate(templateID, req.Name, req.Description, req.EnvVars)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, project)
}

// importCompose 导入 compose.
func (h *handler) importCompose(c *gin.Context) {
	var req ImportComposeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	project, err := h.mgr.ImportCompose(req.Content, req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, project)
}
