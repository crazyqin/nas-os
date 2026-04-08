# SMB Spotlight Phase1 设计文档

> 兵部任务 | v2.404.0 | 2026-04-09

## 1. 概述

SMB Spotlight集成旨在为NAS-OS提供与macOS Spotlight搜索的完全兼容性，使Mac用户能够通过Finder的Spotlight功能直接搜索SMB共享中的文件。

### 1.1 目标

- 实现macOS Spotlight查询语法解析
- 支持kMDItem属性映射
- 提供文件元数据索引方案
- 集成Bleve全文索引引擎
- 支持中文分词

### 1.2 范围

Phase1聚焦核心功能：
- 文件名/路径索引
- 基础元数据索引（大小、时间、类型）
- Spotlight查询语法解析
- macOS兼容的API响应格式

---

## 2. macOS Spotlight API 研究

### 2.1 Spotlight查询语法

macOS Spotlight使用`mdfind`命令行工具或`MDQuery` API进行搜索：

```bash
# 基础查询
mdfind "搜索关键词"

# 属性查询
mdfind 'kMDItemDisplayName == "*.pdf"'
mdfind 'kMDItemContentType == "com.adobe.pdf"'

# 复合查询
mdfind '(kMDItemDisplayName == "*.txt") && (kMDItemFSContentChangeDate >= $time.today)'

# 范围查询
mdfind -onlyin /path/to/search "关键词"
```

### 2.2 核心kMDItem属性

| 属性名 | 说明 | 类型 |
|--------|------|------|
| `kMDItemDisplayName` | 显示名称 | String |
| `kMDItemFSName` | 文件名 | String |
| `kMDItemPath` | 文件路径 | String |
| `kMDItemFSSize` | 文件大小 | Number |
| `kMDItemFSCreationDate` | 创建时间 | Date |
| `kMDItemFSContentChangeDate` | 修改时间 | Date |
| `kMDItemContentType` | UTI类型标识 | String |
| `kMDItemKind` | 类型描述 | String |
| `kMDItemKeywords` | 关键词 | Array |
| `kMDItemTextContent` | 文本内容 | String |

### 2.3 UTI (Uniform Type Identifier) 映射

macOS使用UTI标识文件类型：

```
public.plain-text     -> .txt
public.markdown       -> .md
com.adobe.pdf         -> .pdf
com.microsoft.word.doc -> .doc
org.openxmlformats.wordprocessingml.document -> .docx
public.jpeg           -> .jpg, .jpeg
public.png            -> .png
public.mpeg-4         -> .mp4
com.apple.quicktime-movie -> .mov
```

### 2.4 SMB Spotlight协议

SMB协议通过`spotlight`扩展支持Spotlight搜索：

```ini
# smb.conf配置
[share]
    spotlight = yes
    spotlight indexing paths = /path/to/share
```

SMB客户端发送Spotlight查询时，服务端需要：
1. 解析查询语法
2. 执行索引搜索
3. 返回符合macOS格式的结果

---

## 3. 文件元数据索引方案

### 3.1 索引架构

