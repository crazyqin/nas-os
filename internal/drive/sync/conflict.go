package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConflictResolver handles sync conflict resolution.
// Supports multiple strategies inspired by Synology Drive.
type ConflictResolver struct {
	mu         sync.RWMutex
	policy     ConflictStrategy
	records    []*ConflictRecord
	maxRecords int
}

// ConflictRecord represents a recorded conflict.
type ConflictRecord struct {
	ID         string           `json:"id"`
	TaskID     string           `json:"task_id"`
	FilePath   string           `json:"file_path"`
	LocalHash  string           `json:"local_hash"`
	RemoteHash string           `json:"remote_hash"`
	Policy     ConflictStrategy `json:"policy"`
	Resolved   bool             `json:"resolved"`
	Resolution string           `json:"resolution,omitempty"`
}

// NewConflictResolver creates a new conflict resolver.
func NewConflictResolver(policy ConflictStrategy) *ConflictResolver {
	return &ConflictResolver{
		policy:     policy,
		records:    make([]*ConflictRecord, 0),
		maxRecords: 1000,
	}
}

// Resolve handles conflict resolution for a sync action.
// Returns (resolved, error) - resolved=true means the action was applied.
func (cr *ConflictResolver) Resolve(ctx context.Context, action *SyncAction, localRoot, remoteRoot string, remote RemoteStorage) (bool, error) {
	if action.Type != "conflict" || action.Local == nil || action.Remote == nil {
		return false, nil
	}

	localEntry := action.Local
	remoteEntry := action.Remote
	localPath := filepath.Join(localRoot, action.Path)
	remotePath := filepath.Join(remoteRoot, action.Path)

	switch cr.policy {
	case ConflictNewerWins:
		if localEntry.ModTime.After(remoteEntry.ModTime) {
			err := remote.Put(ctx, localPath, remotePath)
			cr.recordAction(action, "keep_local", err)
			return err == nil, err
		}
		err := remote.Get(ctx, remotePath, localPath)
		cr.recordAction(action, "keep_remote", err)
		return err == nil, err

	case ConflictKeepBoth:
		conflictPath := localPath + fmt.Sprintf(" (conflict %s)", time.Now().Format("2006-01-02"))
		data, err := os.ReadFile(localPath)
		if err != nil {
			return false, fmt.Errorf("read conflict file: %w", err)
		}
		if err := os.WriteFile(conflictPath, data, 0644); err != nil {
			return false, fmt.Errorf("write conflict copy: %w", err)
		}
		if err := remote.Get(ctx, remotePath, localPath); err != nil {
			return false, fmt.Errorf("download after keep both: %w", err)
		}
		cr.recordAction(action, "keep_both", nil)
		return true, nil

	case ConflictAsk:
		cr.recordAction(action, "ask_user", nil)
		return false, nil

	default:
		err := remote.Get(ctx, remotePath, localPath)
		cr.recordAction(action, "keep_remote", err)
		return err == nil, err
	}
}

func (cr *ConflictResolver) recordAction(action *SyncAction, resolution string, err error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if len(cr.records) >= cr.maxRecords {
		var active []*ConflictRecord
		for _, r := range cr.records {
			if !r.Resolved {
				active = append(active, r)
			}
		}
		cr.records = active
	}

	rec := &ConflictRecord{
		ID:         fmt.Sprintf("conflict-%d", len(cr.records)+1),
		FilePath:   action.Path,
		LocalHash:  action.Local.Checksum,
		RemoteHash: action.Remote.Checksum,
		Policy:     cr.policy,
		Resolved:   resolution != "ask_user",
		Resolution: resolution,
	}
	cr.records = append(cr.records, rec)
}

// GetUnresolved returns unresolved conflicts.
func (cr *ConflictResolver) GetUnresolved() []*ConflictRecord {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	var unresolved []*ConflictRecord
	for _, r := range cr.records {
		if !r.Resolved {
			unresolved = append(unresolved, r)
		}
	}
	return unresolved
}

// MarkResolved marks a conflict as resolved.
func (cr *ConflictResolver) MarkResolved(id string, resolution string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for _, r := range cr.records {
		if r.ID == id {
			r.Resolved = true
			r.Resolution = resolution
			return nil
		}
	}
	return ErrConflictNotFound
}

// SetPolicy updates the conflict policy.
func (cr *ConflictResolver) SetPolicy(policy ConflictStrategy) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.policy = policy
}

// Count returns unresolved conflict count.
func (cr *ConflictResolver) Count() int {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	count := 0
	for _, r := range cr.records {
		if !r.Resolved {
			count++
		}
	}
	return count
}
