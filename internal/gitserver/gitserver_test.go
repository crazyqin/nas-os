package gitserver

import (
	"os"
	"testing"
)

func TestCreateRepo(t *testing.T) {
	dir, err := os.MkdirTemp("", "gitserver-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	svc := NewService(dir, 1024)

	repo, err := svc.CreateRepo(CreateRepoRequest{
		Name:    "test-repo",
		Owner:   "admin",
		QuotaMB: 100,
	})
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if repo.Name != "test-repo" {
		t.Errorf("expected name test-repo, got %s", repo.Name)
	}
	if repo.Owner != "admin" {
		t.Errorf("expected owner admin, got %s", repo.Owner)
	}
	if repo.Visibility != "private" {
		t.Errorf("expected private visibility, got %s", repo.Visibility)
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("expected main branch, got %s", repo.DefaultBranch)
	}

	// duplicate
	_, err = svc.CreateRepo(CreateRepoRequest{Name: "test-repo", Owner: "admin"})
	if err == nil {
		t.Error("expected duplicate error")
	}
}

func TestListRepos(t *testing.T) {
	dir, err := os.MkdirTemp("", "gitserver-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	svc := NewService(dir, 1024)
	_, _ = svc.CreateRepo(CreateRepoRequest{Name: "r1", Owner: "alice"})
	_, _ = svc.CreateRepo(CreateRepoRequest{Name: "r2", Owner: "bob"})
	_, _ = svc.CreateRepo(CreateRepoRequest{Name: "r3", Owner: "alice"})

	all := svc.ListRepos("")
	if len(all) != 3 {
		t.Errorf("expected 3 repos, got %d", len(all))
	}

	alice := svc.ListRepos("alice")
	if len(alice) != 2 {
		t.Errorf("expected 2 alice repos, got %d", len(alice))
	}
}

func TestDeleteRepo(t *testing.T) {
	dir, err := os.MkdirTemp("", "gitserver-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	svc := NewService(dir, 1024)
	repo, _ := svc.CreateRepo(CreateRepoRequest{Name: "to-delete", Owner: "admin"})

	if err := svc.DeleteRepo(repo.ID); err != nil {
		t.Fatalf("DeleteRepo: %v", err)
	}

	_, err = svc.GetRepo(repo.ID)
	if err == nil {
		t.Error("expected not found error")
	}
}

func TestCollaborators(t *testing.T) {
	dir, err := os.MkdirTemp("", "gitserver-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	svc := NewService(dir, 1024)
	repo, _ := svc.CreateRepo(CreateRepoRequest{Name: "collab-test", Owner: "admin"})

	if err := svc.AddCollaborator(repo.ID, "bob", "write"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddCollaborator(repo.ID, "charlie", "read"); err != nil {
		t.Fatal(err)
	}

	collabs := svc.collabs[repo.ID]
	if len(collabs) != 2 {
		t.Errorf("expected 2 collaborators, got %d", len(collabs))
	}

	// update role
	if err := svc.AddCollaborator(repo.ID, "bob", "admin"); err != nil {
		t.Fatal(err)
	}
	collabs = svc.collabs[repo.ID]
	if len(collabs) != 2 {
		t.Errorf("expected still 2 collaborators after update, got %d", len(collabs))
	}

	// remove
	if err := svc.RemoveCollaborator(repo.ID, "charlie"); err != nil {
		t.Fatal(err)
	}
	collabs = svc.collabs[repo.ID]
	if len(collabs) != 1 {
		t.Errorf("expected 1 collaborator after remove, got %d", len(collabs))
	}
}

func TestWebhooks(t *testing.T) {
	dir, err := os.MkdirTemp("", "gitserver-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	svc := NewService(dir, 1024)
	repo, _ := svc.CreateRepo(CreateRepoRequest{Name: "hook-test", Owner: "admin"})

	hook, err := svc.CreateWebhook(repo.ID, "https://example.com/webhook", "secret123", []string{"push", "tag"})
	if err != nil {
		t.Fatal(err)
	}
	if hook.URL != "https://example.com/webhook" {
		t.Errorf("unexpected URL: %s", hook.URL)
	}

	hooks := svc.ListWebhooks(repo.ID)
	if len(hooks) != 1 {
		t.Errorf("expected 1 webhook, got %d", len(hooks))
	}

	if err := svc.DeleteWebhook(repo.ID, hook.ID); err != nil {
		t.Fatal(err)
	}
	hooks = svc.ListWebhooks(repo.ID)
	if len(hooks) != 0 {
		t.Errorf("expected 0 webhooks after delete, got %d", len(hooks))
	}
}

func TestCreateRepoValidation(t *testing.T) {
	dir, err := os.MkdirTemp("", "gitserver-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	svc := NewService(dir, 1024)

	_, err = svc.CreateRepo(CreateRepoRequest{Owner: "admin"})
	if err == nil {
		t.Error("expected name required error")
	}

	_, err = svc.CreateRepo(CreateRepoRequest{Name: "test"})
	if err == nil {
		t.Error("expected owner required error")
	}
}
