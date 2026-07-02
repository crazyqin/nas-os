// Package onboardingwizard 提供开箱引导向导核心管理器
package onboardingwizard

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 引导管理器.
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	sessions map[string]*Session
}

// NewManager 创建引导管理器.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:   logger,
		sessions: make(map[string]*Session),
	}
}

// CreateSession 创建引导会话.
func (m *Manager) CreateSession(req CreateSessionRequest) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	template := GetTemplate(req.TemplateType)
	if template == nil {
		return nil, fmt.Errorf("template %q not found", req.TemplateType)
	}

	now := time.Now()
	session := &Session{
		ID:           uuid.New().String(),
		TemplateType: req.TemplateType,
		CreatedAt:    now,
		UpdatedAt:    now,
		CustomData:   make(map[string]any),
	}

	// 从模板创建步骤
	steps := make([]*Step, 0, len(template.Steps))
	for _, sc := range template.Steps {
		step := &Step{
			ID:       uuid.New().String(),
			Type:     sc.Type,
			Required: sc.Required,
			Status:   StepStatusPending,
			Order:    sc.Order,
		}
		step.Name, step.Description = getStepInfo(sc.Type)
		steps = append(steps, step)
	}
	session.Steps = steps

	// 初始化进度
	total := len(steps)
	session.Progress = &Progress{
		TotalSteps:     total,
		CompletedSteps: 0,
		SkippedSteps:   0,
		CurrentStep:    steps[0],
		Percentage:     0,
		StartedAt:      now,
		IsCompleted:    false,
	}

	m.sessions[session.ID] = session
	m.logger.Info("引导会话已创建",
		zap.String("session_id", session.ID),
		zap.String("template", string(req.TemplateType)))

	return session, nil
}

// GetSession 获取引导会话.
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	return session, nil
}

// ListSessions 列出所有会话.
func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})
	return sessions
}

// CompleteStep 完成步骤.
func (m *Manager) CompleteStep(sessionID, stepID string, data any) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	step := findStep(session.Steps, stepID)
	if step == nil {
		return nil, fmt.Errorf("step %q not found in session", stepID)
	}

	if step.Status == StepStatusCompleted {
		return nil, fmt.Errorf("step %q already completed", stepID)
	}

	if step.Status == StepStatusSkipped {
		return nil, fmt.Errorf("step %q was skipped, unskip it first", stepID)
	}

	now := time.Now()
	step.Status = StepStatusCompleted
	step.CompletedAt = &now
	step.Data = data

	// 更新进度
	m.updateProgress(session)

	session.UpdatedAt = now
	m.logger.Info("步骤已完成",
		zap.String("session_id", sessionID),
		zap.String("step_id", stepID),
		zap.String("step_type", string(step.Type)))

	return session, nil
}

// SkipStep 跳过步骤.
func (m *Manager) SkipStep(sessionID, stepID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	step := findStep(session.Steps, stepID)
	if step == nil {
		return nil, fmt.Errorf("step %q not found in session", stepID)
	}

	if step.Required {
		return nil, fmt.Errorf("step %q is required and cannot be skipped", stepID)
	}

	if step.Status == StepStatusCompleted {
		return nil, fmt.Errorf("step %q already completed", stepID)
	}

	if step.Status == StepStatusSkipped {
		return nil, fmt.Errorf("step %q already skipped", stepID)
	}

	now := time.Now()
	step.Status = StepStatusSkipped
	step.SkippedAt = &now

	// 更新进度
	m.updateProgress(session)

	session.UpdatedAt = now
	m.logger.Info("步骤已跳过",
		zap.String("session_id", sessionID),
		zap.String("step_id", stepID),
		zap.String("step_type", string(step.Type)))

	return session, nil
}

// UnskipStep 取消跳过步骤.
func (m *Manager) UnskipStep(sessionID, stepID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	step := findStep(session.Steps, stepID)
	if step == nil {
		return nil, fmt.Errorf("step %q not found in session", stepID)
	}

	if step.Status != StepStatusSkipped {
		return nil, fmt.Errorf("step %q is not skipped", stepID)
	}

	step.Status = StepStatusPending
	step.SkippedAt = nil

	// 更新进度
	m.updateProgress(session)

	session.UpdatedAt = time.Now()
	m.logger.Info("步骤已取消跳过",
		zap.String("session_id", sessionID),
		zap.String("step_id", stepID))

	return session, nil
}

