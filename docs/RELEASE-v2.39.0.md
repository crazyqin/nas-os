# NAS-OS v2.39.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 新增功能

### 项目治理完善 (礼部)
- 版本号统一更新至 v2.39.0
- CHANGELOG.md 格式规范化
- MILESTONES.md 里程碑进度更新

### 文档体系完善
- docs/ 目录文档结构优化
- 版本发布说明完善
- 贡献指南更新

## 变更内容

- 版本号一致性检查和更新
- 文档索引优化

## 升级说明

### Docker 用户

```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.39.0

# 停止旧容器
docker stop nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.39.0
```

### 二进制用户

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.39.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

## 完整变更日志

详见 [CHANGELOG.md](../CHANGELOG.md)

---

**礼部** - 品牌营销、内容创作