package aichatbot

import (
	"fmt"
	"sync"
	"time"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeSystem    MessageType = "system"
)

// IntentType 意图类型
type IntentType string

const (
	IntentShareFolderCreate IntentType = "share_folder_create"
	IntentShareFolderDelete IntentType = "share_folder_delete"
	IntentSystemStatus      IntentType = "system_status"
	IntentDiskUsage         IntentType = "disk_usage"
	IntentServiceRestart    IntentType = "service_restart"
	IntentScheduleTask      IntentType = "schedule_task"
	IntentBatchOperation    IntentType = "batch_operation"
	IntentGeneralQuery      IntentType = "general_query"
	IntentUnknown           IntentType = "unknown"
)

// Language 语言
type Language string

const (
	LangChinese Language = "zh"
	LangEnglish Language = "en"
	LangJapanese Language = "ja"
)

// Message 对话消息
type Message struct {
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	Type      MessageType `json:"type"`
	Content   string      `json:"content"`
	Language  Language    `json:"language"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

// Session 对话会话
type Session struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Language    Language  `json:"language"`
	Messages    []*Message `json:"messages"`
	Context     map[string]interface{} `json:"context,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Intent 意图识别结果
type Intent struct {
	Type       IntentType            `json:"type"`
	Confidence float64               `json:"confidence"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// Plugin 插件定义
type Plugin struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Intents     []IntentType `json:"intents"`
	Handler     PluginHandler `json:"-"`
	IsEnabled   bool      `json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// PluginHandler 插件处理函数
type PluginHandler func(ctx *PluginContext) (*PluginResponse, error)

// PluginContext 插件上下文
type PluginContext struct {
	Session   *Session  `json:"session"`
	Intent    *Intent   `json:"intent"`
	Message   *Message  `json:"message"`
	Language  Language  `json:"language"`
}

// PluginResponse 插件响应
type PluginResponse struct {
	Content   string                 `json:"content"`
	Actions   []*Action              `json:"actions,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Action 执行动作
type Action struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
	Status     string                 `json:"status"`
	Result     string                 `json:"result,omitempty"`
}

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Schedule    string    `json:"schedule"`
	Action      *Action   `json:"action"`
	IsActive    bool      `json:"is_active"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatStats 聊天统计
type ChatStats struct {
	TotalSessions    int            `json:"total_sessions"`
	TotalMessages    int            `json:"total_messages"`
	ActiveSessions   int            `json:"active_sessions"`
	SessionsByUser   map[string]int `json:"sessions_by_user"`
	ScheduledTasks   int            `json:"scheduled_tasks"`
	InstalledPlugins int            `json:"installed_plugins"`
}

// AIChatbot 智能助手
type AIChatbot struct {
	mu            sync.RWMutex
	sessions      map[string]*Session
	messages      map[string][]*Message
	plugins       map[string]*Plugin
	scheduledTasks map[string]*ScheduledTask
	langTemplates  map[Language]map[string]string
}

// NewAIChatbot 创建智能助手
func NewAIChatbot() *AIChatbot {
	bot := &AIChatbot{
		sessions:      make(map[string]*Session),
		messages:      make(map[string][]*Message),
		plugins:       make(map[string]*Plugin),
		scheduledTasks: make(map[string]*ScheduledTask),
		langTemplates:  make(map[Language]map[string]string),
	}
	bot.initLanguageTemplates()
	return bot
}

// initLanguageTemplates 初始化语言模板
func (bot *AIChatbot) initLanguageTemplates() {
	bot.langTemplates[LangChinese] = map[string]string{
		"greeting":          "你好！我是 NAS AI 助手，有什么可以帮您的？",
		"share_created":     "共享文件夹已创建",
		"share_deleted":     "共享文件夹已删除",
		"system_status":     "系统状态查询完成",
		"task_scheduled":    "定时任务已创建",
		"task_not_found":    "未找到指定任务",
		"session_not_found": "会话不存在",
		"plugin_not_found":  "插件不存在",
		"unknown_intent":    "抱歉，我没有理解您的意思，请换个方式描述",
		"error":             "处理请求时出错",
	}

	bot.langTemplates[LangEnglish] = map[string]string{
		"greeting":          "Hello! I'm NAS AI Assistant, how can I help you?",
		"share_created":     "Share folder created successfully",
		"share_deleted":     "Share folder deleted successfully",
		"system_status":     "System status retrieved",
		"task_scheduled":    "Scheduled task created",
		"task_not_found":    "Task not found",
		"session_not_found": "Session not found",
		"plugin_not_found":  "Plugin not found",
		"unknown_intent":    "Sorry, I didn't understand. Please rephrase your request",
		"error":             "Error processing request",
	}

	bot.langTemplates[LangJapanese] = map[string]string{
		"greeting":          "こんにちは！NAS AIアシスタントです。何かお手伝いできますか？",
		"share_created":     "共有フォルダが作成されました",
		"share_deleted":     "共有フォルダが削除されました",
		"system_status":     "システム状態を取得しました",
		"task_scheduled":    "スケジュールタスクが作成されました",
		"task_not_found":    "タスクが見つかりません",
		"session_not_found": "セッションが見つかりません",
		"plugin_not_found":  "プラグインが見つかりません",
		"unknown_intent":    "申し訳ありません。理解できませんでした。別の言い方でお願いします",
		"error":             "リクエスト処理中にエラーが発生しました",
	}
}

// CreateSession 创建对话会话
func (bot *AIChatbot) CreateSession(sessionID, userID string, lang Language) (*Session, error) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if _, exists := bot.sessions[sessionID]; exists {
		return nil, fmt.Errorf("会话 %s 已存在", sessionID)
	}

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		Language:  lang,
		Messages:  make([]*Message, 0),
		Context:   make(map[string]interface{}),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	bot.sessions[sessionID] = session
	return session, nil
}

// GetSession 获取会话
func (bot *AIChatbot) GetSession(sessionID string) (*Session, error) {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	session, exists := bot.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}
	return session, nil
}

// CloseSession 关闭会话
func (bot *AIChatbot) CloseSession(sessionID string) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	session, exists := bot.sessions[sessionID]
	if !exists {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	session.IsActive = false
	session.UpdatedAt = time.Now()
	return nil
}

// ListSessions 列出用户会话
func (bot *AIChatbot) ListSessions(userID string, activeOnly bool) []*Session {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, session := range bot.sessions {
		if session.UserID != userID {
			continue
		}
		if activeOnly && !session.IsActive {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions
}

// SendMessage 发送消息
func (bot *AIChatbot) SendMessage(sessionID, content string, msgType MessageType) (*Message, error) {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	session, exists := bot.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}

	msg := &Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Type:      msgType,
		Content:   content,
		Language:  session.Language,
		CreatedAt: time.Now(),
	}

	bot.messages[sessionID] = append(bot.messages[sessionID], msg)
	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()

	return msg, nil
}

// GetMessages 获取会话消息
func (bot *AIChatbot) GetMessages(sessionID string, limit int) ([]*Message, error) {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	if _, exists := bot.sessions[sessionID]; !exists {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}

	msgs := bot.messages[sessionID]
	if limit > 0 && limit < len(msgs) {
		return msgs[len(msgs)-limit:], nil
	}
	return msgs, nil
}

// RecognizeIntent 识别意图
func (bot *AIChatbot) RecognizeIntent(content string) *Intent {
	// 简单的关键字匹配（实际应使用 NLP 模型）
	intent := &Intent{
		Type:       IntentUnknown,
		Confidence: 0.5,
		Parameters: make(map[string]interface{}),
	}

	// 共享文件夹相关
	if containsAny(content, []string{"共享文件夹", "share folder", "共有フォルダ"}) {
		if containsAny(content, []string{"创建", "新建", "create", "作成"}) {
			intent.Type = IntentShareFolderCreate
			intent.Confidence = 0.9
			intent.Parameters["name"] = extractFolderName(content)
		} else if containsAny(content, []string{"删除", "移除", "delete", "削除"}) {
			intent.Type = IntentShareFolderDelete
			intent.Confidence = 0.9
			intent.Parameters["name"] = extractFolderName(content)
		}
		return intent
	}

	// 系统状态
	if containsAny(content, []string{"系统状态", "运行状态", "system status", "システム状態"}) {
		intent.Type = IntentSystemStatus
		intent.Confidence = 0.95
		return intent
	}

	// 磁盘使用
	if containsAny(content, []string{"磁盘", "硬盘", "存储空间", "disk", "storage"}) {
		intent.Type = IntentDiskUsage
		intent.Confidence = 0.9
		return intent
	}

	// 服务重启
	if containsAny(content, []string{"重启", "重启服务", "restart", "再起動"}) {
		intent.Type = IntentServiceRestart
		intent.Confidence = 0.85
		intent.Parameters["service"] = extractServiceName(content)
		return intent
	}

	// 定时任务
	if containsAny(content, []string{"定时", "定期", "schedule", "スケジュール"}) {
		intent.Type = IntentScheduleTask
		intent.Confidence = 0.85
		return intent
	}

	// 批量操作
	if containsAny(content, []string{"批量", "batch", "バッチ"}) {
		intent.Type = IntentBatchOperation
		intent.Confidence = 0.8
		return intent
	}

	return intent
}

// ProcessMessage 处理消息并返回响应
func (bot *AIChatbot) ProcessMessage(sessionID, content string) (*Message, error) {
	bot.mu.Lock()
	session, exists := bot.sessions[sessionID]
	if !exists {
		bot.mu.Unlock()
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}
	bot.mu.Unlock()

	// 保存用户消息
	bot.SendMessage(sessionID, content, MessageTypeUser)

	// 识别意图
	intent := bot.RecognizeIntent(content)

	// 查找匹配的插件
	response := bot.executePlugin(session, intent, content)

	// 保存助手响应
	return bot.SendMessage(sessionID, response.Content, MessageTypeAssistant)
}

// executePlugin 执行插件
func (bot *AIChatbot) executePlugin(session *Session, intent *Intent, content string) *PluginResponse {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	lang := session.Language
	if lang == "" {
		lang = LangChinese
	}

	// 查找匹配的插件
	for _, plugin := range bot.plugins {
		if !plugin.IsEnabled {
			continue
		}
		for _, pluginIntent := range plugin.Intents {
			if pluginIntent == intent.Type {
				ctx := &PluginContext{
					Session:  session,
					Intent:   intent,
					Language: lang,
				}
				resp, err := plugin.Handler(ctx)
				if err == nil && resp != nil {
					return resp
				}
			}
		}
	}

	// 默认响应
	return &PluginResponse{
		Content: bot.getTemplate(lang, "unknown_intent"),
	}
}

// InstallPlugin 安装插件
func (bot *AIChatbot) InstallPlugin(plugin *Plugin) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if _, exists := bot.plugins[plugin.ID]; exists {
		return fmt.Errorf("插件 %s 已存在", plugin.ID)
	}

	plugin.IsEnabled = true
	plugin.CreatedAt = time.Now()
	bot.plugins[plugin.ID] = plugin
	return nil
}

