// Package vm provides enhanced VM import with multiple disk formats
// 对标TrueNAS 25.10 VM Import/Export
package vm

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
)

// DiskFormat defines VM disk format types
type DiskFormat string

const (
	FormatQCOW2 DiskFormat = "qcow2"
	FormatQED   DiskFormat = "qed"
	FormatRAW   DiskFormat = "raw"
	FormatVDI   DiskFormat = "vdi"
	FormatVHDX  DiskFormat = "vhdx"
	FormatVMDK  DiskFormat = "vmdk"
)

// ImportConfig defines VM import configuration
type ImportConfig struct {
	Name       string
	Format     DiskFormat
	Source     string
	SecureBoot bool
	UEFI       bool
}

// ImportResult defines VM import result
type ImportResult struct {
	Name      string
	TargetPath string
	Size      uint64
	Duration  float64
}

// ImportService manages VM import operations
type ImportService struct {
	imports map[string]*ImportResult
	mu      sync.RWMutex
}

// NewImportService creates a new import service
func NewImportService() *ImportService {
	return &ImportService{
		imports: make(map[string]*ImportResult),
	}
}

// ImportVM imports a VM from various disk formats
func (s *ImportService) ImportVM(ctx context.Context, cfg *ImportConfig) (*ImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate format
	if !isValidFormat(cfg.Format) {
		return nil, fmt.Errorf("unsupported format: %s", cfg.Format)
	}

	// Determine target path
	targetDir := "/var/lib/nas-os/vm"
	targetPath := filepath.Join(targetDir, cfg.Name+".qcow2")

	// Convert disk format using qemu-img
	if cfg.Format != FormatQCOW2 {
		cmd := exec.CommandContext(ctx, "qemu-img", "convert",
			"-f", string(cfg.Format),
			"-O", "qcow2",
			cfg.Source,
			targetPath)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to convert disk: %w", err)
		}
	} else {
		// Direct copy for QCOW2
		cmd := exec.CommandContext(ctx, "cp", cfg.Source, targetPath)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to copy disk: %w", err)
		}
	}

	result := &ImportResult{
		Name:      cfg.Name,
		TargetPath: targetPath,
	}

	s.imports[cfg.Name] = result
	return result, nil
}

// ExportVM exports a VM to specified format
func (s *ImportService) ExportVM(ctx context.Context, name string, format DiskFormat) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, exists := s.imports[name]
	if !exists {
		return "", fmt.Errorf("VM %s not found", name)
	}

	exportPath := filepath.Join("/var/lib/nas-os/vm/export",
		fmt.Sprintf("%s.%s", name, format))

	// Convert to target format
	cmd := exec.CommandContext(ctx, "qemu-img", "convert",
		"-f", "qcow2",
		"-O", string(format),
		result.TargetPath,
		exportPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to export: %w", err)
	}

	return exportPath, nil
}

// isValidFormat checks if format is supported
func isValidFormat(format DiskFormat) bool {
	switch format {
	case FormatQCOW2, FormatQED, FormatRAW, FormatVDI, FormatVHDX, FormatVMDK:
		return true
	default:
		return false
	}
}

// GetImportProgress returns import progress
func (s *ImportService) GetImportProgress(name string) (*ImportResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, exists := s.imports[name]
	if !exists {
		return nil, fmt.Errorf("VM %s not found", name)
	}
	return result, nil
}