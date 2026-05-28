// Package costpredict - 成本预测单元测试
package costpredict

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupTestRouter() (*gin.Engine, *Handler) {
	gin.SetMode(gin.TestMode)

	config := DefaultCostPredictConfig()
	manager := NewManager(config)
	logger := zap.NewNop()
	handler := NewHandler(manager, logger)

	router := gin.New()
	api := router.Group("/api")
	handler.RegisterRoutes(api)

	return router, handler
}

func TestGetForecast(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/costpredict/forecast?method=linear_regression&horizon=1_year", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	forecasts := data["forecasts"].([]interface{})
	assert.Greater(t, len(forecasts), 0)
}

func TestGetGrowthForecast(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/costpredict/growth?capacity_gb=20000", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	forecasts := data["forecasts"].([]interface{})
	assert.Greater(t, len(forecasts), 0)
}

func TestSetAlert(t *testing.T) {
	router, _ := setupTestRouter()

	reqBody := AlertConfigRequest{
		Name:              "Monthly Budget Alert",
		Type:              AlertBudgetWarning,
		Budget:            1000.0,
		WarningThreshold:  80.0,
		CriticalThreshold: 95.0,
		Enabled:           true,
		Provider:          "aws_s3",
	}

	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/costpredict/alert", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, "Monthly Budget Alert", data["name"])
}

func TestGetReport(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/costpredict/report?provider=ssd", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	assert.NotEmpty(t, data["id"])
	assert.NotEmpty(t, data["summary"])
}

func TestSetAlertInvalidRequest(t *testing.T) {
	router, _ := setupTestRouter()

	// 无效请求 - 阈值设置错误
	reqBody := map[string]interface{}{
		"name":               "Invalid Alert",
		"type":               "budget_warning",
		"budget":             1000.0,
		"warning_threshold":  95.0,
		"critical_threshold": 80.0, // 错误：critical < warning
		"enabled":            true,
	}

	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/costpredict/alert", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}