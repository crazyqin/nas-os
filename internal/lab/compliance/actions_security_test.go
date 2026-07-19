package compliance

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// projectRoot returns an absolute path to a file/dir under the project root,
// resolving correctly regardless of the test working directory.
func projectRoot(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// This file lives at <root>/internal/lab/compliance/actions_security_test.go
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	return filepath.Join(append([]string{root}, elems...)...)
}

func TestGitHubActionsSecurityScanWorkflowBaseline(t *testing.T) {
	workflowPath := projectRoot(t, ".github", "workflows", "security-scan.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read security scan workflow: %v", err)
	}

	yml := string(content)
	requiredSnippets := []string{
		"name: Security Scan",
		"permissions:",
		"contents: read",
		"security-events: write",
		"go install github.com/securego/gosec/v2/cmd/gosec@latest",
		"go install golang.org/x/vuln/cmd/govulncheck@latest",
		"gosec -fmt=sarif -out=gosec.sarif",
		"govulncheck ./...",
		"github/codeql-action/upload-sarif",
		"actions/upload-artifact",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(yml, snippet) {
			t.Errorf("security-scan.yml should contain %q", snippet)
		}
	}

	forbiddenSnippets := []string{
		"pull_request_target:",
		"contents: write",
		"packages: write",
		"secrets.",
	}
	for _, snippet := range forbiddenSnippets {
		if strings.Contains(yml, snippet) {
			t.Errorf("security-scan.yml should not contain high-risk snippet %q", snippet)
		}
	}
}

func TestGitHubActionsDoNotUsePullRequestTarget(t *testing.T) {
	workflowDir := projectRoot(t, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("read workflow dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(workflowDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read workflow %s: %v", entry.Name(), err)
		}
		if strings.Contains(string(content), "pull_request_target:") {
			t.Errorf("%s uses pull_request_target; prefer pull_request with least-privilege permissions", entry.Name())
		}
	}
}

func TestGitHubActionsPinThirdPartyActionsToVersion(t *testing.T) {
	workflowDir := projectRoot(t, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("read workflow dir: %v", err)
	}

	usesRe := regexp.MustCompile(`(?m)^\s*uses:\s*([^\s#]+)`) // e.g. actions/checkout@v5
	unpinned := regexp.MustCompile(`@(main|master|HEAD)$`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(workflowDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read workflow %s: %v", entry.Name(), err)
		}
		for _, match := range usesRe.FindAllStringSubmatch(string(content), -1) {
			ref := match[1]
			if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
				continue
			}
			if !strings.Contains(ref, "@") {
				t.Errorf("%s has unversioned action reference %q", entry.Name(), ref)
			}
			if unpinned.MatchString(ref) {
				t.Errorf("%s pins action %q to a moving branch", entry.Name(), ref)
			}
		}
	}
}
