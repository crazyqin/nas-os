// Package nethealthadvisor converts network interface, security, and quality signals into safe network health guidance.
package nethealthadvisor

import (
	"sort"
	"strings"
	"time"
)

// Signal describes NAS network health signals inspired by TrueNAS 25.04 network improvements
// and Synology network management best practices.
type Signal struct {
	InterfaceName        string
	LinkSpeedMbps        int
	MTU                  int
	IsBonded             bool
	BondSlaveCount       int
	HasIPv6              bool
	HasDDNS              bool
	HasValidCert         bool
	CertDaysLeft         int
	FirewallEnabled      bool
	UPnPEnabled          bool
	RemoteAccessEnabled  bool
	FailedLoginAttempts  int
	ConcurrentUsers      int
	PacketLossPercent    float64
	LatencyMs            float64
	JitterMs             float64
}

// Recommendation is an actionable network hardening suggestion.
type Recommendation struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Priority string   `json:"priority"`
	Reason   string   `json:"reason"`
	Actions  []string `json:"actions"`
}

// Report summarizes current network health and security readiness.
type Report struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	InterfaceName   string           `json:"interface_name"`
	NetworkScore    int              `json:"network_score"`
	HealthStatus    string           `json:"health_status"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Advisor evaluates network health posture for dashboards and notifications.
type Advisor struct{ now func() time.Time }

// New creates a network health advisor.
func New() *Advisor { return &Advisor{now: time.Now} }

// WithNow returns a copy using a deterministic clock for tests.
func (a Advisor) WithNow(now func() time.Time) Advisor {
	if now != nil {
		a.now = now
	}
	return a
}

// Generate builds a deterministic report from current network signals.
func (a Advisor) Generate(s Signal) Report {
	recs := make([]Recommendation, 0, 10)

	// 证书快过期
	if s.HasValidCert && s.CertDaysLeft < 30 {
		priority := "critical"
		if s.CertDaysLeft >= 7 {
			priority = "high"
		}
		recs = append(recs, Recommendation{
			ID:       "renew-expiring-certificate",
			Title:    "续期即将过期的证书",
			Priority: priority,
			Reason:   "证书即将过期，过期后 HTTPS 和远程访问将不可信或直接中断。",
			Actions: []string{
				"在证书管理页面申请续期或重新签发",
				"续期后重启相关服务以确保加载新证书",
				"设置证书到期前 30 天提醒",
			},
		})
	}

	// 未配置防火墙
	if !s.FirewallEnabled {
		recs = append(recs, Recommendation{
			ID:       "enable-firewall",
			Title:    "启用防火墙",
			Priority: "critical",
			Reason:   "未启用防火墙时 NAS 直接暴露在网络中，攻击面极大。",
			Actions: []string{
				"启用系统自带防火墙并仅开放必要端口",
				"禁止外网访问管理面板，或限制来源 IP",
				"定期审查防火墙规则与日志",
			},
		})
	}

	// 未配置 DDNS 但远程访问已启用
	if s.RemoteAccessEnabled && !s.HasDDNS {
		recs = append(recs, Recommendation{
			ID:       "configure-ddns-for-remote-access",
			Title:    "为远程访问配置 DDNS",
			Priority: "high",
			Reason:   "启用远程访问但未配置 DDNS，IP 变更后远程连接将不可达。",
			Actions: []string{
				"在 DDNS 服务商处注册并绑定域名",
				"在 NAS 中配置 DDNS 自动更新客户端",
				"验证 DDNS 记录与当前公网 IP 一致",
			},
		})
	}

	// 丢包率过高
	if s.PacketLossPercent > 1 {
		recs = append(recs, Recommendation{
			ID:       "investigate-packet-loss",
			Title:    "排查网络丢包",
			Priority: "high",
			Reason:   "丢包率超过 1%，会影响文件传输完整性和远程访问体验。",
			Actions: []string{
				"检查网线、交换机端口和网卡驱动状态",
				"排查网络环路或带宽拥塞",
				"使用 ping 和 mtr 定位丢包节点",
			},
		})
	}

	// 失败登录次数过多
	if s.FailedLoginAttempts > 20 {
		recs = append(recs, Recommendation{
			ID:       "harden-login-security",
			Title:    "加强登录安全",
			Priority: "high",
			Reason:   "失败登录次数过多，可能正在遭受暴力破解攻击。",
			Actions: []string{
				"启用两步验证和登录失败自动封禁",
				"修改默认账户密码并禁用未使用账户",
				"检查登录日志定位异常来源 IP",
			},
		})
	}

	// 未启用 IPv6
	if !s.HasIPv6 {
		recs = append(recs, Recommendation{
			ID:       "enable-ipv6",
			Title:    "启用 IPv6 支持",
			Priority: "medium",
			Reason:   "未启用 IPv6 可能导致部分新协议服务连通性受限。",
			Actions: []string{
				"在网络设置中启用 IPv6 并获取前缀",
				"确认路由器和防火墙规则支持 IPv6",
				"验证关键服务在 IPv6 下可正常访问",
			},
		})
	}

	// UPnP 启用且远程访问启用
	if s.UPnPEnabled && s.RemoteAccessEnabled {
		recs = append(recs, Recommendation{
			ID:       "disable-upnp-use-manual-port-forwarding",
			Title:    "关闭 UPnP 改用手动端口映射",
			Priority: "medium",
			Reason:   "UPnP 自动开放端口存在安全隐患，远程访问场景下风险更高。",
			Actions: []string{
				"在路由器上关闭 UPnP 功能",
				"手动配置仅必要端口的转发规则",
				"定期审查已开放的端口列表",
			},
		})
	}

	// 未做链路聚合且并发用户多
	if !s.IsBonded && s.ConcurrentUsers > 5 {
		recs = append(recs, Recommendation{
			ID:       "consider-link-aggregation",
			Title:    "考虑链路聚合",
			Priority: "medium",
			Reason:   "并发用户较多但未做链路聚合，单口带宽可能成为瓶颈。",
			Actions: []string{
				"评估是否有多余网口可用于聚合",
				"在交换机和 NAS 两端配置 LACP",
				"聚合后验证带宽和冗余切换正常",
			},
		})
	}

	// 延迟过高
	if s.LatencyMs > 50 {
		recs = append(recs, Recommendation{
			ID:       "optimize-network-latency",
			Title:    "优化网络延迟",
			Priority: "medium",
			Reason:   "延迟超过 50ms，会影响远程操作和实时同步体验。",
			Actions: []string{
				"检查网络路径中是否有不必要的中转节点",
				"确认 DNS 解析快速且正确",
				"考虑使用 CDN 或就近部署代理节点",
			},
		})
	}

	// 抖动过大
	if s.JitterMs > 20 {
		recs = append(recs, Recommendation{
			ID:       "optimize-qos-for-jitter",
			Title:    "优化 QoS 减少抖动",
			Priority: "medium",
			Reason:   "抖动超过 20ms，会影响实时通信和流媒体体验。",
			Actions: []string{
				"在路由器上启用 QoS 并优先 NAS 流量",
				"排查是否存在带宽争抢或缓冲膨胀",
				"必要时为关键流量划分独立 VLAN",
			},
		})
	}

	sort.SliceStable(recs, func(i, j int) bool {
		left, right := priorityRank(recs[i].Priority), priorityRank(recs[j].Priority)
		if left == right {
			return recs[i].ID < recs[j].ID
		}
		return left < right
	})

	return Report{
		GeneratedAt:     a.now(),
		InterfaceName:   s.InterfaceName,
		NetworkScore:    networkScore(recs),
		HealthStatus:    healthStatus(recs),
		Recommendations: recs,
	}
}

// SummarizeActions returns compact next steps for notifications.
func SummarizeActions(recs []Recommendation) string {
	parts := make([]string, 0, len(recs))
	for _, rec := range recs {
		if len(rec.Actions) == 0 {
			continue
		}
		parts = append(parts, rec.Title+": "+rec.Actions[0])
	}
	return strings.Join(parts, "; ")
}

func healthStatus(recs []Recommendation) string {
	hasCritical, hasHigh := false, false
	for _, rec := range recs {
		switch rec.Priority {
		case "critical":
			hasCritical = true
		case "high":
			hasHigh = true
		}
	}
	switch {
	case hasCritical:
		return "critical"
	case hasHigh:
		return "warning"
	default:
		return "healthy"
	}
}

func priorityRank(priority string) int {
	switch priority {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

func networkScore(recs []Recommendation) int {
	score := 100
	for _, rec := range recs {
		switch rec.Priority {
		case "critical":
			score -= 30
		case "high":
			score -= 18
		case "medium":
			score -= 9
		default:
			score -= 4
		}
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
