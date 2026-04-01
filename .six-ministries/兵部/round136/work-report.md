# 兵部 Round136 工作报告

**任务**: Dashboard全局搜索API + GPU智能调度增强  
**对标**: TrueNAS全局搜索 + 飞牛核显加速人脸  
**完成时间**: 2026-04-01

---

## 一、全局搜索API设计

### 1.1 现有架构分析

nas-os已具备完善的全局搜索基础设施：

**核心组件**:
- `internal/search/global.go` - GlobalSearchService全局搜索服务
- `internal/search/global_handlers.go` - HTTP API处理器
- `internal/search/engine.go` - Bleve全文搜索引擎
- `internal/search/indexer.go` - 文件索引器（支持增量索引）
- `internal/search/query.go` - 高级查询解析器

**已支持的搜索类型**:
| 类型 | 说明 | 状态 |
|------|------|------|
| file | 文件搜索 | ✅ 已实现 |
| setting | 设置项搜索 | ✅ 已实现 |
| app | 应用搜索 | ✅ 已实现 |
| container | 容器搜索 | ✅ 已实现 |
| metadata | 元数据搜索 | ✅ 已实现 |
| api | API端点搜索 | ✅ 已实现 |
| doc | 文档搜索 | ✅ 已实现 |
| log | 日志搜索 | ✅ 已实现 |

### 1.2 API端点设计

**对标TrueNAS全局搜索框功能**

```
POST /api/v1/search/global     - 全局搜索（跨模块）
GET  /api/v1/search/quick      - 快速搜索（自动补全）
GET  /api/v1/search/suggestions - 搜索建议
POST /api/v1/search/files      - 文件专项搜索
POST /api/v1/search/settings   - 设置专项搜索
POST /api/v1/search/apps       - 应用专项搜索
POST /api/v1/search/apis       - API端点搜索
POST /api/v1/search/logs       - 日志搜索
GET  /api/v1/search/categories - 获取搜索分类
GET  /api/v1/search/stats      - 搜索统计
GET  /api/v1/search/history/recent   - 最近搜索
GET  /api/v1/search/history/popular  - 热门搜索
DELETE /api/v1/search/history  - 清除历史
```

### 1.3 搜索请求/响应结构

**请求体**:
```go
type GlobalSearchRequest struct {
    Query      string                   `json:"query"`         // 搜索关键词
    Types      []GlobalSearchResultType `json:"types"`         // 结果类型过滤
    Limit      int                      `json:"limit"`         // 每类结果上限
    TotalLimit int                      `json:"totalLimit"`    // 总结果上限
    MinScore   float64                  `json:"minScore"`      // 最小匹配分数
    IncludeRaw bool                     `json:"includeRaw"`    // 包含原始数据
    Fuzzy      bool                     `json:"fuzzy"`         // 模糊搜索
    Locale     string                   `json:"locale"`        // 语言环境
}
```

**响应体**:
```go
type GlobalSearchResponse struct {
    Query       string               `json:"query"`
    Took        time.Duration        `json:"took"`       // 搜索耗时
    Total       int                  `json:"total"`
    Files       []GlobalSearchResult `json:"files"`
    Settings    []GlobalSearchResult `json:"settings"`
    Apps        []GlobalSearchResult `json:"apps"`
    Containers  []GlobalSearchResult `json:"containers"`
    APIs        []GlobalSearchResult `json:"apis"`
    Docs        []GlobalSearchResult `json:"docs"`
    Logs        []GlobalSearchResult `json:"logs"`
    Suggestions []string             `json:"suggestions"` // 搜索建议
    Facets      map[string]int       `json:"facets"`      // 分面统计
}
```

### 1.4 高级查询语法

对标TrueNAS搜索语法：

| 语法 | 示例 | 说明 |
|------|------|------|
| 普通搜索 | `docker` | 全文搜索 |
| 精确匹配 | `"storage pool"` | 短语精确匹配 |
| 必须包含 | `+backup` | 结果必须包含backup |
| 排除词 | `-temp` | 排除包含temp的结果 |
| 字段过滤 | `name:config` | 按字段过滤 |
| 类型过滤 | `type:pdf` | 文件类型过滤 |
| 路径过滤 | `path:/data` | 路径前缀过滤 |
| 大小过滤 | `size:>10MB` | 大于10MB |
| 时间过滤 | `date:>2024-01-01` | 指定日期后 |
| 正则表达式 | `/docker.*compose/` | 正则匹配 |

