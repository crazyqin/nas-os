package gitserver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// NewService 创建 Git 服务.
func NewService(basePath string, quotaTotalMB int64) *Service {
	return &Service{
		repos:      make(map[string]*Repository),
		collabs:    make(map[string][]*Collaborator),
		hooks:      make(map[string][]*WebHook),
		basePath:   basePath,
		quotaTotal: quotaTotalMB,
	}
}

// CreateRepo 创建 Git 仓库.
func (s *Service) CreateRepo(req CreateRepoRequest) (*Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("repository name is required")
	}
	if req.Owner == "" {
		return nil, fmt.Errorf("owner is required")
	}
	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	repoPath := filepath.Join(s.basePath, req.Owner, req.Name+".git")

	if _, err := os.Stat(repoPath); err == nil {
		return nil, fmt.Errorf("repository %s already exists", req.Name)
	}

	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// git init --bare
	cmd := exec.Command("git", "init", "--bare", "--initial-branch="+req.DefaultBranch, repoPath)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git init: %w", err)
	}

	now := time.Now()
	repo := &Repository{
		ID:            generateRepoID(),
		Name:          req.Name,
		Description:   req.Description,
		Owner:         req.Owner,
		Path:          repoPath,
		IsBare:        true,
		DefaultBranch: req.DefaultBranch,
		Visibility:    req.Visibility,
		QuotaMB:       req.QuotaMB,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.repos[repo.ID] = repo
	return repo, nil
}

// GetRepo 获取仓库信息.
func (s *Service) GetRepo(id string) (*Repository, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	repo, ok := s.repos[id]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", id)
	}
	return repo, nil
}

// ListRepos 列出仓库.
func (s *Service) ListRepos(owner string) []*Repository {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Repository
	for _, r := range s.repos {
		if owner == "" || r.Owner == owner {
			result = append(result, r)
		}
	}
	return result
}

// DeleteRepo 删除仓库.
func (s *Service) DeleteRepo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, ok := s.repos[id]
	if !ok {
		return fmt.Errorf("repository %s not found", id)
	}

	if err := os.RemoveAll(repo.Path); err != nil {
		return fmt.Errorf("remove repo: %w", err)
	}

	delete(s.repos, id)
	delete(s.collabs, id)
	delete(s.hooks, id)
	return nil
}

// ListBranches 列出分支.
func (s *Service) ListBranches(repoID string) ([]*Branch, error) {
	s.mu.RLock()
	repo, ok := s.repos[repoID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("repository %s not found", repoID)
	}

	cmd := exec.Command("git", "--git-dir="+repo.Path, "branch", "-a", "--format=%(refname:short)|%(objectname:short)|%(subject)|%(authorname)|%(committerdate:iso)")
	out, err := cmd.Output()
	if err != nil {
		return []*Branch{}, nil
	}

	var branches []*Branch
	for _, line := range splitLines(string(out)) {
		parts := splitFields(line, 5)
		if len(parts) < 2 {
			continue
		}
		b := &Branch{
			Name:       parts[0],
			CommitHash: parts[1],
		}
		if len(parts) > 2 {
			b.Message = parts[2]
		}
		if len(parts) > 3 {
			b.Author = parts[3]
		}
		if parts[0] == repo.DefaultBranch {
			b.IsDefault = true
		}
		branches = append(branches, b)
	}
	return branches, nil
}

// AddCollaborator 添加协作者.
func (s *Service) AddCollaborator(repoID, username, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.repos[repoID]; !ok {
		return fmt.Errorf("repository %s not found", repoID)
	}

	collabs := s.collabs[repoID]
	for _, c := range collabs {
		if c.Username == username {
			c.Role = role
			return nil
		}
	}

	s.collabs[repoID] = append(collabs, &Collaborator{
		Username: username,
		Role:     role,
		AddedAt:  time.Now(),
	})
	return nil
}

// RemoveCollaborator 移除协作者.
func (s *Service) RemoveCollaborator(repoID, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	collabs := s.collabs[repoID]
	for i, c := range collabs {
		if c.Username == username {
			s.collabs[repoID] = append(collabs[:i], collabs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("collaborator %s not found", username)
}

// CreateWebhook 创建 webhook.
func (s *Service) CreateWebhook(repoID, url, secret string, events []string) (*WebHook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.repos[repoID]; !ok {
		return nil, fmt.Errorf("repository %s not found", repoID)
	}

	hook := &WebHook{
		ID:        generateRepoID(),
		RepoID:    repoID,
		URL:       url,
		Secret:    secret,
		Events:    events,
		Active:    true,
		CreatedAt: time.Now(),
	}

	s.hooks[repoID] = append(s.hooks[repoID], hook)
	return hook, nil
}

// ListWebhooks 列出 webhook.
func (s *Service) ListWebhooks(repoID string) []*WebHook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hooks[repoID]
}

// DeleteWebhook 删除 webhook.
func (s *Service) DeleteWebhook(repoID, hookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hooks := s.hooks[repoID]
	for i, h := range hooks {
		if h.ID == hookID {
			s.hooks[repoID] = append(hooks[:i], hooks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("webhook %s not found", hookID)
}

// GetRepoStats 获取仓库统计.
func (s *Service) GetRepoStats(repoID string) (map[string]interface{}, error) {
	s.mu.RLock()
	repo, ok := s.repos[repoID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("repository %s not found", repoID)
	}

	stats := map[string]interface{}{
		"id":        repo.ID,
		"name":      repo.Name,
		"size_mb":   repo.SizeMB,
		"quota_mb":  repo.QuotaMB,
	}

	// count commits
	cmd := exec.Command("git", "--git-dir="+repo.Path, "rev-list", "--count", "HEAD")
	if out, err := cmd.Output(); err == nil {
		stats["total_commits"] = string(out)
	}

	return stats, nil
}

var repoCounter struct {
	mu    sync.Mutex
	value int
}

func generateRepoID() string {
	repoCounter.mu.Lock()
	defer repoCounter.mu.Unlock()
	repoCounter.value++
	return fmt.Sprintf("repo-%d-%d", time.Now().UnixNano(), repoCounter.value)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string, max int) []string {
	var fields []string
	start := 0
	for i := 0; i < len(s) && len(fields) < max-1; i++ {
		if s[i] == '|' {
			fields = append(fields, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		fields = append(fields, s[start:])
	}
	return fields
}
