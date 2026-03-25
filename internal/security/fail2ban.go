package security

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Fail2BanManager 失败登录保护管理器.
type Fail2BanManager struct {
	config          Fail2BanConfig
	failedAttempts  map[string][]FailedLoginAttempt // IP -> 失败尝试列表
	bannedIPs       map[string]*BannedIP
	accountLockouts map[string]*AccountLockout // Username -> 锁定状态
	mu              sync.RWMutex
	notifyFunc      func(alert Alert) // 通知回调
}

// NewFail2BanManager 创建失败登录保护管理器.
func NewFail2BanManager() *Fail2BanManager {
	return &Fail2BanManager{
		config: Fail2BanConfig{
			Enabled:            true,
			MaxAttempts:        5,
			WindowMinutes:      10,
			BanDurationMinutes: 60,
			AutoUnban:          true,
			NotifyOnBan:        true,
			ProtectedServices:  []string{"ssh", "webui", "api"},
		},
		failedAttempts:  make(map[string][]FailedLoginAttempt),
		bannedIPs:       make(map[string]*BannedIP),
		accountLockouts: make(map[string]*AccountLockout),
	}
}

// SetNotifyFunc 设置通知回调函数.
func (f2m *Fail2BanManager) SetNotifyFunc(notifyFn func(alert Alert)) {
	f2m.mu.Lock()
	defer f2m.mu.Unlock()
	f2m.notifyFunc = notifyFn
}

// GetConfig 获取配置.
func (f2m *Fail2BanManager) GetConfig() Fail2BanConfig {
	f2m.mu.RLock()
	defer f2m.mu.RUnlock()
	return f2m.config
}

// UpdateConfig 更新配置.
func (f2m *Fail2BanManager) UpdateConfig(config Fail2BanConfig) error {
	f2m.mu.Lock()
	defer f2m.mu.Unlock()

	// 验证配置
	if config.MaxAttempts < 1 {
		return fmt.Errorf("最大尝试次数必须大于 0")
	}
	if config.WindowMinutes < 1 {
		return fmt.Errorf("时间窗口必须大于 0 分钟")
	}
	if config.BanDurationMinutes < 1 {
		return fmt.Errorf("封禁时长必须大于 0 分钟")
	}

	f2m.config = config

	// 如果禁用，解封所有 IP
	if !config.Enabled {
		for ip := range f2m.bannedIPs {
			_ = f2m.unbanIPInternal(ip)
		}
		f2m.bannedIPs = make(map[string]*BannedIP)
	}

	return nil
}

// RecordFailedLogin 记录失败登录尝试.
func (f2m *Fail2BanManager) RecordFailedLogin(ip, username, userAgent, reason string) error {
	f2m.mu.Lock()
	defer f2m.mu.Unlock()

	if !f2m.config.Enabled {
		return nil
	}

	now := time.Now()

	// 记录失败尝试
	attempt := FailedLoginAttempt{
		IP:        ip,
		Username:  username,
		Timestamp: now,
		UserAgent: userAgent,
		Reason:    reason,
	}

	f2m.failedAttempts[ip] = append(f2m.failedAttempts[ip], attempt)

	// 清理旧的尝试记录（超出时间窗口）
	windowStart := now.Add(-time.Duration(f2m.config.WindowMinutes) * time.Minute)
	attempts := f2m.failedAttempts[ip]
	validAttempts := make([]FailedLoginAttempt, 0)
	for _, a := range attempts {
		if a.Timestamp.After(windowStart) {
			validAttempts = append(validAttempts, a)
		}
	}
	f2m.failedAttempts[ip] = validAttempts

	// 检查是否达到封禁阈值
	if len(validAttempts) >= f2m.config.MaxAttempts {
		// 封禁 IP
		if err := f2m.banIPLocked(ip, username, len(validAttempts)); err != nil {
			return err
		}

		// 清理该 IP 的失败记录
		delete(f2m.failedAttempts, ip)
	}

	// 检查账户锁定
	f2m.checkAccountLockout(username)

	return nil
}

