# NAS-OS v2.274.0 发布说明

**发布日期**: 2026-03-25
**版本类型**: Stable

---

## 🎉 新功能

### 🤖 AI相册 - 以文搜图

基于 CLIP 模型的智能照片搜索功能，支持自然语言搜索照片。

**功能亮点**:
- 支持自然语言搜索照片（如 "海边的日落"）
- 场景/物体/人物智能识别
- 多语言搜索支持（中/英/日/韩）

**使用方式**:
```bash
# API 调用
curl -X POST http://localhost:8080/api/v1/photos/search \
  -H "Content-Type: application/json" \
  -d '{"query": "海边的日落", "limit": 10}'
```

---

## 📊 竞品研究更新

### 飞牛 fnOS 1.0 正式版分析
- 免费策略、国产定位
- AI 相册能力研究
- FN Connect 内网穿透方案

### 群晖 DSM 7.3 Synology Tiering
- 冷热数据分层技术分析
- 智能迁移策略研究

### TrueNAS Electric Eel 25.10
- Docker 迁移方案
- RAIDZ 扩展功能

---

## 📚 文档更新

| 文档 | 变更 |
|------|------|
| README.md | 版本号同步至 v2.274.0 |
| CHANGELOG.md | 添加 v2.274.0 更新日志 |
| USER_GUIDE.md | 版本号更新，内容优化 |
| API_GUIDE.md | 版本号更新至 v2.274.0 |
| api.yaml | OpenAPI 规范版本更新 |
| COMPETITOR_ANALYSIS_20260325.md | 补充差异化优势说明 |

---

## 🏆 差异化优势

### 独家功能（竞品无）

| 功能 | 说明 | 竞品状态 |
|------|------|----------|
| 🔒 WriteOnce | 不可变存储(WORM)，防篡改/防勒索 | 群晖/飞牛/TrueNAS 均无 |
| 📊 Fusion Pool | 智能热冷数据分层 | TrueNAS无，飞牛无 |
| 🛡️ KEV/EPSS/LEV | 三级漏洞安全评估 | 竞品无此评估体系 |

### 领先功能

| 功能 | 说明 |
|------|------|
| 🔥 Hot Spare | 热备盘自动故障切换 |
| 📈 SSD 三级预警 | 寿命预测+健康评分+预警 |
| ☁️ 多云挂载 | 阿里云/腾讯云/AWS/GDrive/OneDrive |
| 🚫 自动封锁 | SMB/NFS 防暴力破解 |

---

## 🔄 升级指南

### Docker 升级

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.274.0
docker-compose up -d
```

### 二进制升级

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.274.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

---

## 📋 已知问题

暂无

---

## 🙏 贡献者

感谢所有为本版本做出贡献的开发者！

---

## 📞 支持

- **文档**: https://nas-os.dev/docs
- **Issues**: https://github.com/crazyqin/nas-os/issues
- **讨论区**: https://github.com/crazyqin/nas-os/discussions

---

*礼部 发布*
*2026-03-25*