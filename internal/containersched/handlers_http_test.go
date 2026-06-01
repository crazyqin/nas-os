// Package containersched - HTTP Handler 测试
package containersched

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

func setupHandlers() (*Handlers, *Manager) {
	m := NewManager()
	h := NewHandlers(m)
	return h, m
}

func TestHandlerCreateNodeHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	body := CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
		Role: NodeRoleWorker,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/containersched/nodes", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusCreated, w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("响应码不匹配: %d", resp.Code)
	}
}

func TestHandlerListNodesHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	// 创建一些节点
	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.CreateNode(CreateNodeRequest{
		Name: "node2",
		Host: "192.168.1.101",
	})

	req := httptest.NewRequest("GET", "/api/v1/containersched/nodes", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerGetNodeHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	node, _ := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
	})

	req := httptest.NewRequest("GET", "/api/v1/containersched/nodes/"+node.ID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerUpdateNodeHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	node, _ := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
	})

	newName := "updated-node"
	body := UpdateNodeRequest{
		Name: &newName,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/containersched/nodes/"+node.ID, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerDeleteNodeHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	node, _ := m.CreateNode(CreateNodeRequest{
		Name: "test-node",
		Host: "192.168.1.100",
	})

	req := httptest.NewRequest("DELETE", "/api/v1/containersched/nodes/"+node.ID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerScheduleHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})

	body := ScheduleRequest{
		ContainerID:   "container-1",
		ContainerName: "web",
		Image:         "nginx:latest",
		Resources: &ResourceRequest{
			CPUCores:    1,
			MemoryBytes: 512 * 1024 * 1024,
		},
		Priority: PriorityNormal,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/containersched/schedule", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerEnqueueHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	body := ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
		Priority:    PriorityHigh,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/containersched/queue", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusCreated, w.Code)
	}
}

func TestHandlerGetQueueStatusHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/containersched/queue", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerListPlacementsHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
	})

	req := httptest.NewRequest("GET", "/api/v1/containersched/placements", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerAutoScaleHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	// 创建自动扩缩容策略
	policy := AutoScalePolicy{
		Enabled:       true,
		MinReplicas:   1,
		MaxReplicas:   10,
		ScaleUpStep:   2,
		ScaleDownStep: 1,
		Metrics: []ScaleMetric{
			{Type: MetricTypeCPU, Target: 80},
		},
	}
	jsonBody, _ := json.Marshal(policy)

	req := httptest.NewRequest("POST", "/api/v1/containersched/autoscale/web", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusCreated, w.Code)
	}
}

func TestHandlerPowerSaveHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	// 获取节能模式配置
	req := httptest.NewRequest("GET", "/api/v1/containersched/powersave", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 更新节能模式配置
	enabled := true
	threshold := 0.4
	updateBody := UpdatePowerSaveRequest{
		Enabled:   &enabled,
		Threshold: &threshold,
	}
	jsonBody, _ := json.Marshal(updateBody)

	req = httptest.NewRequest("PUT", "/api/v1/containersched/powersave", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerStatsHTTP(t *testing.T) {
	h, m := setupHandlers()
	r := setupTestRouter(h)

	m.CreateNode(CreateNodeRequest{
		Name: "node1",
		Host: "192.168.1.100",
	})
	m.Schedule(&ScheduleRequest{
		ContainerID: "container-1",
		Image:       "nginx",
	})

	req := httptest.NewRequest("GET", "/api/v1/containersched/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}
}

func TestHandlerNotFoundHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	req := httptest.NewRequest("GET", "/api/v1/containersched/nodes/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusNotFound, w.Code)
	}
}

func TestHandlerBadRequestHTTP(t *testing.T) {
	h, _ := setupHandlers()
	r := setupTestRouter(h)

	// 发送无效 JSON
	req := httptest.NewRequest("POST", "/api/v1/containersched/nodes", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusBadRequest, w.Code)
	}
}
