# 第三方硬盘验证框架设计

## 背景

群晖 DSM 7.3 放开了第三方硬盘限制，推出了硬盘验证计划。我们需要类似的兼容性管理框架，确保 NAS-OS 能够安全、可靠地使用第三方硬盘。

## 设计目标

1. **兼容性检测** - 自动识别硬盘型号，验证兼容性
2. **健康监控增强** - 扩展 SMART 监控，支持更多硬盘型号
3. **数据驱动** - 维护兼容性数据库，支持在线更新
4. **安全告警** - 不兼容或高风险硬盘及时预警

---

## 一、硬盘兼容性检测框架

### 1.1 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                    Disk Compatibility Framework              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Scanner   │  │  Verifier   │  │   Alert Manager     │  │
│  │  硬盘扫描   │→ │  兼容验证   │→ │   告警通知          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│         ↓                ↓                    ↓              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Compatibility Database                   │    │
│  │  - 官方认证硬盘列表                                    │    │
│  │  - 社区验证硬盘列表                                    │    │
│  │  - 风险硬盘黑名单                                      │    │
│  │  - SMART 属性映射表                                    │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 核心组件

#### 1.2.1 硬盘识别器 (DiskIdentifier)

```go
type DiskIdentifier struct {
    // 从 SMART 数据提取硬盘特征
    // - 厂商 (Vendor): WDC, Seagate, Samsung, Toshiba, etc.
    // - 型号 (Model): WD Red, IronWolf, 870 EVO, etc.
    // - 固件版本 (Firmware)
    // - 容量 (Capacity)
    // - 接口类型 (Interface): SATA, NVMe, SAS
}

type DiskIdentity struct {
    Vendor       string    // 厂商
    Model        string    // 型号
    ModelFamily  string    // 型号系列
    Serial       string    // 序列号
    Firmware     string    // 固件版本
    Capacity     uint64    // 容量 (bytes)
    Interface    string    // 接口类型
    RotationRate int       // 转速 (RPM, SSD=0)
    IsSSD        bool      // 是否为 SSD
    FormFactor   string    // 物理尺寸: 2.5", 3.5", M.2
}
```

#### 1.2.2 兼容性验证器 (CompatibilityVerifier)

```go
type CompatibilityVerifier struct {
    db           *CompatibilityDB
    rules        []CompatibilityRule
    riskAnalyzer *RiskAnalyzer
}

type CompatibilityResult struct {
    DiskID       string            // 硬盘标识
    Identity     *DiskIdentity     // 硬盘身份信息
    Status       CompatibilityStatus // 兼容状态
    Level        CompatibilityLevel  // 兼容等级
    Issues       []CompatibilityIssue // 问题列表
    Warnings     []string          // 警告信息
    Recommendations []string       // 建议
    VerifiedAt   time.Time         // 验证时间
    ExpiresAt    time.Time         // 验证过期时间
}

type CompatibilityStatus string
const (
    StatusVerified    CompatibilityStatus = "verified"     // 官方验证
    StatusCommunity   CompatibilityStatus = "community"    // 社区验证
    StatusUnknown     CompatibilityStatus = "unknown"      // 未知
    StatusCaution     CompatibilityStatus = "caution"      // 需谨慎
    StatusUnsupported CompatibilityStatus = "unsupported"  // 不支持
    StatusBlocked     CompatibilityStatus = "blocked"      // 已阻止
)

type CompatibilityLevel int
const (
    LevelA CompatibilityLevel = 5  // 完全兼容
    LevelB CompatibilityLevel = 4  // 良好兼容
    LevelC CompatibilityLevel = 3  // 基本兼容
    LevelD CompatibilityLevel = 2  // 有限兼容
    LevelF CompatibilityLevel = 1  // 不兼容
)
```

#### 1.2.3 风险分析器 (RiskAnalyzer)

```go
type RiskAnalyzer struct {
    // 分析硬盘潜在风险
}

type RiskAssessment struct {
    DiskID          string
    OverallRisk     RiskLevel       // 风险等级
    RiskFactors     []RiskFactor    // 风险因素
    Mitigations     []string        // 缓解措施
    RecommendedActions []string     // 建议操作
}

type RiskLevel string
const (
    RiskNone     RiskLevel = "none"
    RiskLow      RiskLevel = "low"
    RiskMedium   RiskLevel = "medium"
    RiskHigh     RiskLevel = "high"
    RiskCritical RiskLevel = "critical"
)

type RiskFactor struct {
    Category    string    // firmware, capacity, age, usage, etc.
    Description string    // 描述
    Severity    RiskLevel // 严重程度
    Impact      string    // 影响
}
```

