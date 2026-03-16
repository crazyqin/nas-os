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

func TestQuotaError_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      *QuotaError
		expected bool
	}{
		{"QuotaNotFound", &QuotaError{code: ErrCodeQuotaNotFound}, true},
		{"PolicyNotFound", &QuotaError{code: ErrCodePolicyNotFound}, true},
		{"QuotaExists", &QuotaError{code: ErrCodeQuotaExists}, false},
		{"InvalidInput", &QuotaError{code: ErrCodeInvalidInput}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.err.NotFound(); result != tt.expected {
				t.Errorf("NotFound() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQuotaError_BadRequest(t *testing.T) {
	tests := []struct {
		name     string
		err      *QuotaError
		expected bool
	}{
		{"InvalidInput", &QuotaError{code: ErrCodeInvalidInput}, true},
		{"InvalidLimit", &QuotaError{code: ErrCodeInvalidLimit}, true},
		{"QuotaNotFound", &QuotaError{code: ErrCodeQuotaNotFound}, false},
		{"QuotaExists", &QuotaError{code: ErrCodeQuotaExists}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.err.BadRequest(); result != tt.expected {
				t.Errorf("BadRequest() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQuotaError_Conflict(t *testing.T) {
	tests := []struct {
		name     string
		err      *QuotaError
		expected bool
	}{
		{"QuotaExists", &QuotaError{code: ErrCodeQuotaExists}, true},
		{"QuotaNotFound", &QuotaError{code: ErrCodeQuotaNotFound}, false},
		{"InvalidInput", &QuotaError{code: ErrCodeInvalidInput}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.err.Conflict(); result != tt.expected {
				t.Errorf("Conflict() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQuotaError_Error(t *testing.T) {
	err := &QuotaError{message: "test error message"}
	if err.Error() != "test error message" {
		t.Errorf("Error() = %v, want %v", err.Error(), "test error message")
	}
}

func TestQuotaError_Code(t *testing.T) {
	err := &QuotaError{code: ErrCodeQuotaNotFound}
	if err.Code() != ErrCodeQuotaNotFound {
		t.Errorf("Code() = %v, want %v", err.Code(), ErrCodeQuotaNotFound)
	}
}

func TestQuotaError_Details(t *testing.T) {
	details := map[string]interface{}{"key": "value"}
	err := &QuotaError{details: details}
	if err.Details()["key"] != "value" {
		t.Errorf("Details() = %v, want %v", err.Details(), details)
	}

	// Nil details
	err2 := &QuotaError{}
	if err2.Details() != nil {
		t.Errorf("Details() should be nil for empty error")
	}
}

func TestNewQuotaError(t *testing.T) {
	err := NewQuotaError(ErrCodeQuotaNotFound, "not found")
	if err.code != ErrCodeQuotaNotFound {
		t.Errorf("code = %v, want %v", err.code, ErrCodeQuotaNotFound)
	}
	if err.message != "not found" {
		t.Errorf("message = %v, want %v", err.message, "not found")
	}

	// With details
	details := map[string]interface{}{"key": "value"}
	err2 := NewQuotaError(ErrCodeQuotaExists, "exists", details)
	if err2.Details()["key"] != "value" {
		t.Errorf("Details should contain key=value")
	}
}

func TestQuotaError_WithDetails(t *testing.T) {
	err := &QuotaError{code: ErrCodeQuotaNotFound, message: "not found"}
	details := map[string]interface{}{"volume": "test"}
	result := err.WithDetails(details)

	if result != err {
		t.Error("WithDetails should return the same error")
	}
	if err.Details()["volume"] != "test" {
		t.Errorf("Details should contain volume=test")
	}
}

func TestToAPIError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected *QuotaError
	}{
		{"Nil", nil, nil},
		{"QuotaError", &QuotaError{code: ErrCodeQuotaNotFound, message: "test"}, &QuotaError{code: ErrCodeQuotaNotFound, message: "test"}},
		{"ErrQuotaNotFound", ErrQuotaNotFound, ErrQuotaNotFoundAPI},
		{"ErrQuotaExists", ErrQuotaExists, ErrQuotaExistsAPI},
		{"ErrQuotaExceeded", ErrQuotaExceeded, ErrQuotaExceededAPI},
		{"ErrUserNotFound", ErrUserNotFound, ErrUserNotFoundAPI},
		{"ErrGroupNotFound", ErrGroupNotFound, ErrGroupNotFoundAPI},
		{"ErrVolumeNotFound", ErrVolumeNotFound, ErrVolumeNotFoundAPI},
		{"ErrInvalidLimit", ErrInvalidLimit, ErrInvalidLimitAPI},
		{"ErrCleanupPolicyNotFound", ErrCleanupPolicyNotFound, ErrPolicyNotFoundAPI},
		{"Unknown error", errors.New("unknown"), &QuotaError{code: ErrCodeInvalidInput, message: "unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToAPIError(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("ToAPIError() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Errorf("ToAPIError() = nil, want non-nil")
				return
			}
			if result.code != tt.expected.code {
				t.Errorf("code = %v, want %v", result.code, tt.expected.code)
			}
		})
	}
}

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectStatus int
		expectCode   int
	}{
		{"QuotaNotFound", ErrQuotaNotFound, 404, ErrCodeQuotaNotFound},
		{"QuotaExists", ErrQuotaExists, 409, ErrCodeQuotaExists},
		{"InvalidInput", errors.New("invalid"), 400, ErrCodeInvalidInput},
		{"PolicyNotFound", ErrCleanupPolicyNotFound, 404, ErrCodePolicyNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			ErrorResponse(c, tt.err)

			if w.Code != tt.expectStatus {
				t.Errorf("status = %v, want %v", w.Code, tt.expectStatus)
			}
		})
	}
}

func TestErrorResponse_Nil(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ErrorResponse(c, nil)

	// Should not write anything for nil error
	if w.Code != http.StatusOK {
		t.Errorf("status should remain 200 for nil error, got %v", w.Code)
	}
}

func TestValidateQuotaInput(t *testing.T) {
	tests := []struct {
		name    string
		input   QuotaInput
		wantErr bool
	}{
		{"Valid user quota", QuotaInput{Type: QuotaTypeUser, TargetID: "user1", HardLimit: 1000, SoftLimit: 800}, false},
		{"Valid group quota", QuotaInput{Type: QuotaTypeGroup, TargetID: "group1", HardLimit: 10000, SoftLimit: 8000}, false},
		{"Valid directory quota", QuotaInput{Type: QuotaTypeDirectory, TargetID: "/data/dir", HardLimit: 50000, SoftLimit: 40000}, false},
		{"Empty type", QuotaInput{Type: "", TargetID: "user1", HardLimit: 1000}, true},
		{"Invalid type", QuotaInput{Type: "invalid", TargetID: "user1", HardLimit: 1000}, true},
		{"Empty target ID", QuotaInput{Type: QuotaTypeUser, TargetID: "", HardLimit: 1000}, true},
		{"Zero hard limit", QuotaInput{Type: QuotaTypeUser, TargetID: "user1", HardLimit: 0}, true},
		{"Soft > Hard", QuotaInput{Type: QuotaTypeUser, TargetID: "user1", HardLimit: 1000, SoftLimit: 1500}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuotaInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQuotaInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCleanupPolicyInput(t *testing.T) {
	tests := []struct {
		name    string
		input   CleanupPolicyInput
		wantErr bool
	}{
		{"Valid age policy", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: CleanupPolicyAge, Action: "delete", MaxAge: 30}, false},
		{"Valid size policy", CleanupPolicyInput{Name: "policy2", VolumeName: "vol1", Type: CleanupPolicySize, Action: "move", MinSize: 1024}, false},
		{"Valid quota policy", CleanupPolicyInput{Name: "policy3", VolumeName: "vol1", Type: CleanupPolicyQuota, Action: "delete", QuotaPercent: 80}, false},
		{"Valid access policy", CleanupPolicyInput{Name: "policy4", VolumeName: "vol1", Type: CleanupPolicyAccess, Action: "archive", MaxAccessAge: 90}, false},
		{"Empty name", CleanupPolicyInput{Name: "", VolumeName: "vol1", Type: CleanupPolicyAge, Action: "delete"}, true},
		{"Empty volume", CleanupPolicyInput{Name: "policy1", VolumeName: "", Type: CleanupPolicyAge, Action: "delete"}, true},
		{"Empty type", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: "", Action: "delete"}, true},
		{"Empty action", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: CleanupPolicyAge, Action: ""}, true},
		{"Age with zero max age", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: CleanupPolicyAge, Action: "delete", MaxAge: 0}, true},
		{"Size with zero min size", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: CleanupPolicySize, Action: "delete", MinSize: 0}, true},
		{"Quota with invalid percent", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: CleanupPolicyQuota, Action: "delete", QuotaPercent: 0}, true},
		{"Quota with percent > 100", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: CleanupPolicyQuota, Action: "delete", QuotaPercent: 150}, true},
		{"Access with zero max access age", CleanupPolicyInput{Name: "policy1", VolumeName: "vol1", Type: CleanupPolicyAccess, Action: "delete", MaxAccessAge: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCleanupPolicyInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCleanupPolicyInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"Valid ID", "user-123", false},
		{"Empty ID", "", true},
		{"Too long ID", string(make([]byte, 65)), true},
		{"Max length ID", string(make([]byte, 64)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid path", "/data/users/test", false},
		{"Empty path", "", true},
		{"Too long path", string(make([]byte, 1025)), true},
		{"Max length path", string(make([]byte, 1024)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVolumeName(t *testing.T) {
	tests := []struct {
		name    string
		volume  string
		wantErr bool
	}{
		{"Valid volume", "volume-1", false},
		{"Empty volume", "", true},
		{"Too long volume", string(make([]byte, 65)), true},
		{"Max length volume", string(make([]byte, 64)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumeName(tt.volume)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVolumeName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestErrorCodes(t *testing.T) {
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
			t.Errorf("%s = %d, want between 1000-2000", name, code)
		}
	}
}

func TestPredefinedErrors(t *testing.T) {
	errors := []*QuotaError{
		ErrQuotaNotFoundAPI,
		ErrQuotaExistsAPI,
		ErrQuotaExceededAPI,
		ErrUserNotFoundAPI,
		ErrGroupNotFoundAPI,
		ErrVolumeNotFoundAPI,
		ErrInvalidLimitAPI,
		ErrPolicyNotFoundAPI,
		ErrInvalidInputAPI,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Predefined error should not be nil")
		}
		if err.message == "" {
			t.Error("Predefined error should have message")
		}
		if err.code == 0 {
			t.Error("Predefined error should have code")
		}
	}
}