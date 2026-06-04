package composevisual

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	mgr := NewManager()
	rg := r.Group("/api/v1")
	RegisterRoutes(rg, mgr)
	return r, mgr
}

func TestCreateProject(t *testing.T) {
	r, _ := setupTestRouter()
	body := `{"name":"test-project","description":"测试项目","tags":["test"]}`
	req, _ := http.NewRequest("POST", "/api/v1/composevisual/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)
	var resp ComposeProject
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "test-project", resp.Name)
	assert.Equal(t, "测试项目", resp.Description)
	assert.Equal(t, StatusDraft, resp.Status)
	assert.NotEmpty(t, resp.ID)
}

func TestListProjects(t *testing.T) {
	r, mgr := setupTestRouter()
	mgr.CreateProject(&CreateProjectRequest{Name: "p1"})
	mgr.CreateProject(&CreateProjectRequest{Name: "p2"})

	req, _ := http.NewRequest("GET", "/api/v1/composevisual/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(2), resp["total"])
}

func TestGetProject(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "test"})

	req, _ := http.NewRequest("GET", "/api/v1/composevisual/projects/"+project.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp ComposeProject
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "test", resp.Name)
}

func TestGetProjectNotFound(t *testing.T) {
	r, _ := setupTestRouter()
	req, _ := http.NewRequest("GET", "/api/v1/composevisual/projects/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)
}

func TestDeleteProject(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "to-delete"})

	req, _ := http.NewRequest("DELETE", "/api/v1/composevisual/projects/"+project.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// 验证已删除
	req2, _ := http.NewRequest("GET", "/api/v1/composevisual/projects/"+project.ID, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 404, w2.Code)
}

func TestAddService(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "test"})

	body := `{"name":"web","image":"nginx:latest","ports":[{"hostPort":8080,"containerPort":80,"protocol":"tcp"}]}`
	req, _ := http.NewRequest("POST", "/api/v1/composevisual/projects/"+project.ID+"/services", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)
	var resp ServiceNode
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "web", resp.Name)
	assert.Equal(t, "nginx:latest", resp.Image)
	assert.Equal(t, 1, len(resp.Ports))
	assert.NotNil(t, resp.Resources) // 自动推荐资源
}

func TestDeleteService(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "test"})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "web", Image: "nginx:latest"})

	req, _ := http.NewRequest("DELETE", "/api/v1/composevisual/projects/"+project.ID+"/services/web", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// 验证服务已删除
	p, _ := mgr.GetProject(project.ID)
	assert.Equal(t, 0, len(p.Services))
}

func TestExportCompose(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "test"})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "web", Image: "nginx:latest", Ports: []PortMapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}}})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "db", Image: "mysql:8.0", DependsOn: []string{"web"}})

	req, _ := http.NewRequest("POST", "/api/v1/composevisual/projects/"+project.ID+"/export", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp ExportComposeResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp.Content, "version: '3.8'")
	assert.Contains(t, resp.Content, "nginx:latest")
	assert.Contains(t, resp.Content, "mysql:8.0")
}

func TestListTemplates(t *testing.T) {
	r, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/composevisual/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp TemplateSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.GreaterOrEqual(t, resp.Total, 10) // 至少10个模板
}

func TestInstantiateTemplate(t *testing.T) {
	r, _ := setupTestRouter()

	body := `{"name":"my-wp","description":"我的WordPress"}`
	req, _ := http.NewRequest("POST", "/api/v1/composevisual/templates/wordpress/instantiate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)
	var resp ComposeProject
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "my-wp", resp.Name)
	assert.GreaterOrEqual(t, len(resp.Services), 2) // wordpress + db
}

func TestImportCompose(t *testing.T) {
	r, _ := setupTestRouter()

	yamlContent := `version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80/tcp"
  app:
    image: node:18-alpine
    depends_on:
      - web`

	body, _ := json.Marshal(ImportComposeRequest{Content: yamlContent, Name: "imported"})
	req, _ := http.NewRequest("POST", "/api/v1/composevisual/import", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)
	var resp ComposeProject
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "imported", resp.Name)
	assert.Equal(t, 2, len(resp.Services))
}

func TestGetTopology(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "test"})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "nginx", Image: "nginx:latest", DependsOn: []string{"app"}})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "app", Image: "node:18", DependsOn: []string{"db"}})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "db", Image: "postgres:15"})

	req, _ := http.NewRequest("GET", "/api/v1/composevisual/projects/"+project.ID+"/topology", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp TopologyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 3, len(resp.Topology.Nodes))
	assert.Equal(t, 2, len(resp.Topology.Edges))
	assert.GreaterOrEqual(t, len(resp.StartOrder), 2) // db->app->nginx 分层
}

