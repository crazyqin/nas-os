# NAS-OS v2.3.0 发布说明

**发布日期**: 2026-03-28  
**版本类型**: Stable  

---

## 🎉 版本亮点

v2.3.0 版本带来了智能存储管理增强，包括存储分层系统、FTP/SFTP 服务器、压缩存储和文件标签系统，帮助用户更高效地管理和组织数据。

---

## ✨ 新增功能

### 🗂️ 存储分层系统 (Storage Tiering)

智能化的数据分层管理，根据访问频率自动迁移数据，优化存储性能与成本。

**核心功能**:
- 热/冷数据自动分层
- SSD 缓存层配置
- HDD 存储层配置
- 云存储归档层配置
- 访问频率统计
- 自动迁移规则
- 分层状态可视化

**分层策略**:
| 层级 | 存储 | 用途 | 性能 |
|------|------|------|------|
| Hot | SSD | 频繁访问数据 | 高速读写 |
| Cold | HDD | 较少访问数据 | 标准性能 |
| Archive | 云存储 | 归档数据 | 低成本 |

**快速开始**:
```bash
# 配置热数据层
nasctl tiering config --type hot --path /data/ssd-cache

# 创建分层策略
nasctl tiering policy create \
  --name photos-archive \
  --source hot \
  --target cold \
  --rule "access_age:30" \
  --schedule "0 3 * * *"
```

---

### 📡 FTP 服务器

完整的 FTP 服务支持，兼容传统 FTP 客户端。

**核心功能**:
- 被动/主动模式支持
- 匿名登录支持
- 用户认证集成
- 虚拟目录映射
- 带宽限制配置
- WebUI 配置界面

**使用场景**:
- 传统 FTP 客户端访问
- 批量文件传输
- 旧系统兼容

**快速开始**:
```bash
# 启用 FTP 服务
nasctl ftp enable --port 21 --passive-ports 50000-51000

# 配置带宽限制
nasctl ftp config --bandwidth-limit 10MB
```

---

### 🔐 SFTP 服务器

基于 SSH 的安全文件传输服务。

**核心功能**:
- SSH 密钥认证
- 用户权限隔离
- chroot 目录限制
- 安全文件传输
- WebUI 配置界面

**安全特性**:
- 端到端加密传输
- 公钥/密码双认证
- 用户目录隔离
- 连接数限制

**快速开始**:
```bash
# 启用 SFTP 服务
nasctl sftp enable --port 2222

# 生成主机密钥
nasctl sftp generate-key

# 添加用户公钥
nasctl sftp add-key --user admin --key ~/.ssh/id_rsa.pub
```

---

### 🗜️ 压缩存储

透明的数据压缩，节省存储空间。

**核心功能**:
- 文件级压缩（透明压缩）
- 块级压缩
- 压缩算法选择（zstd/lz4/gzip）
- 压缩率统计
- 自动压缩策略

**压缩算法对比**:
| 算法 | 压缩率 | 速度 | 适用场景 |
|------|--------|------|----------|
| zstd | 高 | 快 | 通用推荐 |
| lz4 | 中 | 最快 | 高性能场景 |
| gzip | 最高 | 较慢 | 归档场景 |

---

### 🏷️ 文件标签系统

通过标签组织和分类文件。

**核心功能**:
- 标签 CRUD 操作
- 标签颜色和图标
- 文件标签关联
- 批量标签操作
- 按标签搜索
- 标签云显示

**使用场景**:
- 文件分类管理
- 项目文件标记
- 重要文件标识
- 快速检索

**快速开始**:
```bash
# 创建标签
nasctl tags create --name "重要" --color red --icon star

# 为文件添加标签
nasctl tags add --tag important --files /data/docs/report.pdf

# 按标签搜索
nasctl tags search --tag important
```

---

## 🔧 改进优化

### 测试覆盖率提升
| 模块 | 测试覆盖率 |
|------|-----------|
| tiering | 78.5% |
| ftp | 82.3% |
| sftp | 85.1% |
| tags | 76.8% |

### 性能优化
- 优化大文件传输性能
- 改进压缩算法选择逻辑
- 优化存储分层调度

---

## 📦 API 变更

### 新增端点

**存储分层**:
```
GET    /api/v1/tiering/tiers
GET    /api/v1/tiering/tiers/:type
PUT    /api/v1/tiering/tiers/:type
GET    /api/v1/tiering/policies
POST   /api/v1/tiering/policies
GET    /api/v1/tiering/policies/:id
PUT    /api/v1/tiering/policies/:id
DELETE /api/v1/tiering/policies/:id
POST   /api/v1/tiering/policies/:id/execute
POST   /api/v1/tiering/migrate
GET    /api/v1/tiering/tasks
GET    /api/v1/tiering/status
```

**FTP**:
```
GET    /api/v1/ftp/config
PUT    /api/v1/ftp/config
GET    /api/v1/ftp/status
POST   /api/v1/ftp/restart
```

**SFTP**:
```
GET    /api/v1/sftp/config
PUT    /api/v1/sftp/config
GET    /api/v1/sftp/status
POST   /api/v1/sftp/restart
```

**文件标签**:
```
GET    /api/v1/tags
POST   /api/v1/tags
GET    /api/v1/tags/:id
PUT    /api/v1/tags/:id
DELETE /api/v1/tags/:id
POST   /api/v1/tags/:id/files
DELETE /api/v1/tags/:id/files
GET    /api/v1/tags/:id/files
POST   /api/v1/tags/batch
GET    /api/v1/tags/cloud
```

---

## 📚 文档更新

| 文档 | 说明 |
|------|------|
| TIERING_GUIDE.md | 存储分层配置指南 |
| FTP_SFTP_GUIDE.md | FTP/SFTP 服务器配置 |
| FILE_TAGS_GUIDE.md | 文件标签系统使用说明 |
| API_GUIDE.md | 新增模块 API 文档 |

---

## 🐛 Bug 修复

- 修复存储分层在大量文件时的性能问题
- 修复 FTP 被动模式端口范围配置问题
- 修复 SFTP 在高并发下的连接泄漏
- 改进压缩存储的内存管理

---

## ⚠️ 升级说明

### 从 v2.2.0 升级

```bash
# 停止服务
systemctl stop nas-os

# 备份配置
cp -r /etc/nas-os /etc/nas-os.bak

# 升级
nasd upgrade --from v2.2.0

# 启动服务
systemctl start nas-os
```

### 配置变更

新增配置节：
```yaml
# 存储分层配置
tiering:
  enabled: true
  hot_tier:
    path: /data/ssd-cache
    capacity: 500GB
  cold_tier:
    path: /data/hdd-storage
    capacity: 10TB

# FTP 配置
ftp:
  enabled: false
  port: 21
  passive_ports: "50000-51000"
  max_connections: 50

# SFTP 配置
sftp:
  enabled: false
  port: 22
  auth_methods:
    - password
    - publickey

# 文件标签配置
tags:
  enabled: true
```

---

## 🔜 下一步计划

### v2.4.0 (计划中)

- LDAP/AD 集成
- 高级权限管理增强
- 审计日志完善
- 多语言支持扩展

---

## 🙏 致谢

感谢所有参与 v2.3.0 开发和测试的贡献者！

---

**发布团队**: NAS-OS 礼部  
**发布日期**: 2026-03-28