```
┌─────────────────────────────────────────────────────┐
│                  SMB Share Path                      │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌───────────────┐  ┌───────────────┐              │
│  │   File Index  │  │ Content Index │              │
│  │  (元数据)      │  │  (全文内容)    │              │
│  └───┬───────────┘  └───┬───────────┘              │
│      │                  │                          │
│      ▼                  ▼                          │
│  ┌───────────────────────────────┐                │
│  │      Bleve Full-Text Index    │                │
│  │   (倒排索引 + CJK分词)         │                │
│  └───────────────────────────────┘                │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 3.2 元数据索引结构

```go
type FileInfo struct {
    Path         string            // 文件路径
    Name         string            // 文件名
    Size         int64             // 文件大小
    ModTime      time.Time         // 修改时间
    Type         string            // kMDItemContentType (UTI)
    Kind         string            // kMDItemKind (本地化描述)
    Extension    string            // 扩展名
    IsDirectory  bool              // 是否目录
    Attributes   map[string]string // 扩展属性
    IndexedAt    time.Time         // 索引时间
    Score        float64           // 搜索评分
}
```

### 3.3 索引构建策略

**增量索引流程：**

1. **初始扫描** - 全量遍历共享目录
2. **监听变更** - 使用fsnotify监听文件事件
3. **批量处理** - 每批次索引100个文件
4. **定时刷新** - 5分钟检查更新

**索引优化：**

- 跳过隐藏文件（`.*`开头）
- 跳过排除路径（`.bleve_index`等）
- 限制文件大小（<10MB）
- 异步后台构建

---

## 4. Bleve全文索引集成

### 4.1 Bleve配置

```go
func createIndexMapping() mapping.IndexMapping {
    indexMapping := bleve.NewIndexMapping()
    
    // 文档映射
    docMapping := bleve.NewDocumentMapping()
    
    // 路径字段 - keyword analyzer（精确匹配）
    pathField := bleve.NewTextFieldMapping()
    pathField.Analyzer = keyword.Name
    docMapping.AddFieldMappingsAt("path", pathField)
    
    // 内容字段 - CJK分词器（中文支持）
    contentField := bleve.NewTextFieldMapping()
    contentField.Analyzer = "cjk"  // 支持中日韩文字
    docMapping.AddFieldMappingsAt("content", contentField)
    
    // 名称字段 - 标准分词
    nameField := bleve.NewTextFieldMapping()
    docMapping.AddFieldMappingsAt("name", nameField)
    
    // 数值字段
    sizeField := bleve.NewNumericFieldMapping()
    docMapping.AddFieldMappingsAt("size", sizeField)
    
    // 日期字段
    modTimeField := bleve.NewDateTimeFieldMapping()
    docMapping.AddFieldMappingsAt("modTime", modTimeField)
    
    return indexMapping
}
```

### 4.2 搜索API

```go
type BleveSearchRequest struct {
    Query       string   // 搜索关键词
    Paths       []string // 路径限制
    Extensions  []string // 扩展名过滤
    MinSize     int64    // 最小大小
    MaxSize     int64    // 最大大小
    FromDate    *time.Time
    ToDate      *time.Time
    MaxResults  int      // 分页限制
    Offset      int      // 分页偏移
    Highlight   bool     // 高亮匹配
    Fuzzy       bool     // 模糊搜索
    SortBy      string   // 排序字段
    SortDesc    bool     // 降序
    Fields      []string // 搜索字段
}
```

### 4.3 中文分词

Bleve内置CJK分词器支持中文：

```go
// CJK分词器配置
contentFieldMapping.Analyzer = "cjk"

// 效果示例
输入: "中国人工智能发展"
分词: ["中国", "人工", "人工智能", "智能", "发展"]
```

**停用词过滤：**

```go
func getStopWords(language string) map[string]bool {
    zhStopWords := []string{
        "的", "是", "在", "有", "和", "了", "不", "这", "那", "之",
        "为", "与", "以", "及", "其", "或", "但", "如", "而", "也",
    }
    // ...
}
```

---

## 5. Spotlight查询解析器

### 5.1 查询语法解析

```go
func ParseQuery(query string) map[string]interface{} {
    result := make(map[string]interface{})
    
    // 支持的语法格式：
    // 1. 简单关键词: "关键词"
    // 2. 属性匹配: 'kMDItemDisplayName == "xxx"'
    // 3. 复合查询: '(attr1 == val1) && (attr2 == val2)'
    // 4. OR查询: 'attr1 == val1 OR attr2 == val2'
    
    parts := strings.Split(query, "OR")
    for _, part := range parts {
        part = strings.TrimSpace(part)
        
        if strings.Contains(part, "==") {
            kv := strings.SplitN(part, "==", 2)
            attr := strings.TrimSpace(kv[0])
            val := strings.Trim(strings.TrimSpace(kv[1]), "\"'")
            
            internalAttr := mapSpotlightAttr(attr)
            result[internalAttr] = val
        } else {
            result["name"] = strings.Trim(part, "\"'")
        }
    }
    
    return result
}
```

### 5.2 属性映射表

```go
var spotlightAttrMap = map[string]string{
    // Spotlight属性 -> 内部字段
    "kMDItemDisplayName":          "name",
    "kMDItemFSName":               "name",
    "kMDItemPath":                 "path",
    "kMDItemFSSize":               "size",
    "kMDItemFSCreationDate":       "created",
    "kMDItemFSContentChangeDate":  "modified",
    "kMDItemContentType":          "type",
    "kMDItemKind":                 "kind",
    "kMDItemKeywords":             "keywords",
    "kMDItemTitle":                "title",
    "kMDItemAuthors":              "author",
    "kMDItemPixelWidth":           "width",
    "kMDItemPixelHeight":          "height",
    "kMDItemDurationSeconds":      "duration",
}
```

---

## 6. API设计

### 6.1 Spotlight搜索API

```yaml
# /api/smb/spotlight/search
POST /api/smb/spotlight/search
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
      modTime: "2026-04-09T00:00:00Z"
      type: "com.adobe.pdf"
      kind: "PDF文档"
      attributes:
        kMDItemDisplayName: "report.pdf"
        kMDItemFSSize: "1024000"
        kMDItemContentType: "com.adobe.pdf"
      score: 95.0
  total: 100
  took: 25  # 查询耗时(ms)