---

## 二、SMART 监控增强

### 2.1 扩展 SMART 属性映射

不同厂商的硬盘 SMART 属性 ID 可能不同，需要建立映射表：

```go
type SMARTAttributeMapping struct {
    Vendor      string            // 厂商
    ModelPrefix string            // 型号前缀
    Attributes  map[string]uint   // 属性名 -> ID 映射
    
    // 示例:
    // Samsung 870 EVO:
    //   "wear_leveling" -> 177
    //   "erase_fail" -> 182
    //   "media_wearout" -> 233
    //
    // WDC Red:
    //   "load_cycle" -> 193
    //   "realloc_sectors" -> 5
}

type SMARTThresholdConfig struct {
    Attribute    string    // 属性名
    WarningValue uint      // 警告阈值
    CriticalValue uint     // 严重阈值
    Comparison   string    // "gt" 或 "lt"
    
    // 示例:
    // Reallocated_Sector_Ct:
    //   WarningValue: 10, CriticalValue: 100, Comparison: "gt"
    // Available_Spare:
    //   WarningValue: 10, CriticalValue: 5, Comparison: "lt"
}
```

### 2.2 厂商特定监控

```go
// 厂商特定监控接口
type VendorMonitor interface {
    // 厂商名称
    Vendor() string
    
    // 支持的型号前缀
    SupportedModels() []string
    
    // 解析 SMART 数据
    ParseSMART(data []byte) (*VendorSMARTData, error)
    
    // 计算健康评分
    CalculateHealth(smart *VendorSMARTData) *HealthScore
    
    // 获取预期寿命 (TBW)
    GetExpectedLife(identity *DiskIdentity) *ExpectedLife
    
    // 特定属性阈值
    AttributeThresholds() map[string]*SMARTThresholdConfig
}

// 厂商 SMART 数据
type VendorSMARTData struct {
    // 通用属性
    Temperature     uint
    PowerOnHours    uint64
    PowerCycles     uint64
    
    // SSD 专用
    PercentageUsed  float64   // 已用寿命百分比
    AvailableSpare  float64   // 可用备用空间
    MediaErrors     uint64
    HostWrites      uint64    // 主机写入量
    NANDWrites     uint64    // NAND 写入量
    
    // HDD 专用
    ReallocSectors  uint64
    PendingSectors  uint64
    SeekErrors      uint64
    LoadCycles      uint64
    
    // 原始属性
    RawAttributes   map[uint]*SMARTAttribute
}

// 预期寿命信息
type ExpectedLife struct {
    TBW            uint64    // 总写入量 (TB)
    MTBF           uint64    // 平均无故障时间 (小时)
    WarrantyYears  int       // 保修年限
    EnduranceLevel string    // 耐久等级
}
```

### 2.3 实现厂商监控器

#### Samsung SSD Monitor

```go
type SamsungSSDMonitor struct{}

func (m *SamsungSSDMonitor) Vendor() string { return "Samsung" }

func (m *SamsungSSDMonitor) ParseSMART(data []byte) (*VendorSMARTData, error) {
    // 解析 Samsung 特有的 SMART 属性
    // 属性 177: Wear_Leveling_Count
    // 属性 179: Used_Rsvd_Blk_Cnt_Tot
    // 属性 233: Media_Wearout_Indicator
}
```

#### Seagate HDD Monitor

```go
type SeagateHDDMonitor struct{}

func (m *SeagateHDDMonitor) Vendor() string { return "Seagate" }

func (m *SeagateHDDMonitor) ParseSMART(data []byte) (*VendorSMARTData, error) {
    // 解析 Seagate 特有的 SMART 属性
    // 属性 1: Raw_Read_Error_Rate
    // 属性 7: Seek_Error_Rate
    // 属性 188: Command_Timeout
}
```

---

## 三、兼容性数据库结构

### 3.1 数据库架构

