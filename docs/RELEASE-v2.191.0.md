# NAS-OS v2.191.0 发布说明

**发布日期**: 2026-03-17

## 改进

### 代码质量
- 改进 defer 错误处理模式
- 统一使用命名返回值和 err 变量捕获 defer 中的错误

### 文档更新 (礼部)
- 版本号同步至 v2.191.0
- README.md 版本信息更新
- docs/README.md 版本号同步
- docs/README_EN.md 版本号同步
- docs/api.yaml API 文档版本同步
- docs/swagger.json OpenAPI 文档版本同步
- docs/swagger.yaml 版本同步

## 下载

### 二进制文件
- [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.191.0/nasd-linux-amd64)
- [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.191.0/nasd-linux-arm64)
- [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.191.0/nasd-linux-armv7)

### Docker
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.191.0
```

## 升级说明

从 v2.190.0 升级无破坏性变更，直接替换二进制文件或更新 Docker 镜像即可。