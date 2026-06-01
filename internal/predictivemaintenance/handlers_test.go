package predictivemaintenance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandler() (*Engine, *gin.Engine) {
	engine := New(Config{Enabled: true})
	engine.RegisterComponent("cpu-0", ComponentCPU, "主CPU")
	engine.RegisterComponent("disk-0", ComponentDisk, "主盘")
	for i := 0; i < 15; i++ {
		engine.RecordMetric("cpu-0", float64(50+i))
	}
	handler := NewHandler(engine)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return engine, router
}

func TestHandler_ListComponents(t *testing.T) {
	_, router := setupHandler()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/predictive-maintenance/components", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestHandler_GetHealth(t *testing.T) {
	_, router := setupHandler()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/predictive-maintenance/components/cpu-0", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetHealth_NotFound(t *testing.T) {
	_, router := setupHandler()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/predictive-maintenance/components/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_Predict(t *testing.T) {
	_, router := setupHandler()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/predictive-maintenance/components/cpu-0/predict", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["success"].(bool))
}

func TestHandler_CheckAll(t *testing.T) {
	_, router := setupHandler()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/predictive-maintenance/check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_CreateSchedule(t *testing.T) {
	_, router := setupHandler()
	body := `{"componentId":"cpu-0","type":"preventive","title":"清理风扇","priority":1}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/predictive-maintenance/schedules", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandler_CreateSchedule_Invalid(t *testing.T) {
	_, router := setupHandler()
	body := `{"type":"preventive"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/predictive-maintenance/schedules", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ListSchedules(t *testing.T) {
	_, router := setupHandler()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/predictive-maintenance/schedules", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
