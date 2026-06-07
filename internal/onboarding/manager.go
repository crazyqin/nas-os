// Package onboarding 提供新手引导核心管理逻辑
package onboarding

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Manager 新手引导管理器
type Manager struct {
	mu            sync.RWMutex
	wizards       map[string]*Wizard
	guides        map[string]*Guide
	bestPractices map[string]*BestPractice
	progress      map[string]*Progress // key: userID:wizardID
}

// NewManager 创建新手引导管理器
func NewManager() *Manager {
	m := &Manager{
		wizards:       make(map[string]*Wizard),
		guides:        make(map[string]*Guide),
		bestPractices: make(map[string]*BestPractice),
		progress:      make(map[string]*Progress),
	}

	m.initDefaultWizard()
	m.initDefaultGuides()
	m.initDefaultBestPractices()

	return m
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) initDefaultWizard() {
	now := time.Now()
	wizard := &Wizard{
		ID:          "wizard-default",
		Name:        "NAS-OS 初始化向导",
		Description: "帮助您完成 NAS-OS 的首次配置",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []*Step{
			{ID: "step-network", WizardID: "wizard-default", Name: "网络配置", Description: "设置网络连接和 IP 地址", Position: 0, Type: "config", CreatedAt: now},
			{ID: "step-storage", WizardID: "wizard-default", Name: "存储池创建", Description: "创建存储池和卷", Position: 1, Type: "config", CreatedAt: now},
			{ID: "step-user", WizardID: "wizard-default", Name: "用户账户", Description: "创建管理员账户和设置密码", Position: 2, Type: "security", CreatedAt: now},
			{ID: "step-sharing", WizardID: "wizard-default", Name: "文件共享", Description: "配置 SMB/NFS 文件共享", Position: 3, Type: "config", CreatedAt: now},
			{ID: "step-backup", WizardID: "wizard-default", Name: "备份策略", Description: "设置自动备份计划", Position: 4, Type: "config", IsOptional: true, CreatedAt: now},
		},
	}

	m.wizards[wizard.ID] = wizard
}

func (m *Manager) initDefaultGuides() {
	guides := []*Guide{
		{ID: "guide-dashboard", Title: "仪表盘概览", Description: "了解 NAS-OS 仪表盘各项指标", Category: "getting-started", Content: "仪表盘显示系统状态、存储使用、网络流量等关键指标。", Icon: "dashboard", Tags: []string{"入门", "仪表盘"}, Duration: 5, CreatedAt: time.Now()},
		{ID: "guide-storage", Title: "存储管理入门", Description: "学习如何管理存储池和共享文件夹", Category: "storage", Content: "存储管理是 NAS 的核心功能，包括创建存储池、配置 RAID、管理共享文件夹。", Icon: "storage", Tags: []string{"存储", "入门"}, Duration: 10, CreatedAt: time.Now()},
		{ID: "guide-backup", Title: "数据备份指南", Description: "设置自动备份保护数据安全", Category: "backup", Content: "数据备份是防止数据丢失的关键。支持本地备份、远程备份和云备份。", Icon: "backup", Tags: []string{"备份", "安全"}, Duration: 8, CreatedAt: time.Now()},
		{ID: "guide-security", Title: "安全设置指南", Description: "加固 NAS 安全防护", Category: "security", Content: "安全设置包括防火墙配置、用户权限管理、SSL 证书等。", Icon: "security", Tags: []string{"安全", "防护"}, Duration: 12, CreatedAt: time.Now()},
		{ID: "guide-docker", Title: "Docker 容器入门", Description: "使用 Docker 部署应用", Category: "apps", Content: "Docker 容器化部署应用，支持镜像管理和容器编排。", Icon: "docker", Tags: []string{"Docker", "容器", "应用"}, Duration: 15, CreatedAt: time.Now()},
	}

	for _, g := range guides {
		m.guides[g.ID] = g
	}
}

func (m *Manager) initDefaultBestPractices() {
	practices := []*BestPractice{
		{ID: "bp-raid", Title: "RAID 配置建议", Description: "根据使用场景选择合适的 RAID 级别", Category: "storage", Content: "RAID 1 适合重要数据镜像，RAID 5 适合容量和安全平衡，RAID 6 适合大容量高安全需求。", Tags: []string{"RAID", "存储"}, Priority: 1, CreatedAt: time.Now()},
		{ID: "bp-321", Title: "3-2-1 备份策略", Description: "3 份副本、2 种介质、1 份异地", Category: "backup", Content: "重要数据保持 3 份副本，使用 2 种不同存储介质，其中 1 份存放在异地。", Tags: []string{"备份", "策略"}, Priority: 1, CreatedAt: time.Now()},
		{ID: "bp-permission", Title: "最小权限原则", Description: "只授予用户必要的最小权限", Category: "security", Content: "遵循最小权限原则，按需分配用户权限，定期审计权限配置。", Tags: []string{"安全", "权限"}, Priority: 2, CreatedAt: time.Now()},
		{ID: "bp-update", Title: "定期系统更新", Description: "保持系统和应用最新版本", Category: "maintenance", Content: "定期检查并安装系统更新，修复安全漏洞和性能问题。建议开启自动安全更新。", Tags: []string{"维护", "更新"}, Priority: 2, CreatedAt: time.Now()},
		{ID: "bp-monitor", Title: "系统监控告警", Description: "配置关键指标监控和告警", Category: "monitoring", Content: "监控 CPU、内存、磁盘、网络等关键指标，设置阈值告警及时发现问题。", Tags: []string{"监控", "告警"}, Priority: 3, CreatedAt: time.Now()},
	}

	for _, p := range practices {
		m.bestPractices[p.ID] = p
	}
}