// UninstallPlugin 卸载插件
func (bot *AIChatbot) UninstallPlugin(pluginID string) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if _, exists := bot.plugins[pluginID]; !exists {
		return fmt.Errorf("插件 %s 不存在", pluginID)
	}

	delete(bot.plugins, pluginID)
	return nil
}

// EnablePlugin 启用插件
func (bot *AIChatbot) EnablePlugin(pluginID string) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	plugin, exists := bot.plugins[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 不存在", pluginID)
	}

	plugin.IsEnabled = true
	return nil
}

// DisablePlugin 禁用插件
func (bot *AIChatbot) DisablePlugin(pluginID string) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	plugin, exists := bot.plugins[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 不存在", pluginID)
	}

	plugin.IsEnabled = false
	return nil
}

// ListPlugins 列出插件
func (bot *AIChatbot) ListPlugins(enabledOnly bool) []*Plugin {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	plugins := make([]*Plugin, 0)
	for _, plugin := range bot.plugins {
		if enabledOnly && !plugin.IsEnabled {
			continue
		}
		plugins = append(plugins, plugin)
	}
	return plugins
}

// CreateScheduledTask 创建定时任务
func (bot *AIChatbot) CreateScheduledTask(task *ScheduledTask) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if _, exists := bot.scheduledTasks[task.ID]; exists {
		return fmt.Errorf("定时任务 %s 已存在", task.ID)
	}

	task.IsActive = true
	task.CreatedAt = time.Now()
	bot.scheduledTasks[task.ID] = task
	return nil
}

