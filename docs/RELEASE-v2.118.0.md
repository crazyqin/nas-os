# NAS-OS v2.118.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable  
**版本重点**: Go 版本升级

---

## 🚀 版本亮点

### Go 1.26 升级

本次版本主要更新 Dockerfile 中的 Go 编译环境至 **Go 1.26**，带来以下改进：

- 🏎️ **性能提升**: Go 1.26 包含多项性能优化
- 🔒 **安全更新**: 最新安全补丁
- 🧹 **工具链改进**: 更好的编译器和构建工具

---

## 📦 更新内容

### Changed

| 组件 | 变更 |
|------|------|
| Dockerfile | Go 版本 1.25 → 1.26 |
| 基础镜像 | golang:1.26-alpine |
| VERSION | v2.118.0 |
| 文档 | 版本号同步 |

---

## 🐳 Docker 部署

```bash
# 拉取最新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.118.0

# 运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.118.0
```

---

## 🔧 技术细节

### Dockerfile 变更

```dockerfile
# 之前
FROM golang:1.25-alpine AS builder

# 之后
FROM golang:1.26-alpine AS builder
```

### 兼容性

- ✅ 现有配置文件兼容
- ✅ API 接口无变更
- ✅ 数据格式无变更

---

## 📥 下载

| 平台 | 下载链接 |
|------|----------|
| Linux AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.118.0/nasd-linux-amd64) |
| Linux ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.118.0/nasd-linux-arm64) |
| Docker | `ghcr.io/crazyqin/nas-os:v2.118.0` |

---

## 📚 相关文档

- [部署指南](DEPLOYMENT_GUIDE_v2.5.0.md)
- [用户手册](USER_GUIDE.md)
- [API 文档](API_GUIDE.md)
- [更新日志](CHANGELOG.md)

---

*六部协同开发 · 工部主导*