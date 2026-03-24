# 安全风险指标模块

## 概述

安全风险指标模块实现了基于 KEV（Known Exploited Vulnerabilities）和 EPSS（Exploit Prediction Scoring System）的漏洞风险评分系统，参考群晖 DSM 7.3 的安全风险指标功能设计。

## 核心功能

### 1. KEV（已知被利用漏洞）数据库集成

#### 数据来源
- **CISA KEV Catalog**: 美国网络安全和基础设施安全局维护的已知被利用漏洞目录
- **数据源 URL**: `https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json`

#### KEV 数据结构
```go
type KEVInfo struct {
    CVEID                      string     // CVE 编号
    VendorProject              string     // 厂商/项目
    Product                    string     // 产品名称
    VulnerabilityName          string     // 漏洞名称
    DateAdded                  time.Time  // 加入目录日期
    ShortDescription           string     // 简短描述
    RequiredAction             string     // 需要采取的行动
    DueDate                    *time.Time // 修复期限
    KnownRansomwareCampaignUse string     // 是否被勒索软件使用 (Known/Unknown)
    Notes                      string     // 备注
    ActiveExploitation         bool       // 是否存在活跃利用
}
```

#### KEV API 方法
| 方法 | 描述 |
|------|------|
| `FetchKEVCatalog()` | 从 CISA 获取最新 KEV 目录 |
| `GetKEVInfo(cveID)` | 获取指定 CVE 的 KEV 信息 |
| `IsInKEV(cveID)` | 检查 CVE 是否在 KEV 目录中 |
| `SearchKEV(query)` | 搜索 KEV 目录 |
| `FilterKEVByVendor(vendor)` | 按厂商过滤 |
| `FilterKEVByProduct(product)` | 按产品过滤 |
| `FilterKEVByRansomware()` | 获取与勒索软件相关的漏洞 |
| `GetOverdueKEVVulnerabilities()` | 获取过期未修复的漏洞 |

### 2. EPSS（漏洞利用预测评分系统）集成

#### 数据来源
- **FIRST EPSS API**: Forum of Incident Response and Security Teams 提供的 EPSS 数据
- **数据源 URL**: `https://api.first.org/data/v1/epss`

#### EPSS 数据结构
```go
type EPSSData struct {
    CVEID       string    // CVE 编号
    EPSSScore   float64   // EPSS 概率评分 (0-1)
    Percentile  float64   // 百分位排名 (0-1)
    Date        time.Time // 数据日期
}
```

#### EPSS 阈值配置
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `EPSSThreshold` | 0.1 | EPSS 分数阈值，超过此值视为高风险 |
| `EPSSPercentileThreshold` | 0.9 | 百分位阈值，超过此值视为高风险 |

#### EPSS API 方法
| 方法 | 描述 |
|------|------|
| `FetchEPSSData(cveIDs)` | 批量获取 EPSS 数据（每次最多 100 个） |
| `GetEPSSData(cveID)` | 获取单个 CVE 的 EPSS 数据 |

### 3. 综合风险评分

#### 评分因子与权重
```go
type RiskScoreFactors struct {
    CVSSWeight       float64  // CVSS 权重 (25%)
    EPSSWeight       float64  // EPSS 权重 (20%)
    KEVWeight        float64  // KEV 权重 (25%)
    AgeWeight        float64  // 漏洞年龄权重 (10%)
    AssetWeight      float64  // 资产重要性权重 (10%)
    ExploitWeight    float64  // 已知利用权重 (5%)
    RansomwareWeight float64  // 勒索软件权重 (5%)
}
```

#### 风险评分计算公式
```
RiskScore = CVSS_贡献 + EPSS_贡献 + KEV_贡献 + 年龄_贡献 + 
            资产_贡献 + 利用_贡献 + 勒索软件_贡献

其中：
- CVSS_贡献 = CVSSScore × 10 × CVSSWeight
- EPSS_贡献 = EPSSScore × 100 × EPSSWeight
- KEV_贡献 = IsInKEV ? 100 × KEVWeight : 0
- 年龄_贡献 = 年龄因子 × 100 × AgeWeight
  - < 30天: 1.0
  - < 90天: 0.8
  - < 365天: 0.6
  - >= 365天: 0.4
- 资产_贡献 = AssetImportance × 100 × AssetWeight
- 利用_贡献 = IsInKEV ? 100 × ExploitWeight : 0
- 勒索软件_贡献 = IsRansomwareRelated ? 100 × RansomwareWeight : 0
```

#### 风险等级划分
| 分数范围 | 风险等级 |
|----------|----------|
| 80-100 | Critical (严重) |
| 60-79 | High (高危) |
| 40-59 | Medium (中危) |
| 0-39 | Low (低危) |

#### 可利用性评估
| 状态 | 判断条件 |
|------|----------|
| `known` | 在 KEV 目录中 |
| `likely` | EPSS 分数 ≥ 阈值 或 百分位 ≥ 阈值 |
| `potential` | EPSS 分数 > 0.01 |
| `none` | 无利用迹象 |

