# NAS-OS v2.97.0 发布说明

> **发布日期**: 2026-03-16  
> **版本类型**: Stable  
> **维护部门**: 礼部

## 📝 版本概览

v2.97.0 是一个常规维护版本，主要包含文档体系更新和版本号同步。

## 🔄 变更内容

### 文档更新
- 更新 README.md 版本信息至 v2.97.0
- 更新 CHANGELOG.md 添加 v2.96.0/v2.97.0 记录
- 更新 docs/README.md 文档索引版本号
- 更新 docs/api.yaml OpenAPI 规范版本

### 版本同步
- VERSION 文件更新至 v2.97.0
- Docker 镜像标签更新准备
- 下载链接更新

## 📦 获取本版本

### Docker
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.97.0
```

### 二进制下载
```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.97.0/nasd-linux-amd64

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.97.0/nasd-linux-arm64
```

## 📋 升级说明

从 v2.96.0 升级无需特殊操作，直接替换二进制文件或重启 Docker 容器即可。

## 🙏 贡献者

感谢所有参与本版本开发的贡献者！

---

*发布说明由礼部自动生成*