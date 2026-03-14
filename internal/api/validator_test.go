// Package api 提供 API 验证器测试
package api

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestValidateUsername(t *testing.T) {
	InitValidator()

	tests := []struct {
		username string
		valid    bool
	}{
		{"validuser", true},
		{"ValidUser123", true},
		{"user_name", true},
		{"ab", false},        // too short
		{"2username", false}, // starts with number
		{"_username", false}, // starts with underscore
		{"user-name", false}, // contains hyphen
		{"", false},          // empty
		{"a", false},         // too short
		{"thisisaverylongusernamethatexceedsthirtytwocharacters", false}, // too long
	}

	for _, tt := range tests {
		err := ValidateVar(tt.username, "username")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("Username %q: expected valid=%v, got valid=%v (err=%v)", tt.username, tt.valid, isValid, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	InitValidator()

	tests := []struct {
		password string
		valid    bool
	}{
		{"password123", true},
		{"123456", true},
		{"short", false},
		{"", false},
		{"      ", true}, // spaces are valid (6 chars)
		{"     ", false}, // 5 chars, too short
	}

	for _, tt := range tests {
		err := ValidateVar(tt.password, "password")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("Password %q: expected valid=%v, got valid=%v", tt.password, tt.valid, isValid)
		}
	}
}

func TestValidateVolumeName(t *testing.T) {
	InitValidator()

	tests := []struct {
		name  string
		valid bool
	}{
		{"myvolume", true},
		{"my-volume", true},
		{"my_volume", true},
		{"volume123", true},
		{"", false},
		{"volume with space", false},
		{"volume@name", false},
		{"volume.name", false},
	}

	for _, tt := range tests {
		err := ValidateVar(tt.name, "volume_name")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("Volume name %q: expected valid=%v, got valid=%v", tt.name, tt.valid, isValid)
		}
	}
}

func TestValidateContainerName(t *testing.T) {
	InitValidator()

	tests := []struct {
		name  string
		valid bool
	}{
		{"mycontainer", true},
		{"my-container", true},
		{"my_container", true},
		{"container.name", true},
		{"Container123", true},
		{"123container", false}, // must start with letter
		{"_container", false},   // must start with letter
		{"", false},
		{"container with space", false},
	}

	for _, tt := range tests {
		err := ValidateVar(tt.name, "container_name")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("Container name %q: expected valid=%v, got valid=%v", tt.name, tt.valid, isValid)
		}
	}
}

func TestValidateIP(t *testing.T) {
	InitValidator()

	tests := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"", false},
		{"256.1.1.1", false},
		{"1.1.1", false},
		{"not an ip", false},
	}

	for _, tt := range tests {
		err := ValidateVar(tt.ip, "ip")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("IP %q: expected valid=%v, got valid=%v", tt.ip, tt.valid, isValid)
		}
	}
}

func TestValidatePort(t *testing.T) {
	InitValidator()

	tests := []struct {
		port  int
		valid bool
	}{
		{80, true},
		{443, true},
		{8080, true},
		{1, true},
		{65535, true},
		{0, false},
		{-1, false},
		{65536, false},
	}

	for _, tt := range tests {
		err := ValidateVar(tt.port, "port")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("Port %d: expected valid=%v, got valid=%v", tt.port, tt.valid, isValid)
		}
	}
}

func TestValidateHostname(t *testing.T) {
	InitValidator()

	tests := []struct {
		hostname string
		valid    bool
	}{
		{"localhost", true},
		{"example.com", true},
		{"sub.example.com", true},
		{"my-host", true},
		{"", false},
		{".", false},
		{"host..name", false},
		{"-hostname", false},
		{"hostname-", false},
	}

	for _, tt := range tests {
		err := ValidateVar(tt.hostname, "hostname")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("Hostname %q: expected valid=%v, got valid=%v", tt.hostname, tt.valid, isValid)
		}
	}
}

func TestValidatePath(t *testing.T) {
	InitValidator()

	tests := []struct {
		path  string
		valid bool
	}{
		{"/absolute/path", true},
		{"/data/file.txt", true},
		{"./relative/path", true},
		{"", false},
		{"/path/../etc/passwd", false},
		{"../etc/passwd", false},
	}

	for _, tt := range tests {
		err := ValidateVar(tt.path, "path")
		isValid := err == nil
		if isValid != tt.valid {
			t.Errorf("Path %q: expected valid=%v, got valid=%v", tt.path, tt.valid, isValid)
		}
	}
}

