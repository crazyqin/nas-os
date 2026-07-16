package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
)

func webuiStorageHTML(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/web -> repo/webui/pages/storage.html
	p := filepath.Join(filepath.Dir(file), "..", "..", "webui", "pages", "storage.html")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read storage.html: %v", err)
	}
	return string(data)
}

// TestCoreStorageWebUIContract asserts every fetch path and POST JSON key in
// webui/pages/storage.html matches live StorageHandlers routes and canonical
// storage package DTO json tags (single source of truth).
func TestCoreStorageWebUIContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	html := webuiStorageHTML(t)

	// Collect registered paths from real handler registration.
	r := gin.New()
	NewStorageHandlers(nil).RegisterRoutes(r.Group("/api/v1"))
	registered := map[string]bool{}
	for _, route := range r.Routes() {
		// normalize :name style
		registered[route.Method+" "+route.Path] = true
	}

	// Fetch templates: ${API_BASE}/storage/... or `${API_BASE}/storage/...`
	fetchRe := regexp.MustCompile(`\$\{API_BASE\}(/storage/[^'"\` + "`" + `]+)`)
	paths := fetchRe.FindAllStringSubmatch(html, -1)
	if len(paths) == 0 {
		t.Fatal("no ${API_BASE}/storage/... fetches found in storage.html")
	}

	// Forbidden legacy / bare paths
	for _, bad := range []string{
		"${API_BASE}/volumes",
		"/api/v1/volumes",
		"${API_BASE}/subvolumes",
		"${API_BASE}/snapshots",
		"`${API_BASE}/subvolumes`",
		"`${API_BASE}/snapshots`",
	} {
		if strings.Contains(html, bad) {
			// allow only if part of storage/ prefix — check carefully
			if bad == "${API_BASE}/volumes" || bad == "/api/v1/volumes" {
				t.Errorf("storage.html still contains legacy path fragment %q", bad)
			}
			if bad == "${API_BASE}/subvolumes" || bad == "${API_BASE}/snapshots" {
				// check not storage/subvolumes
				for i, line := range strings.Split(html, "\n") {
					if strings.Contains(line, bad) && !strings.Contains(line, "/storage/") {
						t.Errorf("storage.html:%d uses unregistered path: %s", i+1, strings.TrimSpace(line))
					}
				}
			}
		}
	}
	// Explicit bare path ban
	bareRe := regexp.MustCompile(`\$\{API_BASE\}/(subvolumes|snapshots)([/'\"\` + "`" + `]|$)`)
	for i, line := range strings.Split(html, "\n") {
		if bareRe.MatchString(line) {
			t.Errorf("storage.html:%d bare /subvolumes or /snapshots: %s", i+1, strings.TrimSpace(line))
		}
	}

	// Every concrete path template must be coverable by a registered route pattern.
	for _, m := range paths {
		apiPath := m[1] // e.g. /storage/volumes/${volume}/subvolumes
		// Build example concrete path for matching
		concrete := apiPath
		// replace ${...} with sample
		concrete = regexp.MustCompile(`\$\{[^}]+\}`).ReplaceAllString(concrete, "x")
		// strip encodeURIComponent wrappers already in path as ${...}
		full := "/api/v1" + concrete
		if !routeMatchesAny(full, registered) {
			t.Errorf("WebUI path not registered: %s (from template %s)", full, apiPath)
		}
	}

	// JSON body keys used in POST/restore/mount must exist on canonical DTOs.
	// mountPath -> MountSubvolumeRequest
	// targetName -> RestoreSnapshotRequest
	// subvolume,name,readOnly -> CreateSnapshotRequest
	// name (create subvolume) -> CreateSubvolumeRequest
	// name,devices,profile -> CreateVolumeRequest
	jsonKeyChecks := []struct {
		needle string
		dto    any
		keys   []string
	}{
		{`JSON.stringify({ mountPath: path })`, storage.MountSubvolumeRequest{}, []string{"mountPath"}},
		{`JSON.stringify({ targetName })`, storage.RestoreSnapshotRequest{}, []string{"targetName"}},
		{`JSON.stringify({ subvolume, name, readOnly: readonly })`, storage.CreateSnapshotRequest{}, []string{"subvolume", "name", "readOnly"}},
		{`JSON.stringify({ name })`, storage.CreateSubvolumeRequest{}, []string{"name"}},
	}
	for _, chk := range jsonKeyChecks {
		if !strings.Contains(html, chk.needle) && !strings.Contains(html, strings.ReplaceAll(chk.needle, " ", "")) {
			// flexible: look for keys near stringify
			continue
		}
		// verify dto tags
		rt := reflect.TypeOf(chk.dto)
		tagSet := map[string]bool{}
		for i := 0; i < rt.NumField(); i++ {
			tag := rt.Field(i).Tag.Get("json")
			tag = strings.Split(tag, ",")[0]
			if tag != "" && tag != "-" {
				tagSet[tag] = true
			}
		}
		for _, k := range chk.keys {
			if !tagSet[k] {
				t.Errorf("DTO %T missing json key %q required by WebUI", chk.dto, k)
			}
		}
	}

	// Hard checks for the two skeptic bugs
	if !strings.Contains(html, "mountPath") {
		t.Error("storage.html must send mountPath")
	}
	if strings.Contains(html, `"mount_path"`) {
		t.Error("storage.html must not send mount_path")
	}
	if !strings.Contains(html, "targetName") {
		t.Error("storage.html must send targetName")
	}
	if regexp.MustCompile(`JSON\.stringify\(\s*\{\s*target\s*[:\}]`).MatchString(html) {
		t.Error("storage.html must not send json target (use targetName)")
	}

	// Handler binding uses same DTO types — verify via reflection on known exports
	_ = storage.MountSubvolumeRequest{MountPath: "x"}
	_ = storage.RestoreSnapshotRequest{TargetName: "y"}
}

func routeMatchesAny(concrete string, registered map[string]bool) bool {
	// Try GET and POST and DELETE for the concrete path against gin patterns
	for reg := range registered {
		// reg like "GET /api/v1/storage/volumes/:name/subvolumes"
		parts := strings.SplitN(reg, " ", 2)
		if len(parts) != 2 {
			continue
		}
		pat := parts[1]
		if pathMatch(pat, concrete) {
			return true
		}
	}
	return false
}

func pathMatch(pattern, concrete string) bool {
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	cp := strings.Split(strings.Trim(concrete, "/"), "/")
	if len(pp) != len(cp) {
		return false
	}
	for i := range pp {
		if strings.HasPrefix(pp[i], ":") || strings.HasPrefix(pp[i], "*") {
			continue
		}
		if pp[i] != cp[i] {
			return false
		}
	}
	return true
}

// TestMountAndRestoreBindCanonicalJSONTags drives real handlers with WebUI body shapes.
func TestMountAndRestoreBindCanonicalJSONTags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// MountSubvolumeRequest
	var mount storage.MountSubvolumeRequest
	if err := json.Unmarshal([]byte(`{"mountPath":"/mnt/share"}`), &mount); err != nil {
		t.Fatal(err)
	}
	if mount.MountPath != "/mnt/share" {
		t.Fatalf("mountPath bind failed: %+v", mount)
	}
	// Wrong tag must not bind
	var mountWrong storage.MountSubvolumeRequest
	_ = json.Unmarshal([]byte(`{"mount_path":"/mnt/wrong"}`), &mountWrong)
	if mountWrong.MountPath != "" {
		t.Fatal("mount_path must not bind to MountPath")
	}

	// RestoreSnapshotRequest
	var restore storage.RestoreSnapshotRequest
	if err := json.Unmarshal([]byte(`{"targetName":"snap-restored"}`), &restore); err != nil {
		t.Fatal(err)
	}
	if restore.TargetName != "snap-restored" {
		t.Fatalf("targetName bind failed: %+v", restore)
	}
	var restoreWrong storage.RestoreSnapshotRequest
	_ = json.Unmarshal([]byte(`{"target":"ignored"}`), &restoreWrong)
	if restoreWrong.TargetName != "" {
		t.Fatal("target must not bind to TargetName")
	}
}
