# NAS-OS v2.121.0 发布说明

> **发布日期**: 2026-03-16
> **版本类型**: Stable

## 📋 版本概述

v2.121.0 是一个质量改进版本，主要提升测试覆盖率，增强系统稳定性。

## 🔄 变更摘要

### 测试覆盖率提升

| 模块 | 改进内容 |
|------|----------|
| VM/Snapshot | 新增快照管理单元测试 |
| Web | 新增 Web 模块单元测试 |
| 测试报告 | 生成覆盖率统计报告 |

### 文档同步

- VERSION 文件更新至 v2.121.0
- README.md 版本信息同步
- API 文档版本同步
- CHANGELOG.md 更新

## 📦 下载

### 二进制文件

```bash
# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.121.0/nasd-linux-amd64

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v2.121.0/nasd-linux-arm64
```

### Docker

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.121.0
```

## 📚 文档

- [快速开始指南](QUICKSTART.md)
- [用户手册](USER_GUIDE.md)
- [API 文档](API_GUIDE.md)
- [常见问题](FAQ.md)

## 🔗 相关链接

- [GitHub Releases](https://github.com/crazyqin/nas-os/releases/tag/v2.121.0)
- [Docker Hub](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

---

**六部协同开发** | 礼部文档同步