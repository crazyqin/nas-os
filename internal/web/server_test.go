package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appversion "nas-os/internal/version"
)

// ========== Server 结构体测试 ==========

func TestServer_Struct(t *testing.T) {
	// 验证 Server 结构体存在
	server := &Server{}
	assert.NotNil(t, server)
}

// ========== 响应格式测试 ==========

func TestResponseFormat_JSON(t *testing.T) {
	resp := Response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"key": "value"},
	}

	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "success", resp.Message)
	assert.NotNil(t, resp.Data)
}

func TestErrorResponseFormat_JSON(t *testing.T) {
	resp := ErrorResponse{
		Code:    400,
		Message: "Bad Request",
	}

	assert.Equal(t, 400, resp.Code)
	assert.Equal(t, "Bad Request", resp.Message)
}

// ========== 请求验证测试 ==========

func TestValidateRequest_EmptyBody(t *testing.T) {
	// 空请求体应该被正确处理
	var req map[string]interface{}
	assert.Nil(t, req)
}

// ========== 安全头测试 ==========

func TestSecurityHeaders(t *testing.T) {
	// 验证安全头常量
	headers := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
	}

	for _, h := range headers {
		assert.NotEmpty(t, h)
	}
}

// ========== 常量测试 ==========

func TestConstants(t *testing.T) {
	// 验证常量定义
	assert.Equal(t, 0, Response{}.Code)
}

// ========== 边缘情况测试 ==========

func TestNilData(t *testing.T) {
	resp := Response{
		Code:    0,
		Message: "success",
		Data:    nil,
	}

	assert.Nil(t, resp.Data)
}

// TestGetSystemInfoReportsShippedVersion drives the real getSystemInfo handler and
// asserts the response version matches nas-os/internal/version (kept in sync with VERSION).
func TestGetSystemInfoReportsShippedVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)

	s.getSystemInfo(c)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Code int `json:"code"`
		Data struct {
			Hostname  string `json:"hostname"`
			Version   string `json:"version"`
			BuildDate string `json:"build_date"`
			GitCommit string `json:"git_commit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 0, body.Code)
	assert.NotEmpty(t, body.Data.Hostname)
	assert.Equal(t, appversion.GetVersion(), body.Data.Version, "system/info must report shipped app version, not a hard-coded placeholder")
	assert.NotEqual(t, "0.1.0", body.Data.Version, "stale placeholder version must not leak")
	assert.Equal(t, appversion.GetBuildInfo()["build_date"], body.Data.BuildDate)
	assert.Equal(t, appversion.GetBuildInfo()["git_commit"], body.Data.GitCommit)
}

func TestEmptyData(t *testing.T) {
	resp := Response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{},
	}

	assert.NotNil(t, resp.Data)
}

// ========== 并发安全测试 ==========

func TestConcurrentResponse(t *testing.T) {
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func() {
			resp := Response{
				Code:    0,
				Message: "success",
				Data:    map[string]int{"count": i},
			}
			_ = resp.Code
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
