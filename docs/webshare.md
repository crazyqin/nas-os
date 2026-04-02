# WebShare 用户手册

**版本**: v2.372.0 | **更新**: 2026-04-01 | **对标**: TrueNAS/群晖文档风格

---

## 一、功能概述

### 1.1 什么是 WebShare

WebShare 是 NAS-OS 的浏览器文件访问服务，让用户无需配置 SMB/NFS 客户端，即可通过浏览器直接浏览、上传、下载和管理文件。

**适用场景**:
- 远程办公时快速访问公司文件
- 与客户/合作伙伴临时共享文件
- 手机/平板等移动设备访问NAS文件
- 不具备网络存储配置能力的普通用户

### 1.2 核心功能一览

| 功能类别 | 功能详情 |
|----------|----------|
| **文件浏览** | 目录树浏览、文件预览、元数据查看 |
| **文件操作** | 上传、下载、创建、重命名、移动、删除 |
| **分享管理** | 限时分享、限次分享、密码保护、链接管理 |
| **内容搜索** | 全文检索、文件名搜索、语义搜索 |
| **批量操作** | 批量下载、批量上传、批量删除 |

---

## 二、快速上手

### 2.1 开启 WebShare 服务

**步骤**:

1. 登录 NAS-OS WebUI 管理界面
2. 进入「控制面板」→「服务管理」→「WebShare」
3. 勾选「启用 WebShare 服务」
4. 配置服务端口（默认 8080）
5. 点击「应用」

**验证**: 浏览器访问 `http://NAS地址:8080/webshare`

### 2.2 基本文件浏览

**访问路径**: `http://NAS地址:8080/webshare`

**界面布局**:

```
┌─────────────────────────────────────────────────────────────┐
│  [面包屑导航] / 共享文件夹 / 文档 /                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────────────────────────────┐  │
│  │ 侧边栏      │  │  文件列表                           │  │
│  │ ───────────│  │  ─────────────────────────────────  │  │
│  │ 共享文件夹  │  │  📄 工作报告.docx    2.3MB  2026-03 │  │
│  │ ├─ 文档     │  │  📊 数据分析.xlsx    1.5MB  2026-03 │  │
│  │ ├─ 图片     │  │  📁 项目资料/         -     -       │  │
│  │ ├─ 视频     │  │  🖼️ 产品照片.jpg     5.2MB  2026-03 │  │
│  │ └─ 备份     │  │  📄 会议记录.pdf     800KB  2026-03 │  │
│  └─────────────┘  └─────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  [上传] [新建文件夹] [搜索]                    [批量下载]   │
└─────────────────────────────────────────────────────────────┘
```

---

## 三、功能详解

### 3.1 文件浏览与预览

#### 目录导航

| 操作 | 说明 |
|------|------|
| 点击目录 | 进入该目录 |
| 点击面包屑 | 返回上级目录 |
| 侧边栏展开 | 显示完整目录树 |
| 回车键搜索 | 快速定位文件 |

#### 文件预览

| 文件类型 | 预览方式 |
|----------|----------|
| 图片 (jpg/png/gif) | 缩略图预览 + 点击放大 |
| PDF 文档 | 内嵌 PDF 预览器 |
| 文本文件 (txt/md) | 文本内容预览 |
| 音频文件 | 内嵌播放器 |
| 视频文件 | 内嵌播放器 |
| Office 文档 | 需安装预览插件 |

**预览配置**:

```yaml
webshare:
  preview:
    enabled: true
    max_size: 50MB        # 预览文件大小限制
    image_resize: 800     # 图片预览分辨率
    text_max_chars: 10000 # 文本预览字符数
```

### 3.2 文件上传

#### 单文件上传

1. 点击「上传」按钮
2. 选择本地文件
3. 等待上传完成
4. 查看上传进度条

#### 拖拽上传

- 直接拖拽文件到文件列表区域
- 支持多文件同时拖拽
- 拖拽时显示上传目标路径

#### 大文件上传

**配置参数**:

```yaml
webshare:
  upload:
    chunk_size: 10MB       # 分片大小
    max_file_size: 5GB     # 单文件大小限制
    max_total_size: 20GB   # 单次上传总量限制
    timeout: 3600          # 上传超时时间（秒）
```

