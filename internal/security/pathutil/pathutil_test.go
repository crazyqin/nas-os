package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePath(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "pathutil_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试子目录和文件
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0750); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		baseDir   string
		userPath  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid relative path",
			baseDir:  tmpDir,
			userPath: "subdir/test.txt",
			wantErr:  false,
		},
		{
			name:     "valid path with .",
			baseDir:  tmpDir,
			userPath: "./subdir/test.txt",
			wantErr:  false,
		},
		{
			name:      "path traversal with ..",
			baseDir:   tmpDir,
			userPath:  "../etc/passwd",
			wantErr:   true,
			errSubstr: "path traversal",
		},
		{
			name:      "deep path traversal",
			baseDir:   tmpDir,
			userPath:  "subdir/../../../etc/passwd",
			wantErr:   true,
			errSubstr: "path traversal",
		},
		{
			name:     "empty path",
			baseDir:  tmpDir,
			userPath: "",
			wantErr:  false, // 空路径会返回基目录
		},
		{
			name:     "absolute path within base",
			baseDir:  tmpDir,
			userPath: tmpDir + "/subdir/test.txt",
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
				if tt.errSubstr != "" && !contains(err.Error(), tt.errSubstr) {
					t.Errorf("SafePath() error = %v, want substring %v", err, tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("SafePath() unexpected error: %v", err)
				}
				if result == "" {
					t.Error("SafePath() returned empty path")
				}
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantErr  bool
	}{
		{"valid path", "/var/lib/data", false},
		{"valid relative", "data/file.txt", false},
		{"path traversal", "../etc/passwd", true},
		{"null byte", "/var/lib/data\x00.txt", true},
		{"empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if tt.wantErr && err == nil {
				t.Error("ValidatePath() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidatePath() unexpected error: %v", err)
			}
		})
	}
}

func TestIsWithinBase(t *testing.T) {
	tests := []struct {
		name       string
		baseDir    string
		targetPath string
		want       bool
	}{
		{"within base", "/var/lib", "/var/lib/data", true},
		{"same as base", "/var/lib", "/var/lib", true},
		{"outside base", "/var/lib", "/etc/passwd", false},
		{"parent directory", "/var/lib", "/var", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWithinBase(tt.baseDir, tt.targetPath); got != tt.want {
				t.Errorf("IsWithinBase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}