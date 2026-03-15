# NAS-OS v2.93.0 Release Notes

**发布日期**: 2026-03-16
**版本类型**: Stable

## 更新内容

### Changed
- **版本号更新** (吏部)
  - 版本号更新至 v2.93.0
  - README.md 版本信息同步

### Fixed
- **测试代码修复** (司礼监)
  - 修复 app_version_test.go 签名不匹配问题
  - 更新测试辅助函数适配新 API

## 升级说明

### Docker 用户
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.93.0
docker stop nasd
docker rm nasd
docker run -d --name nasd --restart unless-stopped \
  -p 8080:8080 -v /data:/data -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.93.0
```

### 二进制用户
```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.93.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.93.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd
```

## 完整变更日志

详见 [CHANGELOG.md](CHANGELOG.md)

---

**下载地址**: https://github.com/crazyqin/nas-os/releases/tag/v2.93.0