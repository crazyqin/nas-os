package downloader

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandler(t *testing.T) (*Manager, *gin.Engine) {
	tempDir := t.TempDir()
	logger := zap.NewNop()

	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	h := NewHandler(m)
	h.RegisterRoutes(api)

	return m, router
}

func TestNewHandler(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	h := NewHandler(m)
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if h.manager == nil {
		t.Error("Handler.manager should not be nil")
	}
}

func TestHandler_ListTasks(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	// Create some tasks
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/1.zip", Name: "Task 1"})
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/2.zip", Name: "Task 2"})

	req, _ := http.NewRequest("GET", "/api/downloader/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListTasks status = %d, expected 200", w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !resp.Success {
		t.Error("Response should be successful")
	}
}

func TestHandler_ListTasksWithStatus(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/1.zip", Name: "Task 1"})
	m.UpdateTask(task.ID, UpdateTaskRequest{Status: StatusDownloading})

	req, _ := http.NewRequest("GET", "/api/downloader/tasks?status=downloading", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListTasks status = %d, expected 200", w.Code)
	}
}

func TestHandler_CreateTask(t *testing.T) {
	_, router := setupTestHandler(t)

	body := CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Test File",
		Type: TypeHTTP,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CreateTask status = %d, expected 200, body: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !resp.Success {
		t.Errorf("Response should be successful, error: %s", resp.Error)
	}
}

func TestHandler_CreateTaskEmptyURL(t *testing.T) {
	_, router := setupTestHandler(t)

	body := CreateTaskRequest{
		URL:  "",
		Name: "Test File",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreateTask status = %d, expected 400", w.Code)
	}
}

func TestHandler_CreateTaskInvalidBody(t *testing.T) {
	_, router := setupTestHandler(t)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreateTask status = %d, expected 400", w.Code)
	}
}

func TestHandler_GetTask(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	req, _ := http.NewRequest("GET", "/api/downloader/tasks/"+task.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetTask status = %d, expected 200", w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !resp.Success {
		t.Error("Response should be successful")
	}
}

func TestHandler_GetTaskNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	req, _ := http.NewRequest("GET", "/api/downloader/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GetTask status = %d, expected 404", w.Code)
	}
}

func TestHandler_UpdateTask(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	body := UpdateTaskRequest{Status: StatusPaused}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/downloader/tasks/"+task.ID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("UpdateTask status = %d, expected 200", w.Code)
	}
}

func TestHandler_UpdateTaskNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	body := UpdateTaskRequest{Status: StatusPaused}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/downloader/tasks/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("UpdateTask status = %d, expected 500", w.Code)
	}
}

func TestHandler_UpdateTaskInvalidBody(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	req, _ := http.NewRequest("PUT", "/api/downloader/tasks/"+task.ID, bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("UpdateTask status = %d, expected 400", w.Code)
	}
}

func TestHandler_DeleteTask(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	req, _ := http.NewRequest("DELETE", "/api/downloader/tasks/"+task.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DeleteTask status = %d, expected 200", w.Code)
	}

	// Verify task is deleted
	_, exists := m.GetTask(task.ID)
	if exists {
		t.Error("Task should be deleted")
	}
}

func TestHandler_DeleteTaskWithFiles(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{
		URL:      "https://example.com/test.zip",
		Name:     "test.zip",
		DestPath: t.TempDir(),
	})

	req, _ := http.NewRequest("DELETE", "/api/downloader/tasks/"+task.ID+"?delete_files=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DeleteTask status = %d, expected 200", w.Code)
	}
}

func TestHandler_DeleteTaskNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	req, _ := http.NewRequest("DELETE", "/api/downloader/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("DeleteTask status = %d, expected 500", w.Code)
	}
}

func TestHandler_StartTask(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	req, _ := http.NewRequest("POST", "/api/downloader/tasks/"+task.ID+"/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("StartTask status = %d, expected 200", w.Code)
	}
}

func TestHandler_StartTaskNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks/nonexistent/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("StartTask status = %d, expected 500", w.Code)
	}
}

func TestHandler_PauseTask(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	req, _ := http.NewRequest("POST", "/api/downloader/tasks/"+task.ID+"/pause", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PauseTask status = %d, expected 200", w.Code)
	}
}

func TestHandler_PauseTaskNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks/nonexistent/pause", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("PauseTask status = %d, expected 500", w.Code)
	}
}

func TestHandler_ResumeTask(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})
	m.PauseTask(task.ID)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks/"+task.ID+"/resume", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ResumeTask status = %d, expected 200", w.Code)
	}
}

func TestHandler_ResumeTaskNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks/nonexistent/resume", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("ResumeTask status = %d, expected 500", w.Code)
	}
}

