# TrueSearch 全文索引设计文档

## 概述

TrueSearch是对标TrueNAS 26 WebShare+TrueSearch的全文搜索功能，为nas-os提供高性能文件名和内容索引。

## 技术选型分析

### 方案对比

| 方案 | 优势 |劣势 | 适用场景 |
|------|------|------|----------|
| **Bleve** | 纯Go实现、无外部依赖、已集成 | 性能有限、不支持分布式 | 已在internal/search使用 ✅ |
| **Meilisearch** | 高性能、易用、支持多语言 | 需外部服务、内存占用大 | 大规模搜索场景 |
| **Elasticsearch** | 分布式、功能强大 | 资源消耗大、运维复杂 | 企业级搜索 |

### 当前实现

nas-os已在`internal/search/`实现Bleve搜索引擎：
- `engine.go` - Bleve搜索引擎核心
- `indexer.go` - 文件索引器（增量索引）
- `global.go` - 全局搜索服务
- `query.go` - 查询处理
- `chinese/segmenter.go` - 中文分词支持

### 推荐方案

**阶段1**: 增强现有Bleve实现（P0）
- 优化中文分词
- 增强内容索引能力
- 改进查询性能

**阶段2**: 可选Meilisearch集成（P2）
- 大规模部署场景
- 需要更复杂搜索功能时

## 架构设计

### 核心模块

```
internal/search/
├── engine.go          # Bleve搜索引擎
├── indexer.go         # 文件索引器
├── query.go           # 查询处理
├── chinese/           # 中文分词
│   ├── segmenter.go   # 分词实现
│   └── analyzer.go    # 分析器
├── global/            # 全局搜索
│   ├── service.go     # 搜索服务
│   └── handlers.go    # HTTP handlers
└── spotlight.go       # macOS Spotlight兼容
```

### 索引流程

1. **文件扫描**: FileIndexer扫描指定目录
2. **增量索引**: 检查文件modtime/hash变化
3. **内容提取**: 对文本文件提取内容（限制10MB）
4. **索引写入**: Bleve批量索引
5. **查询服务**: 提供RESTful搜索API

### 索引配置

```go
IndexConfig{
    IndexPath:    "/var/lib/nas-os/search/index.bleve",
    MaxFileSize:  10 * 1024 * 1024, // 10MB
    Workers:      4,
    IndexContent: true,
    BatchSize:    100,
    TextExts:     []string{".txt", ".md", ".go", ".json", ".yaml", ".html", ".css", ".js"},
    ExcludeDirs:  []string{".git", "node_modules", "vendor", ".cache"},
}
```

## API设计

### 搜索请求

```json
POST /api/v1/search
{
  "query": "搜索关键词",
  "paths": ["/data/docs"],
  "types": ["txt", "md"],
  "minSize": 0,
  "maxSize": 10485760,
  "limit": 50,
  "offset": 0,
  "sortBy": "score",
  "sortDesc": true
}
```

### 搜索响应

```json
{
  "success": true,
  "data": {
    "results": [
      {
        "path": "/data/docs/readme.md",
        "name": "readme.md",
        "ext": ".md",
        "size": 2048,
        "modTime": "2026-04-09T12:00:00Z",
        "score": 0.85,
        "highlights": [
          {"field": "content", "fragments": ["...关键词..."]}
        ]
      }
    ],
    "total": 100,
    "took_ms": 25
  }
}
```

## 性能目标

| 指标 | 目标 | 说明 |
|------|------|------|
| 索引速度 | 1000文件/秒 | 4线程并行 |
| 查询延迟 | <100ms | 10万文件索引 |
| 索引大小 | 5-10%原文件 | 文本文件压缩 |
| 内存占用 | <200MB | 索引服务 |

## TrueNAS对标分析

| 特性 | TrueNAS TrueSearch | nas-os状态 |
|------|-------------------|------------|
| 文件名搜索 | ✅ | ✅ 已有 |
| 内容搜索 | ✅ | ✅ 已有 |
| 中文支持 | ⚠️ 有限 | ✅ 分词优化 |
| Spotlight兼容 | ✅ macOS | ✅ spotlight.go |
| WebShare集成 | ✅ | ✅ webshare模块 |

## 开发计划

### Phase 1 (v2.435.0)
- ✅ Bleve引擎已实现
- ✅ 中文分词已集成
- ✅ 文件索引器已实现
- 🚧 性能优化

### Phase 2 (v2.450.0)
- 📋 内容索引增强
- 📋 搜索建议
- 📋 高级过滤

### Phase 3 (v2.500.0)
- 📋 分布式索引（可选Meilisearch）
- 📋 AI语义搜索增强

## 参考资源

- Bleve文档: https://blevesearch.github.io/bleve/
- TrueNAS TrueSearch: https://www.truenas.com/docs/
- Meilisearch: https://www.meilisearch.com/

---

**文档版本**: v1.0
**创建日期**: 2026-04-09
**部门**: 兵部软件工程