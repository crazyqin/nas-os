# NAS-OS v2.46.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 改进优化

### 📚 文档体系完善 (礼部)
- 版本发布说明格式规范化
- README.md 版本信息更新
- 发布文档模板优化
- 所有文档版本号同步至 v2.46.0

### 🔗 下载链接更新 (礼部)
- GitHub Release 下载链接更新
- Docker 镜像版本标签更新
- 支持三种架构：amd64、arm64、armv7

## 升级说明

### Docker 用户

```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.46.0

# 停止旧容器
docker stop nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.46.0
```

### 二进制用户

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.46.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

## 完整变更日志

详见 [CHANGELOG.md](../CHANGELOG.md)

---

**礼部** - 品牌营销与内容创作