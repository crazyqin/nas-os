package quota

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestQuotaError_Error 测试错误消息.
func TestQuotaError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *QuotaError
		expected string
	}{
		{
			name:     "quota not found",
			err:      ErrQuotaNotFoundAPI,
			expected: "配额不存在",
		},
		{
			name:     "quota exists",
			err:      ErrQuotaExistsAPI,
			expected: "配额已存在",
		},
		{
			name:     "quota exceeded",
			err:      ErrQuotaExceededAPI,
			expected: "超出配额限制",
		},
		{
			name:     "user not found",
			err:      ErrUserNotFoundAPI,
			expected: "用户不存在",
		},
		{
			name:     "group not found",
			err:      ErrGroupNotFoundAPI,
			expected: "用户组不存在",
		},
		{
			name:     "volume not found",
			err:      ErrVolumeNotFoundAPI,
			expected: "卷不存在",
		},
		{
			name:     "invalid limit",
			err:      ErrInvalidLimitAPI,
			expected: "无效的配额限制",
		},
		{
			name:     "policy not found",
			err:      ErrPolicyNotFoundAPI,
			expected: "清理策略不存在",
		},
		{
			name:     "invalid input",
			err:      ErrInvalidInputAPI,
			expected: "无效的输入参数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message '%s', got '%s'", tt.expected, tt.err.Error())
			}
		})
	}
}

// TestQuotaError_Code 测试错误码.
func TestQuotaError_Code(t *testing.T) {
	tests := []struct {
		name         string
		err          *QuotaError
		expectedCode int
	}{
		{"quota not found", ErrQuotaNotFoundAPI, ErrCodeQuotaNotFound},
		{"quota exists", ErrQuotaExistsAPI, ErrCodeQuotaExists},
		{"quota exceeded", ErrQuotaExceededAPI, ErrCodeQuotaExceeded},
		{"user not found", ErrUserNotFoundAPI, ErrCodeUserNotFound},
		{"group not found", ErrGroupNotFoundAPI, ErrCodeGroupNotFound},
		{"volume not found", ErrVolumeNotFoundAPI, ErrCodeVolumeNotFound},
		{"invalid limit", ErrInvalidLimitAPI, ErrCodeInvalidLimit},
		{"policy not found", ErrPolicyNotFoundAPI, ErrCodePolicyNotFound},
		{"invalid input", ErrInvalidInputAPI, ErrCodeInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code() != tt.expectedCode {
				t.Errorf("Expected code %d, got %d", tt.expectedCode, tt.err.Code())
			}
		})
	}
}

// TestQuotaError_Details 测试错误详情.
func TestQuotaError_Details(t *testing.T) {
	details := map[string]interface{}{
		"volume": "test-volume",
		"user":   "test-user",
	}

	err := NewQuotaError(ErrCodeQuotaNotFound, "配额不存在", details)
	if err.Details() == nil {
		t.Error("Expected details to be non-nil")
	}
	if err.Details()["volume"] != "test-volume" {
		t.Errorf("Expected volume to be 'test-volume', got %v", err.Details()["volume"])
	}

	// 测试没有详情的情况
	err2 := NewQuotaError(ErrCodeQuotaNotFound, "配额不存在")
	if err2.Details() != nil {
		t.Error("Expected details to be nil")
	}
}

// TestQuotaError_WithDetails 测试添加详情.
func TestQuotaError_WithDetails(t *testing.T) {
	err := ErrQuotaNotFoundAPI.WithDetails(map[string]interface{}{
		"path": "/data/test",
	})

	if err.Details() == nil {
		t.Fatal("Expected details to be non-nil")
	}
	if err.Details()["path"] != "/data/test" {
		t.Errorf("Expected path to be '/data/test', got %v", err.Details()["path"])
	}
}

// TestQuotaError_NotFound 测试 NotFound 接口.
func TestQuotaError_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      *QuotaError
		expected bool
	}{
		{"quota not found", ErrQuotaNotFoundAPI, true},
		{"policy not found", ErrPolicyNotFoundAPI, true},
		{"quota exists", ErrQuotaExistsAPI, false},
		{"invalid input", ErrInvalidInputAPI, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.NotFound() != tt.expected {
				t.Errorf("Expected NotFound() to return %v, got %v", tt.expected, tt.err.NotFound())
			}
		})
	}
}