**断点续传**: 大文件上传支持断点续传，网络中断后可继续上传。

### 3.3 文件下载

| 下载方式 | 操作方法 |
|----------|----------|
| 单文件下载 | 点击文件右侧下载图标 |
| 批量下载 | 勾选多个文件 → 点击「批量下载」 |
| 目录下载 | 点击目录右侧下载图标（打包为ZIP） |
| 右键下载 | 右键文件 → 选择「下载」 |

**下载配置**:

```yaml
webshare:
  download:
    batch_limit: 100       # 批量下载文件数量限制
    zip_compression: fast  # 打包压缩方式 (fast/best/none)
    max_speed: 50MB/s      # 最大下载速度限制
```

### 3.4 文件操作

#### 创建文件夹

1. 点击「新建文件夹」按钮
2. 输入文件夹名称
3. 点击「确认」

#### 重命名

1. 右键点击文件
2. 选择「重命名」
3. 输入新名称
4. 点击「确认」

#### 移动/复制

1. 选中文件
2. 右键选择「移动到」或「复制到」
3. 选择目标目录
4. 点击「确认」

#### 删除

1. 选中文件
2. 右键选择「删除」或点击「删除」按钮
3. 确认删除操作
4. 文件移入回收站（可恢复）

### 3.5 分享链接

#### 创建分享链接

**步骤**:

1. 选中文件或目录
2. 点击「创建分享」按钮
3. 配置分享参数
4. 点击「生成链接」
5. 复制分享链接

**分享参数配置**:

| 参数 | 说明 | 默认值 |
|------|------|--------|
| 过期时间 | 分享链接有效期 | 7天 |
| 访问次数 | 最大访问次数限制 | 无限制 |
| 密码保护 | 设置访问密码 | 无 |
| 下载权限 | 允许/禁止下载 | 允许 |
| 预览权限 | 允许/禁止预览 | 允许 |

#### 分享链接类型

| 类型 | 使用场景 | 配置建议 |
|------|----------|----------|
| **限时分享** | 短期合作、临时交付 | 过期时间1-24小时 |
| **限次分享** | 限制传播范围 | 访问次数1-5次 |
| **密码分享** | 安全要求高 | 设置强密码 |
| **公开分享** | 大范围分发 | 无密码，长期有效 |

#### 分享链接管理

**入口**: 「控制面板」→「WebShare」→「分享管理」

**管理功能**:

| 操作 | 说明 |
|------|------|
| 查看列表 | 查看所有分享链接及状态 |
| 禁用链接 | 暂停链接访问权限 |
| 删除链接 | 彻底删除分享链接 |
| 查看统计 | 查看链接访问次数、下载次数 |
| 延长有效期 | 修改链接过期时间 |

### 3.6 内容搜索

#### 搜索入口

- 文件列表上方搜索框
- 快捷键 Ctrl+F / Cmd+F

#### 搜索类型

| 搜索类型 | 说明 | 示例 |
|----------|------|------|
| **文件名搜索** | 搜索文件名匹配 | `工作报告` |
| **全文检索** | 搜索文件内容 | `财务分析` |
| **语义搜索** | 自然语言搜索 | `去年夏天的海边照片` |
| **元数据搜索** | 搜索文件属性 | `类型:图片 大于:5MB` |

#### 搜索语法

```
# 基本搜索
工作报告          # 文件名包含"工作报告"

# 类型过滤
类型:图片         # 仅图片文件
类型:文档         # 仅文档文件

# 大小过滤
大于:10MB         # 大于10MB的文件
小于:1MB          # 小于1MB的文件

# 时间过滤
日期:2026-03      # 2026年3月的文件
修改:今天         # 今天修改的文件

# 组合搜索
类型:图片 大于:5MB 日期:2026-03
```

---

## 四、权限与安全

### 4.1 权限继承

WebShare 继承 NAS-OS 用户权限体系：

| 权限类型 | 说明 |
|----------|------|
| **读取权限** | 可浏览、下载、预览文件 |
| **写入权限** | 可上传、创建、修改文件 |
| **删除权限** | 可删除文件和目录 |
| **管理权限** | 可配置分享、修改权限 |

