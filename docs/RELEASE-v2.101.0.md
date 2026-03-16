# NAS-OS v2.101.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable

## 更新内容

### Changed
- 版本号更新至 v2.101.0
- README.md 版本信息同步
- Docker 镜像标签更新

### Improved
- docs/README.md 版本更新至 v2.101.0
- docs/api.yaml API 文档版本同步
- docs/API_GUIDE.md 版本更新
- docs/FAQ.md 版本更新
- docs/QUICKSTART.md 版本更新
- docs/TROUBLESHOOTING.md 版本更新
- docs/USER_GUIDE.md 版本更新

## 升级说明

### Docker 用户

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.101.0
docker stop nasd
docker rm nasd
# 使用新镜像重新启动容器
```

### 二进制用户

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.101.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

## 下载

| 平台 | 文件 | SHA256 |
|------|------|--------|
| Linux AMD64 | nasd-linux-amd64 | - |
| Linux ARM64 | nasd-linux-arm64 | - |
| Docker | ghcr.io/crazyqin/nas-os:v2.101.0 | - |

---

**六部协同开发** | 礼部文档更新