# NAS-OS v2.102.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable

## 更新内容

### Changed
- **版本号更新** (吏部)
  - 版本号更新至 v2.102.0
  - README.md 版本信息同步
  - Docker 镜像标签更新

### Improved
- **文档体系完善** (礼部)
  - docs/README.md 版本更新至 v2.102.0
  - docs/api.yaml API 文档版本同步
  - docs/API_GUIDE.md 版本更新
  - docs/FAQ.md 版本更新
  - docs/QUICKSTART.md 版本更新
  - docs/TROUBLESHOOTING.md 版本更新
  - docs/USER_GUIDE.md 版本更新

## 下载

### 二进制文件
- [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.102.0/nasd-linux-amd64)
- [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.102.0/nasd-linux-arm64)

### Docker 镜像
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.102.0
```

## 升级说明

从 v2.101.0 升级：
1. 停止当前服务
2. 替换二进制文件或拉取新 Docker 镜像
3. 重启服务

无破坏性变更，可平滑升级。