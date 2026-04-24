# TrueSearch Phase2 设计文档

**版本**: v2.463.0
**部门**: 兵部
**日期**: 2026-04-24

---

## 1. 概述

TrueSearch是nas-os对标TrueNAS 26全文搜索功能的核心模块。Phase1已完成基础索引架构，Phase2目标：
- 全文内容索引优化
- 多语言支持
- 搜索性能提升

---

## 2. Phase2 目标

| 目标 | 描述 | 优先级 |
|------|------|--------|
| 内容索引优化 | 支持PDF/Word/Markdown全文提取 | P0 |
| 多语言支持 | 中文/英文/日文分词 | P1 |
| 性能优化 | 索引速度提升50% | P1 |
| 实时增量索引 | 5分钟自动刷新 | P0 |

---

## 3. 技术架构

### 3.1 索引引擎

```
┌─────────────────────────────────────────────────┐
│                  TrueSearch Engine              │
├─────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │ 文件监控 │→│ 内容提取 │→│ 分词索引 │      │
│  └──────────┘  └──────────┘  └──────────┘      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │ 增量更新 │→│ 存储索引 │→│ 搜索服务 │      │
│  └──────────┘  └──────────┘  └──────────┘      │
└─────────────────────────────────────────────────┘
```

### 3.2 核心组件

| 组件 | 功能 | 实现 |
|------|------|------|
| FileWatcher | 文件变更监控 | fsnotify |
| ContentExtractor | 内容提取 | pdfcpu + unidoc |
| Tokenizer | 分词处理 | gojieba |
| IndexStore | 病毒存储 | meilisearch |
| SearchAPI | 搜索接口 | REST API |

---

## 4. API设计

### 4.1 搜索接口

```go
// POST /api/v1/search
type SearchRequest struct {
    Query       string   `json:"query"`
    Path        string   `json:"path,omitempty"`     // 搜索范围
    Types       []string `json:"types,omitempty"`    // 文件类型过滤
    Languages   []string `json:"languages,omitempty"` // 语言过滤
    MaxResults  int      `json:"max_results"`        // 最大结果数
    Highlight   bool     `json:"highlight"`          // 高亮匹配
}

type SearchResponse struct {
    Results    []SearchResult `json:"results"`
    Total      int            `json:"total"`
    TookMs     int            `json:"took_ms"`
    Highlights map[string]string `json:"highlights,omitempty"`
}

type SearchResult struct {
    Path       string   `json:"path"`
    Name       string   `json:"name"`
    Size       int64    `json:"size"`
    ModTime    string   `json:"mod_time"`
    Score      float64  `json:"score"`
    Snippet    string   `json:"snippet"`
}
```

### 4.2 管理接口

```go
// GET /api/v1/search/status
type IndexStatus struct {
    TotalFiles    int   `json:"total_files"`
    IndexedFiles  int   `json:"indexed_files"`
    PendingFiles  int   `json:"pending_files"`
    LastUpdate    string `json:"last_update"`
    IndexSize     int64  `json:"index_size"`
}

// POST /api/v1/search/reindex
type ReindexRequest struct {
    Path string `json:"path"` // 重新索引路径
    Force bool `json:"force"` // 强制重建
}
```

---

## 5. 性能目标

| 指标 | Phase1 | Phase2目标 | 提升 |
|------|--------|------------|------|
| 索引速度 | 100文件/分钟 | 150文件/分钟 | 50% |
| 搜索延迟 | 200ms | 50ms | 75% |
| 内存占用 | 500MB | 300MB | 40% |
| 索引大小 | 1:10 | 1:5 | 50% |

---

## 6. 对标TrueNAS TrueSearch

| 功能 | TrueNAS | nas-os Phase2 | 对标状态 |
|------|---------|---------------|----------|
| 文件名搜索 | ✅ | ✅已有 | 🟢持平 |
| 内容搜索 | ✅ | ✅本轮完成 | 🟢对标 |
| 元数据搜索 | ✅ | 📋下一轮 | 🟡跟进 |
| 实时索引 | ✅ | ✅5分钟 | 🟢持平 |
| 高亮显示 | ✅ | ✅本轮 | 🟢对标 |

---

## 7. 实现计划

| 阶段 | 时间 | 任务 |
|------|------|------|
| M1 | 第235轮 | API设计 + 架构文档 |
| M2 | 第236轮 | 内容提取器实现 |
| M3 | 第237轮 | 分词器集成 |
| M4 | 第238轮 | 性能优化 |
| M5 | 第239轮 | 测试与发布 |

---

**兵部签名**: 兵部
**提交时间**: 2026-04-24