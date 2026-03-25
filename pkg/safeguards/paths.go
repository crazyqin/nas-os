// Package safeguards provides security utilities
package safeguards

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SafeNameRegex matches safe identifier characters.
var SafeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ValidateSafeIdentifier validates an identifier is safe for use in commands/paths.
func ValidateSafeIdentifier(id, fieldName string) error {
	if id == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	if len(id) > 128 {
		return fmt.Errorf("%s too long (max 128 chars)", fieldName)
	}
	if !SafeNameRegex.MatchString(id) {
		return fmt.Errorf("%s contains invalid characters (only alphanumeric, underscore, hyphen allowed)", fieldName)
	}
	return nil
}

// ValidateSafePath validates a path doesn't contain traversal characters.
func ValidateSafePath(path string) error {
	// Check for null bytes
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("path contains null byte")
	}

	// Check for path traversal
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains traversal sequence")
	}

	return nil
}

// SecureJoin safely joins base path with user-provided path.
func SecureJoin(baseDir, userPath string) (string, error) {
	// Clean the user path
	cleanUserPath := filepath.Clean(userPath)

	// Remove leading slashes to prevent absolute paths
	cleanUserPath = strings.TrimPrefix(cleanUserPath, "/")
	cleanUserPath = strings.TrimPrefix(cleanUserPath, "\\")

	// Join with base
	fullPath := filepath.Join(baseDir, cleanUserPath)

	// Get absolute paths
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}

	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve full path: %w", err)
	}

	// Verify result is under base directory
	if !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) && absFull != absBase {
		return "", fmt.Errorf("path traversal detected")
	}

	return absFull, nil
}

// GenerateSecureID generates a cryptographically secure random ID.
func GenerateSecureID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand 失败是致命错误
	}
	return hex.EncodeToString(b)
}

// IsAllowedPath checks if a path is within allowed directories.
func IsAllowedPath(path string, allowedDirs []string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
			return true
		}
	}
	return false
}

// SafeFileOp wraps file operations with safety checks.
func SafeFileOp(baseDir, userPath string, op func(safePath string) error) error {
	safePath, err := SecureJoin(baseDir, userPath)
	if err != nil {
		return err
	}
	return op(safePath)
}

// ValidateAndSanitizeFilename validates and sanitizes a filename.
func ValidateAndSanitizeFilename(name string) (string, error) {
	// Remove dangerous characters
	name = strings.Map(func(r rune) rune {
		if r == '\x00' || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)

	// Check length
	if len(name) == 0 {
		return "", fmt.Errorf("filename cannot be empty")
	}
	if len(name) > 255 {
		name = name[:255]
	}

	// Check for reserved names
	reserved := []string{".", "..", "CON", "PRN", "AUX", "NUL"}
	upper := strings.ToUpper(name)
	for _, r := range reserved {
		if upper == r {
			return "", fmt.Errorf("reserved filename: %s", name)
		}
	}

	return name, nil
}

// SafeReadFile reads a file after validating the path.
func SafeReadFile(baseDir, userPath string) ([]byte, error) {
	safePath, err := SecureJoin(baseDir, userPath)
	if err != nil {
		return nil, err
	}
	// #nosec G304 -- safePath is validated by SecureJoin above
	return os.ReadFile(safePath)
}

// SafeWriteFile writes to a file after validating the path.
func SafeWriteFile(baseDir, userPath string, data []byte, perm os.FileMode) error {
	safePath, err := SecureJoin(baseDir, userPath)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	return os.WriteFile(safePath, data, perm)
}

// SafeRemove removes a file after validating the path.
func SafeRemove(baseDir, userPath string) error {
	safePath, err := SecureJoin(baseDir, userPath)
	if err != nil {
		return err
	}
	return os.Remove(safePath)
}

// SafeMkdirAll creates directories after validating the path.
func SafeMkdirAll(baseDir, userPath string, perm os.FileMode) error {
	safePath, err := SecureJoin(baseDir, userPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(safePath, perm)
}
