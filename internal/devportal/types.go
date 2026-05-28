// Package devportal 开发者门户 - NAS 内置开发平台.
// 提供 Git 仓库托管、代码浏览、CI/CD 流水线、代码审查、Docker 构建、
// API 文档生成、开发者密钥管理、Webhook 与集成功能.
package devportal

import (
	"sync"
	"time"
)

// ==================== 仓库管理 ====================

// Repository Git 仓库.
type Repository struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Owner         string    `json:"owner"`
	Path          string    `json:"path"`
	DefaultBranch string    `json:"default_branch"`
	Visibility    string    `json:"visibility"` // public, private, internal
	Language      string    `json:"language"`   // 主要编程语言
	Topics        []string  `json:"topics,omitempty"`
	StarCount     int       `json:"star_count"`
	ForkCount     int       `json:"fork_count"`
	SizeKB        int64     `json:"size_kb"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ArchivedAt    *time.Time `json:"archived_at,omitempty"`
}

// CreateRepoRequest 创建仓库请求.
type CreateRepoRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Owner         string   `json:"owner"`
	Visibility    string   `json:"visibility"`
	DefaultBranch string   `json:"default_branch"`
	Topics        []string `json:"topics,omitempty"`
	InitREADME    bool     `json:"init_readme"` // 是否自动创建 README
	License       string   `json:"license"`     // 许可证模板
}

// ==================== 代码浏览 ====================

// FileEntry 文件/目录条目.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"` // 文件权限
	SHA     string    `json:"sha"`
	ModTime time.Time `json:"mod_time"`
}

// FileContent 文件内容.
type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"` // text, base64 (binary)
	Size     int64  `json:"size"`
	SHA      string `json:"sha"`
	Language string `json:"language"` // 语法高亮语言
}

// SearchResult 代码搜索结果.
type SearchResult struct {
	Path       string `json:"path"`
	LineNumber int    `json:"line_number"`
	Line       string `json:"line"`
	Context    string `json:"context"` // 上下文（前后各几行）
	Score      float64 `json:"score"` // 相关度分数
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	RepoID    string `json:"repo_id"`
	Branch    string `json:"branch"`    // 默认 main
	Query     string `json:"query"`
	Path      string `json:"path"`      // 限定搜索路径
	Extension string `json:"extension"` // 限定文件扩展名
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

// ==================== CI/CD 流水线 ====================

// Pipeline 流水线定义.
type Pipeline struct {
	ID          string           `json:"id"`
	RepoID      string           `json:"repo_id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Trigger     PipelineTrigger  `json:"trigger"`
	Stages      []PipelineStage  `json:"stages"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// PipelineTrigger 流水线触发条件.
type PipelineTrigger struct {
	Branches   []string `json:"branches"`    // 触发分支模式
	Events     []string `json:"events"`      // push, tag, pr, schedule
	Cron       string   `json:"cron"`        // 定时触发（cron 表达式）
	ManualOnly bool     `json:"manual_only"` // 仅手动触发
}

// PipelineStage 流水线阶段.
type PipelineStage struct {
	Name     string            `json:"name"`     // build, test, deploy 等
	Steps    []PipelineStep    `json:"steps"`
	DependsOn []string          `json:"depends_on,omitempty"` // 依赖的阶段
	Parallel bool              `json:"parallel"`             // 步骤是否并行
}

// PipelineStep 流水线步骤.
type PipelineStep struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`   // Docker 镜像
	Script  []string          `json:"script"`  // 执行脚本
	EnvVars map[string]string `json:"env_vars,omitempty"`
	Timeout int               `json:"timeout"` // 超时秒数
	When    string            `json:"when"`    // always, on_success, on_failure
}

// PipelineRun 流水线运行记录.
type PipelineRun struct {
	ID         string             `json:"id"`
	PipelineID string             `json:"pipeline_id"`
	RepoID     string             `json:"repo_id"`
	Branch     string             `json:"branch"`
	Commit     string             `json:"commit"`
	Status     string             `json:"status"` // pending, running, success, failed, cancelled
	Trigger    string             `json:"trigger"` // push, manual, schedule, tag
	Stages     []StageRun         `json:"stages"`
	Logs       []LogEntry         `json:"logs,omitempty"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt *time.Time         `json:"finished_at,omitempty"`
	Duration   int                `json:"duration"` // 秒
	TriggerBy  string             `json:"trigger_by"`
}

// StageRun 阶段运行记录.
type StageRun struct {
	Name      string     `json:"name"`
	Status    string     `json:"status"` // pending, running, success, failed, skipped
	StartedAt time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Duration  int        `json:"duration"` // 秒
}

// LogEntry 日志条目.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Stage     string    `json:"stage"`
	Step      string    `json:"step"`
	Level     string    `json:"level"` // info, warn, error, debug
	Message   string    `json:"message"`
}