---

## 二、搜索索引构建

### 2.1 文件索引器（已实现）

**增量索引机制**:
- 基于文件修改时间(ModTime)检测变化
- 内容哈希比对（文本文件）
- 状态持久化（避免重复索引）

**配置项**:
```go
type IndexerConfig struct {
    Workers         int      `json:"workers"`         // 并行索引线程数
    BatchSize       int      `json:"batchSize"`       // 批量索引大小
    MaxFileSize     int64    `json:"maxFileSize"`     // 最大索引文件
    EnableWatch     bool     `json:"enableWatch"`     // 实时监控
    ExcludePatterns []string `json:"excludePatterns"` // 排除目录
    IndexHidden     bool     `json:"indexHidden"`     // 索引隐藏文件
}
```

**默认排除模式**:
- `.git`, `.svn`, `.hg`, `node_modules`, `vendor`
- `tmp`, `temp`, `cache`, `.cache`, `__pycache__`
- `.idea`, `.vscode`, `*.tmp`, `*.bak`

### 2.2 设置索引（已实现）

**分类体系**:
| 分类 | 设置项数 | 覆盖范围 |
|------|----------|----------|
| storage | 5 | 存储池、数据集、快照、共享、配额 |
| network | 5 | 接口、路由、DNS、代理、防火墙 |
| system | 7 | 常规、用户、组、时间、任务、告警、日志 |
| security | 5 | SSH、SSL、2FA、审计、RBAC |
| services | 5 | SMB、NFS、WebDAV、FTP、iSCSI |
| containers | 4 | Docker、Compose、Registry、网络 |
| protection | 4 | 备份、复制、云同步、校验 |
| apps | 3 | 商店、已安装、模板 |
| monitoring | 3 | Dashboard、报表、告警 |

### 2.3 API路由索引（已实现）

**已注册API端点**（50+）:
- 存储 API: pools, datasets, snapshots
- 网络 API: interfaces, dns, firewall
- 用户 API: CRUD操作
- 容器 API: 容器管理、镜像、日志
- 文件 API: 上传、下载、列表
- 备份 API: 任务管理、云同步
- 监控 API: 状态、指标、告警
- 安全 API: 认证、证书、审计

### 2.4 建议新增索引

**推荐扩展**:
1. **Dashboard Widget索引** - 支持搜索Widget配置
2. **GPU任务索引** - 搜索GPU分配记录
3. **配置项索引** - 系统配置文件关键词索引

---

## 三、搜索结果排序与权重算法

### 3.1 权重体系（已实现）

**匹配权重**:
| 匹配类型 | 权重 | 说明 |
|----------|------|------|
| 名称精确匹配 | 1.0 | 最高权重 |
| ID匹配 | 0.9 | 唯一标识匹配 |
| 关键词匹配 | 0.8 | 关键词数组匹配 |
| 描述匹配 | 0.7 | 描述字段匹配 |
| 分类匹配 | 0.6 | 分类字段匹配 |
| 标签匹配 | 0.5 | 标签数组匹配 |

**元数据搜索权重**:
```go
func calculateMetadataScore(item MetadataItem, query string) float64 {
    score := 0.0
    // 标题匹配: 0.5
    // 描述匹配: 0.3
    // 标签匹配: 0.2
    // 属性匹配: 0.1
    return min(score, 1.0)
}
```

### 3.2 排序策略

**已实现**:
- 按Score降序排列
- 支持字段排序（size, modTime等）
- 支持升序/降序控制

**建议增强**:
```go
// 增强排序算法建议
type EnhancedSortConfig struct {
    // 综合评分因子
    NameWeight       float64 `json:"nameWeight"`       // 名称权重
    MatchWeight      float64 `json:"matchWeight"`      // 匹配类型权重
    RecencyWeight    float64 `json:"recencyWeight"`    // 时间新鲜度权重
    PopularityWeight float64 `json:"popularityWeight"` // 热度权重
    
    // 类型优先级
    TypePriority map[string]int `json:"typePriority"` 
    // 例: setting=10, app=8, file=5, log=3
}
```

### 3.3 搜索建议生成

**热门搜索关键词**（已实现）:
- 存储池、用户管理、Docker容器、快照
- 网络设置、备份、SSL证书、监控

