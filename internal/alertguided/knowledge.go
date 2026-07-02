package alertguided

import (
	"fmt"
	"strings"
	"sync"
)

// KnowledgeEntry 告警知识条目.
type KnowledgeEntry struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	Category   Category     `json:"category"`
	Severity   Severity     `json:"severity"`
	Causes     []string     `json:"causes"`
	Symptoms   []string     `json:"symptoms"`
	Steps      []RepairStep `json:"steps"`
	References []string     `json:"references,omitempty"`
	Tags       []string     `json:"tags,omitempty"`
}

// RepairStep 修复步骤.
type RepairStep struct {
	Order          int      `json:"order"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Command        string   `json:"command,omitempty"`
	ExpectedResult string   `json:"expectedResult,omitempty"`
	RiskLevel      string   `json:"riskLevel,omitempty"` // low/medium/high
	RequiresAck    bool     `json:"requiresAck,omitempty"`
	IsOptional     bool     `json:"isOptional,omitempty"`
	Alternatives   []string `json:"alternatives,omitempty"`
}

// KnowledgeBase 告警知识库.
type KnowledgeBase struct {
	entries map[string]*KnowledgeEntry
	mu      sync.RWMutex
}

// NewKnowledgeBase 创建知识库并加载内置条目.
func NewKnowledgeBase() *KnowledgeBase {
	kb := &KnowledgeBase{
		entries: make(map[string]*KnowledgeEntry),
	}
	kb.loadBuiltinEntries()
	return kb
}

// Get 根据ID获取知识条目.
func (kb *KnowledgeBase) Get(id string) (*KnowledgeEntry, bool) {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	entry, ok := kb.entries[id]
	return entry, ok
}

// Search 按关键词搜索知识条目.
func (kb *KnowledgeBase) Search(keyword string) []*KnowledgeEntry {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	kw := strings.ToLower(keyword)
	var results []*KnowledgeEntry
	for _, entry := range kb.entries {
		if matchEntry(entry, kw) {
			results = append(results, entry)
		}
	}
	return results
}

// List 列出所有知识条目.
func (kb *KnowledgeBase) List() []*KnowledgeEntry {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	result := make([]*KnowledgeEntry, 0, len(kb.entries))
	for _, e := range kb.entries {
		result = append(result, e)
	}
	return result
}

// Add 添加自定义知识条目.
func (kb *KnowledgeBase) Add(entry *KnowledgeEntry) {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	kb.entries[entry.ID] = entry
}

// LookupByCategory 按类别查找.
func (kb *KnowledgeBase) LookupByCategory(cat Category) []*KnowledgeEntry {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	var results []*KnowledgeEntry
	for _, entry := range kb.entries {
		if entry.Category == cat {
			results = append(results, entry)
		}
	}
	return results
}

func matchEntry(entry *KnowledgeEntry, kw string) bool {
	if strings.Contains(strings.ToLower(entry.Title), kw) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.ID), kw) {
		return true
	}
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), kw) {
			return true
		}
	}
	for _, cause := range entry.Causes {
		if strings.Contains(strings.ToLower(cause), kw) {
			return true
		}
	}
	for _, symptom := range entry.Symptoms {
		if strings.Contains(strings.ToLower(symptom), kw) {
			return true
		}
	}
	return false
}

// loadBuiltinEntries 加载内置告警知识条目.
func (kb *KnowledgeBase) loadBuiltinEntries() {
	entries := []*KnowledgeEntry{
		{
			ID:       "disk_space_low",
			Title:    "磁盘空间不足",
			Category: CategoryStorage,
			Severity: SeverityWarning,
			Causes: []string{
				"日志文件持续增长",
				"临时文件未清理",
				"快照占用过多空间",
				"应用产生大量数据未及时归档",
			},
			Symptoms: []string{
				"df 输出使用率超过 90%",
				"写入操作报 ENOSPC 错误",
				"应用日志记录写入失败",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "查看空间使用详情", Description: "使用 df -h 查看各分区使用率，定位高占用分区", Command: "df -h", ExpectedResult: "找到使用率最高的分区"},
				{Order: 2, Title: "查找大文件目录", Description: "使用 du 命令递归查找占用空间最大的目录", Command: "du -sh /* 2>/dev/null | sort -rh | head -20", ExpectedResult: "定位到具体大目录"},
				{Order: 3, Title: "清理日志文件", Description: "清理过期日志释放空间", Command: "journalctl --vacuum-size=500M", ExpectedResult: "日志空间回收"},
				{Order: 4, Title: "清理临时文件", Description: "删除 /tmp 和 /var/tmp 下的临时文件", Command: "rm -rf /tmp/* /var/tmp/*", RiskLevel: "low"},
				{Order: 5, Title: "检查 ZFS 快照", Description: "如有 ZFS 文件系统，检查快照占用", Command: "zfs list -t snapshot", ExpectedResult: "可删除过期快照释放空间", IsOptional: true},
			},
			Tags: []string{"disk", "storage", "space", "full"},
		},
		{
			ID:       "disk_smart_warning",
			Title:    "磁盘 SMART 异常",
			Category: CategoryHardware,
			Severity: SeverityWarning,
			Causes: []string{
				"磁盘出现坏道",
				"磁盘固件问题",
				"磁盘老化即将失效",
				"供电不稳导致写入异常",
			},
			Symptoms: []string{
				"SMART 健康检查失败",
				"Reallocated_Sector_Ct 持续增长",
				"Current_Pending_Sector 非零",
				"I/O 错误出现在 dmesg 中",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "查看 SMART 详情", Description: "获取磁盘完整 SMART 信息", Command: "smartctl -a /dev/sdX", ExpectedResult: "检查 Reallocated_Sector_Ct、Current_Pending_Sector 等指标"},
				{Order: 2, Title: "运行短自检", Description: "执行磁盘短自检评估健康状态", Command: "smartctl -t short /dev/sdX", ExpectedResult: "自检完成无错误"},
				{Order: 3, Title: "检查系统日志", Description: "检查是否有 I/O 错误", Command: "dmesg | grep -i 'error\\|fail\\|bad'", ExpectedResult: "无磁盘相关错误"},
				{Order: 4, Title: "备份数据", Description: "立即备份该磁盘上的重要数据", RiskLevel: "low", ExpectedResult: "数据已安全备份"},
				{Order: 5, Title: "更换磁盘", Description: "如 SMART 持续恶化，计划更换磁盘", RiskLevel: "medium", RequiresAck: true},
			},
			References: []string{"https://docs.nas-os.com/guides/smart-monitoring"},
			Tags:       []string{"disk", "smart", "health", "hardware"},
		},
		{
			ID:       "cpu_overload",
			Title:    "CPU 过载",
			Category: CategoryPerformance,
			Severity: SeverityWarning,
			Causes: []string{
				"某个进程 CPU 占用异常",
				"定时任务集中执行",
				"编译或转码等 CPU 密集操作",
				"死循环或程序 bug",
			},
			Symptoms: []string{
				"系统响应缓慢",
				"load average 持续高于 CPU 核心数",
				"top 显示 CPU 使用率超过 90%",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "查看 CPU 使用率", Description: "查看当前 CPU 占用最高的进程", Command: "top -bn1 | head -20", ExpectedResult: "找到 CPU 占用最高的进程"},
				{Order: 2, Title: "分析进程", Description: "检查高占用进程是否正常", Command: "ps aux --sort=-%cpu | head -10", ExpectedResult: "确认进程用途"},
				{Order: 3, Title: "检查定时任务", Description: "查看 cron 任务是否有集中执行", Command: "crontab -l", ExpectedResult: "调整任务时间避免集中"},
				{Order: 4, Title: "限制异常进程", Description: "如有异常进程可限制或终止", Command: "renice +19 -p PID", RiskLevel: "medium", IsOptional: true},
			},
			Tags: []string{"cpu", "performance", "load"},
		},
		{
			ID:       "memory_overload",
			Title:    "内存过载",
			Category: CategoryPerformance,
			Severity: SeverityWarning,
			Causes: []string{
				"应用内存泄漏",
				"运行服务过多",
				"缓存未正确配置上限",
				"内存不足需要扩容",
			},
			Symptoms: []string{
				"可用内存不足 10%",
				"swap 使用率持续升高",
				"OOM Killer 触发",
				"系统频繁使用 swap 导致卡顿",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "查看内存使用", Description: "检查整体内存使用情况", Command: "free -h", ExpectedResult: "确认内存和 swap 使用率"},
				{Order: 2, Title: "查找内存大户", Description: "找到占用内存最多的进程", Command: "ps aux --sort=-%mem | head -10", ExpectedResult: "定位高内存进程"},
				{Order: 3, Title: "检查 OOM 日志", Description: "查看是否有 OOM Killer 记录", Command: "dmesg | grep -i oom", ExpectedResult: "了解 OOM 详情"},
				{Order: 4, Title: "重启异常服务", Description: "如有内存泄漏的服务，重启释放内存", RiskLevel: "medium", IsOptional: true},
			},
			Tags: []string{"memory", "ram", "performance", "oom"},
		},
		{
			ID:       "network_down",
			Title:    "网络中断",
			Category: CategoryNetwork,
			Severity: SeverityCritical,
			Causes: []string{
				"网线松动或损坏",
				"交换机/路由器故障",
				"网络配置错误",
				"DNS 解析失败",
				"IP 地址冲突",
			},
			Symptoms: []string{
				"网络接口状态 DOWN",
				"ping 外网不通",
				"服务无法访问",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "检查接口状态", Description: "查看网络接口是否 UP", Command: "ip link show", ExpectedResult: "接口状态为 UP"},
				{Order: 2, Title: "检查 IP 配置", Description: "确认 IP 地址配置正确", Command: "ip addr show", ExpectedResult: "IP 地址正常分配"},
				{Order: 3, Title: "测试连通性", Description: "测试网关和外网连通性", Command: "ping -c 3 8.8.8.8", ExpectedResult: "ping 成功"},
				{Order: 4, Title: "检查 DNS", Description: "测试 DNS 解析", Command: "nslookup nas-os.com", ExpectedResult: "DNS 解析正常"},
				{Order: 5, Title: "检查物理连接", Description: "确认网线是否插好，指示灯是否正常", ExpectedResult: "物理连接正常"},
				{Order: 6, Title: "重启网络服务", Description: "重启网络服务恢复连接", Command: "systemctl restart networking", RiskLevel: "medium"},
			},
			Tags: []string{"network", "connectivity", "ethernet"},
		},
		{
			ID:       "raid_degraded",
			Title:    "RAID 降级",
			Category: CategoryStorage,
			Severity: SeverityCritical,
			Causes: []string{
				"RAID 成员盘故障",
				"磁盘被移除或断开",
				"磁盘 SMART 错误被标记为故障",
			},
			Symptoms: []string{
				"RAID 状态为 degraded",
				"mdadm 报告降级",
				"性能下降",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "检查 RAID 状态", Description: "查看当前 RAID 阵列状态", Command: "cat /proc/mdstat", ExpectedResult: "确认降级的阵列和故障盘"},
				{Order: 2, Title: "定位故障盘", Description: "找到具体故障的磁盘", Command: "mdadm --detail /dev/md0", ExpectedResult: "确认故障设备"},
				{Order: 3, Title: "移除故障盘", Description: "从阵列中移除故障磁盘", Command: "mdadm --manage /dev/md0 --remove /dev/sdX", ExpectedResult: "故障盘已移除"},
				{Order: 4, Title: "添加新盘", Description: "添加新磁盘开始重建", Command: "mdadm --manage /dev/md0 --add /dev/sdY", ExpectedResult: "重建开始"},
				{Order: 5, Title: "监控重建进度", Description: "持续监控重建进度", Command: "watch cat /proc/mdstat", ExpectedResult: "重建完成"},
			},
			Tags: []string{"raid", "storage", "degraded", "mdadm"},
		},
		{
			ID:       "zfs_pool_error",
			Title:    "ZFS 池异常",
			Category: CategoryStorage,
			Severity: SeverityCritical,
			Causes: []string{
				"底层磁盘错误",
				"ZFS 数据校验失败",
				"内存故障导致校验和错误",
				"池配置损坏",
			},
			Symptoms: []string{
				"zpool status 显示 DEGRADED 或 FAULTED",
				"存在校验和错误",
				"数据读取报错",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "检查池状态", Description: "查看 ZFS 池详细状态", Command: "zpool status tank", ExpectedResult: "确认问题设备和错误类型"},
				{Order: 2, Title: "扫描修复", Description: "运行 scrub 修复校验和错误", Command: "zpool scrub tank", ExpectedResult: "scrub 完成无新错误"},
				{Order: 3, Title: "检查磁盘健康", Description: "检查底层磁盘 SMART 状态", Command: "smartctl -a /dev/sdX", ExpectedResult: "磁盘无硬件故障"},
				{Order: 4, Title: "替换故障盘", Description: "如有硬件故障，替换磁盘", Command: "zpool replace tank /dev/sdX /dev/sdY", RiskLevel: "high", RequiresAck: true},
				{Order: 5, Title: "导出重建", Description: "如池配置损坏，尝试导出后重建", RiskLevel: "high", RequiresAck: true, IsOptional: true},
			},
			Tags: []string{"zfs", "storage", "pool", "scrub"},
		},
		{
			ID:       "certificate_expired",
			Title:    "证书过期",
			Category: CategorySecurity,
			Severity: SeverityWarning,
			Causes: []string{
				"SSL/TLS 证书超过有效期",
				"自动续签失败",
				"证书链不完整",
			},
			Symptoms: []string{
				"浏览器提示证书不安全",
				"HTTPS 连接失败",
				"客户端报证书过期错误",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "检查证书状态", Description: "查看当前证书的过期时间", Command: "openssl x509 -in /path/to/cert.pem -noout -dates", ExpectedResult: "确认证书是否过期"},
				{Order: 2, Title: "检查证书链", Description: "验证证书链完整性", Command: "openssl verify -CAfile ca.pem cert.pem", ExpectedResult: "证书链验证通过"},
				{Order: 3, Title: "更新证书", Description: "使用 ACME 或手动更新证书", Command: "certbot renew", ExpectedResult: "证书更新成功"},
				{Order: 4, Title: "重启服务", Description: "重启使用该证书的服务", Command: "systemctl restart nginx", ExpectedResult: "服务加载新证书"},
			},
			Tags: []string{"ssl", "tls", "certificate", "security"},
		},
		{
			ID:       "backup_failed",
			Title:    "备份失败",
			Category: CategorySystem,
			Severity: SeverityWarning,
			Causes: []string{
				"备份目标存储空间不足",
				"网络连接中断",
				"备份服务配置错误",
				"权限不足",
				"源数据损坏",
			},
			Symptoms: []string{
				"备份任务返回非零退出码",
				"备份日志报错",
				"备份目标无新文件",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "查看备份日志", Description: "检查最近的备份任务日志", Command: "journalctl -u backup -n 50", ExpectedResult: "找到具体错误信息"},
				{Order: 2, Title: "检查目标存储", Description: "确认备份目标有足够空间", Command: "df -h /backup", ExpectedResult: "空间充足"},
				{Order: 3, Title: "检查网络连接", Description: "如备份到远程，确认网络正常", Command: "ping -c 3 backup-server", ExpectedResult: "网络连通"},
				{Order: 4, Title: "手动触发备份", Description: "手动执行一次备份验证", Command: "backup-tool run --dry-run", ExpectedResult: "预检通过"},
				{Order: 5, Title: "修复并重试", Description: "根据错误原因修复后重新执行备份", ExpectedResult: "备份成功完成"},
			},
			Tags: []string{"backup", "data-protection", "system"},
		},
		{
			ID:       "ups_battery_low",
			Title:    "UPS 电池低",
			Category: CategoryHardware,
			Severity: SeverityCritical,
			Causes: []string{
				"市电断电且持续时间较长",
				"UPS 电池老化容量下降",
				"UPS 未正确连接",
				"负载超过 UPS 额定容量",
			},
			Symptoms: []string{
				"UPS 电量低于 30%",
				"UPS 报电池低告警",
				"系统收到 shutdown 通知",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "查看 UPS 状态", Description: "检查 UPS 当前电量和状态", Command: "upsc ups", ExpectedResult: "确认电量和负载"},
				{Order: 2, Title: "检查市电", Description: "确认市电是否已恢复", ExpectedResult: "市电正常"},
				{Order: 3, Title: "减少负载", Description: "关闭非必要服务降低 UPS 负载", RiskLevel: "medium", ExpectedResult: "负载下降"},
				{Order: 4, Title: "安全关机", Description: "如电量持续下降，执行安全关机保护数据", Command: "shutdown -h +5 'UPS battery low, scheduled shutdown'", RiskLevel: "high", RequiresAck: true},
				{Order: 5, Title: "更换电池", Description: "如电池老化，安排更换 UPS 电池", ExpectedResult: "电池更换完成", IsOptional: true},
			},
			Tags: []string{"ups", "power", "battery", "hardware"},
		},
		{
			ID:       "service_crash",
			Title:    "服务崩溃",
			Category: CategorySystem,
			Severity: SeverityWarning,
			Causes: []string{
				"程序 bug 导致崩溃",
				"依赖服务不可用",
				"配置文件错误",
				"资源不足（内存/文件描述符）",
			},
			Symptoms: []string{
				"systemd 报服务 failed",
				"服务频繁重启",
				"功能不可用",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "查看服务状态", Description: "检查服务当前状态", Command: "systemctl status service-name", ExpectedResult: "确认服务已停止"},
				{Order: 2, Title: "查看崩溃日志", Description: "检查服务日志找崩溃原因", Command: "journalctl -u service-name -n 100", ExpectedResult: "找到错误信息"},
				{Order: 3, Title: "检查配置", Description: "验证配置文件语法", Command: "service-name --check-config", ExpectedResult: "配置无误"},
				{Order: 4, Title: "重启服务", Description: "尝试重启服务", Command: "systemctl restart service-name", ExpectedResult: "服务启动成功"},
			},
			Tags: []string{"service", "systemd", "crash", "system"},
		},
		{
			ID:       "disk_io_error",
			Title:    "磁盘 I/O 错误",
			Category: CategoryHardware,
			Severity: SeverityCritical,
			Causes: []string{
				"磁盘硬件故障",
				"数据线接触不良",
				"控制器故障",
				"内核 bug",
			},
			Symptoms: []string{
				"dmesg 出现 I/O error",
				"文件系统变为只读",
				"读写操作超时",
			},
			Steps: []RepairStep{
				{Order: 1, Title: "检查内核日志", Description: "查看 I/O 错误详情", Command: "dmesg | grep -i 'io error\\|ata.*error'", ExpectedResult: "确认出错的设备"},
				{Order: 2, Title: "检查磁盘状态", Description: "检查磁盘 SMART 状态", Command: "smartctl -a /dev/sdX", ExpectedResult: "评估磁盘健康"},
				{Order: 3, Title: "检查数据线", Description: "物理检查数据线和接口", ExpectedResult: "连接牢固"},
				{Order: 4, Title: "备份数据", Description: "立即备份该磁盘数据", RiskLevel: "low", ExpectedResult: "数据已备份"},
				{Order: 5, Title: "更换磁盘", Description: "如有硬件故障需更换", RiskLevel: "high", RequiresAck: true},
			},
			Tags: []string{"disk", "io", "hardware", "error"},
		},
	}

	for _, e := range entries {
		kb.entries[e.ID] = e
	}
}

// FormatKnowledgeEntry 格式化知识条目为可读文本.
func FormatKnowledgeEntry(entry *KnowledgeEntry) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", entry.Title)
	fmt.Fprintf(&sb, "分类: %s | 严重级别: %s\n\n", entry.Category, entry.Severity)

	sb.WriteString("## 可能原因\n")
	for _, cause := range entry.Causes {
		fmt.Fprintf(&sb, "- %s\n", cause)
	}

	sb.WriteString("\n## 症状表现\n")
	for _, symptom := range entry.Symptoms {
		fmt.Fprintf(&sb, "- %s\n", symptom)
	}

	sb.WriteString("\n## 修复步骤\n")
	for _, step := range entry.Steps {
		fmt.Fprintf(&sb, "%d. **%s**\n", step.Order, step.Title)
		fmt.Fprintf(&sb, "   %s\n", step.Description)
		if step.Command != "" {
			fmt.Fprintf(&sb, "   ```\n   %s\n   ```\n", step.Command)
		}
		if step.ExpectedResult != "" {
			fmt.Fprintf(&sb, "   预期结果: %s\n", step.ExpectedResult)
		}
		if step.RiskLevel == "high" {
			sb.WriteString("   ⚠️ 高风险操作，需要确认\n")
		}
	}

	if len(entry.References) > 0 {
		sb.WriteString("\n## 参考链接\n")
		for _, ref := range entry.References {
			fmt.Fprintf(&sb, "- %s\n", ref)
		}
	}
	return sb.String()
}
