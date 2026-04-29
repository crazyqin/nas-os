# SMB Spotlight 完整设计文档

> 工部第236轮任务 | 对标TrueNAS 26 SMB Spotlight | 2026-04-24

---

## 1. 概述

### 1.1 项目背景

TrueNAS 26的SMB Spotlight功能是macOS用户的核心需求，允许用户通过Finder的Spotlight直接搜索NAS共享文件。nas-os现有实现（`spotlight_integration.go`）已具备基础框架，需要深化协议层集成。

### 1.2 设计目标

- **协议层集成**: SMB Spotlight扩展协议（mds_rpc）实现
- **macOS完全兼容**: Finder搜索无缝工作
- **高性能索引**: 支持大规模文件场景（>10万文件）
- **中文优化**: CJK分词 + 中文文件名搜索

### 1.3 现有实现状态

| 模块 | 状态 | 位置 |
|------|------|------|
| Spotlight集成框架 | ✅ 完成 | `internal/smb/spotlight_integration.go` |
| 索引器 | ✅ 基础完成 | 同上 `Indexer` |
| MDQuery解析器 | ✅ 基础完成 | 同上 `MDQueryHandler` |
| Bleve全文索引 | ✅ 已有 | `internal/webshare/bleve_index.go` |
| SMB配置集成 | ✅ 已有 | `GenerateSMBSpotlightConfig()` |

---

## 2. Spotlight协议分析

### 2.1 macOS Spotlight架构

macOS Spotlight由以下组件构成：

```
┌─────────────────────────────────────────────────────────────┐
│                    macOS Client                              │
├─────────────────────────────────────────────────────────────┤
│  Finder Spotlight UI                                         │
│       ↓                                                      │
│  MDQuery API / mdfind CLI                                    │
│       ↓                                                      │
│  Spotlight Server (mds)                                      │
│       ↓                                                      │
│  mds_rpc → SMB Spotlight Extension                           │
└─────────────────────────────────────────────────────────────┘
                            ↓ SMB协议
┌─────────────────────────────────────────────────────────────┐
│                    NAS Server (nas-os)                       │
├─────────────────────────────────────────────────────────────┤
│  SMB Spotlight Handler                                       │
│       ↓                                                      │
│  Query Parser (SpotlightQueryParser)                         │
│       ↓                                                      │
│  Index Searcher                                              │
│       ↓                                                      │
│  Result Formatter (kMDItem格式)                              │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 SMB Spotlight协议扩展

TrueNAS使用的SMB Spotlight协议是Samba的`spotlight`模块扩展：

```ini
# smb.conf 关键配置
[global]
    spotlight:backend = tracker  # 或 elasticsearch

[share]
    spotlight = yes              # 启用Spotlight
    spotlight indexing paths = /path/to/share
```

**协议交互流程：**

1. **macOS发起搜索**: Finder → mdfind → mds_rpc
2. **SMB RPC调用**: 通过SMB协议发送`mds_query`请求
3. **服务端解析**: 解析Spotlight查询语法
4. **索引搜索**: 调用后端索引（Tracker/Elasticsearch/Bleve）
5. **结果格式化**: 返回kMDItem属性格式
6. **macOS渲染**: Finder显示搜索结果

### 2.3 kMDItem属性映射详解

**核心属性表：**

| Spotlight属性 | 内部字段 | 类型 | 说明 |
|--------------|---------|------|------|
| `kMDItemDisplayName` | name | String | 显示名称 |
| `kMDItemFSName` | name | String | 文件名 |
| `kMDItemPath` | path | String | 完整路径 |
| `kMDItemFSSize` | size | Number | 文件大小(bytes) |
| `kMDItemFSCreationDate` | created | Date | 创建时间 |
| `kMDItemFSContentChangeDate` | modified | Date | 修改时间 |
| `kMDItemContentType` | type | String | UTI类型标识 |
| `kMDItemKind` | kind | String | 本地化类型描述 |
| `kMDItemKeywords` | keywords | Array | 关键词 |
| `kMDItemTextContent` | content | String | 文本内容 |
| `kMDItemWhereFroms` | source | Array | 来源信息 |
| `kMDItemPixelWidth` | width | Number | 图片宽度 |
| `kMDItemPixelHeight` | height | Number | 图片高度 |
| `kMDItemDurationSeconds` | duration | Number | 视频/音频时长 |

**UTI类型映射（现有实现已覆盖）：**

```go
// internal/smb/spotlight_integration.go getContentType()
contentTypes := map[string]string{
    ".txt":  "public.plain-text",
    ".pdf":  "com.adobe.pdf",
    ".docx": "org.openxmlformats.wordprocessingml.document",
    ".jpg":  "public.jpeg",
    ".mp4":  "public.mpeg-4",
    // ...已完整实现
}
```

### 2.4 Spotlight查询语法详解

**支持语法格式：**

```bash
# 1. 简单关键词搜索
mdfind "搜索关键词"

