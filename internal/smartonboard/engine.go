package smartonboard

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var profileCounter int64

// OnboardStep 引导步骤.
type OnboardStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"` // storage, network, security, backup, app
	Required    bool   `json:"required"`
	Order       int    `json:"order"`
	Status      string `json:"status"` // pending, in_progress, completed, skipped
}

// OnboardProfile 引导配置.
type OnboardProfile struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Steps       []OnboardStep `json:"steps"`
	CreatedAt   time.Time     `json:"createdAt"`
	CompletedAt time.Time     `json:"completedAt,omitempty"`
	Progress    float64       `json:"progress"` // 0-100
}

// SystemHealth 系统健康状态.
type SystemHealth struct {
	Overall    string            `json:"overall"` // healthy, warning, critical
	Score      int               `json:"score"`   // 0-100
	Components map[string]string `json:"components"`
	Issues     []HealthIssue     `json:"issues"`
	CheckedAt  time.Time         `json:"checkedAt"`
}

// HealthIssue 健康问题.
type HealthIssue struct {
	ID          string `json:"id"`
	Component   string `json:"component"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Guidance    string `json:"guidance"` // 引导式修复建议
	Action      string `json:"action"`
}

// SmartOnboard 智能引导式初始化
// 对标 TrueNAS 引导式告警 + 飞牛 onboarding 体验.
type SmartOnboard struct {
	mu       sync.RWMutex
	profiles map[string]*OnboardProfile
	health   *SystemHealth
	issues   []HealthIssue
	stopCh   chan struct{}
	running  bool
}

// NewSmartOnboard 创建智能引导.
func NewSmartOnboard() *SmartOnboard {
	return &SmartOnboard{
		profiles: make(map[string]*OnboardProfile),
		health:   &SystemHealth{Components: make(map[string]string)},
		stopCh:   make(chan struct{}),
	}
}

// Start 启动.
func (s *SmartOnboard) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.healthLoop()
	log.Println("[SmartOnboard] 智能引导系统已启动")
}

// Stop 停止.
func (s *SmartOnboard) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stopCh)
	s.running = false
}

// CreateProfile 创建引导配置.
func (s *SmartOnboard) CreateProfile(name string) *OnboardProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile := &OnboardProfile{
		ID:        fmt.Sprintf("profile-%s-%04d", time.Now().Format("20060102150405"), atomic.AddInt64(&profileCounter, 1)),
		Name:      name,
		Steps:     defaultSteps(),
		CreatedAt: time.Now(),
	}
	s.profiles[profile.ID] = profile
	return profile
}

// CompleteStep 完成步骤.
func (s *SmartOnboard) CompleteStep(profileID, stepID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.profiles[profileID]
	if !ok {
		return false
	}
	for i, step := range profile.Steps {
		if step.ID == stepID {
			profile.Steps[i].Status = "completed"
			s.updateProgress(profile)
			return true
		}
	}
	return false
}

// SkipStep 跳过步骤.
func (s *SmartOnboard) SkipStep(profileID, stepID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.profiles[profileID]
	if !ok {
		return false
	}
	for i, step := range profile.Steps {
		if step.ID == stepID && !step.Required {
			profile.Steps[i].Status = "skipped"
			s.updateProgress(profile)
			return true
		}
	}
	return false
}

func (s *SmartOnboard) updateProgress(profile *OnboardProfile) {
	total := len(profile.Steps)
	done := 0
	for _, step := range profile.Steps {
		if step.Status == "completed" || step.Status == "skipped" {
			done++
		}
	}
	profile.Progress = float64(done) / float64(total) * 100
	if done == total {
		profile.CompletedAt = time.Now()
	}
}

// CheckHealth 检查系统健康.
func (s *SmartOnboard) CheckHealth() *SystemHealth {
	s.mu.Lock()
	defer s.mu.Unlock()

	health := &SystemHealth{
		Components: make(map[string]string),
		CheckedAt:  time.Now(),
	}

	// 模拟健康检查
	components := map[string]string{
		"storage":  "healthy",
		"network":  "healthy",
		"security": "healthy",
		"backup":   "warning",
		"services": "healthy",
	}
	health.Components = components

	issues := make([]HealthIssue, 0)
	// 检查备份
	issues = append(issues, HealthIssue{
		ID:          "backup-not-configured",
		Component:   "backup",
		Severity:    "warning",
		Description: "未配置自动备份策略",
		Guidance:    "建议配置定期备份以保护数据安全。进入「备份管理」创建备份任务。",
		Action:      "配置备份",
	})

	health.Issues = issues
	score := 100
	for _, c := range components {
		switch c {
		case "warning":
			score -= 10
		case "critical":
			score -= 30
		}
	}
	health.Score = score
	if score >= 80 {
		health.Overall = "healthy"
	} else if score >= 50 {
		health.Overall = "warning"
	} else {
		health.Overall = "critical"
	}

	s.health = health
	s.issues = issues
	return health
}

func (s *SmartOnboard) healthLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.CheckHealth()
		case <-s.stopCh:
			return
		}
	}
}

func defaultSteps() []OnboardStep {
	return []OnboardStep{
		{ID: "network", Title: "网络配置", Description: "配置网络接口和DNS", Category: "network", Required: true, Order: 1},
		{ID: "storage", Title: "存储池创建", Description: "创建存储池并配置RAID", Category: "storage", Required: true, Order: 2},
		{ID: "users", Title: "用户管理", Description: "创建管理员和普通用户", Category: "security", Required: true, Order: 3},
		{ID: "shares", Title: "共享文件夹", Description: "创建和配置共享文件夹", Category: "storage", Required: false, Order: 4},
		{ID: "backup", Title: "备份策略", Description: "配置自动备份", Category: "backup", Required: false, Order: 5},
		{ID: "apps", Title: "应用安装", Description: "从应用商店安装所需应用", Category: "app", Required: false, Order: 6},
		{ID: "security", Title: "安全加固", Description: "配置防火墙和访问控制", Category: "security", Required: false, Order: 7},
	}
}

// GetProfiles 获取引导配置.
func (s *SmartOnboard) GetProfiles() []*OnboardProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*OnboardProfile, 0, len(s.profiles))
	for _, p := range s.profiles {
		result = append(result, p)
	}
	return result
}

// GetHealth 获取健康状态.
func (s *SmartOnboard) GetHealth() *SystemHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.health
}

// GetIssues 获取问题列表.
func (s *SmartOnboard) GetIssues() []HealthIssue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.issues
}
