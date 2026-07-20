// Package packageruntime is the ADR-0001 Stage-2 unified package runtime:
// catalog → trust check → Init → Start → (optional HTTP mount) → Stop.
//
// Enablement lists still come from config.ResolvePackages(); this package only
// loads IDs that are both requested and present in the catalog.
package packageruntime

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"nas-os/pkg/hostapi"
)

// Factory constructs a package instance (no Start side effects).
type Factory func(host hostapi.Host) (hostapi.Package, error)

// Entry is a catalog registration.
type Entry struct {
	Meta    hostapi.Meta
	Factory Factory
}

// Status is a loaded package's runtime view.
type Status struct {
	Meta    hostapi.Meta `json:"meta"`
	Loaded  bool         `json:"loaded"`
	Healthy bool         `json:"healthy"`
	Error   string       `json:"error,omitempty"`
}

// HTTPRegister mounts a handler on the admin API (provided by the host adapter).
type HTTPRegister func(method, path string, h http.Handler)

// Runtime owns catalog and live package instances.
type Runtime struct {
	mu       sync.RWMutex
	host     hostapi.Host
	catalog  map[string]Entry
	loaded   map[string]hostapi.Package
	order    []string // start order for reverse stop
	httpReg  HTTPRegister
	lastErrs map[string]string
}

// New creates an empty runtime bound to host. httpReg may be nil if no package
// needs MountHTTP (or will be set later via SetHTTPRegister before Enable).
func New(host hostapi.Host, httpReg HTTPRegister) *Runtime {
	return &Runtime{
		host:     host,
		catalog:  make(map[string]Entry),
		loaded:   make(map[string]hostapi.Package),
		lastErrs: make(map[string]string),
		httpReg:  httpReg,
	}
}

// SetHTTPRegister sets the HTTP mount callback (e.g. gin adapter).
func (r *Runtime) SetHTTPRegister(reg HTTPRegister) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.httpReg = reg
}

// Register adds or replaces a catalog entry. ID is normalized to lower-case.
func (r *Runtime) Register(meta hostapi.Meta, factory Factory) error {
	if r == nil {
		return fmt.Errorf("runtime is nil")
	}
	if factory == nil {
		return fmt.Errorf("factory is nil")
	}
	id := normalizeID(meta.ID)
	if id == "" {
		return fmt.Errorf("package id is empty")
	}
	if meta.Trust == "" {
		meta.Trust = hostapi.TrustSystem
	}
	meta.ID = id

	r.mu.Lock()
	defer r.mu.Unlock()
	r.catalog[id] = Entry{Meta: meta, Factory: factory}
	return nil
}

// CatalogIDs returns sorted registered package IDs.
func (r *Runtime) CatalogIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.catalog))
	for id := range r.catalog {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Known returns true if id is in the catalog.
func (r *Runtime) Known(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.catalog[normalizeID(id)]
	return ok
}

// LoadedIDs returns successfully started package IDs in start order.
func (r *Runtime) LoadedIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Statuses returns catalog packages with load/health snapshot.
func (r *Runtime) Statuses(ctx context.Context) []Status {
	r.mu.RLock()
	ids := make([]string, 0, len(r.catalog))
	for id := range r.catalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	r.mu.RUnlock()

	out := make([]Status, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.statusOne(ctx, id))
	}
	return out
}

func (r *Runtime) statusOne(ctx context.Context, id string) Status {
	r.mu.RLock()
	entry, inCat := r.catalog[id]
	pkg, loaded := r.loaded[id]
	errStr := r.lastErrs[id]
	r.mu.RUnlock()

	st := Status{Loaded: loaded, Error: errStr}
	if inCat {
		st.Meta = entry.Meta
	} else {
		st.Meta = hostapi.Meta{ID: id}
	}
	if loaded && pkg != nil {
		if err := pkg.Health(ctx); err != nil {
			st.Healthy = false
			if st.Error == "" {
				st.Error = err.Error()
			}
		} else {
			st.Healthy = true
		}
	}
	return st
}

