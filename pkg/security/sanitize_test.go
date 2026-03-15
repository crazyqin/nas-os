// Package security provides security utilities for input validation and sanitization
package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePath(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "security-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		baseDir  string
		userPath string
		wantErr  bool
		errType  error
	}{
		{
			name:     "valid path",
			baseDir:  tmpDir,
			userPath: "test.txt",
			wantErr:  false,
		},
		{
			name:     "valid nested path",
			baseDir:  tmpDir,
			userPath: "subdir/test.txt",
			wantErr:  false,
		},
		{
			name:     "path traversal with ..",
			baseDir:  tmpDir,
			userPath: "../etc/passwd",
			wantErr:  true,
			errType:  ErrPathTraversal,
		},
		{
			name:     "path traversal hidden",
			baseDir:  tmpDir,
			userPath: "subdir/../../etc/passwd",
			wantErr:  true,
			errType:  ErrPathTraversal,
		},
		{
			name:     "empty path",
			baseDir:  tmpDir,
			userPath: "",
			wantErr:  false,
		},
		{
			name:     "dot path",
			baseDir:  tmpDir,
			userPath: ".",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafePath(tt.baseDir, tt.userPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SafePath() expected error, got nil")
					return
				}
				if tt.errType != nil && err != tt.errType {
					t.Errorf("SafePath() error = %v, want %v", err, tt.errType)
				}
			} else {
				if err != nil {
					t.Errorf("SafePath() unexpected error: %v", err)
				}
				// Verify result is within base directory
				absBase, _ := filepath.Abs(tt.baseDir)
				if result != absBase && len(result) > 0 {
					if result[:len(absBase)] != absBase {
						t.Errorf("SafePath() result %s not within base %s", result, absBase)
					}
				}
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{"valid filename", "test.txt", false},
		{"valid with spaces", "test file.txt", false},
		{"valid with unicode", "测试文件.txt", false},
		{"empty filename", "", true},
		{"too long filename", string(make([]byte, 256)), true},
		{"path traversal", "test/../file.txt", true},
		{"forward slash", "test/file.txt", true},
		{"backslash", "test\\file.txt", true},
		{"null byte", "test\x00file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.filename)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateFilename() expected error for %q", tt.filename)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateFilename() unexpected error: %v", err)
			}
		})
	}
}

func TestSanitizeCommandArg(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantErr bool
	}{
		{"valid arg", "testfile.txt", false},
		{"valid with spaces", "my file.txt", false},
		{"valid with numbers", "file123.txt", false},
		{"semicolon injection", "file;rm -rf /", true},
		{"pipe injection", "file|cat /etc/passwd", true},
		{"ampersand injection", "file&&whoami", true},
		{"dollar injection", "file$(whoami)", true},
		{"backtick injection", "file`whoami`", true},
		{"newline injection", "file\nwhoami", true},
		{"redirect injection", "file>output.txt", true},
		{"parenthesis injection", "file(whoami)", true},
		{"null byte", "file\x00.txt", false}, // null bytes are removed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeCommandArg(tt.arg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SanitizeCommandArg() expected error for %q", tt.arg)
				}
			} else {
				if err != nil {
					t.Errorf("SanitizeCommandArg() unexpected error: %v", err)
				}
				// Check null byte removal
				if tt.name == "null byte" && result != "file.txt" {
					t.Errorf("SanitizeCommandArg() null byte not removed: %q", result)
				}
			}
		})
	}
}
