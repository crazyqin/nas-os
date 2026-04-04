# WebShare Spotlight搜索设计

> **兵部第161轮任务** - 对标TrueNAS SMB Spotlight
> **参考**: TrueNAS Scale 25.10 Spotlight Implementation

---

## 1. 概述

### 1.1 设计目标

对标TrueNAS SMB Spotlight功能，实现：
- SMB共享文件全文搜索
- 元数据索引（文件名、大小、修改时间、类型）
- 内容索引（文本文件、PDF、Office文档）
- Spotlight客户端兼容（macOS Finder搜索）

### 1.2 TrueNAS Spotlight架构分析

TrueNAS使用以下组件实现Spotlight：
- **Tracker**: GNOME桌面搜索框架，提供元数据索引
- **Elasticsearch**: 可选的高级全文搜索引擎
- **SMB Spotlight模块**: SMB协议扩展，支持mds_query

---

## 2. 系统架构

### 2.1 核心组件

```
┌─────────────────────────────────────────────────────┐
│                    Spotlight API                     │
├─────────────────────────────────────────────────────┤
│  QueryParser → IndexSearcher → ResultFormatter     │
├─────────────────────────────────────────────────────┤
│                 Index Manager                        │
│  (Metadata Index + Content Index + ACL Check)       │
├─────────────────────────────────────────────────────┤
│  FileWatcher → ContentExtractor → IndexWriter       │
├─────────────────────────────────────────────────────┤
│                   Storage Layer                      │
│  (Bleve Index / Elasticsearch / SQLite FTS)         │
└─────────────────────────────────────────────────────┘
```

### 2.2 搜索流程

1. 用户发起搜索请求（SMB Spotlight或Web API）
2. QueryParser解析搜索语法
3. IndexSearcher执行索引查询
4. ACL检查验证访问权限
5. ResultFormatter格式化结果
6. 返回搜索结果

---

## 3. 功能设计

### 3.1 索引类型

| 索引类型 | 内容 | 更新频率 |
|----------|------|----------|
| 元数据索引 | 文件名、路径、大小、时间、类型 | 实时 |
| 内容索引 | 文本内容、PDF、Office | 后台异步 |
| ACL索引 | 权限信息、用户/组 | 缓存更新 |

### 3.2 支持的文件类型

| 类型 | 提取方式 | 说明 |
|------|----------|------|
| 文本文件 | 直接读取 | .txt, .md, .log, .csv |
| PDF | pdftotext | 需要poppler-utils |
| Office | libreoffice | .docx, .xlsx, .pptx |
| 图片 | EXIF/OCR | 元数据+可选OCR |
| 视频 | 元数据提取 | 分辨率、时长、编码 |
| 音频 | 元数据提取 | 艺术家、专辑、时长 |

### 3.3 搜索语法

支持多种搜索模式：
- **关键词搜索**: `filename:report type:pdf`
- **通配符搜索**: `name:*.jpg`
- **范围搜索**: `size:>10MB date:>2024-01-01`
- **布尔搜索**: `report AND finance NOT archived`

---

## 4. API设计

### 4.1 REST API

```yaml
# Spotlight搜索API
GET /api/v1/search/spotlight
  Parameters:
    - query: 搜索关键词
    - path: 搜索路径（可选）
    - type: 文件类型（可选）
    - limit: 结果数量限制
    - offset: 分页偏移

# 索引管理API
POST /api/v1/search/index/rebuild
  Body:
    - path: 重建路径
    - force: 强制重建

GET /api/v1/search/index/status
  Response:
    - indexedFiles: 已索引文件数
    - lastIndexTime: 最后索引时间
    - indexSize: 索引大小
```

### 4.2 SMB Spotlight协议

支持macOS Finder搜索：
- mds_query RPC调用
- kMDItemDisplayName, kMDItemContentType等属性
- 搜索结果格式化为Spotlight格式

---

## 5. 性能考虑

### 5.1 索引优化

- 分片索引：按SMB共享分片
- 增量更新：文件变更触发增量索引
- 后台索引：低优先级后台任务
- 缓存热点：热门搜索结果缓存

### 5.2 资源限制

- 内存限制：索引缓存上限
- CPU限制：后台索引CPU配额
- I/O限制：索引写入速率限制

---

## 6. 安全考虑

### 6.1 权限检查

- 搜索结果必须通过ACL验证
- 用户只能看到有权限的文件
- 管理员搜索可跨权限（审计记录）

### 6.2 审计日志

- 记录所有搜索请求
- 记录搜索关键词和结果数
- 异常搜索模式检测

---

## 7. 实现计划

### Phase 1: 基础实现
- 元数据索引引擎
- 基础搜索API
- 文件变更监听

### Phase 2: 内容索引
- 文本文件内容提取
- PDF/Office文档解析
- 内容搜索支持

### Phase 3: SMB集成
- SMB Spotlight协议实现
- macOS Finder兼容
- 性能优化

---

**预计完成**: 第165轮（2026-04-08）