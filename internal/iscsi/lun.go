package iscsi

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LUNManager manages Logical Unit Numbers for iSCSI targets
type LUNManager struct {
	mu       sync.RWMutex
	basePath string // Base path for file-backed LUNs
}

// NewLUNManager creates a new LUN manager
func NewLUNManager(basePath string) *LUNManager {
	return &LUNManager{
		basePath: basePath,
	}
}

// Create creates a new LUN
func (lm *LUNManager) Create(targetID string, input LUNInput) (*LUN, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Validate input
	if err := lm.validateInput(input); err != nil {
		return nil, err
	}

	// Generate LUN ID
	lunID := generateLUNID(targetID, input.Name)

	// Determine path
	path := input.Path
	if path == "" && input.Type == LUNTypeFile {
		path = filepath.Join(lm.basePath, targetID, fmt.Sprintf("lun_%s.img", input.Name))
	}

	// Set defaults
	blockSize := input.BlockSize
	if blockSize == 0 {
		blockSize = 512
	}

	lun := &LUN{
		ID:        lunID,
		Name:      input.Name,
		Type:      input.Type,
		Path:      path,
		Size:      input.Size,
		BlockSize: blockSize,
		ReadOnly:  input.ReadOnly,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create backing storage
	if input.Type == LUNTypeFile {
		if err := lm.createFileBacking(path, input.Size); err != nil {
			return nil, fmt.Errorf("failed to create file backing: %w", err)
		}
	} else {
		// Verify block device exists
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("block device not found: %w", err)
		}
		// Get actual size of block device
		size, err := lm.getBlockDeviceSize(path)
		if err != nil {
			return nil, fmt.Errorf("failed to get block device size: %w", err)
		}
		lun.Size = size
	}

	return lun, nil
}

// validateInput validates LUN input
func (lm *LUNManager) validateInput(input LUNInput) error {
	if input.Name == "" {
		return fmt.Errorf("LUN name is required")
	}

	if input.Type != LUNTypeFile && input.Type != LUNTypeBlock {
		return fmt.Errorf("invalid LUN type: must be 'file' or 'block'")
	}

	if input.Type == LUNTypeFile {
		if input.Size <= 0 {
			return fmt.Errorf("size is required for file-backed LUN")
		}
		if input.Size < 1024*1024 { // Minimum 1MB
			return fmt.Errorf("minimum LUN size is 1MB")
		}
	}

	if input.Type == LUNTypeBlock && input.Path == "" {
		return fmt.Errorf("path is required for block device LUN")
	}

	return nil
}

// createFileBacking creates a file for file-backed LUN
func (lm *LUNManager) createFileBacking(path string, size int64) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create sparse file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Truncate to desired size (creates sparse file)
	if err := file.Truncate(size); err != nil {
		_ = os.Remove(path)
		return err
	}

	return nil
}

// getBlockDeviceSize gets the size of a block device
func (lm *LUNManager) getBlockDeviceSize(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

// Expand expands a file-backed LUN
func (lm *LUNManager) Expand(lun *LUN, newSize int64) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lun.Type != LUNTypeFile {
		return fmt.Errorf("only file-backed LUNs can be expanded")
	}

	if newSize <= lun.Size {
		return ErrShrinkNotSupported
	}

	// Expand file
	file, err := os.OpenFile(lun.Path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open LUN file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := file.Truncate(newSize); err != nil {
		return fmt.Errorf("failed to expand LUN: %w", err)
	}

	lun.Size = newSize
	lun.UpdatedAt = time.Now()

	return nil
}

// Shrink is not supported - returns error
func (lm *LUNManager) Shrink(lun *LUN, newSize int64) error {
	return ErrShrinkNotSupported
}

// CreateSnapshot creates a snapshot of a LUN
func (lm *LUNManager) CreateSnapshot(lun *LUN, input LUNSnapshotInput) (*LUNSnapshot, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lun.Type == LUNTypeBlock {
		return nil, fmt.Errorf("snapshots not supported for block device LUNs")
	}

	snapshotID := generateSnapshotID(lun.ID, input.Name)
	snapshotPath := lun.Path + "." + input.Name + ".snap"

	// Create copy-on-write snapshot (simplified - in production would use LVM or Btrfs)
	srcFile, err := os.Open(lun.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open LUN: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	// Copy file (in production would use reflink or snapshots)
	// For simplicity, we just create an empty marker file
	// Real implementation would use Btrfs snapshots or LVM
	_ = dstFile.Close()
	_ = os.Remove(snapshotPath) // Remove the copy, just track metadata

	snapshot := &LUNSnapshot{
		ID:        snapshotID,
		Name:      input.Name,
		LUNNumber: lun.Number,
		Size:      lun.Size,
		CreatedAt: time.Now(),
	}

	if lun.Snapshots == nil {
		lun.Snapshots = make([]*LUNSnapshot, 0)
	}
	lun.Snapshots = append(lun.Snapshots, snapshot)
	lun.UpdatedAt = time.Now()

	return snapshot, nil
}

// DeleteSnapshot removes a snapshot
func (lm *LUNManager) DeleteSnapshot(lun *LUN, snapshotID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for i, snap := range lun.Snapshots {
		if snap.ID == snapshotID {
			lun.Snapshots = append(lun.Snapshots[:i], lun.Snapshots[i+1:]...)
			lun.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("snapshot not found")
}

// Delete removes a LUN
func (lm *LUNManager) Delete(lun *LUN) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lun.Type == LUNTypeFile {
		// Remove backing file
		if err := os.Remove(lun.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete LUN file: %w", err)
		}
	}

	return nil
}

// AssignNumber assigns a LUN number to a LUN
func (lm *LUNManager) AssignNumber(lun *LUN, number int) error {
	if number < 0 || number > 255 {
		return fmt.Errorf("LUN number must be 0-255")
	}
	lun.Number = number
	lun.UpdatedAt = time.Now()
	return nil
}

// ValidatePath checks if a path is valid for a LUN
func (lm *LUNManager) ValidatePath(path string, lunType LUNType) error {
	if lunType == LUNTypeBlock {
		// Check if block device exists
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("path does not exist: %s", path)
		}
		// Check if it's a block device
		if info.Mode()&os.ModeDevice == 0 {
			return fmt.Errorf("path is not a block device: %s", path)
		}
	}
	return nil
}

// generateLUNID generates a unique LUN ID
func generateLUNID(targetID, name string) string {
	return fmt.Sprintf("%s-%s", targetID, name)
}

// generateSnapshotID generates a unique snapshot ID
func generateSnapshotID(lunID, name string) string {
	return fmt.Sprintf("%s-snap-%s-%d", lunID, name, time.Now().Unix())
}
