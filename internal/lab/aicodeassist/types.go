// Package aicodeassist 提供 AI 代码助手平台功能，支持代码补全、审查、重构建议、
// 测试用例生成、代码解释及 Git 集成等能力。
package aicodeassist

import "time"

// ProgrammingLanguage 支持的编程语言.
type ProgrammingLanguage string

const (
	LangGo     ProgrammingLanguage = "go"
	LangPython ProgrammingLanguage = "python"
	LangJS     ProgrammingLanguage = "javascript"
	LangRust   ProgrammingLanguage = "rust"
	LangJava   ProgrammingLanguage = "java"
)

// ReviewCategory 代码审查类别.
type ReviewCategory string

const (
	ReviewSecurity ReviewCategory = "security"
	ReviewPerf     ReviewCategory = "performance"
	ReviewStyle    ReviewCategory = "style"
)

// ReviewSeverity 审查问题严重程度.
type ReviewSeverity string

const (
	SeverityInfo     ReviewSeverity = "info"
	SeverityWarning  ReviewSeverity = "warning"
	SeverityError    ReviewSeverity = "error"
	SeverityCritical ReviewSeverity = "critical"
)

// RefactorType 重构类型.
type RefactorType string

const (
	RefactorExtractFunc RefactorType = "extract_function"
	RefactorRename      RefactorType = "rename"
	RefactorSimplify    RefactorType = "simplify"
	RefactorDRY         RefactorType = "dry"
	RefactorErrorHandle RefactorType = "error_handling"
)

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusSuccess    TaskStatus = "success"
	TaskStatusFailed     TaskStatus = "failed"
)