#### 优先级划分
| 优先级 | 分数范围 | 说明 |
|--------|----------|------|
| 1 | 80-100 | 最高优先级，需立即处理 |
| 2 | 60-79 | 高优先级 |
| 3 | 40-59 | 中等优先级 |
| 4 | 0-39 | 低优先级 |

## 使用示例

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "nas-os/internal/security"
)

func main() {
    // 创建风险指标管理器
    config := security.DefaultRiskIndicatorConfig()
    manager := security.NewRiskIndicatorManager(config)
    defer manager.Stop()
    
    // 获取 KEV 目录
    ctx := context.Background()
    if err := manager.FetchKEVCatalog(ctx); err != nil {
        fmt.Printf("获取 KEV 目录失败: %v\n", err)
    }
    
    // 检查特定 CVE 是否在 KEV 中
    cveID := "CVE-2024-0001"
    if manager.IsInKEV(cveID) {
        kevInfo := manager.GetKEVInfo(cveID)
        fmt.Printf("KEV 信息: %+v\n", kevInfo)
    }
    
    // 批量获取 EPSS 数据
    cveIDs := []string{"CVE-2024-0001", "CVE-2024-0002"}
    epssData, _ := manager.FetchEPSSData(ctx, cveIDs)
    for cve, data := range epssData {
        fmt.Printf("%s: EPSS=%.4f, Percentile=%.4f\n", 
            cve, data.EPSSScore, data.Percentile)
    }
    
    // 计算风险评分
    indicator := manager.CalculateRiskScore(
        "CVE-2024-0001",
        9.8,              // CVSS 分数
        1.0,              // 资产重要性
        time.Now().AddDate(0, -1, 0), // 发布日期
    )
    
    fmt.Printf("风险评分: %.2f\n", indicator.RiskScore)
    fmt.Printf("风险等级: %s\n", indicator.RiskLevel)
    fmt.Printf("可利用性: %s\n", indicator.Exploitability)
    fmt.Printf("优先级: %d\n", indicator.Priority)
}
```

### 生成风险报告

```go
func generateReport() {
    manager := security.NewRiskIndicatorManager(
        security.DefaultRiskIndicatorConfig(),
    )
    defer manager.Stop()
    
    // 模拟漏洞列表
    vulnerabilities := []security.VulnerabilityItem{
        {
            CVEID:       "CVE-2024-21763",
            CVSSScore:   9.8,
            FirstDetected: time.Now().AddDate(0, -1, 0),
        },
        {
            CVEID:       "CVE-2024-1709",
            CVSSScore:   10.0,
            FirstDetected: time.Now().AddDate(0, -2, 0),
        },
    }
    
    // 生成风险报告
    report := manager.GenerateRiskReport(vulnerabilities)
    
    fmt.Printf("总漏洞数: %d\n", report.TotalVulnerabilities)
    fmt.Printf("严重: %d, 高危: %d, 中危: %d, 低危: %d\n",
        report.CriticalCount, report.HighCount,
        report.MediumCount, report.LowCount)
    fmt.Printf("KEV 漏洞: %d\n", report.KEVCount)
    fmt.Printf("高 EPSS 漏洞: %d\n", report.HighEPSSCount)
    fmt.Printf("勒索软件相关: %d\n", report.RansomwareRelated)
    
    // 显示前 10 个最高风险漏洞
    for i, ind := range report.TopRisks {
        fmt.Printf("%d. %s - 风险评分: %.2f\n", 
            i+1, ind.CVEID, ind.RiskScore)
    }
}
```

### 搜索和过滤 KEV

```go
func searchKEV() {
    manager := security.NewRiskIndicatorManager(
        security.DefaultRiskIndicatorConfig(),
    )
    defer manager.Stop()
    
    // 获取最新 KEV 目录
    ctx := context.Background()
    manager.FetchKEVCatalog(ctx)
    
    // 搜索特定关键词
    results := manager.SearchKEV("Microsoft")
    fmt.Printf("找到 %d 个相关漏洞\n", len(results))
    
    // 按厂商过滤
    ciscoVulns := manager.FilterKEVByVendor("Cisco")
    fmt.Printf("Cisco 相关漏洞: %d\n", len(ciscoVulns))
    
    // 获取勒索软件相关漏洞
    ransomwareVulns := manager.FilterKEVByRansomware()
    fmt.Printf("勒索软件相关漏洞: %d\n", len(ransomwareVulns))
    
    // 获取过期未修复的漏洞
    overdueVulns := manager.GetOverdueKEVVulnerabilities()
    fmt.Printf("过期未修复: %d\n", len(overdueVulns))
}
```

## 配置选项

```go
type RiskIndicatorConfig struct {
    Enabled                    bool            // 是否启用
    KEVSourceURL              string          // KEV 数据源 URL
    EPSSSourceURL             string          // EPSS 数据源 URL
    KEVUpdateInterval         time.Duration   // KEV 更新间隔
    EPSSUpdateInterval        time.Duration   // EPSS 更新间隔
    CacheDir                  string          // 缓存目录
    RequestTimeout            time.Duration   // 请求超时
    EPSSThreshold             float64         // EPSS 高风险阈值
    EPSSPercentileThreshold   float64         // EPSS 百分位阈值
    ScoreFactors              RiskScoreFactors // 评分因子
    AutoUpdate                bool            // 自动更新
    OfflineMode               bool            // 离线模式
}
```

## 与现有模块集成

### 与漏洞扫描器集成

```go
// 在漏洞扫描结果中添加风险指标
func enhanceVulnerabilityScan() {
    vulnScanner := security.NewVulnerabilityScanner(
        security.DefaultVulnerabilityConfig(),
    )
    riskManager := security.NewRiskIndicatorManager(
        security.DefaultRiskIndicatorConfig(),
    )
    
    // 执行漏洞扫描
    result := vulnScanner.ScanSystem()
    
    // 为扫描结果添加风险指标
    indicators := riskManager.GetRiskIndicators(result.Vulnerabilities)
    
    // 生成增强报告
    report := riskManager.GenerateRiskReport(result.Vulnerabilities)
    
    // 将风险指标附加到漏洞项
    for i, vuln := range result.Vulnerabilities {
        if i < len(indicators) {
            // 更新优先级
            vuln.Priority = indicators[i].Priority
        }
    }
}
```

### 与安全评分引擎集成

```go
// 在安全评分中考虑风险指标
func enhanceSecurityScore() {
    scoreEngine := security.NewScoreEngine(security.ScoreConfig{})
    riskManager := security.NewRiskIndicatorManager(
        security.DefaultRiskIndicatorConfig(),
    )
    
    // 获取风险统计
    vulnerabilities := getVulnerabilities()
    indicators := riskManager.GetRiskIndicators(vulnerabilities)
    stats := riskManager.GetStatistics(indicators)
    
    // 根据风险指标调整安全评分
    adjustment := 0.0
    if stats.KEVPercentage > 10 {
        adjustment -= 10 // KEV 漏洞比例过高，扣分
    }
    if stats.CriticalCount > 0 {
        adjustment -= float64(stats.CriticalCount) * 5
    }
    
    // 更新评分
    // ... (与 ScoreEngine 集成逻辑)
}
```

## 数据持久化

### 缓存文件位置
```
/var/lib/nas-os/risk-indicators/
├── kev_catalog.json    # KEV 目录缓存
└── epss_cache.json     # EPSS 数据缓存
```

### 缓存更新策略
1. **KEV 目录**: 每 24 小时自动更新
2. **EPSS 数据**: 按需获取，自动缓存
3. **离线模式**: 仅使用本地缓存数据

## API 端点设计

### REST API

```
GET /api/v1/security/risk-indicators/status
    获取风险指标管理器状态