# 2. 属性精确匹配
mdfind 'kMDItemDisplayName == "report.pdf"'
mdfind 'kMDItemContentType == "com.adobe.pdf"'

# 3. 通配符搜索
mdfind 'kMDItemDisplayName == "*.pdf"'
mdfind 'kMDItemDisplayName == "report*"'

# 4. 复合AND查询
mdfind '(kMDItemDisplayName == "*.txt") && (kMDItemFSSize > 1000000)'

# 5. OR查询
mdfind 'kMDItemContentType == "public.jpeg" || kMDItemContentType == "public.png"'

# 6. 范围查询
mdfind 'kMDItemFSContentChangeDate >= $time.today'
mdfind 'kMDItemFSSize > 1000000 && kMDItemFSSize < 5000000'

# 7. 时间查询（特殊语法）
mdfind 'kMDItemFSContentChangeDate >= $time.this_week'
mdfind 'kMDItemFSContentChangeDate >= $time.this_month'

# 8. 路径限定
mdfind -onlyin /share/documents "关键词"
```

**现有解析器状态（需增强）：**

```go
// 当前ParseQuery仅支持简单解析
func ParseQuery(query string) map[string]interface{} {
    // TODO: 需增强支持：
    // - 通配符解析 (*)
    // - 范围查询 (>, <, >=, <=)
    // - 时间变量 ($time.today等)
    // - 复合AND/OR嵌套
}
```

---

## 3. 索引服务架构

### 3.1 紳架构设计

``┌─────────────────────────────────────────────────────────────┐
│                    Index Service                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ File Index  │  │ Meta Index  │  │Content Index│         │
│  │  (内存)     │  │  (Bleve)    │  │  (Bleve)    │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                │
│         └────────────────┼────────────────┘                │
│                          ↓                                 │
│               ┌───────────────────┐                        │
│               │   Unified Index   │                        │
│               │   Manager         │                        │
│               └───────────────────┘                        │
│                          ↓                                 │
│  ┌─────────────────────────────────────────────────────┐  │
│  │               Index Backend (Bleve)                  │  │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐          │  │
│  │  │ Metadata  │ │ Content   │ │ ACL       │          │  │
│  │  │ Index     │ │ Index     │ │ Index     │          │  │
│  │  └───────────┘ └───────────┘ └───────────┘          │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 紧引类型定义

**文件元数据索引：**

```go
type FileInfo struct {
    Path         string            `json:"path"`
    Name         string            `json:"name"`
    Size         int64             `json:"size"`
    ModTime      time.Time         `json:"modTime"`
    CreatedTime  time.Time         `json:"createdTime"`   // 新增
    Type         string            `json:"type"`          // kMDItemContentType
    Kind         string            `json:"kind"`          // 本地化类型
    Extension    string            `json:"extension"`
    IsDirectory  bool              `json:"isDirectory"`
    Permissions  string            `json:"permissions"`   // 新增
    Owner        string            `json:"owner"`         // 新增
    Group        string            `json:"group"`         // 新增
    Attributes   map[string]string `json:"attributes"`    // Spotlight属性
    IndexedAt    time.Time         `json:"indexedAt"`
    Score        float64           `json:"score"`
}
```

**内容索引：**

```go
type ContentInfo struct {
    Path        string   `json:"path"`
    TextContent string   `json:"textContent"`   // 提取的文本
    Keywords    []string `json:"keywords"`      // 关键词
    WordCount   int      `json:"wordCount"`
    Excerpt     string   `json:"excerpt"`       // 摘要（200字符）
    Language    string   `json:"language"`      // zh/en
    ContentType string   `json:"contentType"`   // 文件类型
}
```

### 3.3 Bleve索引配置

**索引映射（完善版）：**

```go
func createSpotlightIndexMapping() mapping.IndexMapping {
    indexMapping := bleve.NewIndexMapping()
    
    // 文档映射
    docMapping := bleve.NewDocumentMapping()
    
    // 路径 - keyword analyzer（精确匹配）
    pathField := bleve.NewTextFieldMapping()
    pathField.Analyzer = keyword.Name
    docMapping.AddFieldMappingsAt("path", pathField)
    
    // 名称 - 标准分词 + CJK
    nameField := bleve.NewTextFieldMapping()
    nameField.Analyzer = "standard_cjk"  // 自定义混合分词
    docMapping.AddFieldMappingsAt("name", nameField)
    
    // 内容 - CJK分词（中文优化）
    contentField := bleve.NewTextFieldMapping()
    contentField.Analyzer = "cjk"
    docMapping.AddFieldMappingsAt("content", contentField)
    
    // 关键词 - keyword analyzer
    keywordsField := bleve.NewTextFieldMapping()
    keywordsField.Analyzer = keyword.Name
    docMapping.AddFieldMappingsAt("keywords", keywordsField)
    
    // 数值字段
    sizeField := bleve.NewNumericFieldMapping()
    docMapping.AddFieldMappingsAt("size", sizeField)
    
    // 日期字段
    modTimeField := bleve.NewDateTimeFieldMapping()
    docMapping.AddFieldMappingsAt("modTime", modTimeField)
    
    createdField := bleve.NewDateTimeFieldMapping()
    docMapping.AddFieldMappingsAt("createdTime", createdField)
    
    // 类型 - keyword analyzer
    typeField := bleve.NewTextFieldMapping()
    typeField.Analyzer = keyword.Name
    docMapping.AddFieldMappingsAt("type", typeField)
    
    // 扩展名 - keyword analyzer
    extField := bleve.NewTextFieldMapping()
    extField.Analyzer = keyword.Name
    docMapping.AddFieldMappingsAt("extension", extField)
    
    indexMapping.AddDocumentMapping("file", docMapping)
    
    return indexMapping
}
```

### 3.4 索引构建策略

**分层索引流程：**

```
Phase 1: 元数据索引（快速）
├─ 文件名/路径索引      → 内存map（即时查询）
├─ 基础属性索引        → Bleve（~500ms延迟）
└─ 状态缓存           → Redis可选

