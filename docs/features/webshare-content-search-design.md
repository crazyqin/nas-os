# WebShare 内容索引搜索设计

## 背景

参考 TrueNAS 26 WebShare TrueSearch 功能，nas-os 需要增强内容索引搜索能力。

当前状态: ⚠️ 仅支持文件名搜索  
目标: ✅ 支持文件内容全文搜索

---

## 功能需求

### 1. 内容索引能力

| 类型 | 支持范围 |
|------|----------|
| 文档 | PDF, DOCX, TXT, MD |
| 图片 | EXIF元数据、OCR文字 |
| 音频 | 元数据、歌词文件 |
| 视频 | 元数据、字幕文件 |

### 2. 技术方案

```go
// 内容索引服务架构
type ContentIndexService struct {
    Indexer    *ContentIndexer    // 索引器
    Searcher   *ContentSearcher   // 搜索器
    Scheduler  *IndexScheduler    // 索引调度
    Metadata   *MetadataExtractor // 元数据提取
}
```

### 3. 索引策略

- **增量索引**: 文件变更时自动更新索引
- **全量索引**: 定期扫描重建
- **排除策略**: 加密数据集不索引（参考TrueNAS）
- **存储位置**: 独立索引数据库，不影响主存储

### 4. 搜索接口

```go
// 搜索API
GET /api/v1/search/content
{
    "query": "搜索关键词",
    "type": ["document", "image", "audio", "video"],
    "path": "/共享路径",
    "limit": 50,
    "highlight": true
}
```

### 5. 性能要求

| 指标 | 目标值 |
|------|--------|
| 索引速度 | ≥ 1000文件/分钟 |
| 搜索延迟 | ≤ 200ms |
| 索引大小 | ≤ 原数据10% |
| 内存占用 | ≤ 512MB |

---

## 实现路径

### Phase 1: 基础索引 (P0)
- 文本文件全文索引
- 文档元数据提取
- 基础搜索API

### Phase 2: 增强索引 (P1)
- PDF内容解析
- 图片EXIF索引
- 增量更新机制

### Phase 3: 智能搜索 (P2)
- OCR图片文字识别
- AI语义搜索
- 搜索结果推荐

---

## 参考竞品

| 竞品 | 特性 | 学习点 |
|------|------|--------|
| TrueNAS 26 | TrueSearch | 加密排除、Passkey认证 |
| 群晖 DSM | 全文搜索 | 文档解析、中文支持 |
| 飞牛 fnOS | 搜索引擎 | Web界面集成 |

---

## 开发里程碑

- **M110**: 基础索引框架
- **M111**: 文本文件索引实现
- **M112**: 搜索API完善

---

*兵部 2026-04-04*