func TestHandler_GetStats(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	m.CreateTask(CreateTaskRequest{URL: "https://example.com/1.zip"})
	m.CreateTask(CreateTaskRequest{URL: "https://example.com/2.zip"})

	req, _ := http.NewRequest("GET", "/api/downloader/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetStats status = %d, expected 200", w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !resp.Success {
		t.Error("Response should be successful")
	}
}

func TestHandler_UpdateTaskWithSpeedLimit(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	body := UpdateTaskRequest{
		SpeedLimit: &SpeedLimitConfig{
			DownloadLimit: 1024,
			UploadLimit:   512,
			Enabled:       true,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/downloader/tasks/"+task.ID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("UpdateTask status = %d, expected 200", w.Code)
	}

	updated, _ := m.GetTask(task.ID)
	if updated.SpeedLimit == nil || updated.SpeedLimit.DownloadLimit != 1024 {
		t.Error("SpeedLimit should be updated")
	}
}

func TestHandler_UpdateTaskWithSchedule(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	task, _ := m.CreateTask(CreateTaskRequest{URL: "https://example.com/test.zip", Name: "Test"})

	body := UpdateTaskRequest{
		Schedule: &ScheduleConfig{
			StartTime: "22:00",
			EndTime:   "08:00",
			Enabled:   true,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/downloader/tasks/"+task.ID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("UpdateTask status = %d, expected 200", w.Code)
	}

	updated, _ := m.GetTask(task.ID)
	if updated.Schedule == nil || updated.Schedule.StartTime != "22:00" {
		t.Error("Schedule should be updated")
	}
}

func TestHandler_CreateTaskWithSchedule(t *testing.T) {
	_, router := setupTestHandler(t)

	body := CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Scheduled Download",
		Schedule: &ScheduleConfig{
			StartTime: "23:00",
			EndTime:   "07:00",
			Days:      []int{0, 1, 2, 3, 4},
			Enabled:   true,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CreateTask status = %d, expected 200", w.Code)
	}
}

func TestHandler_CreateTaskWithSpeedLimit(t *testing.T) {
	_, router := setupTestHandler(t)

	body := CreateTaskRequest{
		URL:  "https://example.com/test.zip",
		Name: "Limited Download",
		SpeedLimit: &SpeedLimitConfig{
			DownloadLimit: 2048,
			Enabled:       true,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/downloader/tasks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CreateTask status = %d, expected 200", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"key": "value"}
	writeJSON(c, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("writeJSON status = %d, expected 200", w.Code)
	}
}

func TestWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	writeError(c, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("writeError status = %d, expected 400", w.Code)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "test error" {
		t.Errorf("Error = %s, expected 'test error'", resp.Error)
	}
}

func TestWriteSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"key": "value"}
	writeSuccess(c, data)

	if w.Code != http.StatusOK {
		t.Errorf("writeSuccess status = %d, expected 200", w.Code)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("Response should be successful")
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()
	m, err := NewManager(tempDir, logger)
	if err != nil {
		t.Fatalf("NewManager() returned error: %v", err)
	}
	defer m.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	h := NewHandler(m)
	h.RegisterRoutes(api)

	// Verify routes are registered by checking a few endpoints
	routes := []string{
		"/api/downloader/tasks",
		"/api/downloader/stats",
	}

	for _, route := range routes {
		req, _ := http.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		// Should not return 404 (route not found)
		if w.Code == http.StatusNotFound {
			t.Errorf("Route %s not registered", route)
		}
	}
}

func TestAPIResponse_Structure(t *testing.T) {
	// Test successful response
	successResp := APIResponse{
		Success: true,
		Data:    "test data",
	}

	if !successResp.Success {
		t.Error("Success should be true")
	}

	// Test error response
	errorResp := APIResponse{
		Success: false,
		Error:   "test error",
	}

	if errorResp.Success {
		t.Error("Success should be false")
	}
	if errorResp.Error != "test error" {
		t.Errorf("Error = %s, expected 'test error'", errorResp.Error)
	}
}

func TestHandler_DeleteTaskWithActualFile(t *testing.T) {
	m, router := setupTestHandler(t)
	defer m.Close()

	// Create a temp directory with a file
	destPath := t.TempDir()
	testFile := filepath.Join(destPath, "test.zip")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	task, _ := m.CreateTask(CreateTaskRequest{
		URL:      "https://example.com/test.zip",
		Name:     "test.zip",
		DestPath: destPath,
	})

	req, _ := http.NewRequest("DELETE", "/api/downloader/tasks/"+task.ID+"?delete_files=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DeleteTask status = %d, expected 200", w.Code)
	}

	// Verify file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should be deleted")
	}
}
