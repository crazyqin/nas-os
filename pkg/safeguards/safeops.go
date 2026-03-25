// Package safeguards provides security utilities for common operations
package safeguards

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// SafeName validates that a string contains only safe characters
// Safe characters: alphanumeric, underscore, hyphen.
var safeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateSafeName checks if a name contains only safe characters
// to prevent command injection and path traversal.
func ValidateSafeName(name string, fieldName string) error {
	if name == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	if !safeNameRegex.MatchString(name) {
		return fmt.Errorf("%s contains invalid characters, only alphanumeric, underscore and hyphen are allowed", fieldName)
	}
	if len(name) > 255 {
		return fmt.Errorf("%s is too long (max 255 characters)", fieldName)
	}
	return nil
}

// SanitizePath cleans a file path to prevent path traversal attacks
// It removes any .. components and ensures the path doesn't escape the base directory.
func SanitizePath(baseDir, userPath string) (string, error) {
	// Clean the user path to remove .. components
	cleanPath := filepath.Clean(userPath)

	// Join with base directory
	fullPath := filepath.Join(baseDir, cleanPath)

	// Get absolute paths for comparison
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for base directory: %w", err)
	}

	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for full path: %w", err)
	}

	// Ensure the final path is under the base directory
	if !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) && absFull != absBase {
		return "", fmt.Errorf("path traversal detected: path escapes base directory")
	}

	return absFull, nil
}

// SafeCommandBuilder provides a safe way to build and execute commands
// by using argument lists instead of shell interpretation.
type SafeCommandBuilder struct {
	name string
	args []string
}

// NewCommand creates a new safe command builder.
func NewCommand(name string) *SafeCommandBuilder {
	return &SafeCommandBuilder{
		name: name,
		args: make([]string, 0),
	}
}

// AddArg adds a safe argument (validated as a safe name).
func (b *SafeCommandBuilder) AddArg(arg string, validate bool) *SafeCommandBuilder {
	if validate {
		// Only add if valid, otherwise skip (caller should check)
		if safeNameRegex.MatchString(arg) {
			b.args = append(b.args, arg)
		}
	} else {
		b.args = append(b.args, arg)
	}
	return b
}

// AddLiteral adds a literal argument without validation (for flags, etc.)
func (b *SafeCommandBuilder) AddLiteral(arg string) *SafeCommandBuilder {
	b.args = append(b.args, arg)
	return b
}

// AddPath adds a path argument after sanitization.
func (b *SafeCommandBuilder) AddPath(baseDir, userPath string) error {
	safePath, err := SanitizePath(baseDir, userPath)
	if err != nil {
		return err
	}
	b.args = append(b.args, safePath)
	return nil
}

// Build returns the exec.Cmd for the command.
// Deprecated: Use BuildContext instead to avoid noctx warnings.
func (b *SafeCommandBuilder) Build() *exec.Cmd {
	return exec.Command(b.name, b.args...)
}

// BuildContext returns the exec.Cmd with context for the command.
func (b *SafeCommandBuilder) BuildContext(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, b.name, b.args...)
}

// Args returns the current arguments (for logging/debugging).
func (b *SafeCommandBuilder) Args() []string {
	return append([]string{b.name}, b.args...)
}

// SafeIntConversion safely converts uint64 to int64 with overflow check.
func SafeIntConversion(val uint64) (int64, error) {
	const maxInt64 = ^uint64(0) >> 1
	if val > maxInt64 {
		return 0, fmt.Errorf("value %d overflows int64", val)
	}
	return int64(val), nil
}

// SafeUintConversion safely converts int64 to uint64 with check.
func SafeUintConversion(val int64) (uint64, error) {
	if val < 0 {
		return 0, fmt.Errorf("negative value %d cannot be converted to uint64", val)
	}
	return uint64(val), nil
}

// ValidateFilePath validates that a file path is safe.
func ValidateFilePath(path string) error {
	// Check for null bytes
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("path contains null byte")
	}

	// Check for suspicious patterns
	suspicious := []string{"..", "//", "\\\\", "\x00"}
	for _, s := range suspicious {
		if strings.Contains(path, s) {
			return fmt.Errorf("path contains suspicious pattern: %s", s)
		}
	}

	return nil
}

// IsAllowedCommand checks if a command is in the allowed list.
var allowedCommands = map[string]bool{
	"virsh":      true,
	"qemu-img":   true,
	"rsync":      true,
	"tar":        true,
	"cp":         true,
	"mv":         true,
	"rm":         true,
	"mkdir":      true,
	"chmod":      true,
	"chown":      true,
	"ls":         true,
	"cat":        true,
	"head":       true,
	"tail":       true,
	"grep":       true,
	"find":       true,
	"df":         true,
	"du":         true,
	"mount":      true,
	"umount":     true,
	"losetup":    true,
	"cryptsetup": true,
	"dd":         true,
	"blkid":      true,
	"lsblk":      true,
	"parted":     true,
	"mkfs":       true,
}

// IsCommandAllowed checks if a command is in the allowed list.
func IsCommandAllowed(cmd string) bool {
	return allowedCommands[cmd]
}
