# NAS-OS v2.11.0 Release Notes

**发布日期**: 2026-03-14  
**版本类型**: Stable  
**基于版本**: v2.10.0

---

## 📋 概述

v2.11.0 是一个维护版本，聚焦于 Bug 修复、测试完善和安全审计。本版本解决了 v2.10.0 中发现的 SMB 并发测试问题，并完善了整体代码质量。

---

## 🔧 修复内容

### SMB 并发测试修复

- **问题**: `TestConcurrentCreateShare` 存在竞态条件
- **影响**: CI 测试偶发失败
- **解决**: 优化 `internal/smb/manager.go` 中的并发控制逻辑
- **验证**: 竞态检测器 (`-race`) 通过

---

## 📈 改进内容

### 共享服务完善

| 模块 | 改进 |
|------|------|
| SMB | 边界条件处理优化 |
| NFS | 错误处理增强 |

### CI/CD 优化

- 检查 GitHub Actions 配置
- 优化测试超时设置
- 添加测试结果缓存

---

## 🧪 测试改进

### 测试覆盖率分析

- 生成覆盖率报告
- 识别低覆盖率模块
- 提出改进建议

### 静态分析

- golangci-lint 全部通过
- 代码审查完成

---

## 🔒 安全审计

### 审查内容

- [x] SMB 安全配置审查
- [x] NFS 安全配置审查
- [x] 权限边界检查
- [x] 敏感数据处理检查

### 结果

所有安全审计项目通过，无重大安全问题。

---

## 📚 文档更新

- [x] README.md 版本号更新
- [x] CHANGELOG.md 添加 v2.11.0 条目
- [x] WebUI 仪表板文档更新
- [x] API 文档同步

---

## 🚀 升级指南

### 从 v2.10.0 升级

```bash
# 停止服务
sudo systemctl stop nas-os

# 更新二进制
wget https://github.com/crazyqin/nas-os/releases/download/v2.11.0/nasd-linux-$(uname -m)
chmod +x nasd-linux-$(uname -m)
sudo mv nasd-linux-$(uname -m) /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os
```

### Docker 升级

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.11.0
docker-compose up -d
```

---

## 📦 下载

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.11.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.11.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.11.0/nasd-linux-armv7) |
| Docker | Multi-arch | `ghcr.io/crazyqin/nas-os:v2.11.0` |

---

## 🙏 致谢

感谢所有参与 v2.11.0 开发和测试的贡献者！

---

## 📝 完整变更日志

详见 [CHANGELOG.md](CHANGELOG.md)

---

*六部轮值系统 - 礼部出品*