```

### 6.2 索引管理API

```yaml
# 启用Spotlight
POST /api/smb/spotlight/enable
Request:
  sharePath: "/share/documents"

# 禁用Spotlight
POST /api/smb/spotlight/disable
Request:
  sharePath: "/share/documents"

# 获取索引状态
GET /api/smb/spotlight/status
Response:
  enabled: true
  sharePaths: ["/share/documents"]
  stats:
    totalFiles: 10000
    indexedFiles: 8500
    indexedSize: 500000000
    status: "running"
    progress: 85.0

# 重建索引
POST /api/smb/spotlight/rebuild
Request:
  path: "/share/documents"
```

---

## 7. 与现有代码集成

### 7.1 现有代码结构

```
internal/smb/
├── spotlight_integration.go  # Spotlight集成主模块
├── multichannel.go           # SMB多通道
├── manager.go                # SMB管理器
├── config.go                 # SMB配置
├── handlers.go               # API处理器

internal/webshare/
├── bleve_index.go            # Bleve全文索引
├── content_search.go         # 内容搜索
├── searchindex.go            # 搜索索引
├── webshare.go               # WebShare主模块

internal/web/
├── search.go                 # 全局搜索服务
```

### 7.2 集成方案

**spotlight_integration.go增强：**

```go
// 集成Bleve索引
type SpotlightIntegration struct {
    config      SpotlightConfig
    logger      *zap.Logger
    indexer     *Indexer           // 原有索引器
    bleveIndex  *BleveContentIndex // 新增Bleve索引
    mdquery     *MDQueryHandler
    running     bool
    mu          sync.RWMutex
}

// 初始化时创建Bleve索引
func NewSpotlightIntegration(config SpotlightConfig, logger *zap.Logger) *SpotlightIntegration {
    // ...
    
    // 创建Bleve索引（如果启用内容索引）
    if config.EnableContentIdx {
        bleveConfig := WebShareConfig{BaseDir: config.SharePaths[0]}
        bleveIndex, err := NewBleveContentIndex(bleveConfig, logger)
        if err != nil {
            logger.Warn("Bleve索引初始化失败", zap.Error(err))
        }
        si.bleveIndex = bleveIndex
    }
    
    return si
}
```

### 7.3 API路由注册

```go
// cmd/nasd/main.go
func registerSpotlightRoutes(r *gin.Engine, spotlight *SpotlightIntegration) {
    spotlightGroup := r.Group("/api/smb/spotlight")
    
    spotlightGroup.POST("/search", handleSpotlightSearch)
    spotlightGroup.POST("/enable", handleSpotlightEnable)
    spotlightGroup.POST("/disable", handleSpotlightDisable)
    spotlightGroup.GET("/status", handleSpotlightStatus)
    spotlightGroup.POST("/rebuild", handleSpotlightRebuild)
}
```

---

## 8. 性能考虑

### 8.1 索引性能

| 指标 | 目标值 |
|------|--------|
| 索引速度 | 1000文件/秒 |
| 索引延迟 | <5分钟（增量更新） |
| 内存占用 | <500MB（10万文件） |
| 索引大小 | <10%原始数据 |

### 8.2 搜索性能

| 指标 | 目标值 |
|------|--------|
| 搜索延迟 | <100ms |
| 启发延迟 | <500ms（复杂查询） |
| 并发支持 | 100并发查询 |
| 结果上限 | 1000条/查询 |

### 8.3 优化策略

1. **批量索引** - 每批100文件减少IO
2. **异步构建** - 后台线程不阻塞主服务
3. **增量更新** - 只索引变更文件
4. **缓存结果** - 常用查询缓存
5. **索引压缩** - Bleve内置压缩

---

## 9. 测试计划

### 9.1 功能测试

- Spotlight查询语法解析
- kMDItem属性映射
- 中英文搜索
- 文件类型过滤
- 时间范围过滤

### 9.2 性能测试

- 10万文件索引时间
- 搜索延迟基准
- 并发查询压力测试
- 内存占用监控

### 9.3 兼容性测试

- macOS Finder Spotlight
- SMB客户端搜索
- 不同macOS版本

---

## 10. Phase2规划

Phase2将实现：
- 实时索引（fsnotify监听）
- PDF/Office文档内容提取
- 图片EXIF元数据索引
- 音视频元数据索引
- Spotlight建议/补全

---

## 11. 总结

SMB Spotlight Phase1设计已完成：
- macOS Spotlight API研究
- 文件元数据索引方案
- Bleve全文索引集成
- Spotlight查询解析器
- API设计

核心代码已存在于：
- `internal/smb/spotlight_integration.go`
- `internal/webshare/bleve_index.go`

下一步：完善测试用例，启动API服务。

---

**兵部 | 2026-04-09**