```go
// 兼容性数据库
type CompatibilityDB struct {
    Official     []*OfficialDiskEntry    // 官方认证列表
    Community    []*CommunityDiskEntry   // 社区验证列表
    Blacklist    []*BlacklistEntry       // 黑名单
    SMARTMapping []*SMARTAttributeMapping // SMART 映射
    Thresholds   []*SMARTThresholdConfig  // 阈值配置
}

// 官方认证硬盘条目
type OfficialDiskEntry struct {
    ID              string              `json:"id"`
    Vendor          string              `json:"vendor"`
    Model           string              `json:"model"`
    ModelFamily     string              `json:"modelFamily"`
    Capacity        uint64              `json:"capacity"`
    Interface       string              `json:"interface"`
    FormFactor      string              `json:"formFactor"`
    IsSSD           bool                `json:"isSSD"`
    Compatibility   CompatibilityLevel  `json:"compatibility"`
    ExpectedLife    *ExpectedLife       `json:"expectedLife"`
    SMARTMapping    map[string]uint     `json:"smartMapping"`
    Thresholds      map[string]*AttributeThreshold `json:"thresholds"`
    KnownIssues     []string            `json:"knownIssues"`
    Recommendations []string            `json:"recommendations"`
    VerifiedAt      time.Time           `json:"verifiedAt"`
    UpdatedAt       time.Time           `json:"updatedAt"`
}

// 社区验证硬盘条目
type CommunityDiskEntry struct {
    ID              string              `json:"id"`
    Vendor          string              `json:"vendor"`
    Model           string              `json:"model"`
    Capacity        uint64              `json:"capacity"`
    Compatibility   CompatibilityLevel  `json:"compatibility"`
    UserReports     int                 `json:"userReports"`     // 用户报告数
    PositiveRate    float64             `json:"positiveRate"`    // 正面评价率
    Issues          []string            `json:"issues"`
    Workarounds     []string            `json:"workarounds"`
    SubmittedBy     string              `json:"submittedBy"`
    VerifiedAt      time.Time           `json:"verifiedAt"`
}

// 黑名单条目
type BlacklistEntry struct {
    ID              string        `json:"id"`
    Vendor          string        `json:"vendor"`
    Model           string        `json:"model"`
    Reason          string        `json:"reason"`
    Severity        RiskLevel     `json:"severity"`
    AffectedFirmware []string     `json:"affectedFirmware"`
    Details         string        `json:"details"`
    AddedAt         time.Time     `json:"addedAt"`
    Source          string        `json:"source"` // "official", "community", "vendor"
}

// 属性阈值
type AttributeThreshold struct {
    Warning        uint   `json:"warning"`
    Critical       uint   `json:"critical"`
    Comparison     string `json:"comparison"` // "gt", "lt", "eq"
    Description    string `json:"description"`
}
```

### 3.2 数据库存储

使用 JSON 文件存储，支持热更新：

```
internal/disk/compatibility/
├── database/
│   ├── official.json        # 官方认证列表
│   ├── community.json       # 社区验证列表
│   ├── blacklist.json       # 黑名单
│   ├── smart_mapping.json   # SMART 属性映射
│   └── thresholds.json      # 阈值配置
├── vendor/
│   ├── samsung.go           # Samsung 监控器
│   ├── seagate.go           # Seagate 监控器
│   ├── wdc.go               # WDC 监控器
│   └── toshiba.go           # Toshiba 监控器
├── compatibility.go         # 兼容性验证核心
├── database.go              # 数据库管理
├── scanner.go               # 硬盘扫描
└── risk_analyzer.go         # 风险分析
```

### 3.3 在线更新机制

```go
type DBUpdater struct {
    remoteURL    string
    localPath    string
    version      string
    lastUpdate   time.Time
    checkInterval time.Duration
}

type DBUpdateResult struct {
    Updated       bool      `json:"updated"`
    OldVersion    string    `json:"oldVersion"`
    NewVersion    string    `json:"newVersion"`
    EntriesAdded  int       `json:"entriesAdded"`
    EntriesUpdated int      `json:"entriesUpdated"`
    UpdatedAt     time.Time `json:"updatedAt"`
}

// 定期检查更新（每周）
func (u *DBUpdater) CheckUpdate() (*DBUpdateResult, error) {
    // 1. 请求远程版本信息
    // 2. 比较本地版本
    // 3. 如有新版本，下载更新
    // 4. 验证签名
    // 5. 应用更新
}
```

---

## 四、API 设计

### 4.1 兼容性检查 API

```go
// GET /api/v1/disk/compatibility
// 查询所有硬盘兼容性状态
type ListCompatibilityResponse struct {
    Disks []CompatibilityResult `json:"disks"`
    Total int                   `json:"total"`
}

// GET /api/v1/disk/compatibility/:device
// 查询单个硬盘兼容性
type GetCompatibilityResponse struct {
    Disk    CompatibilityResult `json:"disk"`
    Health  *HealthScore        `json:"health"`
    Risks   *RiskAssessment     `json:"risks"`
}

// POST /api/v1/disk/compatibility/check
// 手动触发兼容性检查
type CheckCompatibilityRequest struct {
    Device string `json:"device"` // 可选，为空则检查所有
    Force  bool   `json:"force"`  // 强制重新检查
}

type CheckCompatibilityResponse struct {
    Results []CompatibilityResult `json:"results"`
    Errors  []string              `json:"errors,omitempty"`
}

// POST /api/v1/disk/compatibility/report
// 上报硬盘兼容性数据（用于社区验证）
type ReportCompatibilityRequest struct {
    Vendor       string `json:"vendor"`
    Model        string `json:"model"`
    Firmware     string `json:"firmware"`
    Capacity     uint64 `json:"capacity"`
    IsWorking    bool   `json:"isWorking"`
    Issues       []string `json:"issues,omitempty"`
    Notes        string `json:"notes,omitempty"`
}
```

