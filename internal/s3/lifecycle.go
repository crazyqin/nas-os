// Package s3 implements S3-compatible object storage for NAS-OS
// This file provides lifecycle management for automatic expiration and storage class transitions.
package s3

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LifecycleConfig represents the lifecycle configuration for a bucket.
type LifecycleConfig struct {
	Rules []LifecycleRule `json:"rules"`
}

// LifecycleRule represents a single lifecycle rule.
type LifecycleRule struct {
	ID                     string              `json:"id"`
	Status                 LifecycleRuleStatus `json:"status"`
	Filter                 *LifecycleFilter    `json:"filter,omitempty"`
	Expiration             *ExpirationAction   `json:"expiration,omitempty"`
	Transition             *TransitionAction   `json:"transition,omitempty"`
	NoncurrentExpiration   *NoncurrentExpiration `json:"noncurrentExpiration,omitempty"`
	NoncurrentTransition   *NoncurrentTransition `json:"noncurrentTransition,omitempty"`
	AbortIncompleteUpload  *AbortIncompleteMultipartUpload `json:"abortIncompleteUpload,omitempty"`
	CreatedAt              time.Time           `json:"createdAt"`
}

// LifecycleRuleStatus represents the status of a lifecycle rule.
type LifecycleRuleStatus string

// Lifecycle rule status constants.
const (
	LifecycleRuleEnabled  LifecycleRuleStatus = "Enabled"
	LifecycleRuleDisabled LifecycleRuleStatus = "Disabled"
)

