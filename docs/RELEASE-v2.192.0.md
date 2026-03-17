# NAS-OS v2.192.0 发布说明

**发布日期**: 2026-03-17  
**版本类型**: Stable

---

## 📋 变更概要

### 兵部 - 代码质量修复
- ✅ 修复 errcheck 错误
- ✅ 代码质量改进

### 工部 - CI/CD 优化
- ✅ CI/CD 配置优化
- ✅ 构建流程改进

### 刑部 - 安全审计
- ✅ 安全审计完成
- ✅ 安全检查通过

### 礼部 - 文档维护
- ✅ 版本号同步至 v2.192.0
- ✅ README.md 版本信息更新
- ✅ docs/README.md 版本号同步
- ✅ docs/README_EN.md 版本号同步
- ✅ internal/version/version.go 版本同步

---

## 🚀 升级指南

### Docker 升级
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.192.0
docker stop nasd
docker rm nasd
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.192.0
```

### 二进制升级
```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.192.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.192.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd
```

---

## 📦 下载

| 平台 | 架构 | 文件 |
|------|------|------|
| Linux | AMD64 | nasd-linux-amd64 |
| Linux | ARM64 | nasd-linux-arm64 |
| Linux | ARMv7 | nasd-linux-armv7 |
| Docker | Multi-arch | ghcr.io/crazyqin/nas-os:v2.192.0 |

---

*NAS-OS 团队*