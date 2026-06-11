package acl

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}
	if manager.acls == nil {
		t.Fatal("Manager.acls is nil")
	}
	if manager.rules == nil {
		t.Fatal("Manager.rules is nil")
	}
}

func TestCreateACL(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name    string
		req     CreateACLRequest
		wantErr bool
	}{
		{
			name: "create root ACL",
			req: CreateACLRequest{
				Path:               "/",
				EntryType:          EntryDirectory,
				Owner:              "admin",
				Group:              "administrators",
				InheritEnabled:     true,
				InheritPermissions: true,
			},
			wantErr: false,
		},
		{
			name: "create nested ACL",
			req: CreateACLRequest{
				Path:               "/test/subdir",
				EntryType:          EntryDirectory,
				Owner:              "user1",
				Group:              "users",
				InheritEnabled:     true,
				InheritPermissions: false,
			},
			wantErr: false,
		},
		{
			name: "duplicate ACL",
			req: CreateACLRequest{
				Path:      "/",
				EntryType: EntryDirectory,
				Owner:     "admin",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl, err := manager.CreateACL(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateACL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && acl == nil {
				t.Error("CreateACL() returned nil ACL")
			}
		})
	}
}

func TestGetACL(t *testing.T) {
	manager := NewManager()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"existing path", "/test", false},
		{"non-existing path", "/nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl, err := manager.GetACL(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetACL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && acl == nil {
				t.Error("GetACL() returned nil ACL")
			}
		})
	}
}

func TestUpdateACL(t *testing.T) {
	manager := NewManager()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	newOwner := "newowner"
	newGroup := "newgroup"
	inheritEnabled := false

	tests := []struct {
		name    string
		path    string
		req     UpdateACLRequest
		wantErr bool
	}{
		{
			name:    "update owner",
			path:    "/test",
			req:     UpdateACLRequest{Owner: newOwner},
			wantErr: false,
		},
		{
			name:    "update group",
			path:    "/test",
			req:     UpdateACLRequest{Group: newGroup},
			wantErr: false,
		},
		{
			name:    "update inherit",
			path:    "/test",
			req:     UpdateACLRequest{InheritEnabled: &inheritEnabled},
			wantErr: false,
		},
		{
			name:    "non-existing path",
			path:    "/nonexistent",
			req:     UpdateACLRequest{Owner: "owner"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl, err := manager.UpdateACL(tt.path, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateACL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.req.Owner != "" && acl.Owner != tt.req.Owner {
					t.Errorf("Owner not updated: got %s, want %s", acl.Owner, tt.req.Owner)
				}
				if tt.req.Group != "" && acl.Group != tt.req.Group {
					t.Errorf("Group not updated: got %s, want %s", acl.Group, tt.req.Group)
				}
				if tt.req.InheritEnabled != nil && acl.InheritEnabled != *tt.req.InheritEnabled {
					t.Errorf("InheritEnabled not updated: got %v, want %v", acl.InheritEnabled, *tt.req.InheritEnabled)
				}
			}
		})
	}
}

func TestDeleteACL(t *testing.T) {
	manager := NewManager()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"existing path", "/test", false},
		{"non-existing path", "/nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.DeleteACL(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteACL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListACLs(t *testing.T) {
	manager := NewManager()

	// Create some ACLs
	paths := []string{"/test1", "/test2", "/test3"}
	for _, path := range paths {
		manager.CreateACL(CreateACLRequest{
			Path:      path,
			EntryType: EntryDirectory,
			Owner:     "admin",
		})
	}

	acls := manager.ListACLs()
	if len(acls) != len(paths) {
		t.Errorf("ListACLs() returned %d ACLs, want %d", len(acls), len(paths))
	}
}