// banIPLocked 封禁 IP（已持有锁）.
func (f2m *Fail2BanManager) banIPLocked(ip, username string, attempts int) error {
	// 检查是否在白名单中
	// 这里应该检查防火墙白名单，简化处理跳过

	now := time.Now()
	expiresAt := now.Add(time.Duration(f2m.config.BanDurationMinutes) * time.Minute)

	bannedIP := &BannedIP{
		IP:        ip,
		Reason:    fmt.Sprintf("失败登录尝试次数过多 (%d 次)", attempts),
		BannedAt:  now,
		ExpiresAt: expiresAt,
		Attempts:  attempts,
	}

	f2m.bannedIPs[ip] = bannedIP

	// 应用到系统防火墙
	if err := f2m.applyBan(ip); err != nil {
		return fmt.Errorf("应用封禁失败：%w", err)
	}

	// 发送通知
	if f2m.config.NotifyOnBan && f2m.notifyFunc != nil {
		alert := Alert{
			ID:          generateAlertID(),
			Timestamp:   now,
			Severity:    "high",
			Type:        "ip_banned",
			Title:       "IP 地址已被封禁",
			Description: fmt.Sprintf("IP %s 因失败登录尝试次数过多 (%d 次) 已被封禁", ip, attempts),
			SourceIP:    ip,
			Username:    username,
			Details: map[string]interface{}{
				"attempts":             attempts,
				"ban_duration_minutes": f2m.config.BanDurationMinutes,
			},
		}
		go f2m.notifyFunc(alert)
	}

	return nil
}

// applyBan 应用封禁到系统防火墙.
func (f2m *Fail2BanManager) applyBan(ip string) error {
	// 使用 iptables 封禁 IP
	cmd := exec.CommandContext(context.Background(), "iptables", "-I", "INPUT", "-s", ip, "-j", "DROP")
	if err := cmd.Run(); err != nil {
		// 非 root 环境，尝试使用 fail2ban-client
		return f2m.applyBanViaFail2Ban(ip)
	}

	// IPv6 支持
	if net.ParseIP(ip).To4() == nil {
		cmd = exec.CommandContext(context.Background(), "ip6tables", "-I", "INPUT", "-s", ip, "-j", "DROP")
		_ = cmd.Run()
	}

	return nil
}

// applyBanViaFail2Ban 通过 fail2ban-client 应用封禁.
func (f2m *Fail2BanManager) applyBanViaFail2Ban(ip string) error {
	// 检查 fail2ban-client 是否存在
	cmd := exec.CommandContext(context.Background(), "which", "fail2ban-client")
	if err := cmd.Run(); err != nil {
		return nil // fail2ban 未安装
	}

	// 使用 fail2ban-client 封禁
	jails := []string{"sshd", "nginx-http-auth", "nas-os-auth"}
	for _, jail := range jails {
		cmd = exec.CommandContext(context.Background(), "fail2ban-client", "set", jail, "banip", ip)
		_ = cmd.Run()
	}

	return nil
}

// UnbanIP 解封 IP.
func (f2m *Fail2BanManager) UnbanIP(ip string) error {
	f2m.mu.Lock()
	defer f2m.mu.Unlock()

	return f2m.unbanIPInternal(ip)
}

// unbanIPInternal 解封 IP（已持有锁）.
func (f2m *Fail2BanManager) unbanIPInternal(ip string) error {
	if _, exists := f2m.bannedIPs[ip]; !exists {
		return fmt.Errorf("IP 未被封禁")
	}

	delete(f2m.bannedIPs, ip)

	// 从系统防火墙移除
	if err := f2m.removeBan(ip); err != nil {
		return fmt.Errorf("移除封禁失败：%w", err)
	}

	return nil
}

// removeBan 从系统防火墙移除封禁.
func (f2m *Fail2BanManager) removeBan(ip string) error {
	cmd := exec.CommandContext(context.Background(), "iptables", "-D", "INPUT", "-s", ip, "-j", "DROP")
	_ = cmd.Run()

	if net.ParseIP(ip).To4() == nil {
		cmd = exec.CommandContext(context.Background(), "ip6tables", "-D", "INPUT", "-s", ip, "-j", "DROP")
		_ = cmd.Run()
	}

	// 也尝试通过 fail2ban-client 解封
	cmd = exec.CommandContext(context.Background(), "which", "fail2ban-client")
	if err := cmd.Run(); err == nil {
		jails := []string{"sshd", "nginx-http-auth", "nas-os-auth"}
		for _, jail := range jails {
			cmd = exec.CommandContext(context.Background(), "fail2ban-client", "set", jail, "unbanip", ip)
			_ = cmd.Run()
		}
	}

	return nil
}

// GetBannedIPs 获取被封禁的 IP 列表.
func (f2m *Fail2BanManager) GetBannedIPs() []*BannedIP {
	f2m.mu.RLock()
	defer f2m.mu.RUnlock()

	ips := make([]*BannedIP, 0, len(f2m.bannedIPs))
	for _, banned := range f2m.bannedIPs {
		ips = append(ips, banned)
	}
	return ips
}

// IsBanned 检查 IP 是否被封禁.
func (f2m *Fail2BanManager) IsBanned(ip string) bool {
	f2m.mu.RLock()
	defer f2m.mu.RUnlock()

	banned, exists := f2m.bannedIPs[ip]
	if !exists {
		return false
	}

	// 检查是否过期
	if time.Now().After(banned.ExpiresAt) {
		return false
	}

	return true
}

