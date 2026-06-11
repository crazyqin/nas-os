package acl

import (
	"testing"
)

func TestPermissionValidation(t *testing.T) {
	tests := []struct {
		name       string
		permission Permission
		wantErr    bool
	}{
		{"valid read", PermRead, false},
		{"valid write", PermWrite, false},
		{"valid delete", PermDelete, false},
		{"valid execute", PermExecute, false},
		{"valid create", PermCreate, false},
		{"valid rename", PermRename, false},
		{"valid move", PermMove, false},
		{"valid copy", PermCopy, false},
		{"valid view_attr", PermViewAttr, false},
		{"valid modify_attr", PermModifyAttr, false},
		{"valid change_perm", PermChangePerm, false},
		{"valid take_owner", PermTakeOwner, false},
		{"valid traverse", PermTraverse, false},
		{"invalid permission", "invalid", true},
		{"empty permission", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.permission.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Permission.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSubjectTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		subjectType SubjectType
		wantErr     bool
	}{
		{"valid user", SubjectUser, false},
		{"valid group", SubjectGroup, false},
		{"invalid type", "invalid", true},
		{"empty type", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.subjectType.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SubjectType.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInheritanceTypeValidation(t *testing.T) {
	tests := []struct {
		name            string
		inheritanceType InheritanceType
		wantErr         bool
	}{
		{"valid full", InheritFull, false},
		{"valid selective", InheritSelective, false},
		{"valid none", InheritNone, false},
		{"valid container", InheritContainer, false},
		{"valid object", InheritObject, false},
		{"invalid type", "invalid", true},
		{"empty type", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.inheritanceType.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("InheritanceType.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty path", "", "/"},
		{"root path", "/", "/"},
		{"simple path", "/test", "/test"},
		{"path with trailing slash", "/test/", "/test"},
		{"path without leading slash", "test", "/test"},
		{"nested path", "/test/sub", "/test/sub"},
		{"path with multiple slashes", "///test///sub///", "///test///sub//"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPathParent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"root", "/", ""},
		{"simple path", "/test", "/"},
		{"nested path", "/test/sub", "/test"},
		{"deep path", "/test/sub/deep", "/test/sub"},
		{"trailing slash", "/test/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PathParent(tt.input)
			if result != tt.expected {
				t.Errorf("PathParent(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsPathAncestor(t *testing.T) {
	tests := []struct {
		name       string
		ancestor   string
		descendant string
		expected   bool
	}{
		{"root is ancestor of all", "/", "/test", true},
		{"parent is ancestor", "/test", "/test/sub", true},
		{"same path", "/test", "/test", true},
		{"not ancestor", "/test", "/other", false},
		{"partial match", "/test", "/testing", false},
		{"empty ancestor", "", "/test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPathAncestor(tt.ancestor, tt.descendant)
			if result != tt.expected {
				t.Errorf("IsPathAncestor(%s, %s) = %v, want %v", tt.ancestor, tt.descendant, result, tt.expected)
			}
		})
	}
}

func TestPermissionGroups(t *testing.T) {
	groups := GetPermissionGroups()
	if len(groups) != 4 {
		t.Errorf("Expected 4 permission groups, got %d", len(groups))
	}

	groupNames := map[string]bool{}
	for _, g := range groups {
		groupNames[g.Name] = true
	}

	expectedGroups := []string{"ReadOnly", "ReadWrite", "Modify", "FullControl"}
	for _, name := range expectedGroups {
		if !groupNames[name] {
			t.Errorf("Missing permission group: %s", name)
		}
	}
}

func TestReadOnlyPermissions(t *testing.T) {
	expected := []Permission{PermRead, PermViewAttr, PermTraverse}
	if len(ReadOnly) != len(expected) {
		t.Errorf("ReadOnly has %d permissions, expected %d", len(ReadOnly), len(expected))
	}
}

func TestReadWritePermissions(t *testing.T) {
	expected := []Permission{PermRead, PermWrite, PermCreate, PermRename, PermCopy, PermViewAttr, PermTraverse}
	if len(ReadWrite) != len(expected) {
		t.Errorf("ReadWrite has %d permissions, expected %d", len(ReadWrite), len(expected))
	}
}

func TestModifyPermissions(t *testing.T) {
	expected := []Permission{PermRead, PermWrite, PermDelete, PermCreate, PermRename, PermMove, PermCopy, PermViewAttr, PermTraverse}
	if len(Modify) != len(expected) {
		t.Errorf("Modify has %d permissions, expected %d", len(Modify), len(expected))
	}
}

func TestFullControlPermissions(t *testing.T) {
	expected := 13 // All 13 permissions
	if len(FullControl) != expected {
		t.Errorf("FullControl has %d permissions, expected %d", len(FullControl), expected)
	}
}