Phase 2: 内容索引（异步）
├─ 文本文件            → 直接提取
├─ PDF文档            → pdftotext提取
├─ Office文档         → libreoffice提取
└─ 图片/视频元数据    → EXIF/ffprobe提取
```

**增量更新策略：**

```go
type IndexUpdateStrategy struct {
    // 文件变更监听
    Watcher       *fsnotify.Watcher
    
    // 批量更新配置
    BatchSize     int           // 每批100文件
    BatchInterval time.Duration // 5秒间隔
    
    // 节流控制
    MaxUpdatesPerSec int        // 每秒最多200更新
    
    // 后台worker
    WorkerCount   int           // 4个worker
    
    // 增量检测
    DetectChanges func(path string) bool
}
```

---

## 4. 与现有SMB服务集成方案

### 4.1 集成架构

```
┌─────────────────────────────────────────────────────────────┐
│                    SMB Manager                               │
│  (internal/smb/manager.go)                                  │
├─────────────────────────────────────────────────────────────┤
│                     ↓                                       │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Spotlight Integration                  │   │
│  │  (internal/smb/spotlight_integration.go)            │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │                                                     │   │
│  │  ┌─────────────┐  ┌─────────────┐                  │   │
│  │  │ Indexer     │  │ MDQuery     │                  │   │
│  │  │ Manager     │  │ Handler     │                  │   │
│  │  └──────┬──────┘  └──────┬──────┘                  │   │
│  │         │                │                         │   │
│  │         └────────────────┼─────────────────────┐   │   │
│  │                          ↓                     │   │   │
│  │               ┌───────────────────┐            │   │   │
│  │               │ Bleve Backend     │            │   │   │
│  │               │ (webshare集成)    │            │   │   │
│  │               └───────────────────┘            │   │   │
│  │                          ↓                     │   │   │
│  │  ┌─────────────────────────────────────────────┤───┘  │
│  │  │              SMB Protocol Handler           │      │
│  │  │  (spotlight RPC extension)                 │      │
│  │  └─────────────────────────────────────────────┘      │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 SMB配置集成

