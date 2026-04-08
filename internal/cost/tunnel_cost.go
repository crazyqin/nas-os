// Package cost - 内网穿透服务成本分析模型
// 对标飞牛FN Connect（免费）+ 群晖QuickConnect/DDNS
package cost

import (
	"context"
	"math"
	"sync"
	"time"
)

// ========== 内网穿透成本类型定义 ==========

// TunnelProvider 穿透服务提供商
type TunnelProvider string

const (
	TunnelProviderSelfHosted TunnelProvider = "self_hosted" // 自建服务
	TunnelProviderFRP        TunnelProvider = "frp"         // FRP公共服务器
	TunnelProviderCloudflare TunnelProvider = "cloudflare"  // Cloudflare Tunnel
	TunnelProviderFNConnect  TunnelProvider = "fn_connect"  // 飞牛FN Connect（免费）
	TunnelProviderSynology   TunnelProvider = "synology"    // 群晖QuickConnect
	TunnelProviderTailscale  TunnelProvider = "tailscale"   // Tailscale
)

// TunnelCostModel 穿透成本模型
type TunnelCostModel struct {
	// 服务提供商
	Provider TunnelProvider `json:"provider"`

	// 显示名称
	DisplayName string `json:"display_name"`

	// 是否免费服务
	IsFree bool `json:"is_free"`

	// ========== 带宽成本 ==========

	// 每月带宽成本（元/Mbps）
	BandwidthCostPerMbps float64 `json:"bandwidth_cost_per_mbps"`

	// 每GB流量成本（元）
	TrafficCostPerGB float64 `json:"traffic_cost_per_gb"`

	// 免费带宽上限（Mbps，0=无限制）
	FreeBandwidthLimit int `json:"free_bandwidth_limit"`

	// 免费流量上限（GB/月，0=无限制）
	FreeTrafficLimitGB int `json:"free_traffic_limit_gb"`

	// ========== 连接成本 ==========

	// 每连接成本（元/月）
	ConnectionCostPerConn float64 `json:"connection_cost_per_conn"`

	// 免费连接数上限
	FreeConnectionLimit int `json:"free_connection_limit"`

	// ========== 服务器成本 ==========

	// 中继服务器成本（元/月）
	RelayServerCost float64 `json:"relay_server_cost"`

	// STUN服务器成本（元/月）
	STUNServerCost float64 `json:"stun_server_cost"`

	// TURN服务器成本（元/月）
	TURNServerCost float64 `json:"turn_server_cost"`

	// ========== 附加成本 ==========

	// SSL证书成本（元/年）
	SSLCostPerYear float64 `json:"ssl_cost_per_year"`

	// 域名成本（元/年）
	DomainCostPerYear float64 `json:"domain_cost_per_year"`

	// 维护成本系数（%）
	MaintenanceCostPercent float64 `json:"maintenance_cost_percent"`

	// ========== 性能参数 ==========

	// 典型延迟（ms）
	TypicalLatencyMs int `json:"typical_latency_ms"`

	// 最大并发连接数
	MaxConcurrentConnections int `json:"max_concurrent_connections"`

	// 支持的协议
	SupportedProtocols []string `json:"supported_protocols"`

	// ========== 更新时间 ==========

	LastUpdated time.Time `json:"last_updated"`
}