// CompletionRequest 代码补全请求.
type CompletionRequest struct {
	Language  ProgrammingLanguage `json:"language" binding:"required"`
	Code      string              `json:"code" binding:"required"`
	CursorPos int                 `json:"cursor_pos"`
	Context   string              `json:"context,omitempty"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

// CompletionResponse 代码补全响应.
type CompletionResponse struct {
	ID          string           `json:"id"`
	Suggestions []CodeSuggestion `json:"suggestions"`
	Duration    time.Duration    `json:"duration"`
	CreatedAt   time.Time        `json:"created_at"`
}

// CodeSuggestion 代码建议.
type CodeSuggestion struct {
	Code        string  `json:"code"`
	Description string  `json:"description,omitempty"`
	Confidence  float64 `json:"confidence"`
}

// ReviewRequest 代码审查请求.
type ReviewRequest struct {
	Language   ProgrammingLanguage `json:"language" binding:"required"`
	Code       string              `json:"code" binding:"required"`
	Categories []ReviewCategory    `json:"categories,omitempty"`
}

// ReviewResponse 代码审查响应.
type ReviewResponse struct {
	ID        string        `json:"id"`
	Issues    []ReviewIssue `json:"issues"`
	Summary   string        `json:"summary"`
	Score     int           `json:"score"`
	Duration  time.Duration `json:"duration"`
	CreatedAt time.Time     `json:"created_at"`
}

// ReviewIssue 审查发现的问题.
type ReviewIssue struct {
	Line       int            `json:"line"`
	Column     int            `json:"column,omitempty"`
	Category   ReviewCategory `json:"category"`
	Severity   ReviewSeverity `json:"severity"`
	Message    string         `json:"message"`
	Suggestion string         `json:"suggestion,omitempty"`
}

// RefactorRequest 代码重构请求.
type RefactorRequest struct {
	Language ProgrammingLanguage `json:"language" binding:"required"`
	Code     string              `json:"code" binding:"required"`
	Type     RefactorType        `json:"type" binding:"required"`
	Target   string              `json:"target,omitempty"`
}

// RefactorResponse 代码重构响应.
type RefactorResponse struct {
	ID          string        `json:"id"`
	Original    string        `json:"original"`
	Refactored  string        `json:"refactored"`
	Explanation string        `json:"explanation"`
	Changes     []Change      `json:"changes"`
	Duration    time.Duration `json:"duration"`
	CreatedAt   time.Time     `json:"created_at"`
}

// Change 代码变更.
type Change struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	OldCode     string `json:"old_code,omitempty"`
	NewCode     string `json:"new_code,omitempty"`
}

// TestGenRequest 测试用例生成请求.
type TestGenRequest struct {
	Language  ProgrammingLanguage `json:"language" binding:"required"`
	Code      string              `json:"code" binding:"required"`
	Framework string              `json:"framework,omitempty"`
	Coverage  bool                `json:"coverage"`
}

// TestGenResponse 测试用例生成响应.
type TestGenResponse struct {
	ID        string        `json:"id"`
	TestCode  string        `json:"test_code"`
	Framework string        `json:"framework"`
	TestCases []TestCase    `json:"test_cases"`
	Duration  time.Duration `json:"duration"`
	CreatedAt time.Time     `json:"created_at"`
}

// TestCase 测试用例.
type TestCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Input       string `json:"input,omitempty"`
	Expected    string `json:"expected,omitempty"`
}

// ExplainRequest 代码解释请求.
type ExplainRequest struct {
	Language ProgrammingLanguage `json:"language" binding:"required"`
	Code     string              `json:"code" binding:"required"`
	Detail   string              `json:"detail,omitempty"` // brief, normal, detailed
}

// ExplainResponse 代码解释响应.
type ExplainResponse struct {
	ID          string        `json:"id"`
	Explanation string        `json:"explanation"`
	Summary     string        `json:"summary"`
	Complexity  string        `json:"complexity,omitempty"`
	Duration    time.Duration `json:"duration"`
	CreatedAt   time.Time     `json:"created_at"`
}

// GitDiffRequest Git diff 分析请求.
type GitDiffRequest struct {
	Diff     string `json:"diff" binding:"required"`
	Language string `json:"language,omitempty"`
}

// GitDiffResponse Git diff 分析响应.
type GitDiffResponse struct {
	ID        string        `json:"id"`
	Summary   string        `json:"summary"`
	Changes   []DiffChange  `json:"changes"`
	Risks     []string      `json:"risks,omitempty"`
	Duration  time.Duration `json:"duration"`
	CreatedAt time.Time     `json:"created_at"`
}

// DiffChange diff 变更条目.
type DiffChange struct {
	File    string `json:"file"`
	Type    string `json:"type"` // added, modified, deleted
	Summary string `json:"summary"`
}

// CommitMsgRequest commit message 生成请求.
type CommitMsgRequest struct {
	Diff     string `json:"diff" binding:"required"`
	Language string `json:"language,omitempty"` // zh or en
	Style    string `json:"style,omitempty"`    // conventional, simple
}

// CommitMsgResponse commit message 生成响应.
type CommitMsgResponse struct {
	ID           string    `json:"id"`
	Message      string    `json:"message"`
	Subject      string    `json:"subject"`
	Body         string    `json:"body,omitempty"`
	Alternatives []string  `json:"alternatives,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// TaskInfo 通用任务信息.
type TaskInfo struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Status    TaskStatus `json:"status"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// AIAssistConfig AI 代码助手配置.
type AIAssistConfig struct {
	Enabled         bool                  `json:"enabled"`
	DefaultLanguage string                `json:"default_language"`
	MaxCodeLength   int                   `json:"max_code_length"`
	MaxTokens       int                   `json:"max_tokens"`
	SupportedLangs  []ProgrammingLanguage `json:"supported_langs"`
}

// DefaultAIAssistConfig 返回默认配置.
func DefaultAIAssistConfig() *AIAssistConfig {
	return &AIAssistConfig{
		Enabled:         true,
		DefaultLanguage: "zh",
		MaxCodeLength:   100000,
		MaxTokens:       4096,
		SupportedLangs: []ProgrammingLanguage{
			LangGo, LangPython, LangJS, LangRust, LangJava,
		},
	}
}

// SupportedLanguages 返回支持的编程语言列表.
func SupportedLanguages() []ProgrammingLanguage {
	return []ProgrammingLanguage{LangGo, LangPython, LangJS, LangRust, LangJava}
}

// IsValidLanguage 检查语言是否受支持.
func IsValidLanguage(lang ProgrammingLanguage) bool {
	for _, l := range SupportedLanguages() {
		if l == lang {
			return true
		}
	}
	return false
}