// GetWizard 获取向导
func (m *Manager) GetWizard(id string) (*Wizard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wizard, ok := m.wizards[id]
	if !ok {
		return nil, fmt.Errorf("wizard not found: %s", id)
	}
	return wizard, nil
}

// ListWizards 列出所有向导
func (m *Manager) ListWizards() []*Wizard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wizards := make([]*Wizard, 0, len(m.wizards))
	for _, w := range m.wizards {
		wizards = append(wizards, w)
	}
	return wizards
}

// CompleteStep 完成步骤
func (m *Manager) CompleteStep(req *CompleteStepRequest) (*Progress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找步骤所属向导
	var wizard *Wizard
	for _, w := range m.wizards {
		for _, s := range w.Steps {
			if s.ID == req.StepID {
				wizard = w
				break
			}
		}
		if wizard != nil {
			break
		}
	}

	if wizard == nil {
		return nil, fmt.Errorf("step not found: %s", req.StepID)
	}

	key := req.UserID + ":" + wizard.ID
	prog, ok := m.progress[key]
	if !ok {
		prog = &Progress{
			UserID:         req.UserID,
			WizardID:       wizard.ID,
			CompletedSteps: make([]string, 0),
			TotalSteps:     len(wizard.Steps),
			StartedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		m.progress[key] = prog
	}

	// 检查是否已完成
	for _, s := range prog.CompletedSteps {
		if s == req.StepID {
			return prog, nil
		}
	}

	// 标记步骤完成
	for _, s := range wizard.Steps {
		if s.ID == req.StepID {
			s.IsCompleted = true
			break
		}
	}

	prog.CompletedSteps = append(prog.CompletedSteps, req.StepID)
	prog.CompletedCount = len(prog.CompletedSteps)
	prog.Percentage = float64(prog.CompletedCount) / float64(prog.TotalSteps) * 100
	prog.UpdatedAt = time.Now()

	if prog.CompletedCount >= prog.TotalSteps {
		prog.IsCompleted = true
		now := time.Now()
		prog.CompletedAt = &now
	}

	return prog, nil
}

// GetGuides 获取功能引导列表
func (m *Manager) GetGuides(category string) []*Guide {
	m.mu.RLock()
	defer m.mu.RUnlock()

	guides := make([]*Guide, 0)
	for _, g := range m.guides {
		if category == "" || g.Category == category {
			guides = append(guides, g)
		}
	}
	return guides
}

// GetGuide 获取单个引导
func (m *Manager) GetGuide(id string) (*Guide, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	guide, ok := m.guides[id]
	if !ok {
		return nil, fmt.Errorf("guide not found: %s", id)
	}
	return guide, nil
}

// RecommendPractice 推荐最佳实践
func (m *Manager) RecommendPractice(category string) []*BestPractice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	practices := make([]*BestPractice, 0)
	for _, p := range m.bestPractices {
		if category == "" || p.Category == category {
			practices = append(practices, p)
		}
	}

	// 按优先级排序
	for i := 0; i < len(practices)-1; i++ {
		for j := i + 1; j < len(practices); j++ {
			if practices[i].Priority > practices[j].Priority {
				practices[i], practices[j] = practices[j], practices[i]
			}
		}
	}

	return practices
}

// GetProgress 获取用户进度
func (m *Manager) GetProgress(userID, wizardID string) (*Progress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := userID + ":" + wizardID
	prog, ok := m.progress[key]
	if !ok {
		// 返回初始进度
		wizard, wOk := m.wizards[wizardID]
		if !wOk {
			return nil, fmt.Errorf("wizard not found: %s", wizardID)
		}
		return &Progress{
			UserID:         userID,
			WizardID:       wizardID,
			CompletedSteps: make([]string, 0),
			TotalSteps:     len(wizard.Steps),
			CompletedCount: 0,
			Percentage:     0,
			IsCompleted:    false,
			StartedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}, nil
	}

	return prog, nil
}

// ResetProgress 重置进度
func (m *Manager) ResetProgress(userID, wizardID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := userID + ":" + wizardID
	delete(m.progress, key)

	// 重置步骤完成状态
	if wizard, ok := m.wizards[wizardID]; ok {
		for _, s := range wizard.Steps {
			s.IsCompleted = false
		}
	}

	return nil
}
