// Package s3 implements S3-compatible object storage for NAS-OS
// This file provides Object Lock (WORM) functionality for immutable storage.
package s3

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LegalHold represents an object legal hold status.
type LegalHold string

// Legal hold status constants.
const (
	// LegalHoldOn enables legal hold on the object.
	LegalHoldOn LegalHold = "ON"
	// LegalHoldOff disables legal hold on the object.
	LegalHoldOff LegalHold = "OFF"
)

// ObjectLockInfo contains object lock metadata for a specific object.
type ObjectLockInfo struct {
	Retention *ObjectRetention `json:"retention,omitempty"`
	LegalHold LegalHold        `json:"legalHold"`
	LockedAt  time.Time        `json:"lockedAt,omitempty"`
}

// ObjectRetention represents the retention configuration for an object.
type ObjectRetention struct {
	Mode            RetentionMode `json:"mode"`
	RetainUntilDate time.Time     `json:"retainUntilDate"`
}

// ObjectLockStatus contains the object lock status for a bucket.
type ObjectLockStatus struct {
	Enabled          bool              `json:"enabled"`
	ObjectLockEnabled bool             `json:"objectLockEnabled"`
	DefaultRetention *RetentionConfig  `json:"defaultRetention,omitempty"`
	LockedObjects    int               `json:"lockedObjects"`
}

// SetObjectLockConfig sets or updates the object lock configuration for a bucket.
// Object lock can only be enabled at bucket creation time in S3; however in this
// implementation we allow enabling it on existing buckets for flexibility.
func (m *Manager) SetObjectLockConfig(bucketName string, config *ObjectLockConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return ErrBucketNotFound
	}

	if config == nil {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidRequest",
			Message: "object lock configuration is required",
		}
	}

	// Validate retention config
	if config.DefaultRetention != nil {
		if err := validateRetentionConfig(config.DefaultRetention); err != nil {
			return &S3Error{
				Code:    400,
				CodeStr: "InvalidRetentionPeriod",
				Message: fmt.Sprintf("invalid retention config: %v", err),
			}
		}
	}

	bucket.ObjectLock = config
	return m.saveConfig()
}

// GetObjectLockConfig returns the object lock configuration for a bucket.
func (m *Manager) GetObjectLockConfig(bucketName string) (*ObjectLockConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	if bucket.ObjectLock == nil {
		return nil, &S3Error{
			Code:    404,
			CodeStr: "ObjectLockConfigurationNotFoundError",
			Message: "Object Lock configuration does not exist for this bucket",
		}
	}

	return bucket.ObjectLock, nil
}

// SetRetention sets or updates the retention period for a specific object.
// In COMPLIANCE mode, the retention cannot be shortened or removed.
// In GOVERNANCE mode, users with special permissions can override.
func (m *Manager) SetRetention(bucketName, key string, retention *ObjectRetention) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return ErrBucketNotFound
	}

	// Check if object lock is enabled on the bucket
	if bucket.ObjectLock == nil || !bucket.ObjectLock.Enabled {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidBucketState",
			Message: "Object Lock is not enabled for this bucket",
		}
	}

	_, objExists := m.objects[bucketName][key]
	if !objExists {
		return ErrObjectNotFound
	}

	if retention == nil {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidRequest",
			Message: "retention configuration is required",
		}
	}

	// Validate mode
	if retention.Mode != RetentionGovernance && retention.Mode != RetentionCompliance {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidRetentionMode",
			Message: "retention mode must be GOVERNANCE or COMPLIANCE",
		}
	}

	// Validate retain-until date
	if retention.RetainUntilDate.Before(time.Now()) {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidRetentionDate",
			Message: "retain-until date must be in the future",
		}
	}

	// Check existing retention - cannot shorten compliance mode
	lockInfo, _ := m.loadObjectLockInfo(bucketName, key)
	if lockInfo != nil && lockInfo.Retention != nil {
		if lockInfo.Retention.Mode == RetentionCompliance {
			if retention.RetainUntilDate.Before(lockInfo.Retention.RetainUntilDate) {
				return &S3Error{
					Code:    403,
					CodeStr: "AccessDenied",
					Message: "cannot shorten retention period in COMPLIANCE mode",
				}
			}
		}
	}

	// Save retention info
	newLockInfo := &ObjectLockInfo{
		Retention: retention,
		LegalHold: LegalHoldOff,
		LockedAt:  time.Now(),
	}
	if lockInfo != nil && lockInfo.LegalHold == LegalHoldOn {
		newLockInfo.LegalHold = LegalHoldOn
	}

	return m.saveObjectLockInfo(bucketName, key, newLockInfo)
}

