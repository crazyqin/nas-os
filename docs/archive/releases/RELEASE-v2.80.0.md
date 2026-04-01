# NAS-OS v2.80.0 发布说明

**发布日期**: 2026-03-16  
**版本类型**: Stable  
**维护团队**: 礼部 (品牌营销/内容创作)

---

## 📝 版本更新

本次版本为文档版本同步更新，主要确保所有文档版本号统一至 v2.80.0。

### 更新内容

| 文档 | 更新内容 |
|------|----------|
| README.md | 版本号更新至 v2.80.0 |
| docs/README.md | 版本号同步更新 |
| docs/USER_GUIDE.md | 用户指南版本更新 |
| docs/FAQ.md | 常见问题文档版本更新 |
| docs/API_GUIDE.md | API 指南版本更新 |
| docs/QUICKSTART.md | 快速开始指南版本更新 |
| docs/TROUBLESHOOTING.md | 故障排查指南版本更新 |
| docs/README_EN.md | 英文文档版本同步 |

---

## 📦 升级指南

### Docker 用户

```bash
# 拉取新版本镜像
docker pull ghcr.io/crazyqin/nas-os:v2.80.0

# 停止旧容器
docker stop nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.80.0
```

### 二进制安装用户

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.80.0/nasd-linux-amd64

# 替换旧版本
sudo systemctl stop nas-os
sudo mv nasd-linux-amd64 /usr/local/bin/nasd
sudo systemctl start nas-os
```

---

## ⚠️ 注意事项

1. **兼容性**: v2.80.0 与 v2.79.0 完全兼容
2. **配置**: 无需修改现有配置文件
3. **数据**: 无数据迁移需求

---

## 📞 支持

- **文档**: [docs/README.md](docs/README.md)
- **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)

---

**礼部 发布**  
*文档完善，版本统一*