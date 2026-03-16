package sftp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTransferTestHandlers(t *testing.T) (*TransferHandlers, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	logger, err := NewTransferLogger(TransferLoggerConfig{Enabled: false})
	require.NoError(t, err)

	handlers := NewTransferHandlers(logger, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	return handlers, router
}

func TestTransferHandlers_RegisterRoutes(t *testing.T) {
	_, router := setupTransferTestHandlers(t)

	routes := []string{
		"GET /api/sftp/transfers",
		"GET /api/sftp/transfers/stats",
		"DELETE /api/sftp/transfers",
		"GET /api/sftp/transfers/config",
		"PUT /api/sftp/transfers/config",
	}

	for _, route := range routes {
		parts := strings.Split(route, " ")
		req := httptest.NewRequest(parts[0], parts[1], nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		// 不应该返回 404
		assert.NotEqual(t, http.StatusNotFound, w.Code, "Route not found: %s", route)
	}
}

func TestTransferHandlers_ListTransfers(t *testing.T) {
	handlers, router := setupTransferTestHandlers(t)

	// 添加测试日志
	handlers.logger.Log(&TransferLog{
		ID:        "test-001",
		Username:  "user1",
		Direction: "upload",
		Success:   true,
		Timestamp: time.Now(),
	})

	tests := []struct {
		name       string
		query      string
		expectCode int
	}{
		{
			name:       "list all",
			query:      "",
			expectCode: http.StatusOK,
		},
		{
			name:       "filter by username",
			query:      "?username=user1",
			expectCode: http.StatusOK,
		},
		{
			name:       "filter by direction",
			query:      "?direction=upload",
			expectCode: http.StatusOK,
		},
		{
			name:       "filter by success",
			query:      "?success=true",
			expectCode: http.StatusOK,
		},
		{
			name:       "with limit",
			query:      "?limit=10",
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/sftp/transfers"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)

			if tt.expectCode == http.StatusOK {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, float64(0), resp["code"])
			}
		})
	}
}

func TestTransferHandlers_GetStats(t *testing.T) {
	handlers, router := setupTransferTestHandlers(t)

	// 添加测试日志
	handlers.logger.Log(&TransferLog{
		Direction:  "upload",
		BytesTrans: 1024,
		Success:    true,
		Bandwidth:  1000,
		Timestamp:  time.Now(),
	})

	tests := []struct {
		name       string
		query      string
		expectCode int
	}{
		{
			name:       "default period",
			query:      "",
			expectCode: http.StatusOK,
		},
		{
			name:       "1 hour period",
			query:      "?period=1h",
			expectCode: http.StatusOK,
		},
		{
			name:       "24 hour period",
			query:      "?period=24h",
			expectCode: http.StatusOK,
		},
		{
			name:       "7 day period",
			query:      "?period=7d",
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/sftp/transfers/stats"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, float64(0), resp["code"])
		})
	}
}

func TestTransferHandlers_ClearLogs(t *testing.T) {
	_, router := setupTransferTestHandlers(t)

	req := httptest.NewRequest("DELETE", "/api/sftp/transfers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestTransferHandlers_GetConfig(t *testing.T) {
	_, router := setupTransferTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/sftp/transfers/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])

	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, data, "enabled")
}