// GetRetention returns the retention configuration for a specific object.
func (m *Manager) GetRetention(bucketName, key string) (*ObjectRetention, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	if _, exists := m.objects[bucketName][key]; !exists {
		return nil, ErrObjectNotFound
	}

	lockInfo, err := m.loadObjectLockInfo(bucketName, key)
	if err != nil {
		return nil, err
	}

	if lockInfo == nil || lockInfo.Retention == nil {
		return nil, &S3Error{
			Code:    404,
			CodeStr: "NoSuchObjectLockConfiguration",
			Message: "The specified object does not have a Object Lock configuration",
		}
	}

	return lockInfo.Retention, nil
}

// SetLegalHold sets or removes the legal hold on a specific object.
// Legal hold is independent of retention and provides an additional protection layer.
func (m *Manager) SetLegalHold(bucketName, key string, hold LegalHold) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return ErrBucketNotFound
	}

	if bucket.ObjectLock == nil || !bucket.ObjectLock.Enabled {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidBucketState",
			Message: "Object Lock is not enabled for this bucket",
		}
	}

	if _, exists := m.objects[bucketName][key]; !exists {
		return ErrObjectNotFound
	}

	if hold != LegalHoldOn && hold != LegalHoldOff {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidLegalHoldStatus",
			Message: "legal hold status must be ON or OFF",
		}
	}

	lockInfo, _ := m.loadObjectLockInfo(bucketName, key)
	if lockInfo == nil {
		lockInfo = &ObjectLockInfo{}
	}

	lockInfo.LegalHold = hold
	if hold == LegalHoldOn && lockInfo.LockedAt.IsZero() {
		lockInfo.LockedAt = time.Now()
	}

	return m.saveObjectLockInfo(bucketName, key, lockInfo)
}

// GetLegalHold returns the legal hold status for a specific object.
func (m *Manager) GetLegalHold(bucketName, key string) (LegalHold, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return LegalHoldOff, ErrBucketNotFound
	}

	if _, exists := m.objects[bucketName][key]; !exists {
		return LegalHoldOff, ErrObjectNotFound
	}

	lockInfo, err := m.loadObjectLockInfo(bucketName, key)
	if err != nil {
		return LegalHoldOff, err
	}

	if lockInfo == nil {
		return LegalHoldOff, nil
	}

	return lockInfo.LegalHold, nil
}

// IsObjectLocked checks if an object is currently locked (by retention or legal hold).
func (m *Manager) IsObjectLocked(bucketName, key string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lockInfo, err := m.loadObjectLockInfo(bucketName, key)
	if err != nil || lockInfo == nil {
		return false, ""
	}

	// Legal hold always locks the object
	if lockInfo.LegalHold == LegalHoldOn {
		return true, "legal hold is active"
	}

	// Check retention period
	if lockInfo.Retention != nil {
		if time.Now().Before(lockInfo.Retention.RetainUntilDate) {
			return true, fmt.Sprintf("retained in %s mode until %s",
				lockInfo.Retention.Mode,
				lockInfo.Retention.RetainUntilDate.Format(time.RFC3339))
		}
	}

	return false, ""
}

// GetObjectLockStatus returns a summary of object lock status for a bucket.
func (m *Manager) GetObjectLockStatus(bucketName string) (*ObjectLockStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	status := &ObjectLockStatus{
		Enabled:          bucket.ObjectLock != nil && bucket.ObjectLock.Enabled,
		ObjectLockEnabled: bucket.ObjectLock != nil && bucket.ObjectLock.Enabled,
		DefaultRetention: nil,
	}

	if bucket.ObjectLock != nil {
		status.DefaultRetention = bucket.ObjectLock.DefaultRetention
	}

	// Count locked objects
	lockDir := m.objectLockDir(bucketName)
	if entries, err := os.ReadDir(lockDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				lockInfo, _ := m.readObjectLockFile(filepath.Join(lockDir, entry.Name()))
				if lockInfo != nil {
					locked := false
					if lockInfo.LegalHold == LegalHoldOn {
						locked = true
					} else if lockInfo.Retention != nil && time.Now().Before(lockInfo.Retention.RetainUntilDate) {
						locked = true
					}
					if locked {
						status.LockedObjects++
					}
				}
			}
		}
	}

	return status, nil
}