**smb.conf Spotlight配置生成：**

```go
// 增强版配置生成
func GenerateSMBSpotlightConfigV2(config SpotlightConfig) string {
    var cfg strings.Builder
    
    if config.Enabled {
        // 全局配置
        cfg.WriteString("[global]\n")
        cfg.WriteString("    spotlight:backend = custom\n")
        cfg.WriteString("    spotlight:index_dir = /var/lib/nas-os/spotlight\n")
        
        // 性能配置
        cfg.WriteString("    spotlight:max_results = 1000\n")
        cfg.WriteString("    spotlight:timeout = 30\n")
        
        // 每个共享配置
        for _, share := range config.ShareConfigs {
            cfg.WriteString(fmt.Sprintf("\n[%s]\n", share.Name))
            cfg.WriteString("    spotlight = yes\n")
            cfg.WriteString("    spotlight indexing paths = ")
            cfg.WriteString(strings.Join(share.Paths, ", "))
            cfg.WriteString("\n")
            
            // 排除路径
            if len(share.ExcludedPaths) > 0 {
                cfg.WriteString("    spotlight exclude paths = ")
                cfg.WriteString(strings.Join(share.ExcludedPaths, ", "))
                cfg.WriteString("\n")
            }
        }
    }
    
    return cfg.String()
}
```

### 4.3 SMB RPC Handler接口

**Spotlight RPC接口设计：**

```go
// SMB Spotlight RPC Handler
type SMBSpotlightRPCHandler struct {
    spotlight   *SpotlightIntegration
    logger      *zap.Logger
}

// HandleMDSQuery 处理macOS mds_query RPC
func (h *SMBSpotlightRPCHandler) HandleMDSQuery(ctx context.Context, req *MDSQueryRequest) (*MDSQueryResponse, error) {
    // 1. 解析Spotlight查询
    query := h.spotlight.mdquery.ParseQuery(req.QueryString)
    
    // 2. 转换为内部搜索请求
    searchReq := SpotlightQuery{
        Query:       req.QueryString,
        Attributes:  req.RequestedAttributes,
        Scope:       req.SearchScope,
        Limit:       req.MaxResults,
        SortBy:      req.SortAttribute,
        SortDesc:    req.SortDescending,
    }
    
    // 3. 执行搜索
    response, err := h.spotlight.Search(ctx, searchReq)
    if err != nil {
        return nil, fmt.Errorf("搜索失败: %w", err)
    }
    
    // 4. 格式化为SMB RPC响应格式
    mdsResp := &MDSQueryResponse{
        QueryID:     req.QueryID,
        Results:     h.formatResults(response.Results),
        TotalCount:  response.Total,
        QueryTime:   response.Took,
    }
    
    return mdsResp, nil
}

// MDSQueryRequest macOS Spotlight RPC请求格式
type MDSQueryRequest struct {
    QueryID           uint64   // 查询ID
    QueryString       string   // Spotlight查询语法
    RequestedAttributes []string // 请求的kMDItem属性
    SearchScope       []string  // 搜索范围路径
    MaxResults        int       // 最大结果数
    SortAttribute     string    // 排序属性
    SortDescending    bool      // 降序排序
    QueryFlags        uint32    // 查询标志位
}

// MDSQueryResponse macOS Spotlight RPC响应格式
type MDSQueryResponse struct {
    QueryID      uint64
    Results      []MDSResultItem
    TotalCount   int
    QueryTime    int64   // ms
    Status       uint32  // 状态码
    ErrorMessage string  // 错误信息
}

// MDSResultItem 单个结果项
type MDSResultItem struct {
    Path       string
    Attributes map[string]interface{}  // kMDItem属性
}
```