func TestTransferHandlers_UpdateConfig(t *testing.T) {
	_, router := setupTransferTestHandlers(t)

	tests := []struct {
		name       string
		body       string
		expectCode int
	}{
		{
			name:       "enable logging",
			body:       `{"enabled": true}`,
			expectCode: http.StatusOK,
		},
		{
			name:       "disable logging",
			body:       `{"enabled": false}`,
			expectCode: http.StatusOK,
		},
		{
			name:       "invalid json",
			body:       `{invalid}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", "/api/sftp/transfers/config", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			if tt.expectCode == http.StatusOK {
				assert.Equal(t, float64(0), resp["code"])
			} else {
				assert.Equal(t, float64(400), resp["code"])
			}
		})
	}
}

func TestTransferHandlers_NilLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewTransferHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 测试空 logger 的情况
	t.Run("ListTransfers with nil logger", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sftp/transfers", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("GetStats with nil logger", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sftp/transfers/stats", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1h", time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"", 24 * time.Hour},        // default
		{"invalid", 24 * time.Hour}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parsePeriod(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestTransferLogger_NewTransferLogger(t *testing.T) {
	t.Run("with disabled logging", func(t *testing.T) {
		config := TransferLoggerConfig{
			Enabled: false,
		}

		logger, err := NewTransferLogger(config)
		require.NoError(t, err)
		require.NotNil(t, logger)
		assert.False(t, logger.IsEnabled())
	})
}

func TestTransferLogger_Log(t *testing.T) {
	logger, _ := NewTransferLogger(TransferLoggerConfig{Enabled: true})

	log := &TransferLog{
		ID:        "test-001",
		Timestamp: time.Now(),
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		SessionID: "session-123",
		Direction: "upload",
		FilePath:  "/test/file.txt",
		FileSize:  1024,
		Success:   true,
		Method:    "sftp",
	}

	logger.Log(log)

	logs := logger.GetLogs(10, 0, nil)
	assert.Len(t, logs, 1)
	assert.Equal(t, "test-001", logs[0].ID)
	assert.Equal(t, "testuser", logs[0].Username)
}

func TestTransferLogger_StartTransfer(t *testing.T) {
	logger, _ := NewTransferLogger(TransferLoggerConfig{Enabled: false})

	log := logger.StartTransfer("testuser", "192.168.1.100", "session-123", "upload", "/test/file.txt", 1024)

	assert.NotEmpty(t, log.ID)
	assert.Equal(t, "testuser", log.Username)
	assert.Equal(t, "192.168.1.100", log.ClientIP)
	assert.Equal(t, "session-123", log.SessionID)
	assert.Equal(t, "upload", log.Direction)
	assert.Equal(t, "/test/file.txt", log.FilePath)
	assert.Equal(t, int64(1024), log.FileSize)
	assert.Equal(t, "sftp", log.Method)
	assert.False(t, log.Timestamp.IsZero())
}

func TestTransferLogger_CompleteTransfer(t *testing.T) {
	logger, _ := NewTransferLogger(TransferLoggerConfig{Enabled: false})

	log := logger.StartTransfer("testuser", "192.168.1.100", "session-123", "download", "/test/file.txt", 2048)
	logger.CompleteTransfer(log, 2048, 2*time.Second, true, "")

	assert.Equal(t, int64(2048), log.BytesTrans)
	assert.Equal(t, int64(2000), log.Duration) // 2秒 = 2000毫秒
	assert.True(t, log.Success)
	assert.Empty(t, log.Error)
	assert.Greater(t, log.Bandwidth, int64(0))
}

func TestTransferLogger_GetLogs(t *testing.T) {
	logger, _ := NewTransferLogger(TransferLoggerConfig{Enabled: true})

	// 添加多条日志
	for i := 0; i < 5; i++ {
		log := &TransferLog{
			ID:        string(rune('a' + i)),
			Username:  "user1",
			Direction: "upload",
			Success:   i%2 == 0,
		}
		logger.Log(log)
	}

	// 获取所有日志
	logs := logger.GetLogs(10, 0, nil)
	assert.Len(t, logs, 5)

	// 测试过滤
	filter := &TransferLogFilter{
		Username: "user1",
	}
	logs = logger.GetLogs(10, 0, filter)
	assert.Len(t, logs, 5)
}

func TestTransferLogger_GetStats(t *testing.T) {
	logger, _ := NewTransferLogger(TransferLoggerConfig{Enabled: true})

	// 添加测试日志
	logger.Log(&TransferLog{
		Direction:  "upload",
		BytesTrans: 1024,
		Success:    true,
		Timestamp:  time.Now(),
	})
	logger.Log(&TransferLog{
		Direction:  "download",
		BytesTrans: 2048,
		Success:    true,
		Timestamp:  time.Now(),
	})
	logger.Log(&TransferLog{
		Direction:  "upload",
		BytesTrans: 512,
		Success:    false,
		Timestamp:  time.Now(),
	})

	stats := logger.GetStats(time.Hour)

	assert.Equal(t, 3, stats.TotalTransfers)
	assert.Equal(t, 2, stats.Uploads)
	assert.Equal(t, 1, stats.Downloads)
	assert.Equal(t, 2, stats.SuccessfulTransfers)
	assert.Equal(t, 1, stats.FailedTransfers)
}

func TestTransferLogger_SetEnabled(t *testing.T) {
	logger, _ := NewTransferLogger(TransferLoggerConfig{Enabled: false})

	assert.False(t, logger.IsEnabled())

	logger.SetEnabled(true)
	assert.True(t, logger.IsEnabled())

	logger.SetEnabled(false)
	assert.False(t, logger.IsEnabled())
}