// TestQuotaError_BadRequest 测试 BadRequest 接口.
func TestQuotaError_BadRequest(t *testing.T) {
	tests := []struct {
		name     string
		err      *QuotaError
		expected bool
	}{
		{"invalid input", ErrInvalidInputAPI, true},
		{"invalid limit", ErrInvalidLimitAPI, true},
		{"quota not found", ErrQuotaNotFoundAPI, false},
		{"quota exists", ErrQuotaExistsAPI, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.BadRequest() != tt.expected {
				t.Errorf("Expected BadRequest() to return %v, got %v", tt.expected, tt.err.BadRequest())
			}
		})
	}
}

// TestQuotaError_Conflict 测试 Conflict 接口.
func TestQuotaError_Conflict(t *testing.T) {
	tests := []struct {
		name     string
		err      *QuotaError
		expected bool
	}{
		{"quota exists", ErrQuotaExistsAPI, true},
		{"quota not found", ErrQuotaNotFoundAPI, false},
		{"invalid input", ErrInvalidInputAPI, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Conflict() != tt.expected {
				t.Errorf("Expected Conflict() to return %v, got %v", tt.expected, tt.err.Conflict())
			}
		})
	}
}

// TestNewQuotaError 测试创建错误.
func TestNewQuotaError(t *testing.T) {
	// 无详情
	err := NewQuotaError(ErrCodeQuotaNotFound, "test error")
	if err == nil {
		t.Fatal("Expected non-nil error")
	}
	if err.Code() != ErrCodeQuotaNotFound {
		t.Errorf("Expected code %d, got %d", ErrCodeQuotaNotFound, err.Code())
	}
	if err.Error() != "test error" {
		t.Errorf("Expected error message 'test error', got '%s'", err.Error())
	}

	// 有详情
	details := map[string]interface{}{"key": "value"}
	err2 := NewQuotaError(ErrCodeQuotaExists, "another error", details)
	if err2.Details() == nil {
		t.Error("Expected details to be non-nil")
	}
}

// TestToAPIError 测试错误转换.
func TestToAPIError(t *testing.T) {
	tests := []struct {
		name         string
		input        error
		expectedCode int
		expectedNil  bool
	}{
		{"nil error", nil, 0, true},
		{"QuotaError", ErrQuotaNotFoundAPI, ErrCodeQuotaNotFound, false},
		{"ErrQuotaNotFound", ErrQuotaNotFound, ErrCodeQuotaNotFound, false},
		{"ErrQuotaExists", ErrQuotaExists, ErrCodeQuotaExists, false},
		{"ErrQuotaExceeded", ErrQuotaExceeded, ErrCodeQuotaExceeded, false},
		{"ErrUserNotFound", ErrUserNotFound, ErrCodeUserNotFound, false},
		{"ErrGroupNotFound", ErrGroupNotFound, ErrCodeGroupNotFound, false},
		{"ErrVolumeNotFound", ErrVolumeNotFound, ErrCodeVolumeNotFound, false},
		{"ErrInvalidLimit", ErrInvalidLimit, ErrCodeInvalidLimit, false},
		{"ErrCleanupPolicyNotFound", ErrCleanupPolicyNotFound, ErrCodePolicyNotFound, false},
		{"unknown error", errors.New("unknown"), ErrCodeInvalidInput, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToAPIError(tt.input)

			if tt.expectedNil {
				if result != nil {
					t.Error("Expected nil result")
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}
			if result.Code() != tt.expectedCode {
				t.Errorf("Expected code %d, got %d", tt.expectedCode, result.Code())
			}
		})
	}
}