// ==================== 代码审查 / 合并请求 ====================

// MergeRequest 合并请求.
type MergeRequest struct {
	ID          string    `json:"id"`
	RepoID      string    `json:"repo_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Author      string    `json:"author"`
	Assignees   []string  `json:"assignees,omitempty"`
	Reviewers   []string  `json:"reviewers,omitempty"`
	Status      string    `json:"status"` // open, merged, closed, draft
	Labels      []string  `json:"labels,omitempty"`
	Commits     int       `json:"commits"`
	Additions   int       `json:"additions"`
	Deletions   int       `json:"deletions"`
	Conflicts   bool      `json:"conflicts"` // 是否有冲突
	ApprovedBy  []string  `json:"approved_by,omitempty"`
	MergedBy    string    `json:"merged_by,omitempty"`
	HeadCommit  string    `json:"head_commit"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MergedAt    *time.Time `json:"merged_at,omitempty"`
}

// ReviewComment 审查评论.
type ReviewComment struct {
	ID         string    `json:"id"`
	MRID       string    `json:"mr_id"`
	Author     string    `json:"author"`
	Content    string    `json:"content"`
	FilePath   string    `json:"file_path,omitempty"` // 文件路径（行内评论）
	LineNumber int       `json:"line_number,omitempty"`
	IsResolved bool      `json:"is_resolved"`
	ReplyTo    string    `json:"reply_to,omitempty"` // 回复某条评论
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// DiffFile 文件差异.
type DiffFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"old_path,omitempty"`
	Status    string `json:"status"` // added, modified, deleted, renamed
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

// CreateMRRequest 创建合并请求.
type CreateMRRequest struct {
	RepoID       string   `json:"repo_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Author       string   `json:"author"`
	Assignees    []string `json:"assignees,omitempty"`
	Reviewers    []string `json:"reviewers,omitempty"`
	Labels       []string `json:"labels,omitempty"`
}

// ==================== Docker 镜像构建 ====================

// DockerBuild Docker 构建任务.
type DockerBuild struct {
	ID         string     `json:"id"`
	RepoID     string     `json:"repo_id"`
	Tag        string     `json:"tag"`
	Dockerfile string     `json:"dockerfile"` // Dockerfile 路径
	Context    string     `json:"context"`    // 构建上下文路径
	Args       map[string]string `json:"args,omitempty"` // 构建参数
	Status     string     `json:"status"` // pending, building, success, failed
	ImageID    string     `json:"image_id,omitempty"`
	ImageSize  int64      `json:"image_size"` // 字节
	Logs       []string   `json:"logs,omitempty"`
	BuildTime  int        `json:"build_time"` // 秒
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	TriggerBy  string     `json:"trigger_by"` // pipeline, manual, push
}

