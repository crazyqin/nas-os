package aiwriter

import (
	"fmt"
	"strings"
	"time"
)

// GenerateText 生成文本.
func (m *Manager) GenerateText(req *WriteRequest) (*WriteResult, error) {
	if req.Content == "" {
		return nil, ErrInvalidInput
	}

	if req.Language == "" {
		req.Language = m.config.DefaultLanguage
	}
	if req.Style == "" {
		req.Style = m.config.DefaultStyle
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = m.config.MaxTokens
	}

	var result string
	var err error

	switch req.TaskType {
	case TaskSummary:
		result, err = m.generateSummary(req)
	case TaskExpand:
		result, err = m.generateExpand(req)
	case TaskRewrite:
		result, err = m.generateRewrite(req)
	case TaskTemplate:
		return nil, ErrInvalidInput
	default:
		return nil, ErrInvalidInput
	}

	if err != nil {
		return nil, err
	}

	writeResult := &WriteResult{
		ID:         fmt.Sprintf("write-%d", time.Now().UnixNano()),
		TaskType:   req.TaskType,
		Content:    req.Content,
		Result:     result,
		Language:   req.Language,
		Style:      req.Style,
		TokenCount: len([]rune(result)),
		CreatedAt:  time.Now(),
	}

	m.addToHistory(writeResult)
	return writeResult, nil
}

// FillTemplate 填充模板.
func (m *Manager) FillTemplate(req *TemplateRequest) (*WriteResult, error) {
	tmpl := m.getTemplate(req.TemplateID)
	if tmpl == nil {
		return nil, ErrNotFound
	}

	result := tmpl.Content
	for key, value := range req.Variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	writeResult := &WriteResult{
		ID:         fmt.Sprintf("write-%d", time.Now().UnixNano()),
		TaskType:   TaskTemplate,
		Content:    req.TemplateID,
		Result:     result,
		Language:   tmpl.Language,
		Style:      StyleFormal,
		TokenCount: len([]rune(result)),
		CreatedAt:  time.Now(),
	}

	m.addToHistory(writeResult)
	return writeResult, nil
}

func (m *Manager) generateSummary(req *WriteRequest) (string, error) {
	content := req.Content
	runes := []rune(content)

	var summary string
	switch req.Language {
	case LangChinese:
		if len(runes) > 200 {
			summary = string(runes[:200]) + "..."
		} else {
			summary = content
		}
	case LangEnglish:
		words := strings.Fields(content)
		if len(words) > 50 {
			summary = strings.Join(words[:50], " ") + "..."
		} else {
			summary = content
		}
	case LangJapanese:
		if len(runes) > 200 {
			summary = string(runes[:200]) + "..."
		} else {
			summary = content
		}
	default:
		summary = content
	}

	return summary, nil
}

func (m *Manager) generateExpand(req *WriteRequest) (string, error) {
	content := req.Content
	var expanded string

	switch req.Style {
	case StyleFormal:
		expanded = fmt.Sprintf("经过深入分析，%s。综上所述，这一观点具有重要的参考价值。", content)
	case StyleCasual:
		expanded = fmt.Sprintf("简单来说，%s。其实仔细想想，这还挺有意思的。", content)
	case StyleTechnical:
		expanded = fmt.Sprintf("基于技术分析，%s。该方案在技术实现上具有可行性。", content)
	default:
		expanded = content
	}

	return expanded, nil
}

func (m *Manager) generateRewrite(req *WriteRequest) (string, error) {
	content := req.Content
	var rewritten string

	switch req.Style {
	case StyleFormal:
		rewritten = fmt.Sprintf("正式表述：%s", content)
	case StyleCasual:
		rewritten = fmt.Sprintf("随便说说：%s", content)
	case StyleTechnical:
		rewritten = fmt.Sprintf("技术描述：%s", content)
	default:
		rewritten = content
	}

	return rewritten, nil
}

func (m *Manager) addToHistory(result *WriteResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, result)
	if len(m.history) > m.config.HistorySize {
		m.history = m.history[len(m.history)-m.config.HistorySize:]
	}
}

func (m *Manager) getTemplate(id string) *Template {
	for _, t := range m.templates {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// ListTemplates 列出模板.
func (m *Manager) ListTemplates() []*Template {
	return m.templates
}

// GetTemplate 获取模板.
func (m *Manager) GetTemplate(id string) (*Template, error) {
	tmpl := m.getTemplate(id)
	if tmpl == nil {
		return nil, ErrNotFound
	}
	return tmpl, nil
}

// GetHistory 获取历史记录.
func (m *Manager) GetHistory() []*WriteResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.history
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	taskCounts := make(map[TaskType]int)
	langCounts := make(map[Language]int)
	for _, h := range m.history {
		taskCounts[h.TaskType]++
		langCounts[h.Language]++
	}

	return map[string]interface{}{
		"total_writes": len(m.history),
		"task_counts":  taskCounts,
		"lang_counts":  langCounts,
		"templates":    len(m.templates),
	}
}
