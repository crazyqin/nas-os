// Package security provides security utilities for input validation and sanitization
package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// ErrPathTraversal path traversal attempt detected.
	ErrPathTraversal = errors.New("path traversal attempt detected")
	// ErrInvalidInput invalid input detected.
	ErrInvalidInput = errors.New("invalid input detected")
	// ErrDangerousChar dangerous character detected.
	ErrDangerousChar = errors.New("dangerous character detected")
)

// SafePath validates and sanitizes file paths to prevent path traversal attacks.
func SafePath(baseDir, userPath string) (string, error) {
	// Clean the path to remove any .. or . components
	cleanPath := filepath.Clean(userPath)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return "", ErrPathTraversal
	}

	// Get absolute paths
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}

	absPath := filepath.Join(absBase, cleanPath)
	absPath = filepath.Clean(absPath)

	// Verify the path is within base directory
	if !strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) && absPath != absBase {
		return "", ErrPathTraversal
	}

	return absPath, nil
}

// ValidateFilename validates a filename for safe use.
func ValidateFilename(name string) error {
	if name == "" || len(name) > 255 {
		return ErrInvalidInput
	}

	// Check for dangerous patterns
	dangerous := []string{"..", "/", "\\", "\x00"}
	for _, d := range dangerous {
		if strings.Contains(name, d) {
			return ErrDangerousChar
		}
	}

	return nil
}

// SanitizeCommandArg sanitizes a command argument to prevent command injection.
func SanitizeCommandArg(arg string) (string, error) {
	// Remove null bytes
	arg = strings.ReplaceAll(arg, "\x00", "")

	// Check for shell metacharacters that could lead to injection
	dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return "", ErrDangerousChar
		}
	}

	return arg, nil
}

// ValidateID validates an ID string (alphanumeric with dashes and underscores).
func ValidateID(id string) error {
	if id == "" || len(id) > 128 {
		return ErrInvalidInput
	}

	matched, err := regexp.MatchString("^[a-zA-Z0-9_-]+$", id)
	if err != nil {
		return fmt.Errorf("ID 验证失败: %w", err)
	}
	if !matched {
		return ErrInvalidInput
	}

	return nil
}

// ValidateIP validates an IP address format.
func ValidateIP(ip string) error {
	if ip == "" {
		return ErrInvalidInput
	}

	// IPv4 pattern
	ipv4Pattern := `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
	matched, err := regexp.MatchString(ipv4Pattern, ip)
	if err != nil {
		return fmt.Errorf("IP 验证失败: %w", err)
	}
	if matched {
		return nil
	}

	// IPv6 pattern (simplified)
	ipv6Pattern := `^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$`
	matched, err = regexp.MatchString(ipv6Pattern, ip)
	if err != nil {
		return fmt.Errorf("IP 验证失败: %w", err)
	}
	if matched {
		return nil
	}

	return ErrInvalidInput
}

// ValidatePort validates a port number.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return ErrInvalidInput
	}
	return nil
}

// IsPathWithinBase checks if a path is within a base directory.
func IsPathWithinBase(path, baseDir string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}

	return strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) || absPath == absBase
}

// SecureFilePermission returns secure file permission (0600).
func SecureFilePermission() os.FileMode {
	return 0600
}

// SecureDirPermission returns secure directory permission (0750).
func SecureDirPermission() os.FileMode {
	return 0750
}
