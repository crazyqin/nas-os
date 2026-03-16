# NAS-OS v2.120.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable

## 更新内容

### 礼部 - 文档版本同步

- ✅ VERSION → v2.120.0
- ✅ internal/version/version.go → 2.120.0
- ✅ README.md 版本信息同步
- ✅ docs/api.yaml API 文档版本同步
- ✅ docs/README.md 文档版本同步
- ✅ CHANGELOG.md 更新

### Changed

- 版本号更新至 v2.120.0
- 文档版本信息同步

## 下载

### 二进制文件

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.120.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.120.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.88.0/nasd-linux-armv7) |

### Docker

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.120.0
```

## 升级说明

从 v2.119.0 升级到 v2.120.0：

1. 停止当前服务：`systemctl stop nas-os`
2. 替换二进制文件或拉取新 Docker 镜像
3. 启动服务：`systemctl start nas-os`

## 完整更新日志

详见 [CHANGELOG.md](CHANGELOG.md)

---

*礼部敬上*