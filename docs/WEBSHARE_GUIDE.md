# WebShare 使用指南

## 功能概述

WebShare 是 NAS-OS 的浏览器文件访问服务，对标 TrueNAS WebShare 功能。

用户无需配置 SMB/NFS 客户端，即可通过浏览器直接浏览、上传、下载文件。

## 核心功能

### 1. 文件浏览
- 支持目录树浏览
- 支持文件预览（图片、PDF、文本）
- 支持文件元数据查看

### 2. 文件操作
- 上传文件（拖拽上传）
- 下载文件（单文件/批量）
- 创建文件夹
- 重命名/移动/删除

### 3. 分享链接
- 生成限时分享链接（可设置过期时间）
- 生成限次分享链接（可设置访问次数）
- 密码保护分享链接
- 分享链接管理（查看、禁用、删除）

### 4. 内容搜索 (TrueSearch)
- 全文检索
- 文件名搜索
- 元数据搜索
- 搜索结果排序与权重

## API 接口

### 文件浏览
```
GET /api/webshare/browse?path=/shared/documents
```

### 文件上传
```
POST /api/webshare/upload
Content-Type: multipart/form-data
```

### 分享链接创建
```
POST /api/webshare/share
{
  "path": "/shared/photos/vacation",
  "expires": "24h",  // 可选：过期时间
  "maxAccess": 10,   // 可选：最大访问次数
  "password": "xxx"  // 可选：密码保护
}
```

### 内容搜索
```
GET /api/webshare/search?q=vacation&type=all
```

## 权限控制

WebShare 继承 NAS-OS 的用户权限体系：
- 只能访问用户有权限的共享文件夹
- 上传/删除操作需要对应权限
- 管理员可配置 WebShare 全局开关

## 安全特性

- 所有操作记录审计日志
- 分享链接有防滥用限制
- 支持防勒索软件实时监控（搭配 Ransomware Defense）

## 与竞品对比

| 功能 | nas-os | TrueNAS | 群晖 |
|------|:------:|:-------:|:----:|
| 浏览器文件浏览 | ✅ | ✅ | ✅ |
| 全文检索 | ✅ | ✅ TrueSearch | ❌ |
| 分享链接 | ✅ | ✅ | ✅ |
| 密码保护分享 | ✅ | ✅ | ✅ |
| 限时分享 | ✅ | ✅ | ✅ |
| 批量下载 | ✅ | ✅ | ✅ |

---

*文档版本: v2.372.0*
*更新日期: 2026-04-01*