//go:build linux
// +build linux

package plugin

import (
	"fmt"
	"syscall"
)

// ApplyResourceLimits applies resource limits to the current process (Linux only)
func ApplyResourceLimits(maxMemoryMB, maxCPUPercent int) error {
	// Set memory limit using setrlimit
	if maxMemoryMB > 0 {
		var rlim syscall.Rlimit
		rlim.Cur = uint64(maxMemoryMB) * 1024 * 1024
		rlim.Max = rlim.Cur
		if err := syscall.Setrlimit(syscall.RLIMIT_AS, &rlim); err != nil {
			return fmt.Errorf("failed to set memory limit: %w", err)
		}
	}

	// CPU limit would require cgroups in production
	// This is a simplified version

	return nil
}
