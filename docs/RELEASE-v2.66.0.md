# NAS-OS v2.66.0 Release Notes

**发布日期**: 2026-03-15

## 概述

v2.66.0 是一个质量提升版本，主要聚焦于测试覆盖率提升和代码质量改进。

## 主要更新

### 📈 改进 (Improved)

#### 测试覆盖率提升 (兵部)
- 核心模块单元测试完善
- 集成测试覆盖率提升
- 边界条件测试补充

#### 代码质量改进 (兵部)
- 静态分析问题修复
- 代码规范统一
- 注释完善

### 📝 更改 (Changed)

#### 文档同步 (礼部)
- 版本号更新至 v2.66.0
- CHANGELOG.md 版本记录同步

## 升级指南

### 从 v2.65.0 升级

#### Docker 部署

```bash
# 停止旧容器
docker stop nasd
docker rm nasd

# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.66.0

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.66.0
```

#### 二进制部署

```bash
# 下载新版本 (AMD64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.66.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

## 已知问题

暂无已知问题。

## 贡献者

感谢所有参与此版本开发的贡献者！

## 下载

- [Linux AMD64](https://github.com/crazyqin/nas-os/releases/download/v2.66.0/nasd-linux-amd64)
- [Linux ARM64](https://github.com/crazyqin/nas-os/releases/download/v2.66.0/nasd-linux-arm64)
- [Linux ARMv7](https://github.com/crazyqin/nas-os/releases/download/v2.66.0/nasd-linux-armv7)
- [Docker 镜像](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

---

**完整变更日志**: 查看 [CHANGELOG.md](../CHANGELOG.md)