### 4.4 API端点设计

**REST API（与SMB RPC并行）：**

```yaml
# Spotlight搜索API
POST /api/v1/smb/spotlight/search
  Request:
    query: "kMDItemDisplayName == '*.pdf'"
    scope: ["/share/documents"]
    attributes: ["kMDItemDisplayName", "kMDItemFSSize", "kMDItemContentType"]
    limit: 50
    sortBy: "kMDItemFSContentChangeDate"
    sortDesc: true
  
  Response:
    query: "kMDItemDisplayName == '*.pdf'"
    results:
      - path: "/share/documents/report.pdf"
        name: "report.pdf"
        size: 1024000
        modTime: "2026-04-24T00:00:00Z"
        type: "com.adobe.pdf"
        kind: "PDF文档"
        attributes:
          kMDItemDisplayName: "report.pdf"
          kMDItemFSSize: 1024000
          kMDItemContentType: "com.adobe.pdf"
        score: 95.0
    total: 100
    took: 25

# 索引管理API
GET /api/v1/smb/spotlight/status
  Response:
    enabled: true
    sharePaths: ["/share/documents"]
    stats:
      totalFiles: 10000
      indexedFiles: 8500
      indexedSize: 500000000
      status: "running"
      progress: 85.0
      lastUpdate: "2026-04-24T10:00:00Z"

POST /api/v1/smb/spotlight/index/rebuild
  Request:
    path: "/share/documents"
    force: true  # 强制全量重建

# 配置API
PUT /api/v1/smb/spotlight/config
  Request:
    enabled: true
    indexerWorkers: 4
    cacheSize: 100
    enableContentIdx: true
    enableChineseSeg: true
```

---

## 5. Phase 1实现计划

### 5.1 Phase 1范围定义

**目标：** 完成基础功能，macOS Finder可搜索文件名

**包含功能：**
- ✅ 文件名/路径索引（已有）
- ✅ 基础元数据索引（已有）
- 🔄 Spotlight查询语法完善
- 🔄 SMB配置集成
- 🔄 API端点完善
- 🔄 macOS兼容性测试

**不包含：**
- 内容全文搜索（Phase 2）
- PDF/Office内容提取（Phase 2）
- 图片/视频元数据（Phase 2）

### 5.2 实现步骤

**Step 1: 查询解析器增强（预计1天）**

```go
// 文件: internal/smb/spotlight_query_parser.go

// 增强版查询解析器
type SpotlightQueryParser struct {
    logger *zap.Logger
}

// ParseQueryV2 增强版解析
func (p *SpotlightQueryParser) ParseQueryV2(query string) *ParsedQuery {
    result := &ParsedQuery{
        Conditions: []QueryCondition{},
        Operators:  []LogicalOperator{},
    }
    
    // 支持语法：
    // 1. 简单关键词
    // 2. 属性 == 值（精确匹配）
    // 3. 属性 == *通配符
    // 4. 属性 > < >= <= 值（范围）
    // 5. AND / OR 逻辑组合
    // 6. $time变量
    
    // 解析逻辑...
    return result
}

type ParsedQuery struct {
    Conditions   []QueryCondition
    Operators    []LogicalOperator  // AND/OR
    TimeVariable string             // $time.today等
    Scope        []string           // -onlyin路径
}

type QueryCondition struct {
    Attribute string
    Operator  string  // ==, >, <, >=, <=, !=
    Value     interface{}
    Wildcard  bool    // 是否包含*
}

type LogicalOperator string
const (
    OpAND LogicalOperator = "AND"
    OpOR  LogicalOperator = "OR"
)
```

**Step 2: SMB配置集成完善（预计0.5天）**