func TestAddACE(t *testing.T) {
	manager := NewManager()

	// Create an ACL first
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name    string
		path    string
		req     AddACERequest
		wantErr bool
	}{
		{
			name: "add valid ACE",
			path: "/test",
			req: AddACERequest{
				Subject:     "user1",
				SubjectType: SubjectUser,
				Permissions: []Permission{PermRead, PermWrite},
				Allowed:     true,
			},
			wantErr: false,
		},
		{
			name: "add ACE with invalid permission",
			path: "/test",
			req: AddACERequest{
				Subject:     "user2",
				SubjectType: SubjectUser,
				Permissions: []Permission{"invalid"},
				Allowed:     true,
			},
			wantErr: true,
		},
		{
			name: "add duplicate ACE",
			path: "/test",
			req: AddACERequest{
				Subject:     "user1",
				SubjectType: SubjectUser,
				Permissions: []Permission{PermRead},
				Allowed:     true,
			},
			wantErr: true,
		},
		{
			name: "non-existing path",
			path: "/nonexistent",
			req: AddACERequest{
				Subject:     "user1",
				SubjectType: SubjectUser,
				Permissions: []Permission{PermRead},
				Allowed:     true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ace, err := manager.AddACE(tt.path, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddACE() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ace == nil {
				t.Error("AddACE() returned nil ACE")
			}
		})
	}
}

func TestUpdateACE(t *testing.T) {
	manager := NewManager()

	// Create ACL and ACE
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	ace, _ := manager.AddACE("/test", AddACERequest{
		Subject:     "user1",
		SubjectType: SubjectUser,
		Permissions: []Permission{PermRead},
		Allowed:     true,
	})

	newAllowed := false
	tests := []struct {
		name    string
		path    string
		aceID   string
		req     UpdateACERequest
		wantErr bool
	}{
		{
			name:    "update existing ACE",
			path:    "/test",
			aceID:   ace.ID,
			req:     UpdateACERequest{Allowed: &newAllowed},
			wantErr: false,
		},
		{
			name:    "non-existing ACE",
			path:    "/test",
			aceID:   "nonexistent",
			req:     UpdateACERequest{Allowed: &newAllowed},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedACE, err := manager.UpdateACE(tt.path, tt.aceID, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateACE() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && updatedACE.Allowed != newAllowed {
				t.Errorf("ACE.Allowed not updated: got %v, want %v", updatedACE.Allowed, newAllowed)
			}
		})
	}
}

func TestRemoveACE(t *testing.T) {
	manager := NewManager()

	// Create ACL and ACE
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	ace, _ := manager.AddACE("/test", AddACERequest{
		Subject:     "user1",
		SubjectType: SubjectUser,
		Permissions: []Permission{PermRead},
		Allowed:     true,
	})

	tests := []struct {
		name    string
		path    string
		aceID   string
		wantErr bool
	}{
		{"remove existing ACE", "/test", ace.ID, false},
		{"non-existing ACE", "/test", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.RemoveACE(tt.path, tt.aceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveACE() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckAccess(t *testing.T) {
	manager := NewManager()

	// Create ACL and ACE
	manager.CreateACL(CreateACLRequest{
		Path:          "/test",
		EntryType:     EntryDirectory,
		Owner:         "admin",
		InheritEnabled: true,
	})

	manager.AddACE("/test", AddACERequest{
		Subject:     "user1",
		SubjectType: SubjectUser,
		Permissions: []Permission{PermRead, PermWrite},
		Allowed:     true,
	})

	tests := []struct {
		name    string
		req     CheckAccessRequest
		allowed bool
	}{
		{
			name: "allowed access",
			req: CheckAccessRequest{
				Subject:    "user1",
				Path:       "/test",
				Permission: PermRead,
			},
			allowed: true,
		},
		{
			name: "denied access - no permission",
			req: CheckAccessRequest{
				Subject:    "user1",
				Path:       "/test",
				Permission: PermDelete,
			},
			allowed: false,
		},
		{
			name: "denied access - wrong subject",
			req: CheckAccessRequest{
				Subject:    "user2",
				Path:       "/test",
				Permission: PermRead,
			},
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.CheckAccess(tt.req)
			if result.Allowed != tt.allowed {
				t.Errorf("CheckAccess() allowed = %v, want %v", result.Allowed, tt.allowed)
			}
		})
	}
}

func TestGetEffectivePermissions(t *testing.T) {
	manager := NewManager()

	// Create ACL and ACE
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	manager.AddACE("/test", AddACERequest{
		Subject:     "user1",
		SubjectType: SubjectUser,
		Permissions: []Permission{PermRead, PermWrite, PermCreate},
		Allowed:     true,
	})

	result := manager.GetEffectivePermissions("user1", "/test")
	if result.Subject != "user1" {
		t.Errorf("Subject = %s, want user1", result.Subject)
	}
	if result.Path != "/test" {
		t.Errorf("Path = %s, want /test", result.Path)
	}
	if len(result.Permissions) != 3 {
		t.Errorf("Permissions count = %d, want 3", len(result.Permissions))
	}
}

func TestSetOwner(t *testing.T) {
	manager := NewManager()

	// Create ACL
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name    string
		path    string
		owner   string
		wantErr bool
	}{
		{"valid update", "/test", "newowner", false},
		{"non-existing path", "/nonexistent", "owner", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SetOwner(tt.path, tt.owner)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetOwner() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				acl, _ := manager.GetACL(tt.path)
				if acl.Owner != tt.owner {
					t.Errorf("Owner = %s, want %s", acl.Owner, tt.owner)
				}
			}
		})
	}
}

