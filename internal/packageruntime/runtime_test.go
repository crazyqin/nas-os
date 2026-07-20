package packageruntime

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"

	"nas-os/pkg/hostapi"
)

type stubPkg struct {
	meta     hostapi.Meta
	inits    int32
	starts   int32
	stops    int32
	mounts   int32
	initErr  error
	startErr error
	withHTTP bool
}

func (p *stubPkg) Meta() hostapi.Meta { return p.meta }
func (p *stubPkg) Init(context.Context, hostapi.Host) error {
	atomic.AddInt32(&p.inits, 1)
	return p.initErr
}
func (p *stubPkg) Start(context.Context) error {
	atomic.AddInt32(&p.starts, 1)
	return p.startErr
}
func (p *stubPkg) Stop(context.Context) error {
	atomic.AddInt32(&p.stops, 1)
	return nil
}
func (p *stubPkg) Health(context.Context) error { return nil }
func (p *stubPkg) MountHTTP(register func(method, path string, h http.Handler)) error {
	atomic.AddInt32(&p.mounts, 1)
	if register != nil {
		register(http.MethodGet, "/"+p.meta.ID+"/ping", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	}
	return nil
}

func testHost() *hostapi.StaticHost {
	return &hostapi.StaticHost{
		Data:   "/data",
		Config: "/cfg",
	}
}

func TestRuntimeEnableDefaultEmpty(t *testing.T) {
	rt := New(testHost(), nil)
	_ = rt.Register(hostapi.Meta{ID: "voicehub", Trust: hostapi.TrustSystem}, func(hostapi.Host) (hostapi.Package, error) {
		return &stubPkg{meta: hostapi.Meta{ID: "voicehub", Trust: hostapi.TrustSystem}}, nil
	})
	loaded, unknown, err := rt.Enable(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 || len(unknown) != 0 {
		t.Fatalf("loaded=%v unknown=%v", loaded, unknown)
	}
	if len(rt.LoadedIDs()) != 0 {
		t.Fatal("nothing should be loaded")
	}
}

func TestRuntimeEnableKnownAndUnknown(t *testing.T) {
	var mounted bool
	rt := New(testHost(), func(method, path string, h http.Handler) {
		mounted = true
		if method != http.MethodGet || path != "/voicehub/ping" {
			t.Fatalf("unexpected mount %s %s", method, path)
		}
	})
	pkg := &stubPkg{meta: hostapi.Meta{ID: "voicehub", Trust: hostapi.TrustSystem}, withHTTP: true}
	_ = rt.Register(pkg.meta, func(hostapi.Host) (hostapi.Package, error) {
		// Return HTTP-capable package via embedding trick: use concrete with MountHTTP
		return pkg, nil
	})

	// Re-register with type that has MountHTTP — stubPkg has MountHTTP so interface satisfied.
	loaded, unknown, err := rt.Enable(context.Background(), []string{"voicehub", "not-real", "VoiceHub"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(loaded, []string{"voicehub"}) {
		t.Fatalf("loaded=%v", loaded)
	}
	if !slices.Equal(unknown, []string{"not-real"}) {
		t.Fatalf("unknown=%v", unknown)
	}
	if atomic.LoadInt32(&pkg.inits) != 1 || atomic.LoadInt32(&pkg.starts) != 1 {
		t.Fatalf("init/start counts init=%d start=%d", pkg.inits, pkg.starts)
	}
	if atomic.LoadInt32(&pkg.mounts) != 1 || !mounted {
		t.Fatal("HTTP mount expected for system package")
	}
	if !slices.Equal(rt.LoadedIDs(), []string{"voicehub"}) {
		t.Fatalf("LoadedIDs=%v", rt.LoadedIDs())
	}
}

func TestRuntimeCommunitySkipsHTTPMount(t *testing.T) {
	var httpCalls int
	rt := New(testHost(), func(string, string, http.Handler) { httpCalls++ })
	pkg := &stubPkg{meta: hostapi.Meta{ID: "com.example.x", Trust: hostapi.TrustCommunity}}
	_ = rt.Register(pkg.meta, func(hostapi.Host) (hostapi.Package, error) { return pkg, nil })

	loaded, _, err := rt.Enable(context.Background(), []string{"com.example.x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded=%v", loaded)
	}
	if atomic.LoadInt32(&pkg.mounts) != 0 || httpCalls != 0 {
		t.Fatal("community package must not mount HTTP in Stage 2")
	}
}

func TestRuntimeTrustDenied(t *testing.T) {
	h := testHost()
	h.MinTrust = hostapi.TrustSystem
	rt := New(h, nil)
	pkg := &stubPkg{meta: hostapi.Meta{ID: "local.pkg", Trust: hostapi.TrustLocal}}
	_ = rt.Register(pkg.meta, func(hostapi.Host) (hostapi.Package, error) { return pkg, nil })

	_, _, err := rt.Enable(context.Background(), []string{"local.pkg"})
	if err == nil {
		t.Fatal("expected trust denial error")
	}
	if len(rt.LoadedIDs()) != 0 {
		t.Fatal("must not load")
	}
}

func TestRuntimeStopAllReverse(t *testing.T) {
	rt := New(testHost(), func(string, string, http.Handler) {})
	var stopOrder []string
	makePkg := func(id string) *stopOrderPkg {
		return &stopOrderPkg{
			stubPkg:   stubPkg{meta: hostapi.Meta{ID: id, Trust: hostapi.TrustSystem}},
			stopOrder: &stopOrder,
		}
	}
	a, b := makePkg("a"), makePkg("b")
	_ = rt.Register(a.meta, func(hostapi.Host) (hostapi.Package, error) { return a, nil })
	_ = rt.Register(b.meta, func(hostapi.Host) (hostapi.Package, error) { return b, nil })
	if _, _, err := rt.Enable(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if err := rt.StopAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stopOrder, []string{"b", "a"}) {
		t.Fatalf("stop order %v", stopOrder)
	}
}

type stopOrderPkg struct {
	stubPkg
	stopOrder *[]string
}

func (p *stopOrderPkg) Stop(ctx context.Context) error {
	*p.stopOrder = append(*p.stopOrder, p.meta.ID)
	return p.stubPkg.Stop(ctx)
}

func TestRuntimeInitFailureNoLoad(t *testing.T) {
	rt := New(testHost(), nil)
	pkg := &stubPkg{
		meta:    hostapi.Meta{ID: "bad", Trust: hostapi.TrustSystem},
		initErr: errors.New("boom"),
	}
	_ = rt.Register(pkg.meta, func(hostapi.Host) (hostapi.Package, error) { return pkg, nil })
	_, _, err := rt.Enable(context.Background(), []string{"bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(rt.LoadedIDs()) != 0 {
		t.Fatal("failed package must not be loaded")
	}
}
