# NAS-OS v2.65.0 Release Notes

**发布日期**: 2026-03-15

## 概述

v2.65.0 是一个维护版本，主要修复 CI/CD 格式问题、更新文档并完成安全审计。

## 主要更新

### 🛠️ 修复 (Fixed)

#### CI/CD 格式修复 (工部)
- 修复 GitHub Actions 工作流格式问题
- 优化构建流程，确保稳定运行

### 📝 更改 (Changed)

#### 文档更新 (礼部)
- 版本号更新至 v2.65.0
- API 文档同步更新
- README 版本信息同步

### 🔒 安全 (Security)

#### 安全审计 (刑部)
- 完成周期性安全审计
- 依赖漏洞检查与修复

## 升级指南

### 从 v2.63.0 升级

#### Docker 部署

```bash
# 停止旧容器
docker stop nasd
docker rm nasd

# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.65.0

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.65.0
```

#### 二进制部署

```bash
# 下载新版本 (AMD64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.65.0/nasd-linux-amd64
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

- [Linux AMD64](https://github.com/crazyqin/nas-os/releases/download/v2.65.0/nasd-linux-amd64)
- [Linux ARM64](https://github.com/crazyqin/nas-os/releases/download/v2.65.0/nasd-linux-arm64)
- [Linux ARMv7](https://github.com/crazyqin/nas-os/releases/download/v2.65.0/nasd-linux-armv7)
- [Docker 镜像](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

---

**完整变更日志**: 查看 [CHANGELOG.md](../CHANGELOG.md)