//go:build !nasd_full

package web

import (
	"log"

	"nas-os/internal/packageruntime"

	"github.com/gin-gonic/gin"
)

// registerSystemPackageCatalog is a no-op in Core builds: HTTP extension
// implementations are not linked. Rebuild with -tags nasd_full to enable them.
func (s *Server) registerSystemPackageCatalog(rt *packageruntime.Runtime, api *gin.RouterGroup) error {
	log.Println("ℹ️  core build: HTTP extensions not linked (need -tags nasd_full for voicehub/netdiag/…)")
	return nil
}

// MountTableHTTPExtensionIDs returns nil in Core builds (no mount table linked).
func MountTableHTTPExtensionIDs() []string {
	return nil
}

func (s *Server) httpRoutesMounted(id string) bool {
	if s == nil {
		return false
	}
	s.httpMountedMu.Lock()
	defer s.httpMountedMu.Unlock()
	_, ok := s.httpMounted[id]
	return ok
}

func (s *Server) markHTTPRoutesMounted(id string) {
	if s == nil {
		return
	}
	s.httpMountedMu.Lock()
	defer s.httpMountedMu.Unlock()
	if s.httpMounted == nil {
		s.httpMounted = make(map[string]struct{})
	}
	s.httpMounted[id] = struct{}{}
}
