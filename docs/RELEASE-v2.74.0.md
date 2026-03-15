# NAS-OS v2.74.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable  
**主题**: 版本发布完善

---

## 版本亮点

v2.74.0 是一个版本发布完善版本，重点同步 v2.73.0 变更记录并完善发布流程。

### 六部协同

本版本由礼部主导完成：

| 部门 | 职责 | 贡献 |
|------|------|------|
| 礼部 | 品牌营销 | CHANGELOG 更新、README 版本同步、发布说明 |

---

## v2.73.0 变更记录

### 新增功能

#### 存储管理 API (兵部)

- **storage_handlers.go** - 存储管理 API 处理器
- `/api/storage/volumes` - 卷管理 API
- `/api/storage/pools` - 存储池管理 API
- `/api/storage/snapshots` - 快照管理 API

#### 成本优化分析 (户部)

- 成本优化分析报告
- 资源使用评估报告

### 改进

#### 里程碑完善 (吏部)

- MILESTONES.md 里程碑记录更新
- M2 Web 管理界面标记为完成
- 项目状态文件更新

#### CI/CD 审查 (工部)

- DevOps 检查报告
- CI/CD 配置审查完成

---

## v2.74.0 变更

### 文档更新 (礼部)

- CHANGELOG.md 添加 v2.73.0/v2.74.0 变更记录
- README.md 版本号和下载链接更新
- 创建 docs/RELEASE-v2.74.0.md 发布说明

---

## 下载

### 二进制文件

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.74.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.74.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.74.0/nasd-linux-armv7) |

### Docker

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.74.0
```

---

## 升级说明

从 v2.73.0 升级到 v2.74.0：

1. 停止服务：`systemctl stop nas-os`
2. 备份配置：`cp -r /etc/nas-os /etc/nas-os.bak`
3. 更新二进制文件或 Docker 镜像
4. 启动服务：`systemctl start nas-os`

---

## 下一版本规划

v2.75.0 计划重点：

- 兵部：API 端点完善
- 工部：性能优化
- 刑部：安全审计
- 礼部：文档完善

---

**六部协同，共建 NAS-OS**