func TestConnectServices(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "test"})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "web", Image: "nginx:latest"})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "app", Image: "node:18"})

	body := `{"from":"app","to":"web","type":"depends"}`
	req, _ := http.NewRequest("POST", "/api/v1/composevisual/projects/"+project.ID+"/connect", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	p, _ := mgr.GetProject(project.ID)
	assert.Equal(t, 1, len(p.Layout.Connections))
	assert.Equal(t, []string{"app"}, p.Services["web"].DependsOn)
}

func TestDeploy(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "test"})
	mgr.AddService(project.ID, &AddServiceRequest{Name: "web", Image: "nginx:latest"})

	req, _ := http.NewRequest("POST", "/api/v1/composevisual/projects/"+project.ID+"/deploy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp DeployResult
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "success", resp.Status)
}

func TestDeployEmptyProject(t *testing.T) {
	r, mgr := setupTestRouter()
	project := mgr.CreateProject(&CreateProjectRequest{Name: "empty"})

	req, _ := http.NewRequest("POST", "/api/v1/composevisual/projects/"+project.ID+"/deploy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestTemplateSearch(t *testing.T) {
	r, _ := setupTestRouter()

	// 按分类搜索
	req, _ := http.NewRequest("GET", "/api/v1/composevisual/templates?category=media", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp TemplateSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.GreaterOrEqual(t, resp.Total, 1) // 至少有 Jellyfin

	// 关键词搜索
	req2, _ := http.NewRequest("GET", "/api/v1/composevisual/templates?query=nginx", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp2 TemplateSearchResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	assert.GreaterOrEqual(t, resp2.Total, 1)
}

func TestStartOrderCalculation(t *testing.T) {
	services := map[string]*ServiceNode{
		"web": {Name: "web", DependsOn: []string{"app"}},
		"app": {Name: "app", DependsOn: []string{"db", "redis"}},
		"db":  {Name: "db"},
		"redis": {Name: "redis"},
	}
	order := CalculateStartOrder(services)
	assert.Equal(t, 3, len(order))
	// 第一层: db, redis（无依赖）
	assert.Contains(t, order[0], "db")
	assert.Contains(t, order[0], "redis")
	// 第二层: app
	assert.Contains(t, order[1], "app")
	// 第三层: web
	assert.Contains(t, order[2], "web")
}

func TestResourceSuggestion(t *testing.T) {
	dbRes := SuggestResources("mysql:8.0")
	assert.Equal(t, "2.0", dbRes.CPUs)
	assert.Equal(t, "2G", dbRes.Memory)

	cacheRes := SuggestResources("redis:7-alpine")
	assert.Equal(t, "1.0", cacheRes.CPUs)
	assert.Equal(t, "512M", cacheRes.Memory)

	proxyRes := SuggestResources("nginx:latest")
	assert.Equal(t, "1.0", proxyRes.CPUs)
	assert.Equal(t, "256M", proxyRes.Memory)

	defaultRes := SuggestResources("myapp:latest")
	assert.Equal(t, "1.0", defaultRes.CPUs)
	assert.Equal(t, "512M", defaultRes.Memory)
}

func TestNodePositionCalculation(t *testing.T) {
	pos0 := CalculateNodePosition(0)
	assert.Equal(t, 100, pos0.X)
	assert.Equal(t, 100, pos0.Y)

	pos3 := CalculateNodePosition(3)
	assert.Equal(t, 100, pos3.X)        // 第二行第一列
	assert.Equal(t, 350, pos3.Y)        // 100 + 180 + 70

	pos1 := CalculateNodePosition(1)
	assert.Equal(t, 450, pos1.X)        // 100 + 280 + 70
}

func TestPortMappingParsing(t *testing.T) {
	pm := ParsePortMapping("8080:80/tcp")
	assert.NotNil(t, pm)
	assert.Equal(t, 8080, pm.HostPort)
	assert.Equal(t, 80, pm.ContainerPort)
	assert.Equal(t, "tcp", pm.Protocol)

	pm2 := ParsePortMapping("127.0.0.1:3306:3306")
	assert.NotNil(t, pm2)
	assert.Equal(t, "127.0.0.1", pm2.IP)
	assert.Equal(t, 3306, pm2.HostPort)
}

func TestVolumeMappingParsing(t *testing.T) {
	vm := ParseVolumeMapping("./data:/app/data")
	assert.NotNil(t, vm)
	assert.Equal(t, "./data", vm.Source)
	assert.Equal(t, "/app/data", vm.Target)
	assert.Equal(t, "bind", vm.Type)
	assert.False(t, vm.ReadOnly)

	vm2 := ParseVolumeMapping("myvolume:/data:ro")
	assert.NotNil(t, vm2)
	assert.Equal(t, "myvolume", vm2.Source)
	assert.Equal(t, "/data", vm2.Target)
	assert.True(t, vm2.ReadOnly)
}
