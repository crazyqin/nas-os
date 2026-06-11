package acl

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	
	manager := NewManager()
	handlers := NewHandlers(manager)
	
	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)
	
	return router, manager
}

func TestHandlerListACLs(t *testing.T) {
	router, manager := setupTestRouter()
	
	// Create some ACLs
	manager.CreateACL(CreateACLRequest{
		Path:      "/test1",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})
	manager.CreateACL(CreateACLRequest{
		Path:      "/test2",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	req, _ := http.NewRequest("GET", "/api/v1/acl", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	
	data := response["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("Expected 2 ACLs, got %d", len(data))
	}
}

func TestHandlerCreateACL(t *testing.T) {
	router, _ := setupTestRouter()

	tests := []struct {
		name       string
		req        CreateACLRequest
		wantStatus int
	}{
		{
			name: "valid request",
			req: CreateACLRequest{
				Path:      "/test",
				EntryType: EntryDirectory,
				Owner:     "admin",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "duplicate path",
			req: CreateACLRequest{
				Path:      "/test",
				EntryType: EntryDirectory,
				Owner:     "admin",
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			req, _ := http.NewRequest("POST", "/api/v1/acl", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandlerGetACL(t *testing.T) {
	router, manager := setupTestRouter()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"existing path", "/test", http.StatusOK},
		{"non-existing path", "/nonexistent", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/acl/path"+tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandlerUpdateACL(t *testing.T) {
	router, manager := setupTestRouter()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name       string
		path       string
		req        UpdateACLRequest
		wantStatus int
	}{
		{
			name:       "update owner",
			path:       "/test",
			req:        UpdateACLRequest{Owner: "newowner"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existing path",
			path:       "/nonexistent",
			req:        UpdateACLRequest{Owner: "owner"},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			req, _ := http.NewRequest("PUT", "/api/v1/acl/path"+tt.path, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandlerDeleteACL(t *testing.T) {
	router, manager := setupTestRouter()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"existing path", "/test", http.StatusOK},
		{"non-existing path", "/nonexistent", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/api/v1/acl/path"+tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandlerAddACE(t *testing.T) {
	router, manager := setupTestRouter()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name       string
		path       string
		req        AddACERequest
		wantStatus int
	}{
		{
			name: "valid ACE",
			path: "/test",
			req: AddACERequest{
				Subject:     "user1",
				SubjectType: SubjectUser,
				Permissions: []Permission{PermRead, PermWrite},
				Allowed:     true,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "invalid permission",
			path: "/test",
			req: AddACERequest{
				Subject:     "user2",
				SubjectType: SubjectUser,
				Permissions: []Permission{"invalid"},
				Allowed:     true,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			req, _ := http.NewRequest("POST", "/api/v1/acl/ace?path="+tt.path, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandlerCheckAccess(t *testing.T) {
	router, manager := setupTestRouter()

	// Create ACL and ACE
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	manager.AddACE("/test", AddACERequest{
		Subject:     "user1",
		SubjectType: SubjectUser,
		Permissions: []Permission{PermRead},
		Allowed:     true,
	})

	tests := []struct {
		name       string
		req        CheckAccessRequest
		wantStatus int
		allowed    bool
	}{
		{
			name: "allowed access",
			req: CheckAccessRequest{
				Subject:    "user1",
				Path:       "/test",
				Permission: PermRead,
			},
			wantStatus: http.StatusOK,
			allowed:    true,
		},
		{
			name: "denied access",
			req: CheckAccessRequest{
				Subject:    "user1",
				Path:       "/test",
				Permission: PermDelete,
			},
			wantStatus: http.StatusOK,
			allowed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			req, _ := http.NewRequest("POST", "/api/v1/acl/check", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			
			data := response["data"].(map[string]interface{})
			if data["allowed"].(bool) != tt.allowed {
				t.Errorf("Expected allowed=%v, got %v", tt.allowed, data["allowed"])
			}
		})
	}
}

func TestHandlerEffectivePermissions(t *testing.T) {
	router, manager := setupTestRouter()

	// Create ACL and ACE
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	manager.AddACE("/test", AddACERequest{
		Subject:     "user1",
		SubjectType: SubjectUser,
		Permissions: []Permission{PermRead, PermWrite},
		Allowed:     true,
	})

	req, _ := http.NewRequest("GET", "/api/v1/acl/effective?subject=user1&path=/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	
	data := response["data"].(map[string]interface{})
	permissions := data["permissions"].([]interface{})
	if len(permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(permissions))
	}
}

func TestHandlerPropagateInheritance(t *testing.T) {
	router, manager := setupTestRouter()

	// Create parent ACL with ACE
	manager.CreateACL(CreateACLRequest{
		Path:          "/parent",
		EntryType:     EntryDirectory,
		Owner:         "admin",
		InheritEnabled: true,
	})

	manager.AddACE("/parent", AddACERequest{
		Subject:     "user1",
		SubjectType: SubjectUser,
		Permissions: []Permission{PermRead},
		Allowed:     true,
		InheritFlags: []InheritanceType{InheritFull},
	})

	// Create child ACL
	manager.CreateACL(CreateACLRequest{
		Path:          "/parent/child",
		EntryType:     EntryDirectory,
		Owner:         "admin",
		InheritEnabled: true,
	})

	req, _ := http.NewRequest("POST", "/api/v1/acl/propagate?path=/parent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerSetOwner(t *testing.T) {
	router, manager := setupTestRouter()

	// Create ACL
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name       string
		path       string
		owner      string
		wantStatus int
	}{
		{"valid update", "/test", "newowner", http.StatusOK},
		{"non-existing path", "/nonexistent", "owner", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"owner": tt.owner})
			req, _ := http.NewRequest("PUT", "/api/v1/acl/owner?path="+tt.path, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandlerSetGroup(t *testing.T) {
	router, manager := setupTestRouter()

	// Create ACL
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name       string
		path       string
		group      string
		wantStatus int
	}{
		{"valid update", "/test", "newgroup", http.StatusOK},
		{"non-existing path", "/nonexistent", "group", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"group": tt.group})
			req, _ := http.NewRequest("PUT", "/api/v1/acl/group?path="+tt.path, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandlerGetPermissionGroups(t *testing.T) {
	router, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/acl/groups", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	
	data := response["data"].([]interface{})
	if len(data) != 4 {
		t.Errorf("Expected 4 permission groups, got %d", len(data))
	}
}

func TestHandlerGetAuditLog(t *testing.T) {
	router, manager := setupTestRouter()

	// Create some entries
	manager.CreateACL(CreateACLRequest{
		Path:      "/test1",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	manager.CreateACL(CreateACLRequest{
		Path:      "/test2",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	req, _ := http.NewRequest("GET", "/api/v1/acl/audit?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	
	data := response["data"].([]interface{})
	if len(data) < 2 {
		t.Errorf("Expected at least 2 audit entries, got %d", len(data))
	}
}