**智能建议**:
```go
// 基于查询生成相关建议
commonSuggestions := map[string][]string{
    "存储": {"存储池", "数据集", "快照", "共享"},
    "网络": {"网络接口", "DNS", "防火墙"},
    "用户": {"用户管理", "用户组", "权限"},
    "备份": {"备份任务", "复制", "云同步"},
}
```

---

## 四、GPU调度策略优化

### 4.1 现有调度策略

**已实现策略**:
| 策略 | 说明 | 适用场景 |
|------|------|----------|
| round-robin | 轮询分配 | 均衡负载 |
| priority | 优先级分配 | 关键任务优先 |
| exclusive | 独占分配 | 高性能需求 |
| least-loaded | 最小负载 | 负载均衡 |
| most-memory | 最大显存 | 大模型任务 |

### 4.2 Intel QuickSync优化方案

**对标飞牛核显加速人脸识别**

当前实现（`internal/face/intel_qsv_acceleration.go`）:
- QSVAccelerator基础框架已创建
- 检测Intel GPU可用性
- 支持启用/禁用配置

**建议增强方案**:

```go
// 增强的GPU调度策略 - Intel QuickSync优先人脸任务
type IntelQuickSyncScheduler struct {
    baseScheduler    *Scheduler
    faceTaskPriority int              // 人脸任务优先级
    qsvDevices       []string         // Intel QSV设备列表
    taskQueue        map[string]int   // 任务类型权重
}

// 任务类型权重配置
var TaskTypeWeights = map[string]int{
    "face_detection":   100,  // 人脸检测最高优先
    "face_recognition": 90,   // 人脸识别次优先
    "video_encode":     70,   // 视频编码
    "video_decode":     60,   // 视频解码
    "general_compute":  50,   // 通用计算
    "batch_processing": 30,   // 批量处理最低
}

// Intel QSV设备选择逻辑
func (s *IntelQuickSyncScheduler) SelectGPUForTask(taskType string) (*GPUDevice, error) {
    // 1. 人脸任务优先分配Intel GPU（核显）
    if taskType == "face_detection" || taskType == "face_recognition" {
        // 查找Intel QuickSync设备
        for _, device := range s.qsvDevices {
            if gpu := s.getGPUByVendor("intel", device); gpu != nil {
                return gpu, nil
            }
        }
    }
    
    // 2. 其他任务按默认策略
    return s.baseScheduler.SelectGPU(req)
}
```

### 4.3 GPU类型检测增强

```go
// 扩展GPUDevice结构
type GPUDevice struct {
    // 现有字段...
    
    // 新增Intel QSV相关字段
    SupportsQSV     bool     `json:"supportsQSV"`     // 是否支持QuickSync
    QSVCapabilities []string `json:"qsvCapabilities"` // QSV能力列表
    VAAPIDevice     string   `json:"vaapiDevice"`     // VA-API设备路径
}

// Intel GPU检测
func detectIntelGPU() ([]GPUDevice, error) {
    // 1. 检查 /dev/dri/renderD128
    // 2. 执行 vainfo 验证VA-API
    // 3. 检查Intel Media SDK状态
    // 4. 获取QSV能力列表
}
```

### 4.4 人脸任务调度流程

