// Package aiwriter AI写作助手 - 文本生成与模板系统
package aiwriter

import (
	"errors"
	"sync"
	"time"
)

// Language 语言类型.
type Language string

const (
	LangChinese  Language = "zh"
	LangEnglish  Language = "en"
	LangJapanese Language = "ja"
)

// WriteStyle 写作风格.
type WriteStyle string

const (
	StyleFormal    WriteStyle = "formal"    // 正式
	StyleCasual    WriteStyle = "casual"    // casual
	StyleTechnical WriteStyle = "technical" // 技术
)

// TaskType 任务类型.
type TaskType string

const (
	TaskSummary  TaskType = "summary"  // 摘要
	TaskExpand   TaskType = "expand"   // 扩写
	TaskRewrite  TaskType = "rewrite"  // 改写
	TaskTemplate TaskType = "template" // 模板填充
)

// TemplateType 模板类型.
type TemplateType string

const (
	TmplEmail    TemplateType = "email"    // 邮件
	TmplReport   TemplateType = "report"   // 报告
	TmplAnnounce TemplateType = "announce" // 公告
)

// WriteRequest 写作请求.
type WriteRequest struct {
	TaskType     TaskType     `json:"task_type"`
	Content      string       `json:"content"`
	Language     Language     `json:"language"`
	Style        WriteStyle   `json:"style"`
	TemplateType TemplateType `json:"template_type,omitempty"`
	MaxTokens    int          `json:"max_tokens,omitempty"`
}

// WriteResult 写作结果.
type WriteResult struct {
	ID         string     `json:"id"`
	TaskType   TaskType   `json:"task_type"`
	Content    string     `json:"content"`
	Result     string     `json:"result"`
	Language   Language   `json:"language"`
	Style      WriteStyle `json:"style"`
	TokenCount int        `json:"token_count"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Template 模板定义.
type Template struct {
	ID          string       `json:"id"`
	Type        TemplateType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Language    Language     `json:"language"`
	Content     string       `json:"content"`
	Variables   []string     `json:"variables"`
}

// TemplateRequest 模板请求.
type TemplateRequest struct {
	TemplateID string            `json:"template_id"`
	Variables  map[string]string `json:"variables"`
}

// Config 配置.
type Config struct {
	DefaultLanguage Language   `json:"default_language"`
	DefaultStyle    WriteStyle `json:"default_style"`
	MaxTokens       int        `json:"max_tokens"`
	HistorySize     int        `json:"history_size"`
}

// Manager 管理器.
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	templates []*Template
	history   []*WriteResult
	dataFile  string
}

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrTemplateEmpty = errors.New("template is empty")
)

// NewManager 创建管理器.
func NewManager(dataFile string) *Manager {
	return &Manager{
		config: &Config{
			DefaultLanguage: LangChinese,
			DefaultStyle:    StyleFormal,
			MaxTokens:       2000,
			HistorySize:     100,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化.
func (m *Manager) Initialize() error {
	m.initTemplates()
	return nil
}

func (m *Manager) initTemplates() {
	m.templates = []*Template{
		{
			ID:          "email-formal",
			Type:        TmplEmail,
			Name:        "正式邮件",
			Description: "商务正式邮件模板",
			Language:    LangChinese,
			Content:     "尊敬的{{recipient}}：\n\n您好！\n\n{{body}}\n\n此致\n敬礼\n\n{{sender}}",
			Variables:   []string{"recipient", "body", "sender"},
		},
		{
			ID:          "report-formal",
			Type:        TmplReport,
			Name:        "工作周报",
			Description: "标准工作周报模板",
			Language:    LangChinese,
			Content:     "# {{title}}\n\n## 本周完成\n{{completed}}\n\n## 下周计划\n{{planned}}\n\n## 问题与建议\n{{issues}}",
			Variables:   []string{"title", "completed", "planned", "issues"},
		},
		{
			ID:          "announce-formal",
			Type:        TmplAnnounce,
			Name:        "公司公告",
			Description: "正式公告模板",
			Language:    LangChinese,
			Content:     "# {{title}}\n\n各部门：\n\n{{content}}\n\n请各部门知悉。\n\n{{department}}\n{{date}}",
			Variables:   []string{"title", "content", "department", "date"},
		},
		{
			ID:          "email-en-formal",
			Type:        TmplEmail,
			Name:        "Formal Email",
			Description: "Formal business email template",
			Language:    LangEnglish,
			Content:     "Dear {{recipient}},\n\n{{body}}\n\nBest regards,\n{{sender}}",
			Variables:   []string{"recipient", "body", "sender"},
		},
		{
			ID:          "report-ja-formal",
			Type:        TmplReport,
			Name:        "業務週報",
			Description: "標準業務週報テンプレート",
			Language:    LangJapanese,
			Content:     "# {{title}}\n\n## 今週の成果\n{{completed}}\n\n## 来週の予定\n{{planned}}\n\n## 課題と提案\n{{issues}}",
			Variables:   []string{"title", "completed", "planned", "issues"},
		},
	}
}
