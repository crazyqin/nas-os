package application

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nas-os/internal/arch"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// TestProductionWebDoesNotImportLab fails if non-test web packages import internal/lab.
func TestProductionWebDoesNotImportLab(t *testing.T) {
	root := repoRoot(t)
	webDir := filepath.Join(root, "internal", "web")
	fset := token.NewFileSet()
	var offenders []string
	err := filepath.Walk(webDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(p, "nas-os/internal/lab/") {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel+": "+p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk web: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("production web must not import lab packages:\n%s", strings.Join(offenders, "\n"))
	}
}

// TestNasdDependencyGraphExcludesLab uses go list -deps so transitive lab coupling
// (e.g. monitor → lab/reports) cannot hide behind direct-import-only scans.
func TestNasdDependencyGraphExcludesLab(t *testing.T) {
	root := repoRoot(t)
	for _, target := range []string{"./cmd/nasd", "./internal/web", "./internal/monitor"} {
		cmd := exec.Command("go", "list", "-deps", target)
		cmd.Dir = root
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("go list -deps %s: %v\n%s", target, err, stderr.String())
		}
		var lab []string
		for _, line := range strings.Split(stdout.String(), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "nas-os/internal/lab/") || line == "nas-os/internal/lab" {
				lab = append(lab, line)
			}
		}
		if len(lab) > 0 {
			t.Fatalf("%s dependency graph still includes lab packages:\n%s", target, strings.Join(lab, "\n"))
		}
	}
}

// TestCoreCatalogExactlyFive locks Core membership.
func TestCoreCatalogExactlyFive(t *testing.T) {
	catalog := ModuleCatalogSnapshot()
	var cores []string
	for name, tier := range catalog {
		if tier == arch.ModuleTierCore {
			cores = append(cores, name)
		}
	}
	want := map[string]bool{
		moduleIdentity: true, moduleStorage: true, moduleNetwork: true,
		moduleSharing: true, moduleSystem: true,
	}
	if len(cores) != 5 {
		t.Fatalf("core count %d want 5: %v", len(cores), cores)
	}
	for _, c := range cores {
		if !want[c] {
			t.Fatalf("unexpected core %q", c)
		}
	}
}

// TestTopLevelInternalAllowlistFrozen fails when new top-level business packages appear
// under internal/ outside lab, extensions, and the frozen allowlist snapshot.
func TestTopLevelInternalAllowlistFrozen(t *testing.T) {
	root := repoRoot(t)
	allowPath := filepath.Join(root, "internal", "application", "toplevel_allowlist.txt")
	data, err := os.ReadFile(allowPath)
	if err != nil {
		t.Fatalf("read allowlist: %v", err)
	}
	allowed := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowed[line] = true
	}
	internalDir := filepath.Join(root, "internal")
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		t.Fatalf("readdir internal: %v", err)
	}
	var unexpected []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "lab" || name == "extensions" {
			continue
		}
		if !allowed[name] {
			unexpected = append(unexpected, name)
		}
	}
	if len(unexpected) > 0 {
		t.Fatalf("new top-level internal packages not on allowlist (add intentionally or put under lab/extensions): %v", unexpected)
	}
}

// TestDeprecatedLabPackagesNotAtTopLevel samples packages that must stay demoted.
func TestDeprecatedLabPackagesNotAtTopLevel(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{
		"activebackup", "filemanager", "media", "selfheal", "ztna",
		"benchmarkpro", "containerpro", "smartrecipe",
	} {
		top := filepath.Join(root, "internal", name)
		if st, err := os.Stat(top); err == nil && st.IsDir() {
			t.Fatalf("%s must not reappear at internal top level", name)
		}
	}
}