### 4.2 用户访问控制

**配置路径**: 「控制面板」→「WebShare」→「访问控制」

| 配置项 | 说明 |
|----------|------|
| 允许用户 | 指定可访问 WebShare 的用户 |
| 禁止用户 | 禁止特定用户访问 |
| 匿名访问 | 允许/禁止匿名用户访问 |
| IP限制 | 仅允许特定IP访问 |

### 4.3 安全特性

| 安全特性 | 说明 |
|----------|------|
| **审计日志** | 所有操作记录审计日志 |
| **防滥用限制** | 分享链接有访问频率限制 |
| **HTTPS支持** | 支持HTTPS加密传输 |
| **病毒扫描** | 上传文件自动病毒扫描 |
| **勒索防护** | 搭配Ransomware Defense实时监控 |

---

## 五、API 接口文档

### 5.1 认证方式

**Token认证**: 在请求头添加 Authorization

```
Authorization: Bearer <token>
```

**获取Token**:

```bash
POST /api/auth/login
{
  "username": "admin",
  "password": "password"
}
```

### 5.2 文件浏览 API

**浏览目录**:

```bash
GET /api/webshare/browse?path=/shared/documents

Response:
{
  "path": "/shared/documents",
  "files": [
    {
      "name": "工作报告.docx",
      "type": "file",
      "size": 2300000,
      "modified": "2026-03-25T10:30:00Z",
      "mime_type": "application/docx"
    },
    {
      "name": "项目资料",
      "type": "directory",
      "modified": "2026-03-20T08:00:00Z"
    }
  ]
}
```

**获取文件信息**:

```bash
GET /api/webshare/file?path=/shared/documents/工作报告.docx

Response:
{
  "name": "工作报告.docx",
  "path": "/shared/documents/工作报告.docx",
  "size": 2300000,
  "modified": "2026-03-25T10:30:00Z",
  "mime_type": "application/docx",
  "checksum": "sha256:abc123..."
}
```

### 5.3 文件上传 API

**单文件上传**:

```bash
POST /api/webshare/upload
Content-Type: multipart/form-data

file: <文件内容>
path: /shared/documents

Response:
{
  "success": true,
  "file": {
    "name": "新文档.docx",
    "path": "/shared/documents/新文档.docx",
    "size": 1500000
  }
}
```

**分片上传**:

```bash
# 初始化分片上传
POST /api/webshare/upload/init
{
  "path": "/shared/videos/large.mp4",
  "size": 5000000000
}

Response:
{
  "upload_id": "upload_abc123",
  "chunk_size": 10000000
}

# 上传分片
POST /api/webshare/upload/chunk
Content-Type: multipart/form-data

upload_id: upload_abc123
chunk_index: 0
chunk: <分片内容>

# 完成上传
POST /api/webshare/upload/complete
{
  "upload_id": "upload_abc123"
}
```

### 5.4 文件下载 API

**下载文件**:

```bash
GET /api/webshare/download?path=/shared/documents/工作报告.docx

Response: 文件流
```

**批量下载**:

```bash
POST /api/webshare/download/batch
{
  "paths": [
    "/shared/documents/工作报告.docx",
    "/shared/documents/数据分析.xlsx"
  ]
}

Response: ZIP文件流
```

### 5.5 分享链接 API

**创建分享**:

```bash
POST /api/webshare/share
{
  "path": "/shared/photos/vacation",
  "expires": "24h",        # 可选：过期时间
  "max_access": 10,        # 可选：最大访问次数
  "password": "secret123", # 可选：密码保护
  "permissions": {
    "preview": true,
    "download": true
  }
}

Response:
{
  "share_id": "share_abc123",
  "url": "https://nas.example.com/s/abc123",
  "expires_at": "2026-04-02T10:00:00Z"
}
```

**获取分享列表**:

```bash
GET /api/webshare/share/list

Response:
{
  "shares": [
    {
      "share_id": "share_abc123",
      "path": "/shared/photos/vacation",
      "url": "https://nas.example.com/s/abc123",
      "created": "2026-04-01T10:00:00Z",
      "expires_at": "2026-04-02T10:00:00Z",
      "access_count": 5
    }
  ]
}
```

