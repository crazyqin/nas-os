# NAS-OS v2.98.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable

---

## 📋 版本概览

v2.98.0 是 NAS-OS 的版本同步更新版本，主要完成版本号统一和文档体系完善。

---

## 🔄 变更内容

### 版本同步 (吏部)

- 版本号升级至 v2.98.0
- README.md 版本信息更新
- 下载链接更新至 v2.98.0
- Docker 镜像标签更新

### 文档完善 (礼部)

- docs/README.md 版本号更新
- docs/api.yaml API 版本号更新
- docs/API_GUIDE.md 版本更新
- docs/FAQ.md 版本更新
- docs/QUICKSTART.md 版本更新
- docs/TROUBLESHOOTING.md 版本更新
- docs/USER_GUIDE.md 版本更新

---

## 📦 安装方式

### 方式一：下载二进制文件

```bash
# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.98.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v2.98.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd
```

### 方式二：Docker 部署

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.98.0

docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.98.0
```

---

## 🔗 相关链接

- [完整 CHANGELOG](CHANGELOG.md)
- [API 文档](api.yaml)
- [用户指南](USER_GUIDE.md)
- [快速开始](QUICKSTART.md)

---

## 🙏 贡献者

感谢所有为本版本做出贡献的团队成员！

---

**NAS-OS 团队**