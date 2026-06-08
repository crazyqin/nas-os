package securityaudit

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HardeningAdvisor 安全加固顾问.
type HardeningAdvisor struct {
	suggestions []HardeningSuggestion
	mu          sync.RWMutex
}

// NewHardeningAdvisor 创建安全加固顾问.
func NewHardeningAdvisor() *HardeningAdvisor {
	h := &HardeningAdvisor{
		suggestions: make([]HardeningSuggestion, 0),
	}
	h.registerDefaultSuggestions()
	return h
}

// registerDefaultSuggestions 注册默认加固建议.
func (h *HardeningAdvisor) registerDefaultSuggestions() {
	defaults := []HardeningSuggestion{
		// 认证加固
		{
			ID:          uuid.New().String(),
			Title:       "启用强密码策略",
			Description: "配置密码复杂度要求，强制使用强密码",
			Category:    HardeningAuth,
			Priority:    PriorityCritical,
			Effort:      EffortLow,
			Impact:      "防止暴力破解和弱密码攻击",
			Steps: []string{
				"设置最小密码长度为 12 位",
				"要求包含大小写字母、数字和特殊字符",
				"启用密码历史检查，防止重复使用",
				"设置密码过期策略，强制定期更换",
			},
			Commands: []string{
				"pam-config --add pam_pwquality",
				"echo 'minlen=12 minclass=4' >> /etc/security/pwquality.conf",
			},
			References: []string{
				"https://wiki.archlinux.org/title/Security#Passwords",
				"https://pages.nist.gov/800-63-3/sp800-63b.html",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "启用多因素认证",
			Description: "为所有管理员账户启用多因素认证",
			Category:    HardeningAuth,
			Priority:    PriorityCritical,
			Effort:      EffortMedium,
			Impact:      "大幅提升账户安全性，防止凭证泄露",
			Steps: []string{
				"安装 Google Authenticator 或其他 TOTP 应用",
				"配置 PAM 模块支持 TOTP",
				"为管理员账户注册 MFA",
				"测试 MFA 登录功能",
			},
			Commands: []string{
				"apt install libpam-google-authenticator",
				"google-authenticator",
			},
			References: []string{
				"https://github.com/google/google-authenticator-libpam",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "配置账户锁定策略",
			Description: "防止暴力破解攻击",
			Category:    HardeningAuth,
			Priority:    PriorityHigh,
			Effort:      EffortLow,
			Impact:      "防止暴力破解攻击",
			Steps: []string{
				"配置失败登录尝试次数限制",
				"设置锁定持续时间",
				"配置锁定通知",
			},
			Commands: []string{
				"echo 'auth required pam_tally2.so deny=5 unlock_time=1800' >> /etc/pam.d/common-auth",
			},
			References: []string{},
			CreatedAt: time.Now(),
		},
		// 网络加固
		{
			ID:          uuid.New().String(),
			Title:       "强化 SSH 配置",
			Description: "增强 SSH 服务的安全性",
			Category:    HardeningNetwork,
			Priority:    PriorityCritical,
			Effort:      EffortLow,
			Impact:      "防止 SSH 暴力破解和未授权访问",
			Steps: []string{
				"禁用 root 远程登录",
				"禁用密码认证，只允许密钥认证",
				"更改默认 SSH 端口",
				"限制允许登录的用户",
				"配置连接超时",
			},
			Commands: []string{
				"sed -i 's/#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config",
				"sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config",
				"sed -i 's/#Port 22/Port 2222/' /etc/ssh/sshd_config",
				"systemctl restart sshd",
			},
			References: []string{
				"https://wiki.archlinux.org/title/Secure_Shell",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "配置防火墙规则",
			Description: "启用并配置防火墙，实施最小开放原则",
			Category:    HardeningNetwork,
			Priority:    PriorityCritical,
			Effort:      EffortMedium,
			Impact:      "阻止未授权的网络访问",
			Steps: []string{
				"启用防火墙",
				"设置默认拒绝策略",
				"只开放必要的端口",
				"配置入站和出站规则",
				"启用日志记录",
			},
			Commands: []string{
				"ufw enable",
				"ufw default deny incoming",
				"ufw default allow outgoing",
				"ufw allow 2222/tcp  # SSH",
				"ufw allow 443/tcp   # HTTPS",
			},
			References: []string{
				"https://help.ubuntu.com/community/UFW",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "禁用不安全的网络服务",
			Description: "关闭不必要的网络服务和端口",
			Category:    HardeningNetwork,
			Priority:    PriorityHigh,
			Effort:      EffortLow,
			Impact:      "减少攻击面",
			Steps: []string{
				"识别所有运行的网络服务",
				"禁用不必要的服务",
				"关闭不安全的协议（如 Telnet、FTP）",
			},
			Commands: []string{
				"systemctl disable telnet",
				"systemctl disable ftp",
				"systemctl disable rsh",
			},
			References: []string{},
			CreatedAt: time.Now(),
		},
		// 系统加固
		{
			ID:          uuid.New().String(),
			Title:       "保持系统更新",
			Description: "定期安装安全更新和补丁",
			Category:    HardeningPatch,
			Priority:    PriorityCritical,
			Effort:      EffortLow,
			Impact:      "修复已知漏洞",
			Steps: []string{
				"配置自动安全更新",
				"定期检查并安装更新",
				"订阅安全公告",
			},
			Commands: []string{
				"apt update && apt upgrade -y",
				"apt install unattended-upgrades",
				"dpkg-reconfigure -plow unattended-upgrades",
			},
			References: []string{
				"https://wiki.debian.org/UnattendedUpgrades",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "配置内核安全参数",
			Description: "优化内核参数以增强安全性",
			Category:    HardeningSystem,
			Priority:    PriorityHigh,
			Effort:      EffortMedium,
			Impact:      "增强系统整体安全性",
			Steps: []string{
				"启用地址空间布局随机化 (ASLR)",
				"禁用 IP 转发（如果不需要）",
				"启用 SYN Cookie 防护",
				"禁用核心转储",
				"限制内核指针泄露",
			},
			Commands: []string{
				"echo 'kernel.randomize_va_space=2' >> /etc/sysctl.conf",
				"echo 'net.ipv4.ip_forward=0' >> /etc/sysctl.conf",
				"echo 'net.ipv4.tcp_syncookies=1' >> /etc/sysctl.conf",
				"echo 'fs.suid_dumpable=0' >> /etc/sysctl.conf",
				"echo 'kernel.kptr_restrict=2' >> /etc/sysctl.conf",
				"sysctl -p",
			},
			References: []string{
				"https://wiki.archlinux.org/title/Security#Kernel",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "配置文件权限",
			Description: "设置关键系统文件的正确权限",
			Category:    HardeningSystem,
			Priority:    PriorityHigh,
			Effort:      EffortLow,
			Impact:      "防止未授权访问敏感文件",
			Steps: []string{
				"设置 /etc/passwd 权限为 644",
				"设置 /etc/shadow 权限为 640",
				"设置 /etc/ssh 权限为 755",
				"设置 SSH 私钥权限为 600",
			},
			Commands: []string{
				"chmod 644 /etc/passwd",
				"chmod 640 /etc/shadow",
				"chmod 755 /etc/ssh",
				"chmod 600 /etc/ssh/ssh_host_*_key",
			},
			References: []string{},
			CreatedAt: time.Now(),
		},
		// 加密加固
		{
			ID:          uuid.New().String(),
			Title:       "启用磁盘加密",
			Description: "对数据分区进行全盘加密",
			Category:    HardeningCrypto,
			Priority:    PriorityCritical,
			Effort:      EffortHigh,
			Impact:      "保护数据在物理介质上的安全",
			Steps: []string{
				"备份重要数据",
				"创建加密分区",
				"格式化并挂载加密分区",
				"迁移数据到加密分区",
				"配置自动挂载",
			},
			Commands: []string{
				"cryptsetup luksFormat /dev/sdX",
				"cryptsetup luksOpen /dev/sdX encrypted_data",
				"mkfs.ext4 /dev/mapper/encrypted_data",
			},
			References: []string{
				"https://wiki.archlinux.org/title/Dm-crypt/Encrypting_an_entire_system",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "配置 TLS 1.3",
			Description: "使用最新的 TLS 协议版本",
			Category:    HardeningCrypto,
			Priority:    PriorityHigh,
			Effort:      EffortLow,
			Impact:      "提升传输层安全性",
			Steps: []string{
				"更新 OpenSSL 到最新版本",
				"配置 Nginx 使用 TLS 1.3",
				"禁用旧版本 TLS",
				"配置安全的密码套件",
			},
			Commands: []string{
				"echo 'ssl_protocols TLSv1.3;' >> /etc/nginx/nginx.conf",
				"echo 'ssl_ciphers TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256;' >> /etc/nginx/nginx.conf",
			},
			References: []string{
				"https://ssl-config.mozilla.org/",
			},
			CreatedAt: time.Now(),
		},
		// 访问控制加固
		{
			ID:          uuid.New().String(),
			Title:       "实施最小权限原则",
			Description: "为用户和进程分配最小必要权限",
			Category:    HardeningAccess,
			Priority:    PriorityHigh,
			Effort:      EffortMedium,
			Impact:      "限制潜在损害范围",
			Steps: []string{
				"审计当前权限分配",
				"创建角色分离的用户组",
				"移除不必要的 sudo 权限",
				"使用 ACL 进行细粒度控制",
			},
			Commands: []string{
				"visudo  # 编辑 sudoers 文件",
				"setfacl -m u:username:rwx /path/to/resource",
			},
			References: []string{
				"https://wiki.archlinux.org/title/Users_and_groups",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "启用审计日志",
			Description: "配置系统审计日志以追踪安全事件",
			Category:    HardeningAudit,
			Priority:    PriorityHigh,
			Effort:      EffortMedium,
			Impact:      "提供安全事件追踪和取证能力",
			Steps: []string{
				"安装 auditd",
				"配置审计规则",
				"设置日志轮转",
				"配置远程日志服务器",
			},
			Commands: []string{
				"apt install auditd",
				"systemctl enable auditd",
				"echo '-w /etc/passwd -p wa -k identity' >> /etc/audit/rules.d/audit.rules",
				"echo '-w /etc/shadow -p wa -k identity' >> /etc/audit/rules.d/audit.rules",
			},
			References: []string{
				"https://wiki.archlinux.org/title/Audit_framework",
			},
			CreatedAt: time.Now(),
		},
		// 备份加固
		{
			ID:          uuid.New().String(),
			Title:       "加密备份数据",
			Description: "对备份数据进行加密保护",
			Category:    HardeningBackup,
			Priority:    PriorityHigh,
			Effort:      EffortMedium,
			Impact:      "保护备份数据不被未授权访问",
			Steps: []string{
				"选择加密算法（AES-256）",
				"生成并安全存储加密密钥",
				"配置备份软件加密选项",
				"测试备份和恢复流程",
			},
			Commands: []string{
				"gpg --symmetric --cipher-algo AES256 backup.tar.gz",
				"restic -r /backup init --password-file key.txt",
			},
			References: []string{
				"https://restic.readthedocs.io/en/latest/030_preparing_a_new_repo.html",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "配置异地备份",
			Description: "实施 3-2-1 备份策略",
			Category:    HardeningBackup,
			Priority:    PriorityHigh,
			Effort:      EffortHigh,
			Impact:      "防止数据丢失",
			Steps: []string{
				"配置本地备份",
				"配置异地备份",
				"配置云备份（可选）",
				"定期测试恢复",
			},
			Commands: []string{
				"rsync -avz /data/ remote:/backup/",
				"rclone sync /data/ remote:backup/",
			},
			References: []string{
				"https://www.backblaze.com/blog/the-3-2-1-backup-strategy/",
			},
			CreatedAt: time.Now(),
		},
		// 容器加固
		{
			ID:          uuid.New().String(),
			Title:       "启用容器镜像扫描",
			Description: "对容器镜像进行安全漏洞扫描",
			Category:    HardeningContainer,
			Priority:    PriorityHigh,
			Effort:      EffortLow,
			Impact:      "防止部署包含漏洞的容器",
			Steps: []string{
				"安装容器扫描工具（Trivy/Clair）",
				"配置 CI/CD 集成扫描",
			"设置漏洞阈值告警",
				"定期扫描本地镜像",
			},
			Commands: []string{
				"apt install trivy",
				"trivy image myapp:latest",
			},
			References: []string{
				"https://aquasecurity.github.io/trivy/",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Title:       "配置容器运行时安全",
			Description: "限制容器运行时权限",
			Category:    HardeningContainer,
			Priority:    PriorityMedium,
			Effort:      EffortMedium,
			Impact:      "限制容器逃逸和权限提升",
			Steps: []string{
				"使用只读根文件系统",
				"限制容器资源",
				"禁用特权模式",
				"使用非 root 用户运行",
				"启用 seccomp 和 AppArmor",
			},
			Commands: []string{
				"docker run --read-only --tmpfs /tmp myapp",
				"docker run --memory=512m --cpus=1 myapp",
				"docker run --security-opt=no-new-privileges myapp",
			},
			References: []string{
				"https://docs.docker.com/engine/security/",
			},
			CreatedAt: time.Now(),
		},
	}
	h.suggestions = defaults
}

// GenerateSuggestions 根据检查结果生成加固建议.
func (h *HardeningAdvisor) GenerateSuggestions(checkResults []SecurityCheckResult) []HardeningSuggestion {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 根据失败的检查项返回相关建议
	suggestions := make([]HardeningSuggestion, 0)

	for _, result := range checkResults {
		if result.Status == StatusFail || result.Status == StatusWarning {
			// 查找相关建议
			for _, s := range h.suggestions {
				if h.isRelated(s, result) {
					suggestions = append(suggestions, s)
				}
			}
		}
	}

	// 去重
	seen := make(map[string]bool)
	unique := make([]HardeningSuggestion, 0)
	for _, s := range suggestions {
		if !seen[s.ID] {
			seen[s.ID] = true
			unique = append(unique, s)
		}
	}

	return unique
}

// isRelated 检查建议是否与检查结果相关.
func (h *HardeningAdvisor) isRelated(suggestion HardeningSuggestion, result SecurityCheckResult) bool {
	// 根据类别匹配
	categoryMap := map[SecurityCheckCategory]HardeningCategory{
		CategoryAuth:      HardeningAuth,
		CategoryNetwork:   HardeningNetwork,
		CategorySystem:    HardeningSystem,
		CategoryFile:      HardeningFile,
		CategoryCrypto:    HardeningCrypto,
		CategoryAccess:    HardeningAccess,
		CategoryPatch:     HardeningPatch,
		CategoryBackup:    HardeningBackup,
		CategoryContainer: HardeningContainer,
	}

	if targetCategory, ok := categoryMap[result.Category]; ok {
		return suggestion.Category == targetCategory
	}

	return false
}

// GetSuggestionsByCategory 按类别获取建议.
func (h *HardeningAdvisor) GetSuggestionsByCategory(category HardeningCategory) []HardeningSuggestion {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]HardeningSuggestion, 0)
	for _, s := range h.suggestions {
		if s.Category == category {
			result = append(result, s)
		}
	}
	return result
}

// GetSuggestion 获取建议详情.
func (h *HardeningAdvisor) GetSuggestion(id string) (*HardeningSuggestion, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, s := range h.suggestions {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("建议 %s 不存在", id)
}

// GenerateReport 生成加固报告.
func (h *HardeningAdvisor) GenerateReport(checkResults []SecurityCheckResult) HardeningReport {
	suggestions := h.GenerateSuggestions(checkResults)

	criticalCount, highCount, mediumCount, lowCount := 0, 0, 0, 0
	appliedCount := 0

	for _, s := range suggestions {
		switch s.Priority {
		case PriorityCritical:
			criticalCount++
		case PriorityHigh:
			highCount++
		case PriorityMedium:
			mediumCount++
		case PriorityLow:
			lowCount++
		}
		if s.Applied {
			appliedCount++
		}
	}

	// 计算应用所有建议后的预计分数提升
	scoreImpact := criticalCount*5 + highCount*3 + mediumCount*1

	return HardeningReport{
		ReportID:      uuid.New().String(),
		GeneratedAt:   time.Now(),
		TotalItems:    len(suggestions),
		CriticalCount: criticalCount,
		HighCount:     highCount,
		MediumCount:   mediumCount,
		LowCount:      lowCount,
		AppliedCount:  appliedCount,
		Suggestions:   suggestions,
		ScoreImpact:   scoreImpact,
	}
}

// Apply 应用加固建议.
func (h *HardeningAdvisor) Apply(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, s := range h.suggestions {
		if s.ID == id {
			h.suggestions[i].Applied = true
			now := time.Now()
			h.suggestions[i].AppliedAt = &now
			return nil
		}
	}
	return fmt.Errorf("建议 %s 不存在", id)
}

// Dismiss 忽略加固建议.
func (h *HardeningAdvisor) Dismiss(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, s := range h.suggestions {
		if s.ID == id {
			// 标记为已应用（忽略也是一种处理）
			h.suggestions[i].Applied = true
			now := time.Now()
			h.suggestions[i].AppliedAt = &now
			return nil
		}
	}
	return fmt.Errorf("建议 %s 不存在", id)
}

// AddSuggestion 添加自定义建议.
func (h *HardeningAdvisor) AddSuggestion(suggestion HardeningSuggestion) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查 ID 是否已存在
	for _, s := range h.suggestions {
		if s.ID == suggestion.ID {
			return fmt.Errorf("建议 ID %s 已存在", suggestion.ID)
		}
	}

	h.suggestions = append(h.suggestions, suggestion)
	return nil
}

// RemoveSuggestion 移除建议.
func (h *HardeningAdvisor) RemoveSuggestion(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, s := range h.suggestions {
		if s.ID == id {
			h.suggestions = append(h.suggestions[:i], h.suggestions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("建议 %s 不存在", id)
}