// checkAccountLockout 检查账户锁定.
func (f2m *Fail2BanManager) checkAccountLockout(username string) {
	// 统计该用户的失败尝试
	attempts := 0
	for _, ipAttempts := range f2m.failedAttempts {
		for _, attempt := range ipAttempts {
			if attempt.Username == username {
				attempts++
			}
		}
	}

	// 如果失败次数过多，锁定账户
	if attempts >= f2m.config.MaxAttempts*2 {
		expiresAt := time.Now().Add(time.Duration(f2m.config.BanDurationMinutes*2) * time.Minute)
		f2m.accountLockouts[username] = &AccountLockout{
			Username:    username,
			LockedAt:    time.Now(),
			ExpiresAt:   &expiresAt,
			FailedCount: attempts,
		}

		// 发送账户锁定通知
		if f2m.notifyFunc != nil {
			alert := Alert{
				ID:          generateAlertID(),
				Timestamp:   time.Now(),
				Severity:    "critical",
				Type:        "account_locked",
				Title:       "账户已被锁定",
				Description: fmt.Sprintf("账户 %s 因多次失败登录尝试已被锁定", username),
				Username:    username,
				Details: map[string]interface{}{
					"failed_count": attempts,
				},
			}
			go f2m.notifyFunc(alert)
		}
	}
}

// IsAccountLocked 检查账户是否被锁定.
func (f2m *Fail2BanManager) IsAccountLocked(username string) bool {
	f2m.mu.RLock()
	defer f2m.mu.RUnlock()

	lockout, exists := f2m.accountLockouts[username]
	if !exists {
		return false
	}

	// 检查是否过期
	if lockout.ExpiresAt != nil && time.Now().After(*lockout.ExpiresAt) {
		return false
	}

	return true
}

// UnlockAccount 解锁账户.
func (f2m *Fail2BanManager) UnlockAccount(username string) error {
	f2m.mu.Lock()
	defer f2m.mu.Unlock()

	if _, exists := f2m.accountLockouts[username]; !exists {
		return fmt.Errorf("账户未被锁定")
	}

	delete(f2m.accountLockouts, username)
	return nil
}

// GetFailedAttempts 获取指定 IP 的失败尝试记录.
func (f2m *Fail2BanManager) GetFailedAttempts(ip string) []FailedLoginAttempt {
	f2m.mu.RLock()
	defer f2m.mu.RUnlock()

	attempts, exists := f2m.failedAttempts[ip]
	if !exists {
		return []FailedLoginAttempt{}
	}

	result := make([]FailedLoginAttempt, len(attempts))
	copy(result, attempts)
	return result
}

// GetStatus 获取失败登录保护状态.
func (f2m *Fail2BanManager) GetStatus() map[string]interface{} {
	f2m.mu.RLock()
	defer f2m.mu.RUnlock()

	now := time.Now()
	activeBans := 0
	expiredBans := 0
	for _, banned := range f2m.bannedIPs {
		if now.Before(banned.ExpiresAt) {
			activeBans++
		} else {
			expiredBans++
		}
	}

	activeLockouts := 0
	for _, lockout := range f2m.accountLockouts {
		if lockout.ExpiresAt == nil || now.Before(*lockout.ExpiresAt) {
			activeLockouts++
		}
	}

	return map[string]interface{}{
		"enabled":              f2m.config.Enabled,
		"max_attempts":         f2m.config.MaxAttempts,
		"window_minutes":       f2m.config.WindowMinutes,
		"ban_duration_minutes": f2m.config.BanDurationMinutes,
		"active_bans":          activeBans,
		"total_bans":           len(f2m.bannedIPs),
		"active_lockouts":      activeLockouts,
		"tracked_ips":          len(f2m.failedAttempts),
	}
}

// CleanupExpired 清理过期的封禁和锁定.
func (f2m *Fail2BanManager) CleanupExpired() {
	f2m.mu.Lock()
	defer f2m.mu.Unlock()

	now := time.Now()

	// 清理过期的封禁
	for ip, banned := range f2m.bannedIPs {
		if now.After(banned.ExpiresAt) {
			delete(f2m.bannedIPs, ip)
			_ = f2m.removeBan(ip)
		}
	}

	// 清理过期的账户锁定
	for username, lockout := range f2m.accountLockouts {
		if lockout.ExpiresAt != nil && now.After(*lockout.ExpiresAt) {
			delete(f2m.accountLockouts, username)
		}
	}

	// 清理旧的失败尝试记录
	windowStart := now.Add(-time.Duration(f2m.config.WindowMinutes) * time.Minute)
	for ip, attempts := range f2m.failedAttempts {
		validAttempts := make([]FailedLoginAttempt, 0)
		for _, a := range attempts {
			if a.Timestamp.After(windowStart) {
				validAttempts = append(validAttempts, a)
			}
		}
		if len(validAttempts) == 0 {
			delete(f2m.failedAttempts, ip)
		} else {
			f2m.failedAttempts[ip] = validAttempts
		}
	}
}