**删除分享**:

```bash
DELETE /api/webshare/share?id=share_abc123

Response:
{
  "success": true
}
```

### 5.6 搜索 API

```bash
GET /api/webshare/search?q=工作报告&type=all

Response:
{
  "results": [
    {
      "name": "工作报告.docx",
      "path": "/shared/documents/工作报告.docx",
      "size": 2300000,
      "snippet": "本报告总结了...",
      "relevance": 0.95
    }
  ],
  "total": 15,
  "page": 1
}
```

---

## 六、配置详解

### 6.1 服务配置

**配置文件**: `/etc/nas-os/webshare.yaml`

```yaml
webshare:
  # 服务基础配置
  enabled: true
  listen: 0.0.0.0:8080
  workers: 4
  
  # 存储路径配置
  root_path: /data/shares
  temp_path: /tmp/webshare
  
  # 上传配置
  upload:
    enabled: true
    chunk_size: 10MB
    max_file_size: 5GB
    max_total_size: 20GB
    timeout: 3600
    
  # 下载配置
  download:
    enabled: true
    batch_limit: 100
    zip_compression: fast
    max_speed: 50MB/s
    
  # 分享配置
  share:
    enabled: true
    default_expire: 7d
    max_expire: 30d
    max_access: 1000
    password_required: false
    
  # 预览配置
  preview:
    enabled: true
    max_size: 50MB
    image_resize: 800
    text_max_chars: 10000
    
  # 搜索配置
  search:
    enabled: true
    index_path: /var/lib/webshare/index
    update_interval: 5m
    
  # 安全配置
  security:
    audit_log: true
    virus_scan: true
    ip_whitelist: []
    anonymous_access: false
```

### 6.2 性能优化

| 配置项 | 建议值 | 说明 |
|----------|--------|------|
| workers | 4-8 | 根据CPU核心数调整 |
| chunk_size | 10-50MB | 大文件上传分片大小 |
| zip_compression | fast | 批量下载压缩方式 |
| max_speed | 50MB/s | 限制下载带宽 |

---

## 七、常见问题

### Q1: 上传大文件失败怎么办？

**A**: 检查以下配置：
1. `max_file_size` 是否足够大
2. `timeout` 是否足够长（建议3600秒）
3. 网络稳定性，使用分片上传

### Q2: 分享链接无法访问？

**A**: 可能原因：
1. 链接已过期 → 延长有效期
2. 达到访问次数限制 → 增加访问次数
3. IP不在白名单 → 检查IP限制配置
4. 服务未启动 → 检查服务状态

### Q3: 文件预览加载慢？

**A**: 优化方案：
1. 减小 `preview.max_size` 限制
2. 减小 `preview.image_resize` 分辨率
3. 启用缓存机制

### Q4: 如何批量上传整个目录？

**A**: 目前支持：
1. 拖拽多个文件批量上传
2. API分片上传大文件
3. 目录打包后上传

---

## 八、与竞品对比

| 功能 | NAS-OS | TrueNAS | 群晖 |
|------|:------:|:-------:|:----:|
| 浏览器文件浏览 | ✅ | ✅ | ✅ |
| 全文检索 | ✅ | ✅ TrueSearch | ✅ |
| 语义搜索 | ✅ | ❌ | ❌ |
| 分享链接 | ✅ | ✅ | ✅ |
| 密码保护分享 | ✅ | ✅ | ✅ |
| 限时分享 | ✅ | ✅ | ✅ |
| 批量下载 | ✅ | ✅ | ✅ |
| 断点续传 | ✅ | ✅ | ✅ |
| 病毒扫描 | ✅ | ⚠️ | ✅ |
| API接口 | ✅ | ✅ | ✅ |

---

## 九、相关文档

| 文档 | 路径 | 说明 |
|------|------|------|
| API完整参考 | `docs/API_GUIDE.md` | API接口详细说明 |
| 用户权限配置 | `docs/USER_GUIDE.md` | 用户管理说明 |
| 勒索防护指南 | `docs/RANSOMWARE_GUIDE.md` | 安全防护配置 |

---

**文档维护**: 礼部 | **版本**: v2.372.0 | **更新**: 2026-04-01