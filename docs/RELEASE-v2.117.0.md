# NAS-OS v2.117.0 发布说明

**发布日期**: 2026-03-16
**发布类型**: Stable

## 版本亮点

v2.117.0 是一个稳定版本，主要更新包括：
- 版本号同步至 v2.117.0
- 文档体系版本信息同步
- API 文档版本更新

## 更新内容

### 文档更新 (礼部)

| 文档 | 更新内容 |
|------|----------|
| README.md | 版本信息更新至 v2.117.0 |
| CHANGELOG.md | 添加 v2.117.0 版本记录 |
| docs/README.md | 版本同步 |
| docs/api.yaml | API 文档版本更新 |

### 版本同步 (吏部)

- VERSION → v2.117.0
- Docker 镜像标签更新
- 下载链接更新

## 升级说明

### Docker 升级
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.117.0
docker stop nasd
docker rm nasd
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.117.0
```

### 二进制升级
```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.117.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.117.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd
```

## 已知问题

无

## 下载

- **GitHub Release**: https://github.com/crazyqin/nas-os/releases/tag/v2.117.0
- **Docker 镜像**: `ghcr.io/crazyqin/nas-os:v2.117.0`

## 完整变更日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解详细变更历史。

---

*此版本由六部协同开发框架自动发布*