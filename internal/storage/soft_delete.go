package storage

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DefaultDeleteGracePeriod is how long a soft-deleted volume stays recoverable.
// After PurgeAt, the reaper drops the pending entry (devices keep FS signatures
// unless the original delete requested AllowWipe, which is only executed on Purge).
const DefaultDeleteGracePeriod = 24 * time.Hour

// PendingVolumeDeletion is a soft-deleted volume held for restore until PurgeAt.
type PendingVolumeDeletion struct {
	Volume     *Volume   `json:"volume"`
	DetachedAt time.Time `json:"detached_at"`
	PurgeAt    time.Time `json:"purge_at"`
	Force      bool      `json:"force"`
	// AllowWipe: when true, PurgePending will call wipefs via DeleteVolume path.
	// Soft delete itself never wipes; wipe only happens on explicit purge/expiry
	// if this flag was set at delete time (rare; prefer allow_wipe with grace 0).
	AllowWipe bool `json:"allow_wipe"`
}

// Soft-delete grace state on Manager (lazy init).
type softDeleteState struct {
	mu       sync.Mutex
	pending  map[string]*PendingVolumeDeletion
	grace    time.Duration // 0 means default
	stopCh   chan struct{}
	started  bool
	reaperWG sync.WaitGroup
}

func (m *Manager) softState() *softDeleteState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.soft == nil {
		m.soft = &softDeleteState{
			pending: make(map[string]*PendingVolumeDeletion),
			grace:   DefaultDeleteGracePeriod,
			stopCh:  make(chan struct{}),
		}
	}
	return m.soft
}

// SetDeleteGracePeriod sets the soft-delete recovery window (min 1m, max 30d).
// Zero resets to DefaultDeleteGracePeriod.
func (m *Manager) SetDeleteGracePeriod(d time.Duration) {
	st := m.softState()
	st.mu.Lock()
	defer st.mu.Unlock()
	if d <= 0 {
		st.grace = DefaultDeleteGracePeriod
		return
	}
	if d < time.Minute {
		d = time.Minute
	}
	if d > 30*24*time.Hour {
		d = 30 * 24 * time.Hour
	}
	st.grace = d
}

// DeleteGracePeriod returns the configured grace window.
func (m *Manager) DeleteGracePeriod() time.Duration {
	st := m.softState()
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.grace <= 0 {
		return DefaultDeleteGracePeriod
	}
	return st.grace
}

// ensureReaper starts the background purge loop once.
func (m *Manager) ensureReaper() {
	st := m.softState()
	st.mu.Lock()
	if st.started {
		st.mu.Unlock()
		return
	}
	st.started = true
	st.reaperWG.Add(1)
	st.mu.Unlock()
	go m.softDeleteReaper()
}

func (m *Manager) softDeleteReaper() {
	st := m.softState()
	defer st.reaperWG.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-st.stopCh:
			return
		case <-ticker.C:
			m.purgeExpiredPending()
		}
	}
}

// StopSoftDeleteReaper stops the grace-period reaper (tests / shutdown).
func (m *Manager) StopSoftDeleteReaper() {
	if m == nil || m.soft == nil {
		return
	}
	st := m.soft
	st.mu.Lock()
	if !st.started {
		st.mu.Unlock()
		return
	}
	select {
	case <-st.stopCh:
		// already closed
	default:
		close(st.stopCh)
	}
	st.mu.Unlock()
	st.reaperWG.Wait()
}

func (m *Manager) purgeExpiredPending() {
	st := m.softState()
	st.mu.Lock()
	now := time.Now()
	var expired []string
	for name, p := range st.pending {
		if p != nil && !p.PurgeAt.After(now) {
			expired = append(expired, name)
		}
	}
	st.mu.Unlock()
	for _, name := range expired {
		if err := m.PurgePending(name); err != nil {
			log.Printf("soft-delete reaper: purge %s: %v", name, err)
		}
	}
}

// SoftDetachVolume unmounts, removes from active volumes, and holds a recoverable
// pending entry until grace expires (or RestorePending / PurgePending).
func (m *Manager) SoftDetachVolume(name string, force bool, allowWipe bool, grace time.Duration) error {
	m.mu.Lock()
	vol, exists := m.volumes[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("卷 %s 不存在", name)
	}
	if len(vol.Subvolumes) > 0 && !force {
		m.mu.Unlock()
		return fmt.Errorf("卷包含 %d 个子卷，请先删除子卷或使用强制删除", len(vol.Subvolumes))
	}
	// Copy volume snapshot for pending
	cp := *vol
	if vol.Devices != nil {
		cp.Devices = append([]string(nil), vol.Devices...)
	}
	if vol.MountPoint != "" && m.client != nil {
		if err := m.client.Unmount(vol.MountPoint); err != nil {
			if !force {
				m.mu.Unlock()
				return fmt.Errorf("卸载失败: %w", err)
			}
		}
	}
	delete(m.volumes, name)
	m.mu.Unlock()

	if grace <= 0 {
		grace = m.DeleteGracePeriod()
	}
	now := time.Now()
	st := m.softState()
	st.mu.Lock()
	st.pending[name] = &PendingVolumeDeletion{
		Volume:     &cp,
		DetachedAt: now,
		PurgeAt:    now.Add(grace),
		Force:      force,
		AllowWipe:  allowWipe,
	}
	st.mu.Unlock()
	m.ensureReaper()
	return nil
}

// ListPendingDeletions returns soft-deleted volumes still in the grace window.
func (m *Manager) ListPendingDeletions() []*PendingVolumeDeletion {
	st := m.softState()
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]*PendingVolumeDeletion, 0, len(st.pending))
	for _, p := range st.pending {
		if p == nil {
			continue
		}
		// shallow copy for safety
		cp := *p
		if p.Volume != nil {
			v := *p.Volume
			cp.Volume = &v
		}
		out = append(out, &cp)
	}
	return out
}

// RestorePending brings a soft-deleted volume back into the active map (no remount).
// Caller may MountVolume afterward.
func (m *Manager) RestorePending(name string) error {
	st := m.softState()
	st.mu.Lock()
	p, ok := st.pending[name]
	if !ok || p == nil || p.Volume == nil {
		st.mu.Unlock()
		return fmt.Errorf("待删除卷 %s 不存在或已过期", name)
	}
	vol := *p.Volume
	if p.Volume.Devices != nil {
		vol.Devices = append([]string(nil), p.Volume.Devices...)
	}
	delete(st.pending, name)
	st.mu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.volumes[name]; exists {
		return fmt.Errorf("卷 %s 已存在，无法恢复", name)
	}
	m.volumes[name] = &vol
	return nil
}

// PurgePending removes a pending soft-delete entry permanently.
// If AllowWipe was set at delete time, runs full DeleteVolume wipe path on devices.
// Otherwise only drops bookkeeping (devices keep filesystem data).
func (m *Manager) PurgePending(name string) error {
	st := m.softState()
	st.mu.Lock()
	p, ok := st.pending[name]
	if !ok || p == nil {
		st.mu.Unlock()
		return fmt.Errorf("待删除卷 %s 不存在", name)
	}
	delete(st.pending, name)
	allowWipe := p.AllowWipe
	force := p.Force
	vol := p.Volume
	st.mu.Unlock()

	if !allowWipe || vol == nil {
		// Soft purge: bookkeeping only (already unregistered from active map).
		return nil
	}
	// Hard purge: re-insert temporarily so DeleteVolume can wipe devices.
	m.mu.Lock()
	m.volumes[name] = vol
	m.mu.Unlock()
	return m.DeleteVolume(name, force)
}
