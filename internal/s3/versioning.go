// Package s3 implements S3-compatible object storage for NAS-OS
// This file provides object versioning management.
package s3

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ObjectVersion represents a specific version of an object.
type ObjectVersion struct {
	VersionID      string            `json:"versionId"`
	Key            string            `json:"key"`
	Size           int64             `json:"size"`
	ETag           string            `json:"etag"`
	LastModified   time.Time         `json:"lastModified"`
	IsLatest       bool              `json:"isLatest"`
	StorageClass   StorageClass      `json:"storageClass"`
	IsDeleteMarker bool              `json:"isDeleteMarker,omitempty"`
	Owner          string            `json:"owner,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// VersionList contains a list of object versions.
type VersionList struct {
	Bucket              string           `json:"bucket"`
	Prefix              string           `json:"prefix"`
	KeyMarker           string           `json:"keyMarker,omitempty"`
	VersionIDMarker     string           `json:"versionIdMarker,omitempty"`
	MaxKeys             int              `json:"maxKeys"`
	IsTruncated         bool             `json:"isTruncated"`
	NextKeyMarker       string           `json:"nextKeyMarker,omitempty"`
	NextVersionIDMarker string           `json:"nextVersionIdMarker,omitempty"`
	Versions            []*ObjectVersion `json:"versions"`
	DeleteMarkers       []*ObjectVersion `json:"deleteMarkers,omitempty"`
}

// VersioningStatusSummary contains versioning status details.
type VersioningStatusSummary struct {
	Status            VersioningStatus `json:"status"`
	VersionCount      int64            `json:"versionCount"`
	DeleteMarkerCount int64            `json:"deleteMarkerCount"`
}

// EnableVersioning enables versioning for a bucket.
func (m *Manager) EnableVersioning(bucketName string) error {
	return m.SetBucketVersioning(bucketName, VersioningConfig{
		Status: VersioningEnabled,
	})
}

// SuspendVersioning suspends versioning for a bucket.
// Existing versions are preserved but new objects won't get version IDs.
func (m *Manager) SuspendVersioning(bucketName string) error {
	return m.SetBucketVersioning(bucketName, VersioningConfig{
		Status: VersioningSuspended,
	})
}

// GetVersioningStatus returns the versioning status for a bucket.
func (m *Manager) GetVersioningStatus(bucketName string) (*VersioningStatusSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	summary := &VersioningStatusSummary{
		Status: bucket.Versioning.Status,
	}

	// Count versions
	versionsDir := m.versionsDir(bucketName)
	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if filepath.Ext(entry.Name()) == ".json" {
				summary.VersionCount++
			}
		}
	}

	return summary, nil
}

// PutObjectVersion creates a new version of an object.
// This is called internally when versioning is enabled.
func (m *Manager) PutObjectVersion(bucketName, key string, obj *Object) error {
	if obj.VersionID == "" {
		obj.VersionID = uuid.New().String()
	}

	version := &ObjectVersion{
		VersionID:    obj.VersionID,
		Key:          key,
		Size:         obj.Size,
		ETag:         obj.ETag,
		LastModified: obj.LastModified,
		IsLatest:     true,
		StorageClass: obj.StorageClass,
		Metadata:     obj.Metadata,
	}

	// Mark previous latest as non-latest
	versions, _ := m.listVersionsInternal(bucketName, key)
	for _, v := range versions {
		if v.IsLatest && !v.IsDeleteMarker {
			v.IsLatest = false
			_ = m.saveVersionInfo(bucketName, key, v)
		}
	}

	return m.saveVersionInfo(bucketName, key, version)
}

// ListVersions lists all versions of objects in a bucket.
func (m *Manager) ListVersions(bucketName, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*VersionList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	result := &VersionList{
		Bucket:          bucketName,
		Prefix:          prefix,
		KeyMarker:       keyMarker,
		VersionIDMarker: versionIDMarker,
		MaxKeys:         maxKeys,
		Versions:        make([]*ObjectVersion, 0),
		DeleteMarkers:   make([]*ObjectVersion, 0),
	}

	// Collect all versions from objects and version metadata
	objects := m.objects[bucketName]
	type versionEntry struct {
		version  *ObjectVersion
		isDelete bool
	}

	var allVersions []versionEntry

	// Current objects
	for key, obj := range objects {
		if prefix != "" && len(key) >= len(prefix) && key[:len(prefix)] != prefix {
			continue
		}
		if prefix != "" && len(key) < len(prefix) {
			continue
		}

		allVersions = append(allVersions, versionEntry{
			version: &ObjectVersion{
				VersionID:    obj.VersionID,
				Key:          key,
				Size:         obj.Size,
				ETag:         obj.ETag,
				LastModified: obj.LastModified,
				IsLatest:     true,
				StorageClass: obj.StorageClass,
				Metadata:     obj.Metadata,
			},
		})
	}

	// Historical versions from disk
	versionsDir := m.versionsDir(bucketName)
	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			v, err := m.readVersionFile(filepath.Join(versionsDir, entry.Name()))
			if err != nil {
				continue
			}
			if prefix != "" && len(v.Key) >= len(prefix) && v.Key[:len(prefix)] != prefix {
				continue
			}
			allVersions = append(allVersions, versionEntry{version: v, isDelete: v.IsDeleteMarker})
		}
	}

	// Sort by key, then by last modified descending
	sort.Slice(allVersions, func(i, j int) bool {
		if allVersions[i].version.Key != allVersions[j].version.Key {
			return allVersions[i].version.Key < allVersions[j].version.Key
		}
		return allVersions[i].version.LastModified.After(allVersions[j].version.LastModified)
	})

	// Apply pagination
	startFound := keyMarker == ""
	count := 0

	for _, entry := range allVersions {
		if !startFound {
			if entry.version.Key == keyMarker {
				if versionIDMarker != "" && entry.version.VersionID != versionIDMarker {
					continue
				}
				startFound = true
				continue
			}
			continue
		}

		if count >= maxKeys {
			result.IsTruncated = true
			result.NextKeyMarker = entry.version.Key
			result.NextVersionIDMarker = entry.version.VersionID
			break
		}

		if entry.isDelete || entry.version.IsDeleteMarker {
			result.DeleteMarkers = append(result.DeleteMarkers, entry.version)
		} else {
			result.Versions = append(result.Versions, entry.version)
		}
		count++
	}

	return result, nil
}

// DeleteVersion deletes a specific version of an object.
func (m *Manager) DeleteVersion(bucketName, key, versionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return ErrBucketNotFound
	}

	// Check if this is the current version
	if obj, exists := m.objects[bucketName][key]; exists && obj.VersionID == versionID {
		// Check object lock
		if locked, reason := m.isObjectLockedNoLock(bucketName, key); locked {
			return &S3Error{
				Code:    403,
				CodeStr: "AccessDenied",
				Message: fmt.Sprintf("object is locked: %s", reason),
			}
		}

		// Remove current object file
		objPath := filepath.Join(m.dataDir, bucketName, key)
		_ = os.Remove(objPath)
		_ = os.Remove(objPath + ".meta")
		delete(m.objects[bucketName], key)

		// Remove version record
		return m.removeVersionInfo(bucketName, key, versionID)
	}

	// Check historical versions
	versionPath := m.versionFilePath(bucketName, key, versionID)
	if _, err := os.Stat(versionPath); os.IsNotExist(err) {
		return ErrObjectNotFound
	}

	// Check object lock for historical version
	lockPath := m.objectLockFilePath(bucketName, key)
	lockInfo, _ := m.readObjectLockFile(lockPath)
	if lockInfo != nil {
		if lockInfo.LegalHold == LegalHoldOn {
			return &S3Error{
				Code:    403,
				CodeStr: "AccessDenied",
				Message: "object version is under legal hold",
			}
		}
		if lockInfo.Retention != nil && time.Now().Before(lockInfo.Retention.RetainUntilDate) {
			return &S3Error{
				Code:    403,
				CodeStr: "AccessDenied",
				Message: "object version is under retention",
			}
		}
	}

	_ = os.Remove(versionPath)
	return nil
}

// CreateDeleteMarker creates a delete marker for an object (S3-style soft delete).
func (m *Manager) CreateDeleteMarker(bucketName, key string) (*ObjectVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	bucket := m.buckets[bucketName]
	if bucket.Versioning.Status != VersioningEnabled {
		return nil, &S3Error{
			Code:    400,
			CodeStr: "InvalidBucketState",
			Message: "versioning is not enabled for this bucket",
		}
	}

	versionID := uuid.New().String()
	deleteMarker := &ObjectVersion{
		VersionID:      versionID,
		Key:            key,
		IsDeleteMarker: true,
		IsLatest:       true,
		LastModified:   time.Now(),
		StorageClass:   StorageClassStandard,
	}

	// Mark previous versions as non-latest
	versions, _ := m.listVersionsInternal(bucketName, key)
	for _, v := range versions {
		if v.IsLatest {
			v.IsLatest = false
			_ = m.saveVersionInfo(bucketName, key, v)
		}
	}

	// Save delete marker
	if err := m.saveVersionInfo(bucketName, key, deleteMarker); err != nil {
		return nil, err
	}

	// Remove current object if it exists
	objPath := filepath.Join(m.dataDir, bucketName, key)
	_ = os.Remove(objPath)
	_ = os.Remove(objPath + ".meta")
	delete(m.objects[bucketName], key)

	return deleteMarker, nil
}

// GetVersionInfo returns information about a specific object version.
func (m *Manager) GetVersionInfo(bucketName, key, versionID string) (*ObjectVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	// Check if it's the current version
	if obj, exists := m.objects[bucketName][key]; exists && obj.VersionID == versionID {
		return &ObjectVersion{
			VersionID:    obj.VersionID,
			Key:          key,
			Size:         obj.Size,
			ETag:         obj.ETag,
			LastModified: obj.LastModified,
			IsLatest:     true,
			StorageClass: obj.StorageClass,
			Metadata:     obj.Metadata,
		}, nil
	}

	// Check historical versions
	v, err := m.readVersionFile(m.versionFilePath(bucketName, key, versionID))
	if err != nil {
		return nil, ErrObjectNotFound
	}

	return v, nil
}

// isObjectLockedNoLock checks lock status without acquiring the mutex (caller must hold lock).
func (m *Manager) isObjectLockedNoLock(bucketName, key string) (bool, string) {
	lockInfo, err := m.loadObjectLockInfo(bucketName, key)
	if err != nil || lockInfo == nil {
		return false, ""
	}

	if lockInfo.LegalHold == LegalHoldOn {
		return true, "legal hold is active"
	}

	if lockInfo.Retention != nil && time.Now().Before(lockInfo.Retention.RetainUntilDate) {
		return true, "object is under retention"
	}

	return false, ""
}

// listVersionsInternal lists versions without acquiring mutex (caller must hold lock).
func (m *Manager) listVersionsInternal(bucketName, key string) ([]*ObjectVersion, error) {
	var versions []*ObjectVersion
	versionsDir := m.versionsDir(bucketName)
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return versions, nil
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		v, err := m.readVersionFile(filepath.Join(versionsDir, entry.Name()))
		if err != nil {
			continue
		}
		if v.Key == key {
			versions = append(versions, v)
		}
	}

	return versions, nil
}

// versionsDir returns the directory for version metadata.
func (m *Manager) versionsDir(bucketName string) string {
	return filepath.Join(m.dataDir, bucketName, ".versions")
}

// versionFilePath returns the file path for a specific version.
func (m *Manager) versionFilePath(bucketName, key, versionID string) string {
	safeKey := filepath.Base(key)
	if safeKey == "." || safeKey == "/" {
		safeKey = "_root_"
	}
	return filepath.Join(m.versionsDir(bucketName), safeKey+"-"+versionID+".json")
}

// saveVersionInfo saves version information to disk.
func (m *Manager) saveVersionInfo(bucketName, key string, version *ObjectVersion) error {
	dir := m.versionsDir(bucketName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create versions directory: %w", err)
	}

	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize version info: %w", err)
	}

	return os.WriteFile(m.versionFilePath(bucketName, key, version.VersionID), data, 0640)
}

// readVersionFile reads a version info file.
func (m *Manager) readVersionFile(path string) (*ObjectVersion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var v ObjectVersion
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	return &v, nil
}

// removeVersionInfo removes a version info file.
func (m *Manager) removeVersionInfo(bucketName, key, versionID string) error {
	path := m.versionFilePath(bucketName, key, versionID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
