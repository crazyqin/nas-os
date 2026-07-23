//go:build !nasd_full

package web

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"nas-os/internal/arch"
	"nas-os/internal/auth"
	"nas-os/internal/config"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/packageruntime"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProductsLinked reports whether this binary was built with product managers.
// Core-only builds (!nasd_full) return false.
func ProductsLinked() bool { return false }

// ExtensionsLinked reports whether official HTTP extension mounts are compiled in.
func ExtensionsLinked() bool { return false }

// Server is the Core-only web server (no product manager types linked).
type Server struct {
	cfg                     *config.Config
	modules                 []arch.Module
	extHolders              *extensionHolders
	pkgRuntime              *packageruntime.Runtime
	communityDiscovered     []packageruntime.DiskManifest
	runtimeEnabledMu        sync.Mutex
	runtimeEnabled          map[string]struct{}
	httpMountedMu           sync.Mutex
	httpMounted             map[string]struct{}
	packageMountMu          sync.RWMutex
	packageMounted          map[string]struct{}
	productRoutesMu         sync.Mutex
	productRoutesRegistered map[string]struct{}
	productReg              *productRegistry
	adminAPI                *gin.RouterGroup
	clusterMu               sync.Mutex
	clusterServices         any
	clusterBootstrap        func() (any, error)
	engine                  *gin.Engine
	httpSrv                 *http.Server
	lifecycleMu             sync.Mutex
	started                 bool
	stopping                bool
	logger                  *zap.Logger
	storageMgr  *storage.Manager
	userMgr     *users.Manager
	mfaMgr      *auth.MFAManager
	smbMgr      *smb.Manager
	nfsMgr      *nfs.Manager
	networkMgr  *network.Manager
	rbacMgr     *auth.RBACManager
	// downloadMgr kept as constructor param wiring into holders under key "downloadMgr"
	// Optional product slots: always empty on Core; held in h for field-compat tests.
	h *holderBag
}

// NewServer constructs a Core-only HTTP server (identity/storage/sharing/system + packages API).
func NewServer(cfg *config.Config, modules []arch.Module, storMgr *storage.Manager, userMgr *users.Manager, mfaMgr *auth.MFAManager, smbMgr *smb.Manager, nfsMgr *nfs.Manager, netMgr *network.Manager, downloadMgr any, logger *zap.Logger) *Server {
	if cfg == nil {
		cfg = config.Default()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := newEngineWithSecurity()

	if bulk := productBulkSurface(cfg); bulk || len(bootWantProducts(cfg)) > 0 {
		log.Println("ℹ️  core build: packages/products requested but managers are not linked; rebuild with -tags nasd_full")
	} else {
		log.Println("ℹ️  core build: non-Core product managers not linked (nasd_full tag absent)")
	}

	s := &Server{
		cfg:         cfg,
		modules:     append([]arch.Module(nil), modules...),
		engine:      engine,
		logger:      logger,
		productReg:  newProductRegistry(),
		h:           newHolderBag(),
		storageMgr:  storMgr,
		userMgr:     userMgr,
		mfaMgr:      mfaMgr,
		smbMgr:      smbMgr,
		nfsMgr:      nfsMgr,
		networkMgr:  netMgr,
		rbacMgr:     auth.NewRBACManager(),
	}
	s.setHolder("downloadMgr", downloadMgr)
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	api := s.registerCorePublicAndAdminGroups()
	s.registerConfiguredExtensions(api)
	s.registerCoreIdentityAndDocs(api)
}

// Start starts the HTTP server (Core build has no product background workers).
func (s *Server) Start(addr string) error {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return nil
	}
	if s.started {
		s.lifecycleMu.Unlock()
		return errors.New("web server already started")
	}
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	s.started = true
	httpSrv := s.httpSrv
	s.lifecycleMu.Unlock()

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Stop shuts down the HTTP server.
func (s *Server) Stop() error {
	s.lifecycleMu.Lock()
	if s.stopping {
		s.lifecycleMu.Unlock()
		return nil
	}
	s.stopping = true
	httpSrv := s.httpSrv
	s.lifecycleMu.Unlock()

	if httpSrv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpSrv.Shutdown(ctx)
}

