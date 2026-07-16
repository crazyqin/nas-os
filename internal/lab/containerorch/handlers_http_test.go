// Package containerorch - HTTP Handler 测试
package containerorch

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1)
	return r
}

func TestHandlerCreateProjectHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	body := CreateProjectRequest{
		Name: "http-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/containerorch/projects", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusCreated, w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("响应码不匹配: %d", resp.Code)
	}
}

func TestHandlerListProjectsHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	// 创建一些项目
	m.CreateProject(CreateProjectRequest{
		Name: "project1",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.CreateProject(CreateProjectRequest{
		Name:      "project2",
		Namespace: "test",
		Services: map[string]*ServiceConfig{
			"api": {Image: "node"},
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetProjectHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "get-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetProjectNotFoundHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/non-existent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusNotFound, w.Code)
	}
}

func TestHandlerStartProjectHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "start-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	req := httptest.NewRequest("POST", "/api/v1/containerorch/projects/"+project.ID+"/start", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerStopProjectHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "stop-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	req := httptest.NewRequest("POST", "/api/v1/containerorch/projects/"+project.ID+"/stop", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerRestartProjectHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "restart-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	req := httptest.NewRequest("POST", "/api/v1/containerorch/projects/"+project.ID+"/restart", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerListServicesHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "services-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
			"api": {Image: "node"},
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/services", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetServiceHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "service-test",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/services/web", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetServiceNotFoundHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "service-notfound",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/services/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusNotFound, w.Code)
	}
}

func TestHandlerScaleServiceHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "scale-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 1},
		},
	})
	m.StartProject(project.ID)

	body := ScaleServiceRequest{Replicas: 3}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/containerorch/projects/"+project.ID+"/services/web/scale", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetStartupOrderHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "order-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DependsOn: []ServiceDependency{{ServiceName: "api"}}},
			"api": {Image: "node"},
		},
	})

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/startup-order", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetHealthReportHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "health-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerUpdateHealthCheckHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "healthcheck-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	body := UpdateHealthCheckRequest{
		ServiceName: "web",
		HealthCheck: &HealthCheckConfig{
			Test:     []string{"CMD", "curl", "-f", "http://localhost"},
			Interval: 30,
			Timeout:  5,
			Retries:  3,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/containerorch/projects/"+project.ID+"/services/web/health-check", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerUpdateResourcesHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "resources-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	body := UpdateResourceLimitsRequest{
		ServiceName: "web",
		Resources: &ResourceLimits{
			CPU:    &CPULimit{Cores: 2},
			Memory: &MemoryLimit{Limit: 1024 * 1024 * 1024},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/containerorch/projects/"+project.ID+"/services/web/resources", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerUpdateAutoScaleHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "autoscale-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	body := UpdateAutoScaleRequest{
		ServiceName: "web",
		AutoScale: &AutoScalePolicy{
			Enabled:     true,
			MinReplicas: 1,
			MaxReplicas: 10,
			Metrics: []ScalingMetric{
				{Type: "cpu", Target: 70.0},
			},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/containerorch/projects/"+project.ID+"/services/web/auto-scale", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetAutoScaleEventsHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "events-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 1, Instances: 1},
		},
	})
	m.StartProject(project.ID)
	m.ScaleService(project.ID, "web", 3)

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/auto-scale-events?limit=10", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetServiceLogsHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "logs-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/logs?tail=10", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetProjectStatsHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "stats-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx", DesiredCount: 2},
		},
	})
	m.StartProject(project.ID)

	req := httptest.NewRequest("GET", "/api/v1/containerorch/projects/"+project.ID+"/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerDeleteProjectHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "delete-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	req := httptest.NewRequest("DELETE", "/api/v1/containerorch/projects/"+project.ID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerDeleteProjectForceHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "force-delete-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})
	m.StartProject(project.ID)

	req := httptest.NewRequest("DELETE", "/api/v1/containerorch/projects/"+project.ID+"?force=true", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerUpdateProjectHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "update-http",
		Services: map[string]*ServiceConfig{
			"web": {Image: "nginx"},
		},
	})

	newName := "updated"
	body := UpdateProjectRequest{Name: &newName}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/containerorch/projects/"+project.ID, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerEvaluateAutoScaleHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	project, _ := m.CreateProject(CreateProjectRequest{
		Name: "evaluate-http",
		Services: map[string]*ServiceConfig{
			"web": {
				Image:        "nginx",
				DesiredCount: 2,
				Instances:    2,
				Deploy: &DeployConfig{
					Replicas: 2,
					AutoScale: &AutoScalePolicy{
						Enabled:     true,
						MinReplicas: 1,
						MaxReplicas: 10,
						Metrics: []ScalingMetric{
							{Type: "cpu", Target: 70.0},
						},
						ScaleUp:   &ScaleRules{StepSize: 2},
						ScaleDown: &ScaleRules{StepSize: 1},
					},
				},
			},
		},
	})
	m.StartProject(project.ID)

	metrics := ContainerMetrics{
		CPU:    CPUMetrics{Percent: 90.0},
		Memory: MemoryMetrics{Percent: 50.0},
	}
	jsonBody, _ := json.Marshal(metrics)

	req := httptest.NewRequest("POST", "/api/v1/containerorch/projects/"+project.ID+"/services/web/evaluate-auto-scale", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}
