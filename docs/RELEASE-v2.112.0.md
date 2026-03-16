# NAS-OS v2.112.0 发布说明

**发布日期**: 2026-03-16
**发布类型**: Stable

## 变更内容

### 版本号更新
- 版本号更新至 v2.112.0
- README.md 版本信息同步
- Docker 镜像标签更新

### 文档更新
- docs/README.md 版本更新至 v2.112.0
- docs/user-guide/README.md 版本同步
- docs/api.yaml API 文档版本更新

## 升级说明

### Docker 升级
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.112.0
docker stop nasd
docker rm nasd
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.112.0
```

### 二进制升级
```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.112.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.112.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd
```

## 完整变更日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解详细变更历史。

---

*此版本由六部协同开发框架自动发布*