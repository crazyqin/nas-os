# NAS-OS v2.122.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable  
**状态**: ✅ 已发布

---

## 🎉 概述

NAS-OS v2.122.0 是一个稳定版本，主要聚焦于测试覆盖率的持续提升和测试质量的优化。

---

## ✨ 更新内容

### 兵部 - 测试覆盖率提升
- ✅ 完善现有测试文件
- ✅ 补充 v2.121.0 单元测试
- ✅ 测试质量持续优化

### 礼部 - 文档版本同步
- ✅ VERSION → v2.122.0
- ✅ internal/version/version.go → 2.122.0
- ✅ README.md 版本信息同步
- ✅ docs/api.yaml API 文档版本同步
- ✅ CHANGELOG.md 更新

---

## 📦 构建产物

| 平台 | 架构 | 下载 |
|------|------|------|
| Linux | amd64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.122.0/nasd-linux-amd64) |
| Linux | arm64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.122.0/nasd-linux-arm64) |
| Linux | armv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.88.0/nasd-linux-armv7) |

### Docker 镜像
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.122.0
```

---

## 🔧 升级指南

### 从 v2.121.0 升级

#### 二进制升级
```bash
# 停止服务
sudo systemctl stop nas-os

# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.122.0/nasd-linux-$(uname -m)
chmod +x nasd-linux-$(uname -m)
sudo mv nasd-linux-$(uname -m) /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os

# 验证版本
nasd --version
```

#### Docker 升级
```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.122.0

# 重启容器（使用新镜像）
docker-compose down && docker-compose up -d
```

---

## 📝 变更日志

### v2.122.0 (2026-03-16)
**改进**
- 测试文件完善
- 单元测试补充
- 测试质量优化

**文档**
- 版本信息同步更新

---

*发布团队：礼部*  
*发布日期：2026-03-16*