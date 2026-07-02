package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SensitivityLevel represents how sensitive a file is.
type SensitivityLevel int

const (
	LevelNormal SensitivityLevel = iota
	LevelSensitive
	LevelBlocked
)

// sensitivePatterns defines files that should be flagged or blocked from sync.
var sensitivePatterns = []struct {
	Pattern string
	Level   SensitivityLevel
}{
	{".env", LevelBlocked},
	{".env.*", LevelBlocked},
	{".password", LevelBlocked},
	{"*.key", LevelBlocked},
	{"*.pem", LevelSensitive},
	{"*.p12", LevelBlocked},
	{"*.pfx", LevelBlocked},
	{"id_rsa", LevelBlocked},
	{"id_ed25519", LevelBlocked},
	{"id_ecdsa", LevelBlocked},
	{".ssh/*", LevelBlocked},
	{".gnupg/*", LevelBlocked},
	{".gitcredentials", LevelBlocked},
	{"credentials.json", LevelBlocked},
	{"token*.json", LevelBlocked},
	{"*.keystore", LevelBlocked},
	{"*.jks", LevelBlocked},
}

// blockedPaths defines system paths that must never be synced.
var blockedPaths = []string{
	"/etc/passwd",
	"/etc/shadow",
	"/etc/ssh",
	"/root/.ssh",
	"/etc/kubernetes",
	"/var/run/secrets",
	"/proc",
	"/sys",
	"/dev",
}

// CheckPathTraversal validates that a sync path doesn't escape allowed boundaries.
func CheckPathTraversal(syncPath, allowedBase string) error {
	absPath, err := filepath.Abs(syncPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	absBase, err := filepath.Abs(allowedBase)
	if err != nil {
		return fmt.Errorf("invalid base path: %w", err)
	}

	// Ensure the sync path is within allowed base
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path traversal detected: %s is outside %s", absPath, absBase)
	}
	return nil
}

// IsBlockedPath checks if a path is in the blocked system paths list.
func IsBlockedPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return true // treat invalid paths as blocked
	}
	for _, blocked := range blockedPaths {
		if strings.HasPrefix(absPath, blocked) {
			return true
		}
	}
	return false
}

// CheckFileSensitivity evaluates whether a file should be synced.
func CheckFileSensitivity(filename string) (SensitivityLevel, string) {
	base := filepath.Base(filename)
	for _, sp := range sensitivePatterns {
		matched, err := filepath.Match(sp.Pattern, base)
		if err != nil {
			continue
		}
		if matched {
			return sp.Level, fmt.Sprintf("file matches sensitive pattern: %s", sp.Pattern)
		}
	}

	// Check directory patterns
	dir := filepath.Dir(filename)
	for _, sp := range sensitivePatterns {
		if strings.Contains(sp.Pattern, "/") {
			patternDir := filepath.Dir(sp.Pattern)
			if strings.Contains(dir, patternDir) {
				return sp.Level, fmt.Sprintf("file in sensitive directory: %s", patternDir)
			}
		}
	}

	return LevelNormal, ""
}

// ComputeFileChecksum computes SHA256 of a file.
func ComputeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyIntegrity checks that a file matches its expected checksum.
func VerifyIntegrity(path, expectedChecksum string) (bool, error) {
	actual, err := ComputeFileChecksum(path)
	if err != nil {
		return false, err
	}
	return actual == expectedChecksum, nil
}

// FilterSensitiveFiles returns files that should be excluded from sync.
func FilterSensitiveFiles(files []string) (allowed []string, blocked []string) {
	for _, f := range files {
		level, reason := CheckFileSensitivity(f)
		switch level {
		case LevelBlocked:
			blocked = append(blocked, fmt.Sprintf("%s (%s)", f, reason))
		case LevelSensitive:
			// Sensitive files are allowed but flagged
			allowed = append(allowed, f)
		case LevelNormal:
			allowed = append(allowed, f)
		}
	}
	return
}