// StartCleanupRoutine 启动定期清理例程.
func (f2m *Fail2BanManager) StartCleanupRoutine(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			f2m.CleanupExpired()
		}
	}()
}

// RecordSuccessfulLogin 记录成功登录（清除该 IP 的失败记录）.
func (f2m *Fail2BanManager) RecordSuccessfulLogin(ip, username string) {
	f2m.mu.Lock()
	defer f2m.mu.Unlock()

	// 清除该 IP 的失败记录
	delete(f2m.failedAttempts, ip)

	// 如果账户被锁定，解锁
	delete(f2m.accountLockouts, username)
}

// generateAlertID 生成告警 ID.
func generateAlertID() string {
	return fmt.Sprintf("alert-%d", time.Now().UnixNano())
}

// ========== Fail2Ban 配置文件生成 ==========

// GenerateFail2BanConfig 生成 fail2ban 配置文件.
func (f2m *Fail2BanManager) GenerateFail2BanConfig() (string, error) {
	f2m.mu.RLock()
	defer f2m.mu.RUnlock()

	config := `# NAS-OS Fail2Ban 配置文件
# 由系统自动生成，请勿手动修改

[DEFAULT]
bantime = %d
findtime = %d
maxretry = %d
action = iptables-multiport[name=nas-os, port="ssh,http,https"]
logpath = /var/log/nas-os/auth.log
          /var/log/auth.log

[nas-os-auth]
enabled = %s
filter = nas-os-auth
logpath = /var/log/nas-os/auth.log
maxretry = %d
bantime = %d
`

	enabled := "true"
	if !f2m.config.Enabled {
		enabled = "false"
	}

	bantime := f2m.config.BanDurationMinutes * 60 // 转换为秒
	findtime := f2m.config.WindowMinutes * 60     // 转换为秒

	return fmt.Sprintf(config,
		bantime,
		findtime,
		f2m.config.MaxAttempts,
		enabled,
		f2m.config.MaxAttempts,
		bantime,
	), nil
}

// InstallFail2BanFilter 安装 fail2ban 过滤器.
func InstallFail2BanFilter() error {
	filter := `[Definition]
failregex = ^.*Failed login attempt.*IP: <HOST>.*$
            ^.*Authentication failed.*IP: <HOST>.*$
            ^.*Invalid credentials.*IP: <HOST>.*$
ignoreregex =
`

	// 写入过滤器文件
	filterPath := "/etc/fail2ban/filter.d/nas-os-auth.conf"
	if err := os.WriteFile(filterPath, []byte(filter), 0640); err != nil {
		return fmt.Errorf("写入过滤器失败：%w", err)
	}

	return nil
}

// HasFail2Ban 检查系统是否安装了 fail2ban.
func HasFail2Ban() bool {
	cmd := exec.CommandContext(context.Background(), "which", "fail2ban-client")
	return cmd.Run() == nil
}

// GetFail2BanStatus 获取 fail2ban 状态.
func GetFail2BanStatus() (string, error) {
	if !HasFail2Ban() {
		return "", fmt.Errorf("fail2ban 未安装")
	}

	cmd := exec.CommandContext(context.Background(), "fail2ban-client", "status")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// GetFail2BanJailStatus 获取指定 jail 的状态.
func GetFail2BanJailStatus(jail string) (string, error) {
	if !HasFail2Ban() {
		return "", fmt.Errorf("fail2ban 未安装")
	}

	cmd := exec.CommandContext(context.Background(), "fail2ban-client", "status", jail)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// ListFail2BanJails 列出所有 jail.
func ListFail2BanJails() ([]string, error) {
	if !HasFail2Ban() {
		return []string{}, nil
	}

	cmd := exec.CommandContext(context.Background(), "fail2ban-client", "status")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// 解析输出，提取 jail 列表
	lines := strings.Split(string(output), "\n")
	jails := []string{}
	for _, line := range lines {
		if strings.Contains(line, "Jail list:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				jailList := strings.TrimSpace(parts[1])
				jails = strings.Split(jailList, ",")
				for i := range jails {
					jails[i] = strings.TrimSpace(jails[i])
				}
			}
		}
	}

	return jails, nil
}