func TestPaginationRequest(t *testing.T) {
	req := PaginationRequest{Page: 2, PageSize: 50}

	if req.GetPage() != 2 {
		t.Errorf("Expected page 2, got %d", req.GetPage())
	}
	if req.GetPageSize() != 50 {
		t.Errorf("Expected pageSize 50, got %d", req.GetPageSize())
	}
	if req.GetOffset() != 50 {
		t.Errorf("Expected offset 50, got %d", req.GetOffset())
	}
}

func TestPaginationRequestDefaults(t *testing.T) {
	req := PaginationRequest{}

	if req.GetPage() != 1 {
		t.Errorf("Expected default page 1, got %d", req.GetPage())
	}
	if req.GetPageSize() != 20 {
		t.Errorf("Expected default pageSize 20, got %d", req.GetPageSize())
	}
}

func TestPaginationRequestMaxPageSize(t *testing.T) {
	req := PaginationRequest{PageSize: 200}

	if req.GetPageSize() != 100 {
		t.Errorf("Expected pageSize capped at 100, got %d", req.GetPageSize())
	}
}

func TestSortRequest(t *testing.T) {
	req := SortRequest{SortBy: "name", SortOrder: "asc"}

	if req.GetSortOrder() != "asc" {
		t.Errorf("Expected sort order 'asc', got %s", req.GetSortOrder())
	}
}

func TestSortRequestDefault(t *testing.T) {
	req := SortRequest{SortBy: "name"}

	if req.GetSortOrder() != "desc" {
		t.Errorf("Expected default sort order 'desc', got %s", req.GetSortOrder())
	}
}

func TestSortRequestInvalid(t *testing.T) {
	req := SortRequest{SortOrder: "invalid"}

	if req.GetSortOrder() != "desc" {
		t.Errorf("Expected default sort order 'desc' for invalid input, got %s", req.GetSortOrder())
	}
}

func TestIDRequest(t *testing.T) {
	req := IDRequest{ID: "test-id"}
	if req.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", req.ID)
	}
}

func TestNameRequest(t *testing.T) {
	req := NameRequest{Name: "test-name"}
	if req.Name != "test-name" {
		t.Errorf("Expected name 'test-name', got %s", req.Name)
	}
}

func TestEnableRequest(t *testing.T) {
	req := EnableRequest{Enabled: true}
	if !req.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

func TestSearchRequest(t *testing.T) {
	req := SearchRequest{Query: "search term"}
	if req.Query != "search term" {
		t.Errorf("Expected query 'search term', got %s", req.Query)
	}
}

func TestTimeRangeRequest(t *testing.T) {
	req := TimeRangeRequest{
		StartTime: "2024-01-01T00:00:00Z",
		EndTime:   "2024-12-31T23:59:59Z",
	}

	if req.StartTime != "2024-01-01T00:00:00Z" {
		t.Errorf("Unexpected start time: %s", req.StartTime)
	}
	if req.EndTime != "2024-12-31T23:59:59Z" {
		t.Errorf("Unexpected end time: %s", req.EndTime)
	}
}

func TestValidateStruct(t *testing.T) {
	type TestStruct struct {
		Name  string `validate:"required"`
		Email string `validate:"required,email"`
	}

	tests := []struct {
		name    string
		input   TestStruct
		wantErr bool
	}{
		{
			name:    "valid",
			input:   TestStruct{Name: "test", Email: "test@example.com"},
			wantErr: false,
		},
		{
			name:    "missing name",
			input:   TestStruct{Email: "test@example.com"},
			wantErr: true,
		},
		{
			name:    "invalid email",
			input:   TestStruct{Name: "test", Email: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatValidationError(t *testing.T) {
	type TestStruct struct {
		Name string `validate:"required"`
	}

	err := Validate(TestStruct{})
	if err == nil {
		t.Fatal("Expected validation error")
	}

	msg := formatValidationError(err)
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestFormatValidationErrorRequired(t *testing.T) {
	type TestStruct struct {
		Field string `validate:"required"`
	}

	err := Validate(TestStruct{})
	if err == nil {
		t.Fatal("Expected validation error")
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		t.Fatal("Expected validator.ValidationErrors")
	}

	msg := formatValidationError(validationErrors)
	if msg == "" {
		t.Error("Expected non-empty error message for required field")
	}
}
