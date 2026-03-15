# NAS-OS v2.92.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable  
**主题**: 文档体系完善

---

## 变更摘要

### Changed
- README.md 版本更新至 v2.92.0
- 下载链接更新至 v2.92.0
- Docker 镜像标签更新
- docs/api.yaml API 文档版本更新

### Improved
- CHANGELOG.md 添加 v2.91.0/v2.92.0 条目
- 文档版本号同步更新
- API 文档版本同步

---

## 更新的文件

| 文件 | 变更内容 |
|------|----------|
| README.md | 版本号 v2.91.0 → v2.92.0 |
| CHANGELOG.md | 添加 v2.91.0/v2.92.0 条目 |
| docs/api.yaml | API 文档版本 2.89.0 → 2.92.0 |
| docs/RELEASE-v2.92.0.md | 新建发布说明 |

---

## 安装方式

### 方式一：下载二进制文件

```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.92.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.92.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7
wget https://github.com/crazyqin/nas-os/releases/download/v2.92.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd
```

### 方式二：Docker 部署

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.92.0

docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.92.0
```

---

## 升级说明

从 v2.91.0 及更早版本升级：
1. 备份配置文件 (`/etc/nas-os/`)
2. 停止当前服务
3. 替换二进制文件或更新 Docker 镜像
4. 启动服务

---

## API 文档

完整的 API 文档请查看：
- [API Guide](./API_GUIDE.md) - API 使用指南
- [api.yaml](./api.yaml) - OpenAPI 3.0 规范

---

## 六部协同开发

本项目采用六部协同开发模式：
- **吏部**: 项目管理、创业孵化
- **户部**: 财务预算、电商运营
- **礼部**: 品牌营销、内容创作
- **兵部**: 软件工程、系统架构
- **工部**: DevOps、服务器运维
- **刑部**: 法务合规、知识产权

---

**礼部**  
**2026-03-16**