```
┌─────────────────────────────────────────────────────────────┐
│                    人脸任务调度流程                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. 任务提交 → 任务类型识别                                   │
│     │                                                       │
│     ▼                                                       │
│  2. 类型判断                                                 │
│     ├─ face_detection → Intel GPU优先                       │
│     ├─ face_recognition → Intel GPU优先                     │
│     ├─ video_encode → 按负载选择                             │
│     └─ general_compute → 默认策略                            │
│     │                                                       │
│     ▼                                                       │
│  3. GPU选择                                                 │
│     ├─ Intel核显可用 → 分配QSV设备                           │
│     ├─ Intel核显不可用 → NVIDIA GPU                          │
│     └─ 无GPU → CPU降级处理                                   │
│     │                                                       │
│     ▼                                                       │
│  4. 资源分配                                                 │
│     ├─ 显存限制配置                                          │
│     ├─ CUDA/QSV核心限制                                     │
│     └─ 独占/共享模式                                         │
│     │                                                       │
│     ▼                                                       │
│  5. 任务执行 → 监控 → 完成/释放                              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 五、与竞品对比

### 5.1 全局搜索对比

| 功能 | nas-os | TrueNAS | 群晖 | 飞牛 |
|------|:------:|:-------:|:----:|:----:|
| 全局搜索框 | ✅ | ✅ | ❌ | ❌ |
| 文件搜索 | ✅ | ✅ | ✅ | ✅ |
| 设置搜索 | ✅ | ✅ | ❌ | ❌ |
| API搜索 | ✅ | ❌ | ❌ | ❌ |
| 日志搜索 | ✅ | ❌ | ❌ | ❌ |
| 搜索建议 | ✅ | ✅ | ❌ | ❌ |
| 高级语法 | ✅ | ✅ | ❌ | ❌ |
| 增量索引 | ✅ | ✅ | ❌ | ❌ |

**nas-os优势**:
- 搜索类型覆盖更广（API、日志、文档）
- 支持正则表达式搜索
- 完善的搜索历史和建议

### 5.2 GPU加速对比

| 功能 | nas-os | TrueNAS | 群晖 | 飞牛 |
|------|:------:|:-------:|:----:|:----:|
| GPU管理 | ✅ | ❌ | ❌ | ❌ |
| NVIDIA支持 | ✅ | ❌ | ❌ | ❌ |
| Intel QSV | ✅框架 | ❌ | ❌ | ✅ |
| 人脸加速 | ✅框架 | ❌ | ❌ | ✅ |
| 调度策略 | 5种 | ❌ | ❌ | 1种 |
| 独占模式 | ✅ | ❌ | ❌ | ❌ |

**nas-os优势**:
- 多厂商GPU支持（NVIDIA + Intel）
- 多种调度策略可选
- 完善的GPU健康监控

---

## 六、实施建议

### 6.1 短期优化（本周）

1. **完善Intel QSV实现**
   - 实现VA-API设备检测
   - 集成Intel Media SDK/oneVPL
   - 添加人脸任务优先调度逻辑

2. **增强搜索排序**
   - 添加时间新鲜度因子
   - 实现类型优先级配置
   - 优化搜索建议算法

### 6.2 中期优化（下周）

1. **搜索性能优化**
   - 添加搜索结果缓存
   - 实现异步索引更新
   - 优化并发搜索性能

2. **GPU监控增强**
   - Intel GPU温度/功耗监控
   - QSV任务执行统计
   - GPU利用率报告

### 6.3 长期优化

1. **智能搜索**
   - 基于用户行为的搜索建议
   - 搜索结果个性化排序
   - 自然语言搜索支持

2. **GPU池化**
   - 多GPU负载均衡
   - GPU任务队列管理
   - 远程GPU共享

---

## 七、代码示例

### 7.1 全局搜索使用

```go
// 创建全局搜索服务
service := NewGlobalSearchService(engine, settingsRegistry, appRegistry, logger)

// 执行全局搜索
req := GlobalSearchRequest{
    Query:    "docker",
    Limit:    5,
    MinScore: 0.3,
}
resp, err := service.GlobalSearch(ctx, req)

// 快速搜索（自动补全）
resp, err := service.QuickSearch(ctx, "stor", 3)

// 获取热门搜索
popular := service.GetPopularSearches()
```

### 7.2 GPU调度使用

```go
// 创建GPU管理器
manager, err := NewManager(config, logger)

// 创建调度器（Intel QSV优先）
scheduler := NewScheduler(manager, "priority", logger)

// 分配GPU（人脸任务优先Intel核显）
req := &GPUAllocation{
    ContainerID: "face-detection-01",
    Priority:    PriorityHigh,
    MemoryLimit: 1024, // 1GB
}
result, err := manager.AllocateGPU(req)
```

---

## 八、总结

nas-os已具备完善的全局搜索和GPU调度基础架构：

**全局搜索**:
- ✅ 8种搜索类型全覆盖
- ✅ 高级查询语法支持
- ✅ 增量索引机制
- ✅ 搜索历史和建议

**GPU调度**:
- ✅ 5种调度策略
- ✅ NVIDIA GPU完整支持
- ✅ Intel QSV框架已搭建
- 🔲 人脸任务优先调度（建议实现）

对标TrueNAS和飞牛的核心功能已实现，建议进一步优化Intel QuickSync人脸任务优先调度逻辑，实现与飞牛同等的人脸识别加速能力。