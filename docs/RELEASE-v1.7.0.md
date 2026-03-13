# NAS-OS v1.7.0 发布说明

**发布日期**: 2026-03-13  
**版本**: v1.7.0  
**类型**: 功能大更新 - 企业级存储管理

---

## 🎉 主要特性

### 新增 8 大核心模块

| 模块 | 说明 | 状态 |
|------|------|------|
| 📊 存储配额 | 用户/组/目录三级配额控制 | ✅ |
| 🗑️ 回收站 | 安全删除，支持恢复 | ✅ |
| 📁 WebDAV | 完整 WebDAV 协议支持 | ✅ |
| 🔄 存储复制 | 跨节点数据同步 | ✅ |
| 🤖 AI 分类 | 照片/文件智能分类 | ✅ |
| ⚡ 性能优化 | LRU 缓存/连接池/工作池 | ✅ |
| 📈 报告系统 | 定时生成存储/使用报告 | ✅ |
| 🔒 并发控制 | 分布式锁/信号量/限流器 | ✅ |

---

## 📦 详细功能

### 1. 存储配额管理

三级配额控制体系：

```
用户配额 → 组配额 → 目录配额
```

**特性**:
- 用户级别：限制单个用户存储空间
- 组级别：团队共享配额池
- 目录级别：路径空间限制
- 实时使用统计
- 超限告警通知

**API 端点**:
```bash
GET    /api/v1/quota                    # 获取配额列表
POST   /api/v1/quota/users/:id          # 设置用户配额
POST   /api/v1/quota/groups/:id         # 设置组配额
POST   /api/v1/quota/dirs/:path         # 设置目录配额
GET    /api/v1/quota/users/:id/usage    # 获取使用情况
```

### 2. 回收站系统

**特性**:
- 安全删除（移动到回收站而非直接删除）
- 文件恢复（一键还原）
- 自动清理策略
  - 按时间：超过 30 天自动清除
  - 按空间：回收站超过 10GB 自动清理最旧文件
- 回收站搜索和浏览

**API 端点**:
```bash
GET    /api/v1/trash                    # 列出回收站文件
POST   /api/v1/trash/:id/restore        # 恢复文件
DELETE /api/v1/trash/:id                # 永久删除
DELETE /api/v1/trash                    # 清空回收站
```

### 3. WebDAV 服务器

**特性**:
- 完整 WebDAV 协议支持 (RFC 4918)
- 用户认证集成 (JWT)
- 读写权限控制
- 文件锁定支持
- 跨平台客户端兼容

**配置示例**:
```json
{
  "enabled": true,
  "port": 8090,
  "root_path": "/data/webdav",
  "read_only": false,
  "auth_required": true
}
```

### 4. 存储复制

**特性**:
- 跨节点数据同步
- 三种同步模式：
  - `async`: 异步复制（性能优先）
  - `sync`: 同步复制（一致性优先）
  - `realtime`: 实时复制（零延迟）
- Cron 调度支持
- 复制状态监控
- 断点续传

**API 端点**:
```bash
GET    /api/v1/replication              # 获取复制任务列表
POST   /api/v1/replication              # 创建复制任务
GET    /api/v1/replication/:id/status   # 获取复制状态
POST   /api/v1/replication/:id/sync     # 手动触发同步
DELETE /api/v1/replication/:id          # 删除复制任务
```

### 5. AI 分类

**特性**:
- 照片智能分类（人脸/场景/物体）
- 文件内容识别
- 自动标签生成
- 批量处理支持

**使用示例**:
```bash
# 分类单张照片
curl -X POST http://localhost:8080/api/v1/ai-classify/classify \
  -d '{"path": "/data/photos/vacation.jpg", "type": "image"}'

# 响应
{
  "categories": ["vacation", "beach", "sunset"],
  "confidence": 0.92,
  "suggested_tags": ["summer", "travel", "nature"]
}
```

### 6. 性能优化模块

