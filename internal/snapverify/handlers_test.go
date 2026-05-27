// Package snapverify 提供快照自动验证测试功能
package snapverify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandler() (*Handler, *gin.Engine) {
	manager := NewSnapVerifyManager()
	handler := NewHandler(manager)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return handler, router
}

func TestRunTest(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("运行完整性测试", func(t *testing.T) {
		body := `{"snapshot_id": "snap-001", "test_type": 1}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/tests", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var test SnapshotTest
		err := json.Unmarshal(w.Body.Bytes(), &test)
		require.NoError(t, err)
		assert.NotEmpty(t, test.ID)
		assert.Equal(t, "snap-001", test.SnapshotID)
		assert.Equal(t, TestTypeIntegrity, test.TestType)
		assert.Equal(t, TestStatusRunning, test.Status)
	})

	t.Run("运行恢复测试", func(t *testing.T) {
		body := `{"snapshot_id": "snap-002", "test_type": 2}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/tests", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("无效的测试类型", func(t *testing.T) {
		body := `{"snapshot_id": "snap-003", "test_type": 6}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/tests", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("缺少必填字段", func(t *testing.T) {
		body := `{"test_type": 1}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/tests", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetTestResult(t *testing.T) {
	handler, router := setupTestHandler()

	// 先创建一个测试并等待完成
	test, err := handler.manager.RunTest(context.Background(), "snap-result-001", TestTypeIntegrity)
	require.NoError(t, err)

	// 等待测试完成
	time.Sleep(3 * time.Second)

	t.Run("获取存在的测试结果", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/tests/"+test.ID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result TestResult
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, test.ID, result.TestID)
	})

	t.Run("获取不存在的测试结果", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/tests/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestListTests(t *testing.T) {
	handler, router := setupTestHandler()

	// 创建一些测试
	handler.manager.RunTest(context.Background(), "snap-list-001", TestTypeIntegrity)
	handler.manager.RunTest(context.Background(), "snap-list-001", TestTypeRestore)
	handler.manager.RunTest(context.Background(), "snap-list-002", TestTypeFileCheck)

	t.Run("列出所有测试", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/tests", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, int(response["total"].(float64)), 3)
	})

	t.Run("按快照ID筛选", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/tests?snapshot_id=snap-list-001", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 2, int(response["total"].(float64)))
	})
}

func TestCreatePolicy(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("创建策略成功", func(t *testing.T) {
		body := `{
			"name": "测试策略",
			"schedule": "0 2 * * *",
			"test_type": 1,
			"auto_repair": false,
			"retention_days": 30,
			"enabled": true
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/policies", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "策略创建成功", response["message"])
	})

	t.Run("缺少策略名称", func(t *testing.T) {
		body := `{
			"schedule": "0 2 * * *",
			"test_type": 1,
			"enabled": true
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/policies", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("重复策略名称", func(t *testing.T) {
		// 先创建一个
		body1 := `{
			"name": "唯一策略",
			"schedule": "0 2 * * *",
			"test_type": 1,
			"enabled": true
		}`
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("POST", "/api/v1/snap-verify/policies", bytes.NewReader([]byte(body1)))
		req1.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusCreated, w1.Code)

		// 再创建同名
		body2 := `{
			"name": "唯一策略",
			"schedule": "0 3 * * *",
			"test_type": 2,
			"enabled": true
		}`
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("POST", "/api/v1/snap-verify/policies", bytes.NewReader([]byte(body2)))
		req2.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusInternalServerError, w2.Code)
	})
}

func TestUpdatePolicy(t *testing.T) {
	handler, router := setupTestHandler()

	// 创建策略
	policy := VerifyPolicy{
		Name:          "待更新策略",
		Schedule:      "0 2 * * *",
		TestType:      TestTypeIntegrity,
		Enabled:       true,
		RetentionDays: 30,
	}
	handler.manager.CreatePolicy(context.Background(), policy)

	policies := handler.manager.ListPolicies()
	var policyID string
	for _, p := range policies {
		if p.Name == "待更新策略" {
			policyID = p.ID
			break
		}
	}

	t.Run("更新策略成功", func(t *testing.T) {
		body := `{
			"name": "已更新策略",
			"schedule": "0 3 * * *",
			"enabled": false
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/snap-verify/policies/"+policyID, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("更新不存在的策略", func(t *testing.T) {
		body := `{"name": "不存在"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/snap-verify/policies/nonexistent", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestDeletePolicy(t *testing.T) {
	handler, router := setupTestHandler()

	// 创建策略
	policy := VerifyPolicy{
		Name:          "待删除策略",
		Schedule:      "0 2 * * *",
		TestType:      TestTypeIntegrity,
		Enabled:       true,
		RetentionDays: 30,
	}
	handler.manager.CreatePolicy(context.Background(), policy)

	policies := handler.manager.ListPolicies()
	var policyID string
	for _, p := range policies {
		if p.Name == "待删除策略" {
			policyID = p.ID
			break
		}
	}

	t.Run("删除策略成功", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/snap-verify/policies/"+policyID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("删除不存在的策略", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/snap-verify/policies/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestListPolicies(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("列出策略", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/policies", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, int(response["total"].(float64)), 2) // 默认策略
	})
}

func TestRunScheduled(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("运行计划任务", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/scheduled", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "计划任务已触发", response["message"])
	})
}

