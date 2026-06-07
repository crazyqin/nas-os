// Package gitserver 自托管 Git 仓库服务.
package gitserver

import (
	"sync"
	"time"
)

// Repository Git 仓库.
type Repository struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Owner         string    `json:"owner"`
	Path          string    `json:"path"`
	IsBare        bool      `json:"is_bare"`
	DefaultBranch string    `json:"default_branch"`
	Visibility    string    `json:"visibility"` // public, private, internal
	QuotaMB       int64     `json:"quota_mb"`
	SizeMB        int64     `json:"size_mb"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Tags          []string  `json:"tags,omitempty"`
}

// Branch 分支信息.
type Branch struct {
	Name       string    `json:"name"`
	CommitHash string    `json:"commit_hash"`
	Message    string    `json:"message"`
	Author     string    `json:"author"`
	UpdatedAt  time.Time `json:"updated_at"`
	IsDefault  bool      `json:"is_default"`
}

// Collaborator 仓库协作者.
type Collaborator struct {
	Username string    `json:"username"`
	Role     string    `json:"role"` // read, write, admin
	AddedAt  time.Time `json:"added_at"`
}

// WebHook webhook 配置.
type WebHook struct {
	ID        string    `json:"id"`
	RepoID    string    `json:"repo_id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"` // push, tag, branch_create, etc.
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Commit 提交记录.
type Commit struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}

// Service Git 服务.
type Service struct {
	mu         sync.RWMutex
	repos      map[string]*Repository
	collabs    map[string][]*Collaborator // repoID -> collaborators
	hooks      map[string][]*WebHook      // repoID -> hooks
	basePath   string
	quotaTotal int64 // 总配额 MB
	usedSpace  int64 // 已用空间 MB
}

// CreateRepoRequest 创建仓库请求.
type CreateRepoRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Owner         string `json:"owner"`
	Visibility    string `json:"visibility"`
	QuotaMB       int64  `json:"quota_mb"`
	InitREADME    bool   `json:"init_readme"`
	DefaultBranch string `json:"default_branch"`
}

// APIError API 错误.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