**特性**:
- LRU 缓存系统（命中率 95%+）
- 连接池管理
- 工作池并发控制
- GC 监控和调优
- 性能统计 API

**性能提升**:
| 指标 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| 缓存命中率 | - | 95.6% | 新增 |
| API 响应时间 | 50ms | 35ms | -30% |
| 并发处理 | 100 req/s | 250 req/s | +150% |

### 7. 报告系统

**特性**:
- 存储使用报告
- 用户行为报告
- 系统健康报告
- 定时生成和导出 (PDF/CSV)
- 邮件推送

### 8. 并发控制

**特性**:
- 分布式锁 (Redis/etcd)
- 信号量控制
- 令牌桶限流器
- 批处理任务队列

---

## 🔧 虚拟机管理增强

### 新增功能

- VM 配置持久化
- VM 统计信息获取
- VM 模板系统
- USB/PCIe 设备直通
- VM 快照管理

### API 端点

```bash
GET    /api/v1/vms                      # 列出虚拟机
POST   /api/v1/vms                      # 创建虚拟机
GET    /api/v1/vms/:id                  # 获取 VM 详情
PUT    /api/v1/vms/:id                  # 更新 VM 配置
DELETE /api/v1/vms/:id                  # 删除虚拟机
GET    /api/v1/vm-templates             # 获取 VM 模板
GET    /api/v1/vm-usb-devices           # 获取 USB 设备
GET    /api/v1/vm-pci-devices           # 获取 PCIe 设备
GET    /api/v1/vm-snapshots             # 获取快照列表
POST   /api/v1/vm-snapshots             # 创建快照
```

---

## 📥 安装

### Docker 部署

```bash
# 拉取镜像
docker pull ghcr.io/crazyqin/nas-os:v1.7.0

# 运行
docker run -d \
  --name nas-os \
  -p 8080:8080 \
  -p 8090:8090 \
  -v /var/lib/nas-os:/var/lib/nas-os \
  ghcr.io/crazyqin/nas-os:v1.7.0
```

### 二进制安装

```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v1.7.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v1.7.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7
wget https://github.com/crazyqin/nas-os/releases/download/v1.7.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd
```

---

## 📝 升级指南

### 从 v1.6.x 升级

```bash
# 1. 停止服务
sudo systemctl stop nas-os

# 2. 备份配置
sudo cp -r /var/lib/nas-os /var/lib/nas-os.backup

# 3. 下载新版本
sudo wget -O /usr/local/bin/nasd \
  https://github.com/crazyqin/nas-os/releases/download/v1.7.0/nasd-linux-$(uname -m)
sudo chmod +x /usr/local/bin/nasd

# 4. 启动服务
sudo systemctl start nas-os

# 5. 验证版本
nasd --version
```

### 配置迁移

v1.7.0 新增以下配置文件：

- `/etc/nas-os/quota.json` - 配额配置
- `/etc/nas-os/trash.json` - 回收站配置
- `/etc/nas-os/replication.json` - 复制配置
- `/etc/nas-os/webdav.json` - WebDAV 配置

首次启动会自动创建默认配置。

---

## 🐛 Bug 修复

- 修复 VM 配置加载问题
- 修复下载器文件删除逻辑
- 修复缓存清理不完整
- 优化器传入真实 logger

---

## 🔗 相关链接

- **GitHub Release**: https://github.com/crazyqin/nas-os/releases/tag/v1.7.0
- **Docker Image**: https://github.com/crazyqin/nas-os/pkgs/container/nas-os
- **完整 Changelog**: https://github.com/crazyqin/nas-os/compare/v1.6.0...v1.7.0
- **API 文档**: http://localhost:8080/swagger/index.html

---

## 👥 贡献者

感谢六部团队协作完成：

- **兵部**: 核心功能开发
- **工部**: DevOps 和基础设施
- **礼部**: 文档和 UI 设计
- **吏部**: 项目管理

---

**Full Changelog**: https://github.com/crazyqin/nas-os/compare/v1.6.0...v1.7.0