// TestErrorResponse 测试错误响应.
func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   int
	}{
		{"quota not found", ErrQuotaNotFound, http.StatusNotFound, ErrCodeQuotaNotFound},
		{"quota exists", ErrQuotaExists, http.StatusConflict, ErrCodeQuotaExists},
		{"invalid input", ErrInvalidInputAPI, http.StatusBadRequest, ErrCodeInvalidInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			ErrorResponse(c, tt.err)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestValidateQuotaInput 测试配额输入验证.
func TestValidateQuotaInput(t *testing.T) {
	tests := []struct {
		name        string
		input       QuotaInput
		expectError bool
	}{
		{
			name: "valid user quota",
			input: QuotaInput{
				Type:      QuotaTypeUser,
				TargetID:  "user1",
				HardLimit: 1000000,
				SoftLimit: 800000,
			},
			expectError: false,
		},
		{
			name: "valid group quota",
			input: QuotaInput{
				Type:      QuotaTypeGroup,
				TargetID:  "group1",
				HardLimit: 10000000,
				SoftLimit: 8000000,
			},
			expectError: false,
		},
		{
			name: "valid directory quota",
			input: QuotaInput{
				Type:      QuotaTypeDirectory,
				TargetID:  "/data/test",
				HardLimit: 5000000,
				SoftLimit: 4000000,
			},
			expectError: false,
		},
		{
			name: "missing type",
			input: QuotaInput{
				TargetID:  "user1",
				HardLimit: 1000000,
			},
			expectError: true,
		},
		{
			name: "invalid type",
			input: QuotaInput{
				Type:      "invalid",
				TargetID:  "user1",
				HardLimit: 1000000,
			},
			expectError: true,
		},
		{
			name: "missing target ID",
			input: QuotaInput{
				Type:      QuotaTypeUser,
				HardLimit: 1000000,
			},
			expectError: true,
		},
		{
			name: "zero hard limit",
			input: QuotaInput{
				Type:      QuotaTypeUser,
				TargetID:  "user1",
				HardLimit: 0,
			},
			expectError: true,
		},
		{
			name: "soft limit greater than hard limit",
			input: QuotaInput{
				Type:      QuotaTypeUser,
				TargetID:  "user1",
				HardLimit: 1000000,
				SoftLimit: 2000000,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuotaInput(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}

// TestValidateCleanupPolicyInput 测试清理策略输入验证.
func TestValidateCleanupPolicyInput(t *testing.T) {
	tests := []struct {
		name        string
		input       CleanupPolicyInput
		expectError bool
	}{
		{
			name: "valid age policy",
			input: CleanupPolicyInput{
				Name:       "age-policy",
				VolumeName: "volume1",
				Type:       CleanupPolicyAge,
				Action:     "delete",
				MaxAge:     30,
			},
			expectError: false,
		},
		{
			name: "valid size policy",
			input: CleanupPolicyInput{
				Name:       "size-policy",
				VolumeName: "volume1",
				Type:       CleanupPolicySize,
				Action:     "move",
				MinSize:    1048576,
			},
			expectError: false,
		},
		{
			name: "valid quota policy",
			input: CleanupPolicyInput{
				Name:         "quota-policy",
				VolumeName:   "volume1",
				Type:         CleanupPolicyQuota,
				Action:       "alert",
				QuotaPercent: 80,
			},
			expectError: false,
		},
		{
			name: "valid access policy",
			input: CleanupPolicyInput{
				Name:         "access-policy",
				VolumeName:   "volume1",
				Type:         CleanupPolicyAccess,
				Action:       "archive",
				MaxAccessAge: 90,
			},
			expectError: false,
		},
		{
			name: "missing name",
			input: CleanupPolicyInput{
				VolumeName: "volume1",
				Type:       CleanupPolicyAge,
				Action:     "delete",
			},
			expectError: true,
		},
		{
			name: "missing volume name",
			input: CleanupPolicyInput{
				Name:   "policy1",
				Type:   CleanupPolicyAge,
				Action: "delete",
			},
			expectError: true,
		},
		{
			name: "missing type",
			input: CleanupPolicyInput{
				Name:       "policy1",
				VolumeName: "volume1",
				Action:     "delete",
			},
			expectError: true,
		},
		{
			name: "missing action",
			input: CleanupPolicyInput{
				Name:       "policy1",
				VolumeName: "volume1",
				Type:       CleanupPolicyAge,
			},
			expectError: true,
		},
		{
			name: "age policy with zero max age",
			input: CleanupPolicyInput{
				Name:       "policy1",
				VolumeName: "volume1",
				Type:       CleanupPolicyAge,
				Action:     "delete",
				MaxAge:     0,
			},
			expectError: true,
		},
		{
			name: "size policy with zero min size",
			input: CleanupPolicyInput{
				Name:       "policy1",
				VolumeName: "volume1",
				Type:       CleanupPolicySize,
				Action:     "delete",
				MinSize:    0,
			},
			expectError: true,
		},
		{
			name: "quota policy with invalid percent",
			input: CleanupPolicyInput{
				Name:         "policy1",
				VolumeName:   "volume1",
				Type:         CleanupPolicyQuota,
				Action:       "alert",
				QuotaPercent: 0,
			},
			expectError: true,
		},
		{
			name: "quota policy with percent over 100",
			input: CleanupPolicyInput{
				Name:         "policy1",
				VolumeName:   "volume1",
				Type:         CleanupPolicyQuota,
				Action:       "alert",
				QuotaPercent: 150,
			},
			expectError: true,
		},
		{
			name: "access policy with zero max access age",
			input: CleanupPolicyInput{
				Name:         "policy1",
				VolumeName:   "volume1",
				Type:         CleanupPolicyAccess,
				Action:       "archive",
				MaxAccessAge: 0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCleanupPolicyInput(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}

// TestValidateID 测试 ID 验证.
func TestValidateID(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		expectError bool
	}{
		{"valid id", "user123", false},
		{"empty id", "", true},
		{"id too long", string(make([]byte, 65)), true},
		{"id at max length", string(make([]byte, 64)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateID(tt.id)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}

// TestValidatePath 测试路径验证.
func TestValidatePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"valid path", "/data/test", false},
		{"empty path", "", true},
		{"path too long", "/" + string(make([]byte, 1024)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}

// TestValidateVolumeName 测试卷名验证.
func TestValidateVolumeName(t *testing.T) {
	tests := []struct {
		name        string
		volumeName  string
		expectError bool
	}{
		{"valid name", "volume1", false},
		{"empty name", "", true},
		{"name too long", string(make([]byte, 65)), true},
		{"name at max length", string(make([]byte, 64)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumeName(tt.volumeName)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}

// TestErrorCodeConstants 测试错误码常量.
func TestErrorCodeConstants(t *testing.T) {
	codes := map[string]int{
		"ErrCodeQuotaNotFound":  ErrCodeQuotaNotFound,
		"ErrCodeQuotaExists":    ErrCodeQuotaExists,
		"ErrCodeQuotaExceeded":  ErrCodeQuotaExceeded,
		"ErrCodeUserNotFound":   ErrCodeUserNotFound,
		"ErrCodeGroupNotFound":  ErrCodeGroupNotFound,
		"ErrCodeVolumeNotFound": ErrCodeVolumeNotFound,
		"ErrCodeInvalidLimit":   ErrCodeInvalidLimit,
		"ErrCodePolicyNotFound": ErrCodePolicyNotFound,
		"ErrCodeInvalidInput":   ErrCodeInvalidInput,
		"ErrCodeAlertNotFound":  ErrCodeAlertNotFound,
	}

	for name, code := range codes {
		if code < 1000 || code > 2000 {
			t.Errorf("Error code %s has unexpected value %d", name, code)
		}
	}
}

// BenchmarkQuotaError_Error 基准测试.
func BenchmarkQuotaError_Error(b *testing.B) {
	err := ErrQuotaNotFoundAPI
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

// BenchmarkToAPIError 基准测试.
func BenchmarkToAPIError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ToAPIError(ErrQuotaNotFound)
	}
}

// BenchmarkValidateQuotaInput 基准测试.
func BenchmarkValidateQuotaInput(b *testing.B) {
	input := QuotaInput{
		Type:      QuotaTypeUser,
		TargetID:  "user1",
		HardLimit: 1000000,
		SoftLimit: 800000,
	}
	for i := 0; i < b.N; i++ {
		_ = ValidateQuotaInput(input)
	}
}
