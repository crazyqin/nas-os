package web

import "sync"

// holderBag stores optional/product/bulk managers without bloating Server with
// 90+ typed fields. Keys are stable field names (e.g. "dockerMgr").
type holderBag struct {
	mu sync.RWMutex
	m  map[string]any
}

func newHolderBag() *holderBag {
	return &holderBag{m: make(map[string]any)}
}

// setHolder stores or clears an optional manager. nil deletes the key.
func (s *Server) setHolder(key string, v any) {
	if s == nil || key == "" {
		return
	}
	if s.h == nil {
		s.h = newHolderBag()
	}
	s.h.mu.Lock()
	defer s.h.mu.Unlock()
	if s.h.m == nil {
		s.h.m = make(map[string]any)
	}
	if v == nil {
		delete(s.h.m, key)
		return
	}
	s.h.m[key] = v
}

// hasHolder reports a non-nil holder for key.
func (s *Server) hasHolder(key string) bool {
	if s == nil || s.h == nil {
		return false
	}
	s.h.mu.RLock()
	defer s.h.mu.RUnlock()
	v, ok := s.h.m[key]
	return ok && v != nil
}

// holderAs returns a typed optional manager, or the zero value if missing/wrong type.
func holderAs[T any](s *Server, key string) T {
	var zero T
	if s == nil || s.h == nil {
		return zero
	}
	s.h.mu.RLock()
	defer s.h.mu.RUnlock()
	v, ok := s.h.m[key]
	if !ok || v == nil {
		return zero
	}
	t, ok := v.(T)
	if !ok {
		return zero
	}
	return t
}

// holderKeys returns a snapshot of non-nil holder keys (tests / diagnostics).
func (s *Server) holderKeys() []string {
	if s == nil || s.h == nil {
		return nil
	}
	s.h.mu.RLock()
	defer s.h.mu.RUnlock()
	out := make([]string, 0, len(s.h.m))
	for k, v := range s.h.m {
		if v != nil {
			out = append(out, k)
		}
	}
	return out
}
