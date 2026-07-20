package packageruntime

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nas-os/pkg/hostapi"
)

func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverDir_EmptyAndMissing(t *testing.T) {
	got, err := DiscoverDir("")
	if err != nil || got != nil {
		t.Fatalf("empty dir: got=%v err=%v", got, err)
	}
	got, err = DiscoverDir(filepath.Join(t.TempDir(), "nope"))
	if err != nil || got != nil {
		t.Fatalf("missing: got=%v err=%v", got, err)
	}
}

func TestDiscoverDir_FindsFixture(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "hello-host"), `{
  "id": "com.example.hello-host",
  "name": "Hello Host",
  "version": "1.0.0",
  "trust": "local",
  "capabilities": ["host.sdk"],
  "host_api": "1.0.0",
  "entry": "host-sdk",
  "description": "example third-party package"
}`)
	// Noise file without manifest — ignored.
	_ = os.MkdirAll(filepath.Join(root, "empty"), 0o750)

	found, err := DiscoverDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].ID != "com.example.hello-host" {
		t.Fatalf("found=%+v", found)
	}
	if found[0].Path == "" {
		t.Fatal("path should be set")
	}
}

func TestValidateDiskManifest_RejectsSystemTrustAndHTTPAdmin(t *testing.T) {
	err := ValidateDiskManifest(DiskManifest{ID: "x", Trust: "system"})
	if err == nil {
		t.Fatal("system trust must be rejected")
	}
	err = ValidateDiskManifest(DiskManifest{
		ID: "x", Trust: "community", Capabilities: []string{"http.admin"},
	})
	if err == nil {
		t.Fatal("http.admin must be rejected for community")
	}
}

func TestCommunityLoadLifecycleAndNoHTTPPrivilege(t *testing.T) {
	root := t.TempDir()
	data := t.TempDir()
	writeManifest(t, filepath.Join(root, "hello-host"), `{
  "id": "com.example.hello-host",
  "version": "1.0.0",
  "trust": "local",
  "entry": "host-sdk"
}`)
	manifests, err := DiscoverDir(root)
	if err != nil || len(manifests) != 1 {
		t.Fatalf("discover: %v %+v", err, manifests)
	}

	var logs []string
	host := &hostapi.StaticHost{
		Data:   data,
		Config: t.TempDir(),
		Log:    func(f string, a ...any) { logs = append(logs, f) },
	}
	var httpMounts int
	rt := New(host, func(string, string, http.Handler) { httpMounts++ })

	// Official system package also registered — community must not steal system privileges.
	_ = rt.Register(hostapi.Meta{ID: "voicehub", Trust: hostapi.TrustSystem}, func(hostapi.Host) (hostapi.Package, error) {
		return &stubPkg{meta: hostapi.Meta{ID: "voicehub", Trust: hostapi.TrustSystem}, withHTTP: true}, nil
	})

	reg, err := rt.RegisterDiscovered(manifests)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(reg, []string{"com.example.hello-host"}) {
		t.Fatalf("registered=%v", reg)
	}
	// Discovered but not enabled → nothing loaded.
	if len(rt.LoadedIDs()) != 0 {
		t.Fatal("discover must not auto-enable")
	}

	loaded, unknown, err := rt.Enable(context.Background(), []string{"com.example.hello-host"})
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown) != 0 || !slices.Equal(loaded, []string{"com.example.hello-host"}) {
		t.Fatalf("loaded=%v unknown=%v", loaded, unknown)
	}
	if httpMounts != 0 {
		t.Fatalf("community package must not mount HTTP, mounts=%d", httpMounts)
	}
	// Host SDK path used: marker file under DataPath.
	marker := filepath.Join(data, "community-packages", "com.example.hello-host", "started")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected start marker via Host DataPath: %v", err)
	}
	body, _ := os.ReadFile(marker)
	if !strings.Contains(string(body), "host_api="+hostapi.APIVersion) {
		t.Fatalf("marker should include host API version: %s", body)
	}

	// Capability elevation denied at enable time.
	_ = rt.Register(hostapi.Meta{
		ID: "evil", Trust: hostapi.TrustCommunity,
		Capabilities: []hostapi.Capability{hostapi.CapHTTPAdmin},
	}, func(hostapi.Host) (hostapi.Package, error) {
		return NewHostSDKPackage(hostapi.Meta{
			ID: "evil", Trust: hostapi.TrustCommunity,
			Capabilities: []hostapi.Capability{hostapi.CapHTTPAdmin},
		}, root), nil
	})
	_, _, err = rt.Enable(context.Background(), []string{"evil"})
	if err == nil {
		t.Fatal("http.admin community package must fail enable")
	}
}