### 4.2 兼容性数据库管理 API

```go
// GET /api/v1/disk/compatibility/db/status
// 获取数据库状态
type DBStatusResponse struct {
    Version       string    `json:"version"`
    LastUpdate    time.Time `json:"lastUpdate"`
    OfficialCount int       `json:"officialCount"`
    CommunityCount int      `json:"communityCount"`
    BlacklistCount int      `json:"blacklistCount"`
}

// POST /api/v1/disk/compatibility/db/update
// 触发数据库更新
type UpdateDBResponse struct {
    Result *DBUpdateResult `json:"result"`
    Error  string          `json:"error,omitempty"`
}
```

---

## 五、告警策略

### 5.1 兼容性告警规则

```go
type CompatibilityAlertRule struct {
    // 规则 ID
    ID string `json:"id"`
    
    // 触发条件
    Trigger CompatibilityAlertTrigger `json:"trigger"`
    
    // 告警级别
    Severity AlertSeverity `json:"severity"`
    
    // 告警消息模板
    MessageTemplate string `json:"messageTemplate"`
    
    // 建议操作
    RecommendedActions []string `json:"recommendedActions"`
}

type CompatibilityAlertTrigger string
const (
    // 不兼容硬盘检测到
    TriggerIncompatible CompatibilityAlertTrigger = "incompatible"
    // 风险硬盘检测到
    TriggerHighRisk CompatibilityAlertTrigger = "high_risk"
    // 黑名单硬盘检测到
    TriggerBlacklist CompatibilityAlertTrigger = "blacklist"
    // 固件版本有问题
    TriggerFirmwareIssue CompatibilityAlertTrigger = "firmware_issue"
    // 未知硬盘检测到
    TriggerUnknown CompatibilityAlertTrigger = "unknown"
)
```

### 5.2 告警示例

```json
{
  "type": "compatibility",
  "severity": "critical",
  "device": "/dev/sda",
  "message": "检测到黑名单硬盘: WDC WD40EFRX-68N32N0 (固件版本存在已知数据丢失风险)",
  "details": {
    "vendor": "WDC",
    "model": "WD40EFRX-68N32N0",
    "firmware": "82.00A82",
    "reason": "固件版本存在缺陷，可能导致数据丢失",
    "blacklistSource": "vendor"
  },
  "recommendedActions": [
    "立即备份所有数据",
    "检查是否有固件更新",
    "考虑更换硬盘"
  ]
}
```

---

## 六、实现计划

### 阶段一：基础框架 (Week 1)
- [x] 兼容性检测核心模块
- [x] 硬盘识别器
- [x] 兼容性验证器
- [x] 基础数据库结构

### 阶段二：SMART 增强 (Week 2)
- [x] SMART 属性映射
- [x] 厂商特定监控器接口
- [x] Samsung/Seagate/WDC 监控器实现
- [x] 阈值配置

### 阶段三：风险分析 (Week 3)
- [x] 风险分析器
- [x] 告警系统集成
- [x] API 端点实现
- [x] 测试覆盖

### 阶段四：数据库与在线更新 (Week 4)
- [ ] 初始数据库填充
- [ ] 在线更新机制
- [ ] 社区上报功能
- [ ] 文档完善

---

## 七、测试策略

### 7.1 单元测试
- 硬盘识别器测试
- 兼容性验证逻辑测试
- SMART 属性解析测试
- 风险分析测试

### 7.2 集成测试
- 与现有 SMART 监控模块集成
- 与告警系统集成
- API 端点测试

### 7.3 模拟测试
- 模拟各种硬盘型号
- 模拟 SMART 数据输出
- 模拟告警场景

---

## 八、参考

- 群晖硬盘兼容性列表: https://www.synology.com/compatibility
- SMART 属性参考: https://www.smartmontools.org/wiki/Attributes
- 厂商 SMART 文档:
  - Samsung SSD: https://www.samsung.com/semiconductor/
  - Seagate: https://www.seagate.com/
  - WDC: https://www.westerndigital.com/