GET /api/v1/security/risk-indicators/kev
    获取 KEV 目录

GET /api/v1/security/risk-indicators/kev/search?q={query}
    搜索 KEV 目录

GET /api/v1/security/risk-indicators/kev/vendor/{vendor}
    按厂商过滤 KEV

GET /api/v1/security/risk-indicators/kev/ransomware
    获取勒索软件相关漏洞

GET /api/v1/security/risk-indicators/kev/overdue
    获取过期未修复漏洞

GET /api/v1/security/risk-indicators/epss/{cve}
    获取指定 CVE 的 EPSS 数据

POST /api/v1/security/risk-indicators/calculate
    计算风险评分
    Body: { "cve_id": "CVE-2024-0001", "cvss_score": 9.8 }

POST /api/v1/security/risk-indicators/report
    生成风险报告
    Body: { "vulnerabilities": [...] }

POST /api/v1/security/risk-indicators/refresh
    刷新 KEV 和 EPSS 数据
```

## 最佳实践

### 1. 优先处理 KEV 漏洞
KEV 目录中的漏洞已被证实存在实际利用，应优先修复：
- 检查 KEV 目录中是否有相关产品漏洞
- 关注 CISA 规定的修复期限
- 优先处理与勒索软件相关的漏洞

### 2. 使用 EPSS 预测风险
EPSS 可帮助识别可能被利用的漏洞：
- EPSS 分数 > 0.1 且百分位 > 90% 的漏洞应高度关注
- 结合 CVSS 分数综合评估
- 关注 EPSS 分数的趋势变化

### 3. 综合风险评分
使用综合风险评分而非单一指标：
- 考虑 CVSS、EPSS、KEV 等多个维度
- 根据资产重要性调整权重
- 定期更新评分因子

### 4. 自动化更新
启用自动更新确保数据最新：
- KEV 目录每日更新
- EPSS 数据按需获取
- 定期检查过期漏洞

## 参考资料

- [CISA Known Exploited Vulnerabilities Catalog](https://www.cisa.gov/known-exploited-vulnerabilities-catalog)
- [FIRST EPSS API Documentation](https://www.first.org/epss/api)
- [NIST CVSS Calculator](https://nvd.nist.gov/vuln-metrics/cvss)
- [群晖 DSM 安全中心](https://www.synology.com/en-global/dsm/feature/security_center)

## 版本历史

| 版本 | 日期 | 变更说明 |
|------|------|----------|
| 1.0.0 | 2026-03-24 | 初始版本，实现 KEV 和 EPSS 集成 |