// objectLockDir returns the directory path for object lock metadata.
func (m *Manager) objectLockDir(bucketName string) string {
	return filepath.Join(m.dataDir, bucketName, ".lock")
}

// objectLockFilePath returns the file path for a specific object's lock info.
func (m *Manager) objectLockFilePath(bucketName, key string) string {
	// Replace path separators to create a flat file name
	safeName := filepath.Base(key)
	if safeName == "." || safeName == "/" {
		safeName = "_root_"
	}
	return filepath.Join(m.objectLockDir(bucketName), safeName+".lock.json")
}

// loadObjectLockInfo loads object lock info from disk.
func (m *Manager) loadObjectLockInfo(bucketName, key string) (*ObjectLockInfo, error) {
	lockPath := m.objectLockFilePath(bucketName, key)
	return m.readObjectLockFile(lockPath)
}

// readObjectLockFile reads a lock info file.
func (m *Manager) readObjectLockFile(lockPath string) (*ObjectLockInfo, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	var lockInfo ObjectLockInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return nil, fmt.Errorf("failed to parse lock file: %w", err)
	}

	return &lockInfo, nil
}

// saveObjectLockInfo saves object lock info to disk.
func (m *Manager) saveObjectLockInfo(bucketName, key string, lockInfo *ObjectLockInfo) error {
	lockDir := m.objectLockDir(bucketName)
	if err := os.MkdirAll(lockDir, 0750); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	lockPath := m.objectLockFilePath(bucketName, key)
	data, err := json.MarshalIndent(lockInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize lock info: %w", err)
	}

	if err := os.WriteFile(lockPath, data, 0640); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

// validateRetentionConfig validates a retention configuration.
func validateRetentionConfig(config *RetentionConfig) error {
	if config.Mode != RetentionGovernance && config.Mode != RetentionCompliance {
		return fmt.Errorf("retention mode must be GOVERNANCE or COMPLIANCE")
	}

	if config.Days == 0 && config.Years == 0 {
		return fmt.Errorf("retention period must specify days or years")
	}

	if config.Days < 0 || config.Years < 0 {
		return fmt.Errorf("retention period cannot be negative")
	}

	if config.Days > 0 && config.Years > 0 {
		return fmt.Errorf("retention period cannot specify both days and years")
	}

	// AWS S3 Object Lock supports up to 100 years retention
	if config.Years > 100 {
		return fmt.Errorf("retention period cannot exceed 100 years")
	}

	if config.Days > 36500 {
		return fmt.Errorf("retention period cannot exceed 36500 days (100 years)")
	}

	return nil
}

// ComplianceMode represents compliance mode configuration.
type ComplianceMode struct {
	Enabled          bool      `json:"enabled"`
	RetentionDays    int       `json:"retentionDays"`
	RetentionYears   int       `json:"retentionYears"`
	LockEnabled      bool      `json:"lockEnabled"`
	LockRetainDate   time.Time `json:"lockRetainDate,omitempty"`
}

// SetComplianceMode enables compliance mode with specified retention on a bucket.
// This is a convenience method that configures object lock in COMPLIANCE mode.
func (m *Manager) SetComplianceMode(bucketName string, days, years int) error {
	config := &ObjectLockConfig{
		Enabled: true,
		DefaultRetention: &RetentionConfig{
			Mode:  RetentionCompliance,
			Days:  days,
			Years: years,
		},
	}
	return m.SetObjectLockConfig(bucketName, config)
}

// GetComplianceMode returns compliance mode information for a bucket.
func (m *Manager) GetComplianceMode(bucketName string) (*ComplianceMode, error) {
	config, err := m.GetObjectLockConfig(bucketName)
	if err != nil {
		return nil, err
	}

	mode := &ComplianceMode{
		Enabled: config.Enabled,
	}

	if config.DefaultRetention != nil {
		mode.RetentionDays = config.DefaultRetention.Days
		mode.RetentionYears = config.DefaultRetention.Years
	}

	return mode, nil
}
