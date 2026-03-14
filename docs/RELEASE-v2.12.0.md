# NAS-OS v2.12.0 Release Notes

**发布日期**: 2026-03-14  
**版本类型**: Stable  
**基于版本**: v2.11.2

---

## 📋 概述

v2.12.0 是一个维护版本，聚焦于 CI/CD 修复、代码质量改进和安全审计。本版本解决了 Go 版本兼容性问题，修复了 SMB 权限验证和 NFS 连接统计的竞态条件，并完成了安全审查。

---

## 🔧 修复内容

### CI/CD 修复

- **Go 版本统一**: 统一所有工作流到 Go 1.25（稳定版）
- **golangci-lint v2 兼容**: 修复配置格式兼容性问题
- **Docker 基础镜像**: 更新到 `golang:1.25-alpine`

### 代码质量改进

| 模块 | 修复内容 |
|------|----------|
| SMB | 修复共享权限验证逻辑 |
| NFS | 修复连接统计计数器竞态条件 |
| 并发 | 改进并发安全性 |

---

## 🔒 安全审计

### 审查结果

| 项目 | 状态 |
|------|------|
| 安全评分 | **B+** |
| SMB/NFS 权限边界 | ✅ 通过 |
| 高危漏洞 | ✅ 无 |

---

## 📈 技术改进

### Go 版本升级

- 从 Go 1.24 升级到 Go 1.25.0
- 更新 CI/CD 工作流配置
- 更新 Dockerfile 基础镜像

### golangci-lint v2

- 更新配置格式到 v2 兼容
- 更新 golangci-lint-action 到 v7

---

## 📚 文档更新

- [x] README.md 版本号更新到 v2.12.0
- [x] CHANGELOG.md 添加 v2.12.0 条目
- [x] 版本路线图更新

---

## 🚀 升级指南

### 从 v2.11.x 升级

```bash
# 停止服务
sudo systemctl stop nas-os

# 更新二进制
wget https://github.com/crazyqin/nas-os/releases/download/v2.12.0/nasd-linux-$(uname -m)
chmod +x nasd-linux-$(uname -m)
sudo mv nasd-linux-$(uname -m) /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os
```

### Docker 升级

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.12.0
docker-compose up -d
```

---

## 📦 下载

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.12.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.12.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.12.0/nasd-linux-armv7) |
| Docker | Multi-arch | `ghcr.io/crazyqin/nas-os:v2.12.0` |

---

## 🙏 致谢

感谢所有参与 v2.12.0 开发和测试的贡献者！

---

## 📝 完整变更日志

详见 [CHANGELOG.md](CHANGELOG.md)

---

*六部轮值系统 - 礼部出品*