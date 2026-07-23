package web

import "sync"

// productEntry is one runtime-managed product slot (catalog recommended products).
// Typed fields on Server remain mirrors for handlers; the registry is the lifecycle SSOT.
type productEntry struct {
	id     string
	holder any
	stop   func() // optional teardown; may be nil
}

// productRegistry holds product managers by catalog id for ensure/release.
// Concurrent access is protected by mu.
type productRegistry struct {
	mu   sync.Mutex
	byID map[string]*productEntry
}

func newProductRegistry() *productRegistry {
	return &productRegistry{byID: make(map[string]*productEntry)}
}

// put registers or replaces a product entry. Previous stop (if any) is invoked.
func (r *productRegistry) put(id string, holder any, stop func()) {
	if r == nil || id == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = make(map[string]*productEntry)
	}
	if old, ok := r.byID[id]; ok && old != nil && old.stop != nil {
		old.stop()
	}
	r.byID[id] = &productEntry{id: id, holder: holder, stop: stop}
}

// get returns the holder for id, or nil.
func (r *productRegistry) get(id string) any {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.byID[id]; ok && e != nil {
		return e.holder
	}
	return nil
}

// remove stops and drops the entry. Returns true if something was removed.
func (r *productRegistry) remove(id string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	e, ok := r.byID[id]
	if !ok || e == nil {
		r.mu.Unlock()
		return false
	}
	delete(r.byID, id)
	stop := e.stop
	r.mu.Unlock()
	if stop != nil {
		stop()
	}
	return true
}

// drop removes the entry without invoking stop (caller owns teardown).
func (r *productRegistry) drop(id string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.byID, id)
	r.mu.Unlock()
}

// has reports whether id is registered with a non-nil holder.
func (r *productRegistry) has(id string) bool {
	return r.get(id) != nil
}

// ids returns a snapshot of registered product ids.
func (r *productRegistry) ids() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.byID))
	for id, e := range r.byID {
		if e != nil && e.holder != nil {
			out = append(out, id)
		}
	}
	return out
}

// clearAll removes every entry (Stop on each). Used on server shutdown paths.
func (r *productRegistry) clearAll() {
	if r == nil {
		return
	}
	r.mu.Lock()
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	for _, id := range ids {
		r.remove(id)
	}
}