// TunnelUsageStats 穿透使用统计
type TunnelUsageStats struct {
	// 统计时间
	CollectedAt time.Time `json:"collected_at"`

	// 设备ID
	DeviceID string `json:"device_id"`

	// 连接时长（小时）
	ConnectionHours float64 `json:"connection_hours"`

	// 平均带宽使用（Mbps）
	AvgBandwidthMbps float64 `json:"avg_bandwidth_mbps"`

	// 峰值带宽使用（Mbps）
	PeakBandwidthMbps float64 `json:"peak_bandwidth_mbps"`

	// 总流量（GB）
	TotalTrafficGB float64 `json:"total_traffic_gb"`

	// 入流量（GB）
	InboundTrafficGB float64 `json:"inbound_traffic_gb"`

	// 出流量（GB）
	OutboundTrafficGB float64 `json:"outbound_traffic_gb"`

	// 连接数
	ConnectionCount int `json:"connection_count"`

	// P2P成功率（%）
	P2PSuccessRate float64 `json:"p2p_success_rate"`

	// 中继使用率（%）
	RelayUsagePercent float64 `json:"relay_usage_percent"`

	// 平均延迟（ms）
	AvgLatencyMs float64 `json:"avg_latency_ms"`

	// 连接断开次数
	DisconnectCount int `json:"disconnect_count"`
}

