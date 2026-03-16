# NAS-OS v2.132.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable

---

## 🎯 版本亮点

v2.132.0 版本主要进行品牌营销与文档完善工作，为用户提供更专业的产品体验。

---

## 📝 更新内容

### 礼部 - 品牌营销与版本发布

- ✅ VERSION → v2.132.0
- ✅ internal/version/version.go → 2.132.0
- ✅ docs/api.yaml → 2.132.0
- ✅ README.md 版本信息同步
- ✅ CHANGELOG.md 更新
- ✅ docs/RELEASE-v2.132.0.md 发布公告创建

### Changed

- 版本号升级至 v2.132.0
- Docker 镜像标签更新准备

---

## 📦 安装方式

### 方式一：下载二进制文件 (推荐)

```bash
# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/v2.132.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/v2.132.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# 验证安装
nasd --version
```

### 方式二：Docker 部署

```bash
# 拉取镜像
docker pull ghcr.io/crazyqin/nas-os:v2.132.0

# 运行容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.132.0

# 查看日志
docker logs -f nasd
```

### 方式三：源码编译

```bash
cd nas-os
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl
```

---

## 🚀 快速开始

访问 http://localhost:8080

**默认登录凭据**：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **首次登录后请立即修改默认密码！**

---

## 🔗 相关链接

- [完整文档](docs/)
- [API 文档](docs/API_GUIDE.md)
- [快速开始](docs/QUICKSTART.md)
- [问题反馈](https://github.com/crazyqin/nas-os/issues)
- [Docker 镜像](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

---

## 贡献者

感谢所有贡献者的付出！

---

**完整更新日志**: 查看 [CHANGELOG.md](../CHANGELOG.md)