// GetProgress 获取引导进度.
func (m *Manager) GetProgress(sessionID string) (*Progress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	return session.Progress, nil
}

// GetTemplates 获取所有模板.
func (m *Manager) GetTemplates() []*Template {
	return []*Template{
		GetTemplate(TemplateTypeHome),
		GetTemplate(TemplateTypeEnterprise),
		GetTemplate(TemplateTypeDeveloper),
	}
}

// GetRecommendations 获取功能推荐.
func (m *Manager) GetRecommendations(scenario string) []*RecommendedApp {
	apps := make([]*RecommendedApp, 0)
	switch scenario {
	case "home":
		apps = append(apps, &RecommendedApp{
			ID: "plex", Name: "Plex Media Server", Description: "家庭影音媒体中心",
			Icon: "🎬", Category: "media", Reason: "家庭娱乐必备", Tags: []string{"媒体", "流媒体"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "syncthing", Name: "Syncthing", Description: "去中心化文件同步",
			Icon: "🔄", Category: "sync", Reason: "多设备文件同步", Tags: []string{"同步", "备份"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "pi-hole", Name: "Pi-hole", Description: "网络广告拦截",
			Icon: "🛡️", Category: "network", Reason: "家庭网络安全", Tags: []string{"广告拦截", "DNS"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "homeassistant", Name: "Home Assistant", Description: "智能家居控制中心",
			Icon: "🏠", Category: "iot", Reason: "智能家居管理", Tags: []string{"IoT", "自动化"},
		})
	case "office":
		apps = append(apps, &RecommendedApp{
			ID: "nextcloud", Name: "Nextcloud", Description: "私有云协作平台",
			Icon: "☁️", Category: "productivity", Reason: "团队协作必备", Tags: []string{"协作", "云盘"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "onlyoffice", Name: "ONLYOFFICE", Description: "在线文档编辑",
			Icon: "📝", Category: "productivity", Reason: "文档协作", Tags: []string{"文档", "Office"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "gitlab", Name: "GitLab", Description: "代码托管与CI/CD",
			Icon: "🔧", Category: "development", Reason: "团队开发管理", Tags: []string{"Git", "CI/CD"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "vaultwarden", Name: "Vaultwarden", Description: "密码管理器",
			Icon: "🔐", Category: "security", Reason: "企业密码管理", Tags: []string{"密码", "安全"},
		})
	case "development":
		apps = append(apps, &RecommendedApp{
			ID: "gitea", Name: "Gitea", Description: "轻量级Git服务",
			Icon: "🐙", Category: "development", Reason: "代码版本管理", Tags: []string{"Git", "代码"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "portainer", Name: "Portainer", Description: "容器管理平台",
			Icon: "🐳", Category: "devops", Reason: "Docker管理", Tags: []string{"Docker", "容器"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "code-server", Name: "Code Server", Description: "在线VS Code",
			Icon: "💻", Category: "development", Reason: "远程开发", Tags: []string{"IDE", "编程"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "minio", Name: "MinIO", Description: "对象存储服务",
			Icon: "📦", Category: "storage", Reason: "S3兼容存储", Tags: []string{"S3", "存储"},
		})
	case "media":
		apps = append(apps, &RecommendedApp{
			ID: "plex", Name: "Plex Media Server", Description: "家庭影音媒体中心",
			Icon: "🎬", Category: "media", Reason: "媒体管理首选", Tags: []string{"媒体", "流媒体"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "jellyfin", Name: "Jellyfin", Description: "开源媒体系统",
			Icon: "🎭", Category: "media", Reason: "免费开源媒体中心", Tags: []string{"媒体", "开源"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "sonarr", Name: "Sonarr", Description: "剧集自动化管理",
			Icon: "📺", Category: "media", Reason: "自动追剧", Tags: []string{"电视剧", "自动化"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "radarr", Name: "Radarr", Description: "电影自动化管理",
			Icon: "🎥", Category: "media", Reason: "电影管理", Tags: []string{"电影", "自动化"},
		})
	case "backup":
		apps = append(apps, &RecommendedApp{
			ID: "duplicati", Name: "Duplicati", Description: "加密备份工具",
			Icon: "💾", Category: "backup", Reason: "数据备份首选", Tags: []string{"备份", "加密"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "syncthing", Name: "Syncthing", Description: "去中心化文件同步",
			Icon: "🔄", Category: "sync", Reason: "实时文件同步", Tags: []string{"同步", "P2P"},
		})
		apps = append(apps, &RecommendedApp{
			ID: "restic", Name: "Restic", Description: "快速安全备份",
			Icon: "🔒", Category: "backup", Reason: "增量备份", Tags: []string{"备份", "增量"},
		})
	}
	return apps
}

// updateProgress 更新进度（需在写锁下调用）.
func (m *Manager) updateProgress(session *Session) {
	total := len(session.Steps)
	completed := 0
	skipped := 0
	var currentStep *Step

	for _, step := range session.Steps {
		switch step.Status {
		case StepStatusCompleted:
			completed++
		case StepStatusSkipped:
			skipped++
		case StepStatusPending, StepStatusInProgress:
			if currentStep == nil {
				currentStep = step
			}
		}
	}

	percentage := 0.0
	if total > 0 {
		percentage = float64(completed+skipped) / float64(total) * 100
	}

	session.Progress = &Progress{
		TotalSteps:     total,
		CompletedSteps: completed,
		SkippedSteps:   skipped,
		CurrentStep:    currentStep,
		Percentage:     percentage,
		StartedAt:      session.Progress.StartedAt,
		IsCompleted:    completed+skipped == total,
	}

	if session.Progress.IsCompleted {
		now := time.Now()
		session.Progress.CompletedAt = &now
		session.CompletedAt = &now
		session.IsCompleted = true
	}
}

// findStep 查找步骤.
func findStep(steps []*Step, stepID string) *Step {
	for _, step := range steps {
		if step.ID == stepID {
			return step
		}
	}
	return nil
}

// getStepInfo 获取步骤信息.
func getStepInfo(stepType StepType) (name, description string) {
	switch stepType {
	case StepTypeNetwork:
		return "网络配置", "配置网络连接、IP地址和DNS设置"
	case StepTypeStoragePool:
		return "存储池创建", "创建存储池并配置RAID和文件系统"
	case StepTypeUserCreation:
		return "用户创建", "创建管理员账户和初始用户"
	case StepTypeAppInstall:
		return "应用安装", "安装推荐的应用程序"
	case StepTypeRecommend:
		return "功能推荐", "根据使用场景推荐功能和应用"
	default:
		return "未知步骤", ""
	}
}

// GetTemplate 获取模板.
func GetTemplate(templateType TemplateType) *Template {
	switch templateType {
	case TemplateTypeHome:
		return &Template{
			Type:        TemplateTypeHome,
			Name:        "家庭版",
			Description: "适合家庭用户，包含影音、智能家居、文件同步等功能",
			Steps: []StepConfig{
				{Type: StepTypeNetwork, Required: true, Order: 1},
				{Type: StepTypeStoragePool, Required: true, Order: 2},
				{Type: StepTypeUserCreation, Required: true, Order: 3},
				{Type: StepTypeRecommend, Required: false, Order: 4},
				{Type: StepTypeAppInstall, Required: false, Order: 5},
			},
			Apps: []string{"plex", "syncthing", "homeassistant"},
		}
	case TemplateTypeEnterprise:
		return &Template{
			Type:        TemplateTypeEnterprise,
			Name:        "企业版",
			Description: "适合企业用户，包含协作、安全、备份等功能",
			Steps: []StepConfig{
				{Type: StepTypeNetwork, Required: true, Order: 1},
				{Type: StepTypeStoragePool, Required: true, Order: 2},
				{Type: StepTypeUserCreation, Required: true, Order: 3},
				{Type: StepTypeRecommend, Required: false, Order: 4},
				{Type: StepTypeAppInstall, Required: false, Order: 5},
			},
			Apps: []string{"nextcloud", "gitlab", "vaultwarden"},
		}
	case TemplateTypeDeveloper:
		return &Template{
			Type:        TemplateTypeDeveloper,
			Name:        "开发者版",
			Description: "适合开发者，包含代码管理、容器、开发工具等功能",
			Steps: []StepConfig{
				{Type: StepTypeNetwork, Required: true, Order: 1},
				{Type: StepTypeStoragePool, Required: true, Order: 2},
				{Type: StepTypeUserCreation, Required: true, Order: 3},
				{Type: StepTypeRecommend, Required: false, Order: 4},
				{Type: StepTypeAppInstall, Required: false, Order: 5},
			},
			Apps: []string{"gitea", "portainer", "code-server"},
		}
	default:
		return nil
	}
}
