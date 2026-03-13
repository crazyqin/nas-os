# NAS-OS v1.8.0 发布说明

**发布日期**: 2026-03-20  
**版本类型**: Stable  
**代号**: Data Guardian

---

## 🎉 版本亮点

v1.8.0 是 NAS-OS 的又一次重大更新，聚焦于**数据安全**与**智能管理**。三大核心功能全面保障您的数据资产：

- 📜 **文件版本控制** - 自动快照、版本对比、一键还原
- ☁️ **云同步增强** - 多云存储支持、双向同步
- 🗜️ **数据去重** - 文件级/块级去重，节省存储空间

---

## ✨ 新增功能

### 📜 文件版本控制 (File Versioning)

自动保护您的文件历史，轻松恢复任意版本。

| 功能 | 说明 |
|------|------|
| 自动快照 | 基于时间或变更触发，无需手动操作 |
| 版本对比 | Diff 显示，清晰查看变更内容 |
| 一键还原 | 选择任意历史版本，快速恢复 |
| 保留策略 | 按数量/时间/空间灵活配置 |
| 自动清理 | 过期版本自动删除，节省空间 |
| WebUI 管理 | 可视化版本浏览与管理界面 |

**使用示例**:
```bash
# 创建版本快照
nasctl version snapshot /data/important.doc

# 查看版本历史
nasctl version list /data/important.doc

# 恢复到指定版本
nasctl version restore /data/important.doc --version 3

# 对比两个版本
nasctl version diff /data/important.doc --v1 2 --v2 5
```

### ☁️ 云同步增强 (Cloud Sync Enhanced)

支持主流云存储服务商，实现本地与云端的无缝同步。

**支持的云存储**:
- 阿里云 OSS
- 腾讯云 COS
- AWS S3
- Google Drive
- OneDrive
- Backblaze B2
- 任何 S3 兼容存储
- WebDAV

**双向同步功能**:
| 功能 | 说明 |
|------|------|
| 上传同步 | 本地→云端自动上传 |
| 下载同步 | 云端→本地自动下载 |
| 实时同步 | 文件变更即时同步 |
| 增量同步 | 仅传输变更部分，高效省流 |
| 冲突处理 | 自动检测并解决同步冲突 |
| 定时同步 | 灵活的同步计划配置 |
| 状态监控 | 实时查看同步进度与状态 |

**配置示例**:
```yaml
cloudsync:
  tasks:
    - name: "backup-to-oss"
      provider: aliyun-oss
      source: /data/photos
      bucket: my-backup-bucket
      mode: bidirectional
      schedule: "0 */6 * * *"  # 每 6 小时
      conflict_resolution: newer
```

### 🗜️ 数据去重 (Data Deduplication)

智能识别重复数据，大幅节省存储空间。

| 功能 | 说明 |
|------|------|
| 文件级去重 | 检测完全相同的文件 |
| 块级去重 | 内容寻址存储，只存唯一数据块 |
| 跨用户去重 | 多用户间共享重复数据 |
| 去重报告 | 详细的空间节省统计 |
| 策略配置 | 自动/手动去重可选 |
| 自动调度 | 定期执行去重任务 |

**效果示例**:
```
去重前: 1.2 TB
去重后: 780 GB
节省空间: 420 GB (35%)
```

---

## 🔧 改进优化

- 优化版本存储效率，减少磁盘占用
- 优化云同步传输性能，提升同步速度
- 改进去重算法效率，降低 CPU 使用率
- 增强错误处理和重试机制，提高稳定性
- 优化 WebUI 加载速度

---

## 📦 新增模块

| 模块 | 路径 | 说明 |
|------|------|------|
| versioning | internal/versioning/ | 文件版本控制 |
| cloudsync | internal/cloudsync/ | 云同步增强 |
| dedup | internal/dedup/ | 数据去重 |

---

## 📚 新增文档

| 文档 | 路径 | 说明 |
|------|------|------|
| VERSIONING.md | docs/VERSIONING.md | 文件版本控制使用指南 |
| CLOUDSYNC.md | docs/CLOUDSYNC.md | 云同步配置指南 |

---

## 🚀 升级指南

### 从 v1.7.x 升级

**二进制升级**:
```bash
# 停止服务
sudo systemctl stop nas-os

# 备份配置
sudo cp -r /etc/nas-os /etc/nas-os.bak

# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v1.8.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os
```

**Docker 升级**:
```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v1.8.0

# 停止旧容器
docker stop nasd

# 备份数据卷
docker run --rm -v nas-os-data:/data -v $(pwd)/backup:/backup alpine tar czf /backup/data-backup.tar.gz /data

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v1.8.0
```

### 配置迁移

v1.8.0 新增配置项：

```yaml
# 新增：版本控制配置
versioning:
  enabled: true
  retention_days: 30
  max_versions: 50
  auto_snapshot: true
  snapshot_interval: "0 * * * *"  # 每小时

# 新增：云同步配置
cloudsync:
  enabled: true
  providers:
    - name: my-oss
      type: aliyun-oss
      endpoint: oss-cn-hangzhou.aliyuncs.com
      bucket: my-bucket
      access_key: ${ALIYUN_ACCESS_KEY}
      secret_key: ${ALIYUN_SECRET_KEY}

# 新增：数据去重配置
dedup:
  enabled: true
  mode: block  # file | block
  schedule: "0 3 * * *"  # 每天凌晨 3 点
  min_size: 1024  # 最小去重文件大小 (bytes)
```

---

## 📊 性能数据

| 指标 | v1.7.0 | v1.8.0 | 提升 |
|------|--------|--------|------|
| 版本快照速度 | - | 5000 文件/秒 | 新增 |
| 云同步传输 | - | 150 MB/s | 新增 |
| 去重效率 | - | 35% 空间节省 | 新增 |
| API 响应时间 (P95) | 100ms | 85ms | 15% |
| 内存占用 | 256MB | 220MB | 14% |

---

## 🐛 Bug 修复

- 修复云同步大文件传输中断问题
- 修复版本快照时间戳时区问题
- 修复去重任务 CPU 占用过高问题
- 修复 WebUI 版本列表分页问题

---

## 🔐 安全更新

- 更新依赖库到最新安全版本
- 云同步支持服务端加密 (SSE)
- 去重数据块支持 AES-256 加密

---

## 📥 下载

### 二进制文件

| 架构 | 下载链接 |
|------|----------|
| AMD64 (x86_64) | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v1.8.0/nasd-linux-amd64) |
| ARM64 (Orange Pi 5, RPi 4/5) | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v1.8.0/nasd-linux-arm64) |
| ARMv7 (RPi 3, 旧款 ARM) | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v1.8.0/nasd-linux-armv7) |

### Docker 镜像

```bash
docker pull ghcr.io/crazyqin/nas-os:v1.8.0
```

支持架构：`linux/amd64`, `linux/arm64`, `linux/arm/v7`

---

## 🙏 致谢

感谢以下贡献者和测试用户对 v1.8.0 的支持！

---

## 📝 反馈

- 🐛 问题反馈: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 功能建议: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)
- 📧 邮件: support@nas-os.com

---

**NAS-OS 团队**  
*让家庭存储更简单*