// Enable loads each requested ID that exists in the catalog.
// Unknown IDs are skipped (returned in unknown). Already loaded IDs are skipped.
// Order follows the request list after normalization/dedupe.
func (r *Runtime) Enable(ctx context.Context, requested []string) (loaded []string, unknown []string, err error) {
	if r == nil {
		return nil, nil, fmt.Errorf("runtime is nil")
	}
	if r.host == nil {
		return nil, nil, fmt.Errorf("host is nil")
	}

	ids := dedupeNormalize(requested)
	var firstErr error
	for _, id := range ids {
		r.mu.RLock()
		entry, ok := r.catalog[id]
		_, already := r.loaded[id]
		r.mu.RUnlock()

		if !ok {
			unknown = append(unknown, id)
			continue
		}
		if already {
			loaded = append(loaded, id)
			continue
		}

		if err := r.enableOne(ctx, entry); err != nil {
			r.mu.Lock()
			r.lastErrs[id] = err.Error()
			r.mu.Unlock()
			if firstErr == nil {
				firstErr = fmt.Errorf("enable %s: %w", id, err)
			}
			r.host.Logf("package %q enable failed: %v", id, err)
			continue
		}
		loaded = append(loaded, id)
		r.host.Logf("package enabled: %s (trust=%s)", id, entry.Meta.Trust)
	}
	return loaded, unknown, firstErr
}

func (r *Runtime) enableOne(ctx context.Context, entry Entry) error {
	if !r.host.Allows(entry.Meta.Trust) {
		return fmt.Errorf("trust %s not allowed by host", entry.Meta.Trust)
	}
	// Capability gate: community/local cannot receive system-only capabilities.
	if entry.Meta.Trust.Rank() < hostapi.TrustSystem.Rank() {
		for _, cap := range entry.Meta.Capabilities {
			if !cap.AllowsCommunity() {
				return fmt.Errorf("capability %q denied for trust %s", cap, entry.Meta.Trust)
			}
		}
	}
	// Community/local may load lifecycle but cannot mount unrestricted admin HTTP.
	pkg, err := entry.Factory(r.host)
	if err != nil {
		return fmt.Errorf("factory: %w", err)
	}
	if pkg == nil {
		return fmt.Errorf("factory returned nil")
	}
	if err := pkg.Init(ctx, r.host); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := pkg.Start(ctx); err != nil {
		_ = pkg.Stop(ctx)
		return fmt.Errorf("start: %w", err)
	}

	if mounter, ok := pkg.(hostapi.HTTPMounter); ok {
		if entry.Meta.Trust.Rank() < hostapi.TrustSystem.Rank() {
			// Community/local: no HTTP mount in Stage 2.
			r.host.Logf("package %s: HTTP mount skipped (trust=%s)", entry.Meta.ID, entry.Meta.Trust)
		} else {
			r.mu.RLock()
			reg := r.httpReg
			r.mu.RUnlock()
			if reg == nil {
				return fmt.Errorf("HTTP mount required but no register callback")
			}
			if err := mounter.MountHTTP(reg); err != nil {
				_ = pkg.Stop(ctx)
				return fmt.Errorf("mount http: %w", err)
			}
		}
	}

	r.mu.Lock()
	r.loaded[entry.Meta.ID] = pkg
	r.order = append(r.order, entry.Meta.ID)
	delete(r.lastErrs, entry.Meta.ID)
	r.mu.Unlock()
	return nil
}

// Disable stops a single loaded package and removes it from the loaded set.
// Unknown or not-loaded IDs return nil (idempotent). Catalog entry remains.
func (r *Runtime) Disable(ctx context.Context, id string) error {
	if r == nil {
		return fmt.Errorf("runtime is nil")
	}
	id = normalizeID(id)
	if id == "" {
		return fmt.Errorf("package id is empty")
	}
	r.mu.Lock()
	pkg, ok := r.loaded[id]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	delete(r.loaded, id)
	// Remove from start order.
	order := r.order[:0]
	for _, x := range r.order {
		if x != id {
			order = append(order, x)
		}
	}
	r.order = order
	r.mu.Unlock()

	if pkg != nil {
		if err := pkg.Stop(ctx); err != nil {
			return fmt.Errorf("stop %s: %w", id, err)
		}
	}
	if r.host != nil {
		r.host.Logf("package disabled: %s", id)
	}
	return nil
}

// IsLoaded reports whether id is currently started.
func (r *Runtime) IsLoaded(id string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.loaded[normalizeID(id)]
	return ok
}

// StopAll stops loaded packages in reverse start order.
func (r *Runtime) StopAll(ctx context.Context) error {
	r.mu.Lock()
	order := make([]string, len(r.order))
	copy(order, r.order)
	r.mu.Unlock()

	var errs []string
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		r.mu.Lock()
		pkg := r.loaded[id]
		delete(r.loaded, id)
		r.mu.Unlock()
		if pkg == nil {
			continue
		}
		if err := pkg.Stop(ctx); err != nil {
			errs = append(errs, id+": "+err.Error())
		}
	}
	r.mu.Lock()
	r.order = nil
	r.mu.Unlock()
	if len(errs) > 0 {
		return fmt.Errorf("stop: %s", strings.Join(errs, "; "))
	}
	return nil
}

func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func dedupeNormalize(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, raw := range in {
		id := normalizeID(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
