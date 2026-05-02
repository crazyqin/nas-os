# Git Server 用户指南

> **版本**: v2.482.0+ | **适用版本**: NAS-OS v2.482.0 及以上

## 概述

NAS-OS 内置 Git Server，提供自托管 Git 仓库管理。无需依赖 GitHub/GitLab 等第三方服务，所有代码和版本历史存储在本地 NAS 上，保证数据隐私和完全控制。

## 核心特性

- **自托管 Git 仓库**：本地创建和管理 Git 仓库
- **SSH + HTTP 双协议**：SSH 密钥认证和 HTTP(S) 访问
- **Webhook 支持**：推送事件触发 Webhook 通知
- **权限管理**：仓库级别读/写/管理员权限
- **Web 浏览界面**：浏览器查看代码、提交历史、分支
- **仓库模板**：预置常见项目模板（Go/Node/Python/空白）
- **LFS 支持**：大文件存储，适合二进制资产

## 配置步骤

### 1. 启用 Git Server

进入 **服务 → Git Server** 页面：

```
SSH 端口：2222（默认，避免与系统 SSH 冲突）
HTTP 端口：3000（默认）
仓库根目录：/data/git（可自定义）
```

### 2. 创建仓库

```bash
# 通过 Web 界面创建
# 服务 → Git Server → 新建仓库

# 或通过 API
curl -X POST http://localhost:8080/api/v1/git/repos \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "description": "我的项目",
    "private": true,
    "template": "go"
  }'
```

### 3. 配置 SSH 密钥

```bash
# 生成 SSH 密钥（如果没有）
ssh-keygen -t ed25519 -C "your@email.com"

# 在 Web 界面添加公钥
# 用户设置 → SSH 密钥 → 添加

# 或通过 API
curl -X POST http://localhost:8080/api/v1/git/ssh-keys \
  -H "Content-Type: application/json" \
  -d '{"title": "My Laptop", "key": "ssh-ed25519 AAAA..."}'
```

### 4. 克隆和推送

```bash
# SSH 方式（推荐）
git clone ssh://git@your-nas-ip:2222/user/my-project.git

# HTTP 方式
git clone http://your-nas-ip:3000/user/my-project.git

# 推送代码
cd my-project
git add .
git commit -m "initial commit"
git push origin main
```

### 5. 配置 Webhook

```bash
# 为仓库添加 Webhook
curl -X POST http://localhost:8080/api/v1/git/repos/my-project/hooks \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-ci-server/webhook",
    "events": ["push", "pull_request"],
    "secret": "your-webhook-secret"
  }'
```

## 仓库权限

| 权限级别 | 说明 |
|----------|------|
| 只读 (Read) | 可以 clone 和 pull，不能 push |
| 读写 (Write) | 可以 clone、pull 和 push |
| 管理员 (Admin) | 完全控制，包括删除仓库和管理权限 |

```bash
# 授权用户访问仓库
curl -X POST http://localhost:8080/api/v1/git/repos/my-project/collaborators \
  -H "Content-Type: application/json" \
  -d '{"username": "developer1", "permission": "write"}'
```

## 常见问题

### Q: Git Server 会消耗很多资源吗？
不会。Git 本身是轻量级的，只有在 push/pull 时有少量 CPU 和磁盘 IO。

### Q: 如何从 GitHub 迁移仓库？
```bash
# 镜像克隆
git clone --mirror https://github.com/user/repo.git

# 推送到本地 Git Server
cd repo.git
git push --mirror ssh://git@your-nas-ip:2222/user/repo.git
```

### Q: 支持 Git LFS 吗？
支持。大文件（>10MB）自动存入 LFS，仓库历史保持轻量。

```bash
# 启用 LFS
git lfs install
git lfs track "*.psd"
git add .gitattributes
```

### Q: 如何备份 Git 仓库？
仓库数据存储在 NAS 文件系统上，可使用 NAS-OS 的快照和备份功能自动保护。

### Q: 支持 CI/CD 吗？
Webhook 可以触发外部 CI/CD 系统（Jenkins、Drone 等）。未来版本将内置轻量 CI。

---

## 相关指南

- [整机备份与灾难恢复](backup-disaster-recovery-guide.md) — Git 仓库数据自动保护
- [Smart Cron 定时任务](smart-cron.md) — 配合 Webhook 实现自动化
- [LXC 容器沙箱](lxc-sandbox-guide.md) — 在沙箱中测试 CI/CD 流程
- [VPN Server](vpn-server-guide.md) — 安全远程访问 Git Server

## API 参考

### 仓库管理

```bash
# 列出所有仓库
curl http://localhost:8080/api/v1/git/repos

# 获取仓库详情
curl http://localhost:8080/api/v1/git/repos/my-project

# 删除仓库
curl -X DELETE http://localhost:8080/api/v1/git/repos/my-project
```

### 分支管理

```bash
# 列出分支
curl http://localhost:8080/api/v1/git/repos/my-project/branches

# 创建分支
curl -X POST http://localhost:8080/api/v1/git/repos/my-project/branches \
  -H "Content-Type: application/json" \
  -d '{"name": "feature-x", "from": "main"}'
```

### 提交历史

```bash
# 获取提交历史
curl "http://localhost:8080/api/v1/git/repos/my-project/commits?limit=20"

# 获取特定提交详情
curl http://localhost:8080/api/v1/git/repos/my-project/commits/<sha>
```

### SSH 密钥管理

```bash
# 列出 SSH 密钥
curl http://localhost:8080/api/v1/git/ssh-keys

# 删除 SSH 密钥
curl -X DELETE http://localhost:8080/api/v1/git/ssh-keys/<key-id>
```