```go
// 文件: internal/smb/spotlight_config.go

// 与manager.go集成
func (m *Manager) EnableSpotlight(shareName string, config SpotlightShareConfig) error {
    // 1. 更新共享配置
    share := m.shares[shareName]
    share.SpotlightEnabled = true
    share.SpotlightPaths = config.Paths
    
    // 2. 生成smb.conf配置
    spotlightConf := GenerateSMBSpotlightConfigV2(config)
    
    // 3. 写入配置
    err := WriteSmbConf("/etc/samba/smb.conf", spotlightConf)
    
    // 4. 重启SMB服务
    return m.ApplyConfig()
}
```

**Step 3: API端点完善（预计1天）**

```go
// 文件: internal/smb/spotlight_handlers.go

// 注册路由
func RegisterSpotlightRoutes(r *gin.RouterGroup, spotlight *SpotlightIntegration) {
    spotlightGroup := r.Group("/spotlight")
    
    spotlightGroup.POST("/search", handleSpotlightSearch)
    spotlightGroup.GET("/status", handleSpotlightStatus)
    spotlightGroup.POST("/index/rebuild", handleIndexRebuild)
    spotlightGroup.PUT("/config", handleSpotlightConfig)
    spotlightGroup.POST("/enable/:share", handleSpotlightEnable)
    spotlightGroup.POST("/disable/:share", handleSpotlightDisable)
}

// 搜索处理器
func handleSpotlightSearch(c *gin.Context) {
    var req SpotlightSearchRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    ctx := c.Request.Context()
    response, err := spotlight.Search(ctx, req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, response)
}
```

**Step 4: Bleve索引集成（预计1天）**

```go
// 文件: internal/smb/spotlight_bleve.go

// Bleve索引后端集成
type SpotlightBleveBackend struct {
    index       bleve.Index
    indexPath   string
    logger      *zap.Logger
}

func NewSpotlightBleveBackend(indexPath string, logger *zap.Logger) (*SpotlightBleveBackend, error) {
    // 创建或打开索引
    var index bleve.Index
    if _, err := os.Stat(indexPath); os.IsNotExist(err) {
        // 创建新索引
        indexMapping := createSpotlightIndexMapping()
        index, err = bleve.New(indexPath, indexMapping)
    } else {
        // 打开现有索引
        index, err = bleve.Open(indexPath)
    }
    
    if err != nil {
        return nil, err
    }
    
    return &SpotlightBleveBackend{
        index:     index,
        indexPath: indexPath,
        logger:    logger,
    }, nil
}

// IndexFile 索引文件
func (b *SpotlightBleveBackend) IndexFile(file *FileInfo) error {
    return b.index.Index(file.Path, file)
}

// SearchFiles 搜索文件
func (b *SpotlightBleveBackend) SearchFiles(query *ParsedQuery, limit int) ([]FileInfo, error) {
    // 构建Bleve查询
    bleveQuery := b.buildBleveQuery(query)
    
    searchRequest := bleve.NewSearchRequest(bleveQuery)
    searchRequest.Size = limit
    
    // 执行搜索
    result, err := b.index.Search(searchRequest)
    if err != nil {
        return nil, err
    }
    
    // 转换结果
    files := []FileInfo{}
    for _, hit := range result.Hits {
        doc, err := b.index.Document(hit.ID)
        if err != nil {
            continue
        }
        file := FileInfo{}
        // 解析文档...
        files = append(files, file)
    }
    
    return files, nil
}
```

**Step 5: macOS兼容性测试（预计1天）**

```bash
# 测试脚本: tests/spotlight_macos_test.sh

# 测试环境: macOS客户端 + nas-os SMB服务器

# 测试用例:
# 1. Finder Spotlight搜索文件名
# 2. 文件类型过滤 (*.pdf)
# 3. 时间范围过滤
# 4. 路径限定搜索
# 5. 中英文关键词
# 6. 大规模文件场景 (>10000文件)

# 验证点:
# - 搜索结果正确性
# - 响应延迟 (<500ms)
# - kMDItem属性格式正确
# - UTI类型识别正确
```

