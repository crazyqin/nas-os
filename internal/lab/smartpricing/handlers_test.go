// Package smartpricing - 智能定价分析单元测试
package smartpricing

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

	config := DefaultSmartPricingConfig()
	manager := NewManager(config)
	logger := zap.NewNop()
	handler := NewHandler(manager, logger)

	router := gin.New()
	api := router.Group("/api")
	handler.RegisterRoutes(api)

	return router, handler
}

func TestGetPlans(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/smartpricing/plans", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	plans := data["plans"].([]interface{})
	assert.Greater(t, len(plans), 0)
}

func TestCompareCost(t *testing.T) {
	router, _ := setupTestRouter()

	// 准备请求
	reqBody := CostCompareRequest{
		StorageGB:     100,
		MonthlyReads:  10000,
		MonthlyWrites: 5000,
		TransferGB:    10,
		Providers:     []StorageProvider{ProviderAWSS3, ProviderAliyunOSS},
		Tiers:         []StorageTier{TierStandard},
	}

	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/smartpricing/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	comparisons := data["comparisons"].([]interface{})
	assert.Greater(t, len(comparisons), 0)
}

func TestGetRecommendations(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/smartpricing/recommendations?storage_gb=500&provider=aws_s3&tier=standard", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	recommendations := data["recommendations"].([]interface{})
	assert.GreaterOrEqual(t, len(recommendations), 0)
}

func TestGetCostTrends(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/smartpricing/trends?interval=monthly", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	trends := data["trends"].([]interface{})
	assert.Greater(t, len(trends), 0)
}

func TestCompareCostInvalidRequest(t *testing.T) {
	router, _ := setupTestRouter()

	// 无效请求 - 缺少必填字段
	reqBody := map[string]interface{}{
		"storage_gb": -100, // 无效的存储量
	}

	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/smartpricing/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