// TunnelCostReport 穿透成本报告
type TunnelCostReport struct {
	// 报告ID
	ID string `json:"id"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 时间范围
	TimeRange TimeRange `json:"time_range"`

	// 使用统计
	UsageStats TunnelUsageStats `json:"usage_stats"`

	// 成本明细
	CostBreakdown TunnelCostBreakdown `json:"cost_breakdown"`

	// 月度成本估算
	MonthlyCostEstimate float64 `json:"monthly_cost_estimate"`

	// 年度成本估算
	YearlyCostEstimate float64 `json:"yearly_cost_estimate"`

	// 与竞品对比
	CompetitorComparison []TunnelCompetitorCost `json:"competitor_comparison"`

	// 成本建议
	CostSuggestions []TunnelCostSuggestion `json:"cost_suggestions"`

	// ROI分析
	ROIAnalysis TunnelROIAnalysis `json:"roi_analysis"`
}

// TunnelCostBreakdown 穿透成本明细
type TunnelCostBreakdown struct {
	// 带宽成本
	BandwidthCost float64 `json:"bandwidth_cost"`

	// 流量成本
	TrafficCost float64 `json:"traffic_cost"`

	// 连接成本
	ConnectionCost float64 `json:"connection_cost"`

	// 服务器成本
	ServerCost float64 `json:"server_cost"`

	// SSL成本（月摊）
	SSLCostMonthly float64 `json:"ssl_cost_monthly"`

	// 域名成本（月摊）
	DomainCostMonthly float64 `json:"domain_cost_monthly"`

	// 维护成本
	MaintenanceCost float64 `json:"maintenance_cost"`

	// 总成本
	TotalCost float64 `json:"total_cost"`

	// 有效成本（扣除免费额度后）
	EffectiveCost float64 `json:"effective_cost"`
}

// TunnelCompetitorCost 竞品成本对比
type TunnelCompetitorCost struct {
	// 提供商名称
	ProviderName string `json:"provider_name"`

	// 是否免费
	IsFree bool `json:"is_free"`

	// 月度成本
	MonthlyCost float64 `json:"monthly_cost"`

	// 功能限制描述
	Limitations string `json:"limitations"`

	// 与我们方案的差异
	Difference string `json:"difference"`

	// 推荐指数（1-5）
	RecommendationScore int `json:"recommendation_score"`
}

// TunnelCostSuggestion 成本优化建议
type TunnelCostSuggestion struct {
	// 建议类型
	Type string `json:"type"` // reduce_bandwidth, optimize_p2p, change_provider, self_host

	// 建议内容
	Suggestion string `json:"suggestion"`

	// 预期节省（元/月）
	PotentialSavings float64 `json:"potential_savings"`

	// 实施难度
	Difficulty string `json:"difficulty"` // easy, medium, hard

	// 优先级
	Priority int `json:"priority"`
}

// TunnelROIAnalysis ROI分析
type TunnelROIAnalysis struct {
	// 自建vs使用第三方服务
	SelfHostCost5Year float64 `json:"self_host_cost_5_year"`

	// 第三方服务5年成本
	ThirdPartyCost5Year float64 `json:"third_party_cost_5_year"`

	// 节省比例
	SavingsPercent float64 `json:"savings_percent"`

	// 收益平衡点（月）
	BreakEvenMonths int `json:"break_even_months"`

	// 推荐方案
	RecommendedApproach string `json:"recommended_approach"`
}

// ========== 内网穿透成本分析器 ==========

// TunnelCostAnalyzer 穿透成本分析器
type TunnelCostAnalyzer struct {
	mu      sync.RWMutex
	models  map[TunnelProvider]*TunnelCostModel
	usage   map[string]*TunnelUsageStats
	config  *TunnelCostConfig
}

// TunnelCostConfig 分析器配置
type TunnelCostConfig struct {
	// 自建服务器成本（元/月）
	SelfHostServerCost float64 `json:"self_host_server_cost"`

	// 自建带宽成本（元/Mbps）
	SelfHostBandwidthCost float64 `json:"self_host_bandwidth_cost"`

	// 免费用户预期带宽（Mbps）
	FreeUserBandwidthMbps float64 `json:"free_user_bandwidth_mbps"`

	// 付费用户预期带宽（Mbps）
	PaidUserBandwidthMbps float64 `json:"paid_user_bandwidth_mbps"`

	// P2P成功率估算（%）
	P2PSuccessRateEstimate float64 `json:"p2p_success_rate_estimate"`

	// 中继服务器数量
	RelayServerCount int `json:"relay_server_count"`

	// 每中继服务器成本（元/月）
	CostPerRelayServer float64 `json:"cost_per_relay_server"`

	// 维护成本系数（%）
	MaintenanceCostPercent float64 `json:"maintenance_cost_percent"`

	// SSL证书年成本
	SSLCostPerYear float64 `json:"ssl_cost_per_year"`

	// 域名年成本
	DomainCostPerYear float64 `json:"domain_cost_per_year"`
}

// DefaultTunnelCostConfig 默认配置
func DefaultTunnelCostConfig() *TunnelCostConfig {
	return &TunnelCostConfig{
		SelfHostServerCost:      200.0,   // 云服务器约200元/月
		SelfHostBandwidthCost:   0.8,     // 带宽约0.8元/Mbps
		FreeUserBandwidthMbps:   2.0,     // 免费用户平均2Mbps
		PaidUserBandwidthMbps:   10.0,    // 付费用户平均10Mbps
		P2PSuccessRateEstimate:  60.0,    // P2P成功率约60%
		RelayServerCount:        3,       // 3台中继服务器
		CostPerRelayServer:      200.0,   // 每台200元/月
		MaintenanceCostPercent:  10.0,    // 维护成本10%
		SSLCostPerYear:          100.0,   // SSL证书约100元/年
		DomainCostPerYear:       50.0,    // 域名约50元/年
	}
}

// NewTunnelCostAnalyzer 创建穿透成本分析器
func NewTunnelCostAnalyzer(config *TunnelCostConfig) *TunnelCostAnalyzer {
	if config == nil {
		config = DefaultTunnelCostConfig()
	}

	analyzer := &TunnelCostAnalyzer{
		models: make(map[TunnelProvider]*TunnelCostModel),
		usage:  make(map[string]*TunnelUsageStats),
		config: config,
	}

	// 初始化竞品定价模型
	analyzer.initCompetitorModels()

	return analyzer
}

// initCompetitorModels 初始化竞品定价模型
func (a *TunnelCostAnalyzer) initCompetitorModels() {
	// 飞牛FN Connect - 免费
	a.models[TunnelProviderFNConnect] = &TunnelCostModel{
		Provider:               TunnelProviderFNConnect,
		DisplayName:            "飞牛FN Connect",
		IsFree:                 true,
		BandwidthCostPerMbps:   0,
		TrafficCostPerGB:       0,
		FreeBandwidthLimit:     10,    // 约10Mbps免费
		FreeTrafficLimitGB:     100,   // 100GB/月免费流量
		FreeConnectionLimit:     5,    // 5个连接
		TypicalLatencyMs:       50,
		MaxConcurrentConnections: 5,
		SupportedProtocols:     []string{"http", "https", "tcp"},
		LastUpdated:            time.Now(),
	}

	// 群晖QuickConnect - 免费（有设备绑定限制）
	a.models[TunnelProviderSynology] = &TunnelCostModel{
		Provider:               TunnelProviderSynology,
		DisplayName:            "群晖QuickConnect",
		IsFree:                 true,
		BandwidthCostPerMbps:   0,
		TrafficCostPerGB:       0,
		FreeBandwidthLimit:     5,     // 保守估计5Mbps
		FreeTrafficLimitGB:     50,    // 约50GB/月
		FreeConnectionLimit:     3,    // 3个连接
		TypicalLatencyMs:       80,
		MaxConcurrentConnections: 3,
		SupportedProtocols:     []string{"http", "https"},
		LastUpdated:            time.Now(),
	}

	// Cloudflare Tunnel - 免费（有企业付费版）
	a.models[TunnelProviderCloudflare] = &TunnelCostModel{
		Provider:               TunnelProviderCloudflare,
		DisplayName:            "Cloudflare Tunnel",
		IsFree:                 true,
		BandwidthCostPerMbps:   0,
		TrafficCostPerGB:       0,
		FreeBandwidthLimit:     0,     // 无明确限制
		FreeTrafficLimitGB:     0,     // 无明确限制
		FreeConnectionLimit:     100,  // 100个隧道
		TypicalLatencyMs:       30,
		MaxConcurrentConnections: 100,
		SupportedProtocols:     []string{"http", "https", "ssh", "tcp"},
		LastUpdated:            time.Now(),
	}

	// Tailscale - 免费（个人版）
	a.models[TunnelProviderTailscale] = &TunnelCostModel{
		Provider:               TunnelProviderTailscale,
		DisplayName:            "Tailscale",
		IsFree:                 true,
		BandwidthCostPerMbps:   0,
		TrafficCostPerGB:       0,
		FreeBandwidthLimit:     0,
		FreeTrafficLimitGB:     0,
		FreeConnectionLimit:     100,  // 100设备
		TypicalLatencyMs:       20,
		MaxConcurrentConnections: 100,
		SupportedProtocols:     []string{"tcp", "udp"},
		LastUpdated:            time.Now(),
	}

	// 自建方案成本模型
	a.models[TunnelProviderSelfHosted] = &TunnelCostModel{
		Provider:               TunnelProviderSelfHosted,
		DisplayName:            "自建穿透服务",
		IsFree:                 false,
		BandwidthCostPerMbps:   a.config.SelfHostBandwidthCost,
		TrafficCostPerGB:       0.8,
		RelayServerCost:        a.config.CostPerRelayServer * float64(a.config.RelayServerCount),
		STUNServerCost:         0,     // 使用公共STUN服务器
		TURNServerCost:         a.config.CostPerRelayServer,
		SSLCostPerYear:         a.config.SSLCostPerYear,
		DomainCostPerYear:      a.config.DomainCostPerYear,
		MaintenanceCostPercent: a.config.MaintenanceCostPercent,
		TypicalLatencyMs:       40,
		MaxConcurrentConnections: 1000,
		SupportedProtocols:     []string{"http", "https", "tcp", "udp"},
		LastUpdated:            time.Now(),
	}
}

// AnalyzeTunnelCost 分析穿透成本
func (a *TunnelCostAnalyzer) AnalyzeTunnelCost(ctx context.Context, deviceID string, usage TunnelUsageStats) (*TunnelCostReport, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 保存使用统计
	a.usage[deviceID] = &usage

	report := &TunnelCostReport{
		ID:          generateReportID(),
		GeneratedAt: time.Now(),
		TimeRange:   TimeRange{StartTime: usage.CollectedAt.Add(-24 * time.Hour), EndTime: usage.CollectedAt},
		UsageStats:  usage,
	}

	// 计算成本明细
	report.CostBreakdown = a.calculateCostBreakdown(usage)

	// 估算月度成本
	report.MonthlyCostEstimate = a.estimateMonthlyCost(usage)
	report.YearlyCostEstimate = report.MonthlyCostEstimate * 12

	// 竞品对比
	report.CompetitorComparison = a.compareWithCompetitors(usage)

	// 生成成本建议
	report.CostSuggestions = a.generateTunnelSuggestions(usage, report.MonthlyCostEstimate)

	// ROI分析
	report.ROIAnalysis = a.analyzeTunnelROI(usage)

	return report, nil
}

// calculateCostBreakdown 计算成本明细
func (a *TunnelCostAnalyzer) calculateCostBreakdown(usage TunnelUsageStats) TunnelCostBreakdown {
	breakdown := TunnelCostBreakdown{}

	// 自建方案成本计算
	// 带宽成本
	breakdown.BandwidthCost = usage.AvgBandwidthMbps * a.config.SelfHostBandwidthCost

	// 流量成本（一般不计费，但按流量计算服务器负载）
	// 这里按带宽估算流量成本的影响
	breakdown.TrafficCost = usage.TotalTrafficGB * 0.1 // 低流量成本

	// 服务器成本（按用户数分摊）
	// 假设每台服务器支持100用户，当前用户分摊成本
	breakdown.ServerCost = a.config.CostPerRelayServer * float64(a.config.RelayServerCount) / 100

	// 中继使用率影响成本
	relayMultiplier := usage.RelayUsagePercent / 100
	breakdown.ServerCost *= relayMultiplier

	// SSL和域名成本（月摊）
	breakdown.SSLCostMonthly = a.config.SSLCostPerYear / 12
	breakdown.DomainCostMonthly = a.config.DomainCostPerYear / 12

	// 维护成本
	breakdown.MaintenanceCost = (breakdown.ServerCost + breakdown.BandwidthCost) * a.config.MaintenanceCostPercent / 100

	// 总成本
	breakdown.TotalCost = breakdown.BandwidthCost + breakdown.TrafficCost +
		breakdown.ServerCost + breakdown.SSLCostMonthly +
		breakdown.DomainCostMonthly + breakdown.MaintenanceCost

	// 如果P2P成功率高，有效成本更低
	p2pSavingsFactor := 1 - usage.P2PSuccessRate/100 * 0.5 // P2P节省50%中继成本
	breakdown.EffectiveCost = breakdown.TotalCost * p2pSavingsFactor

	return breakdown
}

// estimateMonthlyCost 估算月度成本
func (a *TunnelCostAnalyzer) estimateMonthlyCost(usage TunnelUsageStats) float64 {
	// 基于使用情况估算月度成本
	dailyCost := a.calculateCostBreakdown(usage).EffectiveCost

	// 按连接时长比例调整
	hoursInMonth := 30 * 24
	usageRatio := usage.ConnectionHours / float64(hoursInMonth)
	if usageRatio > 1 {
		usageRatio = 1
	}

	monthlyCost := dailyCost * 30 * usageRatio

	// 考虑P2P成功率
	if usage.P2PSuccessRate > 70 {
		monthlyCost *= 0.5 // 高P2P成功率大幅降低成本
	}

	return monthlyCost
}

// compareWithCompetitors 与竞品对比
func (a *TunnelCostAnalyzer) compareWithCompetitors(usage TunnelUsageStats) []TunnelCompetitorCost {
	comparisons := []TunnelCompetitorCost{}

	// 飞牛FN Connect
	fnConnect := TunnelCompetitorCost{
		ProviderName:       "飞牛FN Connect",
		IsFree:             true,
		MonthlyCost:        0,
		Limitations:        "带宽限制约10Mbps，流量100GB/月，绑定飞牛NAS设备",
		Difference:         "免费服务但有设备绑定限制，适合飞牛用户",
		RecommendationScore: 5,
	}
	comparisons = append(comparisons, fnConnect)

	// 群晖QuickConnect
	synology := TunnelCompetitorCost{
		ProviderName:       "群晖QuickConnect",
		IsFree:             true,
		MonthlyCost:        0,
		Limitations:        "带宽限制约5Mbps，绑定群晖设备，功能相对简单",
		Difference:         "免费但绑定设备，适合群晖用户",
		RecommendationScore: 4,
	}
	comparisons = append(comparisons, synology)

	// Cloudflare Tunnel
	cloudflare := TunnelCompetitorCost{
		ProviderName:       "Cloudflare Tunnel",
		IsFree:             true,
		MonthlyCost:        0,
		Limitations:        "需要域名托管到Cloudflare，有企业版付费选项",
		Difference:         "免费且强大，但需要域名托管",
		RecommendationScore: 5,
	}
	comparisons = append(comparisons, cloudflare)

	// Tailscale
	tailscale := TunnelCompetitorCost{
		ProviderName:       "Tailscale",
		IsFree:             true,
		MonthlyCost:        0,
		Limitations:        "个人版免费100设备，企业版需付费",
		Difference:         "基于WireGuard，安全性高，适合个人用户",
		RecommendationScore: 5,
	}
	comparisons = append(comparisons, tailscale)

	// 自建方案
	selfHost := TunnelCompetitorCost{
		ProviderName:       "自建穿透服务",
		IsFree:             false,
		MonthlyCost:        a.estimateMonthlyCost(usage),
		Limitations:        "需要维护服务器，有带宽成本",
		Difference:         "完全可控，无设备绑定，适合多用户场景",
		RecommendationScore: 3,
	}
	comparisons = append(comparisons, selfHost)

	return comparisons
}

// generateTunnelSuggestions 生成穿透成本建议
func (a *TunnelCostAnalyzer) generateTunnelSuggestions(usage TunnelUsageStats, monthlyCost float64) []TunnelCostSuggestion {
	suggestions := []TunnelCostSuggestion{}

	// P2P优化建议
	if usage.P2PSuccessRate < 50 {
		suggestions = append(suggestions, TunnelCostSuggestion{
			Type:             "optimize_p2p",
			Suggestion:       "优化STUN/TURN配置提高P2P成功率，减少中继依赖",
			PotentialSavings: monthlyCost * 0.3,
			Difficulty:       "medium",
			Priority:         1,
		})
	}

	// 带宽优化建议
	if usage.PeakBandwidthMbps > 5 {
		suggestions = append(suggestions, TunnelCostSuggestion{
			Type:             "reduce_bandwidth",
			Suggestion:       "优化数据传输策略，压缩流量减少带宽消耗",
			PotentialSavings: monthlyCost * 0.2,
			Difficulty:       "easy",
			Priority:         2,
		})
	}

	// 服务选择建议
	if monthlyCost > 50 && usage.TotalTrafficGB < 20 {
		suggestions = append(suggestions, TunnelCostSuggestion{
			Type:             "change_provider",
			Suggestion:       "流量较低建议使用免费服务如Cloudflare Tunnel或Tailscale",
			PotentialSavings: monthlyCost,
			Difficulty:       "easy",
			Priority:         3,
		})
	}

	// 自建建议
	if monthlyCost > 100 && usage.ConnectionHours > 500 {
		suggestions = append(suggestions, TunnelCostSuggestion{
			Type:             "self_host",
			Suggestion:       "高频使用场景建议自建中继服务器降低长期成本",
			PotentialSavings: monthlyCost * 0.5,
			Difficulty:       "hard",
			Priority:         4,
		})
	}

	return suggestions
}

// analyzeTunnelROI ROI分析
func (a *TunnelCostAnalyzer) analyzeTunnelROI(usage TunnelUsageStats) TunnelROIAnalysis {
	roi := TunnelROIAnalysis{}

	// 自建方案5年成本
	selfHostMonthly := a.config.SelfHostServerCost +
		a.config.CostPerRelayServer * float64(a.config.RelayServerCount) +
		usage.AvgBandwidthMbps * a.config.SelfHostBandwidthCost

	// 加入维护和SSL域名成本
	selfHostMonthly += (a.config.SSLCostPerYear + a.config.DomainCostPerYear) / 12
	selfHostMonthly *= 1 + a.config.MaintenanceCostPercent/100

	roi.SelfHostCost5Year = selfHostMonthly * 60 // 5年=60月

	// 第三方付费服务估算（假设使用商业穿透服务）
	// 按带宽和流量估算
	thirdPartyMonthly := usage.AvgBandwidthMbps * 2.0 + usage.TotalTrafficGB * 0.5
	roi.ThirdPartyCost5Year = thirdPartyMonthly * 60

	// 计算节省
	if roi.SelfHostCost5Year > 0 {
		roi.SavingsPercent = (roi.ThirdPartyCost5Year - roi.SelfHostCost5Year) / roi.ThirdPartyCost5Year * 100
	}

	// 收益平衡点
	if selfHostMonthly > 0 && thirdPartyMonthly > selfHostMonthly {
		setupCost := a.config.SelfHostServerCost * 3 // 初始部署成本估算
		roi.BreakEvenMonths = int(math.Ceil(setupCost / (thirdPartyMonthly - selfHostMonthly)))
	} else {
		roi.BreakEvenMonths = 0 // 自建更贵，不建议
	}

	// 推荐方案
	if roi.SavingsPercent > 30 && roi.BreakEvenMonths < 24 {
		roi.RecommendedApproach = "自建穿透服务：长期成本更低，完全可控"
	} else if usage.TotalTrafficGB < 50 {
		roi.RecommendedApproach = "使用免费服务：低流量场景无需自建"
	} else {
		roi.RecommendedApproach = "混合方案：高频数据使用自建，低频数据使用免费服务"
	}

	return roi
}

// GetProviderModel 获取提供商定价模型
func (a *TunnelCostAnalyzer) GetProviderModel(provider TunnelProvider) *TunnelCostModel {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.models[provider]
}

// GetAllProviderModels 获取所有提供商定价模型
func (a *TunnelCostAnalyzer) GetAllProviderModels() map[TunnelProvider]*TunnelCostModel {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[TunnelProvider]*TunnelCostModel)
	for k, v := range a.models {
		result[k] = v
	}
	return result
}

// CalculateServiceCost 计算服务成本（用于定价参考）
func (a *TunnelCostAnalyzer) CalculateServiceCost(userType string, bandwidthMbps float64, trafficGB float64) float64 {
	// 基础成本
	baseCost := a.config.CostPerRelayServer * float64(a.config.RelayServerCount) / 100 // 用户分摊

	// 带宽成本
	bwCost := bandwidthMbps * a.config.SelfHostBandwidthCost

	// 流量成本（服务器负载）
	trafficCost := trafficGB * 0.1

	// 总成本
	totalCost := baseCost + bwCost + trafficCost

	// 按用户类型调整
	switch userType {
	case "free":
		// 免费用户成本上限
		if totalCost > 5 {
			totalCost = 5 // 每免费用户最多成本5元/月
		}
	case "basic":
		// 基础付费用户
		totalCost *= 0.8 // 20%折扣
	case "premium":
		// 高级用户
		totalCost *= 0.6 // 40%折扣
	}

	return totalCost
}

// Helper function
func generateReportID() string {
	return "tunnel-" + time.Now().Format("20060102-150405")
}