### 5.3 时间计划

| 步骤 | 预计时间 | 负责模块 |
|------|---------|---------|
| Step 1 查询解析器增强 | 1天 | `spotlight_query_parser.go` |
| Step 2 SMB配置集成 | 0.5天 | `spotlight_config.go` |
| Step 3 API端点完善 | 1天 | `spotlight_handlers.go` |
| Step 4 Bleve索引集成 | 1天 | `spotlight_bleve.go` |
| Step 5 macOS兼容测试 | 1天 | `tests/spotlight_*.go` |
| **总计** | **4.5天** | |

### 5.4 交付清单

Phase 1完成后应交付：

1. **代码文件**
   - `internal/smb/spotlight_query_parser.go`（新建）
   - `internal/smb/spotlight_bleve.go`（新建）
   - `internal/smb/spotlight_handlers.go`（完善）
   - `internal/smb/spotlight_config.go`（完善）

2. **测试覆盖**
   - 单元测试: `spotlight_*_test.go`
   - macOS兼容测试: 验证Finder搜索

3. **文档**
   - 本设计文档
   - API使用指南
   - macOS用户手册

---

## 6. 性能优化策略

### 6.1 索引性能优化

| 优化项 | 目标值 | 方法 |
|--------|--------|------|
| 索引速度 | 1000文件/秒 | 批量索引 + 并行worker |
| 内存占用 | <500MB (10万文件) | 紧凑数据结构 |
| 索引大小 | <10%原始数据 | Bleve内置压缩 |
| 增量延迟 | <5分钟 | fsnotify实时监听 |

### 6.2 搜索性能优化

| 优化项 | 目标值 | 方法 |
|--------|--------|------|
| 简单搜索延迟 | <100ms | 内存索引优先 |
| 复杂查询延迟 | <500ms | Bleve优化查询 |
| 并发支持 | 100并发 | 搜索信号量控制 |
| 结果缓存 | 80%命中率 | LRU缓存 |

### 6.3 中文搜索优化

```go
// 混合分词器配置
func createCJKAnalyzer() analysis.Analyzer {
    // CJK分词 + 标准分词混合
    // 支持: 中文短语、英文单词、数字
    
    tokenizer := cjk.NewCJKTokenizer()
    
    // 停用词过滤
    zhStopWords := []string{
        "的", "是", "在", "有", "和", "了", "不", "这", "那",
        "为", "与", "以", "及", "其", "或", "但", "如", "而",
    }
    
    // 构建analyzer...
    return analyzer
}
```

---

## 7. 安全考虑

### 7.1 权限检查

- 搜索结果必须通过ACL验证
- 用户只能看到有权限的文件
- 管理员搜索记录审计日志

### 7.2 审计日志

```go
// 搜索审计记录
type SpotlightAuditLog struct {
    Timestamp   time.Time
    User        string
    Query       string
    ResultCount int
    Scope       []string
    SourceIP    string
    Duration    int64  // ms
}
```

---

## 8. 后续规划

### Phase 2 (预计v2.455.0)

- 实时索引（fsnotify监听）
- PDF/Office内容提取
- 图片EXIF元数据
- 音视频元数据
- Spotlight建议/补全

### Phase 3 (预计v2.460.0)

- Elasticsearch后端（可选）
- 分布式索引
- Spotlight高级API
- Web UI搜索界面

---

## 9. 参考资源

### TrueNAS实现

- Samba spotlight模块
- Tracker索引引擎
- Elasticsearch集成

### macOS文档

- [MDQuery API Reference](https://developer.apple.com/documentation/coreservices/mdquery)
- [Spotlight Query Syntax](https://developer.apple.com/library/archive/documentation/Carbon/Conceptual/SpotlightQueries/)
- [Uniform Type Identifiers](https://developer.apple.com/documentation/uniformtypeidentifiers)

### nas-os现有实现

- `internal/smb/spotlight_integration.go`
- `internal/webshare/bleve_index.go`
- `docs/smb-spotlight-phase1.md`

---

**文档版本**: v1.0  
**创建日期**: 2026-04-24  
**工部第236轮**