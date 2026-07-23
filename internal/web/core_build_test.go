//go:build !nasd_full

package web

import (
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCoreBuild_ProductsNotLinked(t *testing.T) {
	if ProductsLinked() {
		t.Fatal("core build must report ProductsLinked()==false")
	}
	if ExtensionsLinked() {
		t.Fatal("core build must report ExtensionsLinked()==false")
	}
	gin.SetMode(gin.TestMode)
	s := NewServer(config.Default(), nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	if s == nil {
		t.Fatal("nil server")
	}
	if s.hasHolder("dockerMgr") || s.hasHolder("photosMgr") || s.hasHolder("vmMgr") || s.hasHolder("trashMgr") {
		t.Fatal("product managers must be nil in core build")
	}
}