// DockerImage Docker 镜像信息.
type DockerImage struct {
	ID        string    `json:"id"`
	RepoID    string    `json:"repo_id"`
	Tag       string    `json:"tag"`
	Digest    string    `json:"digest"`
	SizeMB    int64     `json:"size_mb"`
	Layers    int       `json:"layers"`
	CreatedAt time.Time `json:"created_at"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// CreateBuildRequest 创建构建请求.
type CreateBuildRequest struct {
	RepoID     string            `json:"repo_id"`
	Tag        string            `json:"tag"`
	Branch     string            `json:"branch"`
	Commit     string            `json:"commit"`
	Dockerfile string            `json:"dockerfile"`
	Context    string            `json:"context"`
	Args       map[string]string `json:"args,omitempty"`
	NoCache    bool              `json:"no_cache"`
}

// ==================== API 文档 ====================

// APIDoc API 文档.
type APIDoc struct {
	ID        string    `json:"id"`
	RepoID    string    `json:"repo_id"`
	Version   string    `json:"version"`   // v1, v2
	Title     string    `json:"title"`
	BaseURL   string    `json:"base_url"`
	Format    string    `json:"format"` // openapi, swagger, markdown
	Content   string    `json:"content"` // JSON/YAML 内容
	SourcePath string   `json:"source_path"` // 源文件路径
	Endpoints []Endpoint `json:"endpoints"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Endpoint API 端点.
type Endpoint struct {
	Method      string   `json:"method"` // GET, POST, PUT, DELETE, PATCH
	Path        string   `json:"path"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Deprecated  bool     `json:"deprecated"`
}

// GenerateDocRequest 生成文档请求.
type GenerateDocRequest struct {
	RepoID     string `json:"repo_id"`
	Branch     string `json:"branch"`
	SourcePath string `json:"source_path"` // 源码路径（扫描注释生成）
	Format     string `json:"format"`      // openapi, markdown
	Title      string `json:"title"`
	BaseURL    string `json:"base_url"`
}

// ==================== 开发者密钥管理 ====================

// APIKey 开发者 API 密钥.
type APIKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key"`         // 完整密钥（仅创建时返回）
	KeyPrefix   string    `json:"key_prefix"`  // 密钥前缀（用于展示）
	Owner       string    `json:"owner"`
	Permissions []string  `json:"permissions"` // read, write, admin, build, deploy
	Scopes      []string  `json:"scopes"`      // repo:read, pipeline:write 等
	RateLimit   int       `json:"rate_limit"`  // 每分钟请求限制
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	UsedCount   int64     `json:"used_count"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// CreateKeyRequest 创建密钥请求.
type CreateKeyRequest struct {
	Name        string    `json:"name"`
	Owner       string    `json:"owner"`
	Permissions []string  `json:"permissions"`
	Scopes      []string  `json:"scopes"`
	RateLimit   int       `json:"rate_limit"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// SSHKey SSH 公钥.
type SSHKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	PublicKey string    `json:"public_key"`
	Fingerprint string  `json:"fingerprint"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ==================== Webhook 与集成 ====================

// Webhook 配置.
type Webhook struct {
	ID        string    `json:"id"`
	RepoID    string    `json:"repo_id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"` // push, tag, mr, pipeline, build, release
	Active    bool      `json:"active"`
	ContentType string  `json:"content_type"` // json, form
	InsecureSSL bool    `json:"insecure_ssl"` // 允许不安全 SSL
	LastDelivery *WebhookDelivery `json:"last_delivery,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookDelivery Webhook 投递记录.
type WebhookDelivery struct {
	ID         string    `json:"id"`
	WebhookID  string    `json:"webhook_id"`
	Event      string    `json:"event"`
	StatusCode int       `json:"status_code"`
	Request    string    `json:"request"`  // 请求体
	Response   string    `json:"response"` // 响应体
	Duration   int       `json:"duration"` // 毫秒
	Success    bool      `json:"success"`
	RetryCount int       `json:"retry_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// Integration 第三方集成.
type Integration struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // slack, discord, email, telegram, custom
	Config      map[string]string `json:"config"`
	Events      []string          `json:"events"`
	Enabled     bool              `json:"enabled"`
	RepoID      string            `json:"repo_id,omitempty"` // 仓库级别集成
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CreateWebhookRequest 创建 Webhook 请求.
type CreateWebhookRequest struct {
	RepoID      string   `json:"repo_id"`
	URL         string   `json:"url"`
	Secret      string   `json:"secret"`
	Events      []string `json:"events"`
	ContentType string   `json:"content_type"`
	InsecureSSL bool     `json:"insecure_ssl"`
}

// CreateIntegrationRequest 创建集成请求.
type CreateIntegrationRequest struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Config  map[string]string `json:"config"`
	Events  []string          `json:"events"`
	RepoID  string            `json:"repo_id,omitempty"`
}

// ==================== 统计与仪表盘 ====================

// DevPortalStats 开发者门户统计.
type DevPortalStats struct {
	TotalRepos      int   `json:"total_repos"`
	TotalCommits    int64 `json:"total_commits"`
	TotalMRs        int   `json:"total_merge_requests"`
	OpenMRs         int   `json:"open_merge_requests"`
	TotalPipelines  int   `json:"total_pipelines"`
	SuccessfulRuns  int   `json:"successful_runs"`
	FailedRuns      int   `json:"failed_runs"`
	TotalDockerBuilds int `json:"total_docker_builds"`
	TotalAPIKeys    int   `json:"total_api_keys"`
	TotalWebhooks   int   `json:"total_webhooks"`
	ActiveDevelopers int  `json:"active_developers"`
}

// ==================== Service ====================

// Service 开发者门户核心服务.
type Service struct {
	mu           sync.RWMutex
	repos        map[string]*Repository
	pipelines    map[string]*Pipeline        // pipelineID -> Pipeline
	runs         map[string][]*PipelineRun   // pipelineID -> []PipelineRun
	mrs          map[string][]*MergeRequest  // repoID -> []MergeRequest
	comments     map[string][]*ReviewComment // mrID -> []ReviewComment
	builds       map[string][]*DockerBuild   // repoID -> []DockerBuild
	images       map[string][]*DockerImage   // repoID -> []DockerImage
	docs         map[string][]*APIDoc        // repoID -> []APIDoc
	apiKeys      map[string]*APIKey          // keyID -> APIKey
	keysByPrefix map[string]*APIKey          // keyPrefix -> APIKey (用于快速查找)
	sshKeys      map[string][]*SSHKey        // owner -> []SSHKey
	webhooks     map[string][]*Webhook       // repoID -> []Webhook
	deliveries   map[string][]*WebhookDelivery // webhookID -> []WebhookDelivery
	integrations map[string]*Integration     // integrationID -> Integration
	basePath     string                      // 数据存储根路径
}

// ID 生成器计数器.
var idCounter struct {
	mu    sync.Mutex
	value int
}