func TestSetGroup(t *testing.T) {
	manager := NewManager()

	// Create ACL
	manager.CreateACL(CreateACLRequest{
		Path:      "/test",
		EntryType: EntryDirectory,
		Owner:     "admin",
	})

	tests := []struct {
		name    string
		path    string
		group   string
		wantErr bool
	}{
		{"valid update", "/test", "newgroup", false},
		{"non-existing path", "/nonexistent", "group", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SetGroup(tt.path, tt.group)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				acl, _ := manager.GetACL(tt.path)
				if acl.Group != tt.group {
					t.Errorf("Group = %s, want %s", acl.Group, tt.group)
				}
			}
		})
	}
}

func TestPropagateInheritance(t *testing.T) {
	manager := NewManager()

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

	// Create child ACL (should inherit)
	manager.CreateACL(CreateACLRequest{
		Path:          "/parent/child",
		EntryType:     EntryDirectory,
		Owner:         "admin",
		InheritEnabled: true,
	})

	// Propagate inheritance
	err := manager.PropagateInheritance("/parent")
	if err != nil {
		t.Fatalf("PropagateInheritance() error = %v", err)
	}

	// Check if child inherited permissions
	childACL, _ := manager.GetACL("/parent/child")
	found := false
	for _, ace := range childACL.ACES {
		if ace.Subject == "user1" && ace.AccessType == AccessInherited {
			found = true
			break
		}
	}
	if !found {
		t.Error("Child did not inherit permissions from parent")
	}
}

func TestGetAuditLog(t *testing.T) {
	manager := NewManager()

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

	log := manager.GetAuditLog(10)
	if len(log) < 2 {
		t.Errorf("Audit log has %d entries, want at least 2", len(log))
	}
}

func TestBackwardCompatibility(t *testing.T) {
	manager := NewManager()

	// Test AddRule
	rule := ACLRule{
		ID:          "test-rule-1",
		Path:        "/test",
		Subject:     "user1",
		SubjectType: "user",
		Permissions: []string{"read", "write"},
		Recursive:   true,
		Priority:    10,
	}
	manager.AddRule(rule)

	// Test ListRules
	rules := manager.ListRules()
	if len(rules) != 1 {
		t.Errorf("ListRules() returned %d rules, want 1", len(rules))
	}

	// Test UpdateRule
	rule.Permissions = []string{"read", "write", "delete"}
	err := manager.UpdateRule(rule)
	if err != nil {
		t.Errorf("UpdateRule() error = %v", err)
	}

	// Test RemoveRule
	manager.RemoveRule("test-rule-1")
	rules = manager.ListRules()
	if len(rules) != 0 {
		t.Errorf("ListRules() returned %d rules after removal, want 0", len(rules))
	}
}
