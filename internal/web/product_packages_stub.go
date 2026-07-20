//go:build !nasd_full

package web

import "log"

// ensureProductManager is a no-op in Core-only builds (product managers not linked).
func (s *Server) ensureProductManager(id string) {
	if id == "" {
		return
	}
	log.Printf("ℹ️  core build: product %q not linked (rebuild with -tags nasd_full)", id)
}

// releaseProductManager is a no-op in Core-only builds.
func (s *Server) releaseProductManager(id string) {}

// registerProductRoutes is a no-op in Core-only builds.
func (s *Server) registerProductRoutes(id string) {}