// LifecycleFilter defines which objects a lifecycle rule applies to.
type LifecycleFilter struct {
	Prefix string            `json:"prefix,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// ExpirationAction defines when objects expire and should be deleted.
type ExpirationAction struct {
	Days         int       `json:"days,omitempty"`
	Date         time.Time `json:"date,omitempty"`
	ExpiredObjectDeleteMarker bool `json:"expiredObjectDeleteMarker,omitempty"`
}

// TransitionAction defines when objects should transition to a different storage class.
type TransitionAction struct {
	Days         int          `json:"days,omitempty"`
	Date         time.Time    `json:"date,omitempty"`
	StorageClass StorageClass `json:"storageClass"`
}

// NoncurrentExpiration defines expiration for non-current object versions.
type NoncurrentExpiration struct {
	NoncurrentDays int `json:"noncurrentDays"`
}

// NoncurrentTransition defines transition for non-current object versions.
type NoncurrentTransition struct {
	NoncurrentDays int           `json:"noncurrentDays"`
	StorageClass   StorageClass  `json:"storageClass"`
}

// AbortIncompleteMultipartUpload defines when to abort incomplete multipart uploads.
type AbortIncompleteMultipartUpload struct {
	DaysAfterInitiation int `json:"daysAfterInitiation"`
}

// LifecycleStatus contains the current lifecycle status for a bucket.
type LifecycleStatus struct {
	Enabled      bool                  `json:"enabled"`
	RuleCount    int                   `json:"ruleCount"`
	Rules        []LifecycleRuleStatus `json:"rules"`
	LastEvaluated time.Time            `json:"lastEvaluated,omitempty"`
	ObjectsExpired    int64            `json:"objectsExpired"`
	ObjectsTransitioned int64          `json:"objectsTransitioned"`
}

// LifecycleResult contains the result of applying lifecycle rules.
type LifecycleResult struct {
	Expired      int `json:"expired"`
	Transitioned int `json:"transitioned"`
	Errors       int `json:"errors"`
}

// SetLifecycleConfig sets the lifecycle configuration for a bucket.
func (m *Manager) SetLifecycleConfig(bucketName string, config *LifecycleConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return ErrBucketNotFound
	}

	if config == nil {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidRequest",
			Message: "lifecycle configuration is required",
		}
	}

	// Validate rules
	if err := validateLifecycleConfig(config); err != nil {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidArgument",
			Message: fmt.Sprintf("invalid lifecycle configuration: %v", err),
		}
	}

	// Save to disk
	return m.saveLifecycleConfig(bucketName, config)
}

// GetLifecycleConfig returns the lifecycle configuration for a bucket.
func (m *Manager) GetLifecycleConfig(bucketName string) (*LifecycleConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	config, err := m.loadLifecycleConfig(bucketName)
	if err != nil {
		return nil, err
	}

	if config == nil {
		return nil, &S3Error{
			Code:    404,
			CodeStr: "NoSuchLifecycleConfiguration",
			Message: "The lifecycle configuration does not exist",
		}
	}

	return config, nil
}

// GetLifecycleStatus returns the lifecycle management status for a bucket.
func (m *Manager) GetLifecycleStatus(bucketName string) (*LifecycleStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	config, err := m.loadLifecycleConfig(bucketName)
	if err != nil {
		return nil, err
	}

	status := &LifecycleStatus{
		Enabled: config != nil && len(config.Rules) > 0,
	}

	if config != nil {
		status.RuleCount = len(config.Rules)
		for _, rule := range config.Rules {
			status.Rules = append(status.Rules, rule.Status)
		}
	}

	// Load stats if available
	stats, _ := m.loadLifecycleStats(bucketName)
	if stats != nil {
		status.LastEvaluated = stats.LastEvaluated
		status.ObjectsExpired = stats.ObjectsExpired
		status.ObjectsTransitioned = stats.ObjectsTransitioned
	}

	return status, nil
}

// ApplyLifecycleRules evaluates and applies lifecycle rules for a bucket.
// This should be called periodically (e.g., via cron or a background goroutine).
func (m *Manager) ApplyLifecycleRules(bucketName string) (*LifecycleResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	config, err := m.loadLifecycleConfig(bucketName)
	if err != nil {
		return nil, err
	}

	if config == nil || len(config.Rules) == 0 {
		return &LifecycleResult{}, nil
	}

	result := &LifecycleResult{}
	now := time.Now()

	objects := m.objects[bucketName]
	if objects == nil {
		return result, nil
	}

	// Collect objects to delete or transition
	var toDelete []string
	var toTransition []struct {
		key          string
		storageClass StorageClass
	}

	for key, obj := range objects {
		for _, rule := range config.Rules {
			if rule.Status != LifecycleRuleEnabled {
				continue
			}

			if !objectMatchesFilter(obj, rule.Filter) {
				continue
			}

			// Check expiration
			if rule.Expiration != nil {
				if shouldExpire(obj, rule.Expiration, now) {
					toDelete = append(toDelete, key)
					result.Expired++
					break // object already marked for deletion
				}
			}

			// Check transition
			if rule.Transition != nil {
				newClass, should := shouldTransition(obj, rule.Transition, now)
				if should {
					toTransition = append(toTransition, struct {
						key          string
						storageClass StorageClass
					}{key: key, storageClass: newClass})
					result.Transitioned++
					break
				}
			}
		}
	}

	// Apply deletions
	for _, key := range toDelete {
		objPath := filepath.Join(m.dataDir, bucketName, key)
		if err := os.Remove(objPath); err != nil {
			result.Errors++
			continue
		}
		_ = os.Remove(objPath + ".meta")
		delete(objects, key)
	}

	// Apply transitions
	for _, t := range toTransition {
		if obj, ok := objects[t.key]; ok {
			obj.StorageClass = t.storageClass
		}
	}

	// Save lifecycle stats
	stats := &lifecycleStats{
		LastEvaluated:       now,
		ObjectsExpired:      int64(result.Expired),
		ObjectsTransitioned: int64(result.Transitioned),
	}
	_ = m.saveLifecycleStats(bucketName, stats)

	return result, nil
}

// RemoveLifecycleConfig removes the lifecycle configuration for a bucket.
func (m *Manager) RemoveLifecycleConfig(bucketName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return ErrBucketNotFound
	}

	lifecycleDir := filepath.Join(m.dataDir, bucketName, ".lifecycle")
	return os.RemoveAll(lifecycleDir)
}

// objectMatchesFilter checks if an object matches a lifecycle filter.
func objectMatchesFilter(obj *Object, filter *LifecycleFilter) bool {
	if filter == nil {
		return true // no filter matches all objects
	}

	// Check prefix
	if filter.Prefix != "" {
		if len(obj.Key) < len(filter.Prefix) || obj.Key[:len(filter.Prefix)] != filter.Prefix {
			return false
		}
	}

	// Check tags (simplified - in production would check actual object tags)
	if len(filter.Tags) > 0 {
		if obj.Metadata == nil {
			return false
		}
		for k, v := range filter.Tags {
			if objVal, ok := obj.Metadata[k]; !ok || objVal != v {
				return false
			}
		}
	}

	return true
}

// shouldExpire checks if an object should be expired based on an expiration action.
func shouldExpire(obj *Object, exp *ExpirationAction, now time.Time) bool {
	if !exp.Date.IsZero() {
		return now.After(exp.Date) || now.Equal(exp.Date)
	}
	if exp.Days > 0 {
		expireAt := obj.LastModified.AddDate(0, 0, exp.Days)
		return now.After(expireAt) || now.Equal(expireAt)
	}
	return false
}

// shouldTransition checks if an object should transition storage class.
func shouldTransition(obj *Object, trans *TransitionAction, now time.Time) (StorageClass, bool) {
	if obj.StorageClass == trans.StorageClass {
		return "", false // already in target class
	}

	if !trans.Date.IsZero() {
		if now.After(trans.Date) || now.Equal(trans.Date) {
			return trans.StorageClass, true
		}
		return "", false
	}
	if trans.Days > 0 {
		transitionAt := obj.LastModified.AddDate(0, 0, trans.Days)
		if now.After(transitionAt) || now.Equal(transitionAt) {
			return trans.StorageClass, true
		}
	}
	return "", false
}

// lifecycleStats tracks lifecycle execution statistics.
type lifecycleStats struct {
	LastEvaluated       time.Time `json:"lastEvaluated"`
	ObjectsExpired      int64       `json:"objectsExpired"`
	ObjectsTransitioned int64       `json:"objectsTransitioned"`
}

// lifecycleDir returns the directory for lifecycle metadata.
func (m *Manager) lifecycleDir(bucketName string) string {
	return filepath.Join(m.dataDir, bucketName, ".lifecycle")
}

// saveLifecycleConfig saves lifecycle configuration to disk.
func (m *Manager) saveLifecycleConfig(bucketName string, config *LifecycleConfig) error {
	dir := m.lifecycleDir(bucketName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create lifecycle directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize lifecycle config: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0640)
}

// loadLifecycleConfig loads lifecycle configuration from disk.
func (m *Manager) loadLifecycleConfig(bucketName string) (*LifecycleConfig, error) {
	configPath := filepath.Join(m.lifecycleDir(bucketName), "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read lifecycle config: %w", err)
	}

	var config LifecycleConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse lifecycle config: %w", err)
	}

	return &config, nil
}

// saveLifecycleStats saves lifecycle statistics.
func (m *Manager) saveLifecycleStats(bucketName string, stats *lifecycleStats) error {
	dir := m.lifecycleDir(bucketName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "stats.json"), data, 0640)
}

// loadLifecycleStats loads lifecycle statistics.
func (m *Manager) loadLifecycleStats(bucketName string) (*lifecycleStats, error) {
	statsPath := filepath.Join(m.lifecycleDir(bucketName), "stats.json")
	data, err := os.ReadFile(statsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var stats lifecycleStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// validateLifecycleConfig validates a lifecycle configuration.
func validateLifecycleConfig(config *LifecycleConfig) error {
	if len(config.Rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}

	if len(config.Rules) > 1000 {
		return fmt.Errorf("maximum 1000 rules allowed per bucket")
	}

	seenIDs := make(map[string]bool)
	for i, rule := range config.Rules {
		if err := validateLifecycleRule(i, &rule); err != nil {
			return err
		}
		if rule.ID != "" {
			if seenIDs[rule.ID] {
				return fmt.Errorf("duplicate rule ID: %s", rule.ID)
			}
			seenIDs[rule.ID] = true
		}
	}

	return nil
}

// validateLifecycleRule validates a single lifecycle rule.
func validateLifecycleRule(index int, rule *LifecycleRule) error {
	if rule.Status != LifecycleRuleEnabled && rule.Status != LifecycleRuleDisabled {
		return fmt.Errorf("rule[%d]: status must be Enabled or Disabled", index)
	}

	if rule.Expiration == nil && rule.Transition == nil &&
		rule.NoncurrentExpiration == nil && rule.NoncurrentTransition == nil &&
		rule.AbortIncompleteUpload == nil {
		return fmt.Errorf("rule[%d]: at least one action is required", index)
	}

	if rule.Expiration != nil {
		if rule.Expiration.Days < 0 {
			return fmt.Errorf("rule[%d]: expiration days cannot be negative", index)
		}
		if rule.Expiration.Days > 0 && !rule.Expiration.Date.IsZero() {
			return fmt.Errorf("rule[%d]: cannot specify both days and date for expiration", index)
		}
	}

	if rule.Transition != nil {
		if rule.Transition.Days < 0 {
			return fmt.Errorf("rule[%d]: transition days cannot be negative", index)
		}
		if rule.Transition.Days > 0 && !rule.Transition.Date.IsZero() {
			return fmt.Errorf("rule[%d]: cannot specify both days and date for transition", index)
		}
		if !isValidStorageClass(rule.Transition.StorageClass) {
			return fmt.Errorf("rule[%d]: invalid storage class %s", index, rule.Transition.StorageClass)
		}
	}

	return nil
}

// isValidStorageClass checks if a storage class is valid.
func isValidStorageClass(sc StorageClass) bool {
	switch sc {
	case StorageClassStandard, StorageClassReducedRedundancy,
		StorageClassGlacier, StorageClassDeepArchive:
		return true
	default:
		return false
	}
}
