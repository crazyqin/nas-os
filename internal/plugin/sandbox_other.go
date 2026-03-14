//go:build !linux
// +build !linux

package plugin

// ApplyResourceLimits is a no-op on non-Linux platforms
func ApplyResourceLimits(maxMemoryMB, maxCPUPercent int) error {
	// Resource limits not supported on this platform
	return nil
}