func TestAutoRepair(t *testing.T) {
	handler, router := setupTestHandler()

	// 创建一个测试
	test, err := handler.manager.RunTest(context.Background(), "snap-repair-001", TestTypeIntegrity)
	require.NoError(t, err)

	// 等待测试完成
	time.Sleep(3 * time.Second)

	// 获取结果确认
	result, err := handler.manager.GetTestResult(test.ID)
	require.NoError(t, err)

	t.Run("自动修复", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/tests/"+test.ID+"/repair", nil)
		router.ServeHTTP(w, req)

		// 如果测试未通过，修复应该成功
		if !result.Passed {
			assert.Equal(t, http.StatusOK, w.Code)
		} else {
			// 如果测试已通过，应该返回400
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	})

	t.Run("修复不存在的测试", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/snap-verify/tests/nonexistent/repair", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetStats(t *testing.T) {
	handler, router := setupTestHandler()

	// 创建一些测试
	handler.manager.RunTest(context.Background(), "snap-stats-001", TestTypeIntegrity)
	handler.manager.RunTest(context.Background(), "snap-stats-002", TestTypeRestore)

	// 等待测试完成
	time.Sleep(3 * time.Second)

	t.Run("获取统计信息", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats VerifyStats
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, stats.TotalTests, 2)
	})
}

func TestGenerateReport(t *testing.T) {
	handler, router := setupTestHandler()

	// 创建一个测试
	test, err := handler.manager.RunTest(context.Background(), "snap-report-001", TestTypeIntegrity)
	require.NoError(t, err)

	// 等待测试完成
	time.Sleep(3 * time.Second)

	t.Run("生成JSON报告", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/tests/"+test.ID+"/report?format=json", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})

	t.Run("生成文本报告", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/tests/"+test.ID+"/report?format=text", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
		assert.Contains(t, w.Body.String(), "=== 快照验证测试报告 ===")
	})

	t.Run("报告不存在的测试", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/snap-verify/tests/nonexistent/report", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestManagerRunTest(t *testing.T) {
	manager := NewSnapVerifyManager()

	t.Run("运行测试成功", func(t *testing.T) {
		test, err := manager.RunTest(context.Background(), "snap-mgr-001", TestTypeIntegrity)
		require.NoError(t, err)
		assert.NotEmpty(t, test.ID)
		assert.Equal(t, "snap-mgr-001", test.SnapshotID)
		assert.Equal(t, TestStatusRunning, test.Status)
	})

	t.Run("无效的快照ID", func(t *testing.T) {
		_, err := manager.RunTest(context.Background(), "", TestTypeIntegrity)
		assert.Error(t, err)
	})

	t.Run("无效的测试类型", func(t *testing.T) {
		_, err := manager.RunTest(context.Background(), "snap-mgr-002", TestType(0))
		assert.Error(t, err)
	})
}

func TestManagerPolicyLifecycle(t *testing.T) {
	manager := NewSnapVerifyManager()

	// 创建
	policy := VerifyPolicy{
		Name:          "生命周期测试策略",
		Schedule:      "0 4 * * *",
		TestType:      TestTypeFull,
		AutoRepair:    true,
		RetentionDays: 60,
		Enabled:       true,
	}
	err := manager.CreatePolicy(context.Background(), policy)
	require.NoError(t, err)

	// 列出
	policies := manager.ListPolicies()
	found := false
	var policyID string
	for _, p := range policies {
		if p.Name == "生命周期测试策略" {
			found = true
			policyID = p.ID
			break
		}
	}
	assert.True(t, found)

	// 更新
	updatePolicy := VerifyPolicy{
		Name:    "已更新生命周期策略",
		Enabled: false,
	}
	err = manager.UpdatePolicy(policyID, updatePolicy)
	require.NoError(t, err)

	// 验证更新
	policies = manager.ListPolicies()
	for _, p := range policies {
		if p.ID == policyID {
			assert.Equal(t, "已更新生命周期策略", p.Name)
			assert.False(t, p.Enabled)
		}
	}

	// 删除
	err = manager.DeletePolicy(policyID)
	require.NoError(t, err)

	// 验证删除
	policies = manager.ListPolicies()
	for _, p := range policies {
		assert.NotEqual(t, policyID, p.ID)
	}
}