// DeleteScheduledTask 删除定时任务
func (bot *AIChatbot) DeleteScheduledTask(taskID string) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	if _, exists := bot.scheduledTasks[taskID]; !exists {
		return fmt.Errorf("定时任务 %s 不存在", taskID)
	}

	delete(bot.scheduledTasks, taskID)
	return nil
}

// GetScheduledTask 获取定时任务
func (bot *AIChatbot) GetScheduledTask(taskID string) (*ScheduledTask, error) {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	task, exists := bot.scheduledTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("定时任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListScheduledTasks 列出定时任务
func (bot *AIChatbot) ListScheduledTasks(userID string, activeOnly bool) []*ScheduledTask {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0)
	for _, task := range bot.scheduledTasks {
		if userID != "" && task.UserID != userID {
			continue
		}
		if activeOnly && !task.IsActive {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

// SetSessionLanguage 设置会话语言
func (bot *AIChatbot) SetSessionLanguage(sessionID string, lang Language) error {
	bot.mu.Lock()
	defer bot.mu.Unlock()

	session, exists := bot.sessions[sessionID]
	if !exists {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	session.Language = lang
	session.UpdatedAt = time.Now()
	return nil
}

// GetStats 获取统计信息
func (bot *AIChatbot) GetStats() *ChatStats {
	bot.mu.RLock()
	defer bot.mu.RUnlock()

	stats := &ChatStats{
		SessionsByUser: make(map[string]int),
	}

	stats.TotalSessions = len(bot.sessions)
	for _, msgs := range bot.messages {
		stats.TotalMessages += len(msgs)
	}

	for _, session := range bot.sessions {
		if session.IsActive {
			stats.ActiveSessions++
		}
		stats.SessionsByUser[session.UserID]++
	}

	stats.ScheduledTasks = len(bot.scheduledTasks)
	stats.InstalledPlugins = len(bot.plugins)

	return stats
}

// getTemplate 获取语言模板
func (bot *AIChatbot) getTemplate(lang Language, key string) string {
	if templates, ok := bot.langTemplates[lang]; ok {
		if tmpl, ok := templates[key]; ok {
			return tmpl
		}
	}
	// 默认返回中文
	if templates, ok := bot.langTemplates[LangChinese]; ok {
		if tmpl, ok := templates[key]; ok {
			return tmpl
		}
	}
	return key
}

// containsAny 检查是否包含任意关键字
func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if containsIgnoreCase(s, kw) {
			return true
		}
	}
	return false
}

// containsIgnoreCase 忽略大小写检查
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsStr(toLower(s), toLower(substr)))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// extractFolderName 提取文件夹名
func extractFolderName(content string) string {
	// 简单提取，实际应使用 NLP
	return "new_folder"
}

// extractServiceName 提取服务名
func extractServiceName(content string) string {
	// 简单提取，实际应使用 NLP
	return ""
}
