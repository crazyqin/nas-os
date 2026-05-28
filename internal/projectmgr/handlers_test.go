package projectmgr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.json")

	mgr := NewManager(configPath)
	handler := NewHandler(mgr)

	router := gin.New()
	api := router.Group("/api")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreateProject(t *testing.T) {
	_, _, router := setupTestHandler(t)

	startDate := time.Now()
	endDate := time.Now().AddDate(0, 3, 0)

	body := map[string]interface{}{
		"name":        "测试项目",
		"description": "这是一个测试项目",
		"priority":    "high",
		"start_date":  startDate,
		"end_date":    endDate,
		"budget":      100000,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var project Project
	if err := json.Unmarshal(w.Body.Bytes(), &project); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if project.Name != "测试项目" {
		t.Errorf("expected name '测试项目', got '%s'", project.Name)
	}

	if project.Status != "planning" {
		t.Errorf("expected status 'planning', got '%s'", project.Status)
	}

	if project.Priority != "high" {
		t.Errorf("expected priority 'high', got '%s'", project.Priority)
	}

	if project.Budget != 100000 {
		t.Errorf("expected budget 100000, got %f", project.Budget)
	}
}

func TestListProjects(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// Create two projects
	mgr.CreateProject(CreateProjectRequest{Name: "项目1"}, "user1")
	mgr.CreateProject(CreateProjectRequest{Name: "项目2"}, "user2")

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Projects []*Project `json:"projects"`
		Total    int        `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 projects, got %d", resp.Total)
	}
}

func TestGetProject(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	project, _ := mgr.CreateProject(CreateProjectRequest{Name: "测试项目"}, "user1")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Project
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.ID != project.ID {
		t.Errorf("expected project ID %s, got %s", project.ID, resp.ID)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCreateTask(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	project, _ := mgr.CreateProject(CreateProjectRequest{Name: "测试项目"}, "user1")

	dueDate := time.Now().AddDate(0, 0, 7)
	body := map[string]interface{}{
		"title":           "测试任务",
		"description":     "任务描述",
		"priority":        "high",
		"estimated_hours": 8,
		"due_date":        dueDate,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.ID+"/tasks", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var task Task
	json.Unmarshal(w.Body.Bytes(), &task)

	if task.Title != "测试任务" {
		t.Errorf("expected title '测试任务', got '%s'", task.Title)
	}

	if task.Priority != "high" {
		t.Errorf("expected priority 'high', got '%s'", task.Priority)
	}

	if task.EstimatedHours != 8 {
		t.Errorf("expected estimated_hours 8, got %f", task.EstimatedHours)
	}
}

func TestGetTasks(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	project, _ := mgr.CreateProject(CreateProjectRequest{Name: "测试项目"}, "user1")
	mgr.CreateTask(project.ID, CreateTaskRequest{Title: "任务1"}, "user1", "User 1")
	mgr.CreateTask(project.ID, CreateTaskRequest{Title: "任务2"}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Tasks []*Task `json:"tasks"`
		Total int     `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 tasks, got %d", resp.Total)
	}
}

func TestUpdateTask(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	project, _ := mgr.CreateProject(CreateProjectRequest{Name: "测试项目"}, "user1")
	task, _ := mgr.CreateTask(project.ID, CreateTaskRequest{Title: "测试任务"}, "user1", "User 1")

	body := `{"status":"in_progress","priority":"critical"}`
	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+project.ID+"/tasks/"+task.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Task
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got '%s'", resp.Status)
	}

	if resp.Priority != "critical" {
		t.Errorf("expected priority 'critical', got '%s'", resp.Priority)
	}
}

func TestGetGanttData(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	project, _ := mgr.CreateProject(CreateProjectRequest{Name: "测试项目"}, "user1")

	startDate := time.Now()
	dueDate := time.Now().AddDate(0, 0, 7)
	mgr.CreateTask(project.ID, CreateTaskRequest{
		Title:     "任务1",
		StartDate: &startDate,
		DueDate:   &dueDate,
	}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/gantt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Tasks []*GanttTask `json:"tasks"`
		Total int          `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 1 {
		t.Errorf("expected 1 gantt task, got %d", resp.Total)
	}
}

func TestGetProjectReport(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	project, _ := mgr.CreateProject(CreateProjectRequest{
		Name:   "测试项目",
		Budget: 50000,
	}, "user1")

	// Create and complete some tasks
	task1, _ := mgr.CreateTask(project.ID, CreateTaskRequest{Title: "任务1"}, "user1", "User 1")
	mgr.UpdateTask(project.ID, task1.ID, UpdateTaskRequest{Status: strPtr("done")})

	task2, _ := mgr.CreateTask(project.ID, CreateTaskRequest{Title: "任务2"}, "user1", "User 1")
	mgr.UpdateTask(project.ID, task2.ID, UpdateTaskRequest{Status: strPtr("done")})

	mgr.CreateTask(project.ID, CreateTaskRequest{Title: "任务3"}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/reports", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report ProjectReport
	json.Unmarshal(w.Body.Bytes(), &report)

	if report.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", report.TotalTasks)
	}

	if report.CompletedTasks != 2 {
		t.Errorf("expected 2 completed tasks, got %d", report.CompletedTasks)
	}
}

func strPtr(s string) *string {
	return &s
}
