# NAS-OS v2.106.0 Release Notes

**发布日期**: 2026-03-16  
**版本类型**: Stable  
**主题**: 质量提升与文档完善

---

## 变更摘要

### 文档更新 (礼部)

- API 文档版本同步至 v2.106.0
- README.md 版本信息更新
- 用户指南、FAQ、故障排查指南版本同步
- 英文文档版本同步

### 版本同步 (吏部)

- 版本号更新至 v2.106.0
- Docker 镜像标签更新
- 下载链接更新

---

## 新增功能

本次版本主要聚焦于文档体系完善和版本同步，为后续功能开发奠定基础。

---

## 文档更新详情

### 核心文档

| 文档 | 行数 | 更新内容 |
|------|------|----------|
| docs/API_GUIDE.md | 2695 | API 完整指南，涵盖 27 个功能模块 |
| docs/FAQ.md | 854 | 36 个常见问题解答 |
| docs/USER_GUIDE.md | 501 | 完整用户手册 |
| docs/QUICKSTART.md | - | 快速开始指南 |
| docs/TROUBLESHOOTING.md | - | 故障排查指南 |

### API 文档模块

API_GUIDE.md 涵盖以下模块：

1. **存储管理** - 卷、快照、子卷管理
2. **用户权限** - 用户/组管理、RBAC
3. **LDAP/AD 集成** - 企业目录服务
4. **容器管理** - Docker 容器操作
5. **虚拟机** - VM 创建与管理
6. **监控告警** - 系统监控与告警配置
7. **性能优化** - 系统性能调优
8. **配额管理** - 存储配额配置
9. **回收站** - 文件恢复
10. **WebDAV** - WebDAV 服务
11. **存储复制** - 数据复制
12. **AI 分类** - 智能文件分类
13. **文件版本控制** - 版本管理
14. **云同步** - 云存储同步
15. **数据去重** - 重复数据删除
16. **iSCSI 目标** - iSCSI 服务
17. **快照策略** - 自动快照
18. **存储分层** - 存储层级管理
19. **FTP 服务器** - FTP 服务
20. **SFTP 服务器** - SFTP 服务
21. **文件标签** - 标签管理
22. **请求日志** - 访问日志
23. **Excel 导出** - 报表导出
24. **成本分析** - 存储成本统计
25. **API Gateway** - API 网关
26. **审计日志** - 安全审计
27. **健康检查** - 系统健康状态

### API 子文档

`docs/api/` 目录下包含详细 API 文档：

| 文件 | 说明 |
|------|------|
| audit-api.md | 审计 API |
| billing-api.md | 计费 API |
| dashboard-api.md | 仪表盘 API |
| examples.md | 使用示例 |
| health-api.md | 健康检查 API |
| invoice-api.md | 发票 API |
| monitor-api.md | 监控 API |
| permission-api.md | 权限 API |
| storage-api.md | 存储 API |
| user-api.md | 用户 API |

---

## 升级说明

### 从 v2.105.0 升级

```bash
# Docker 方式
docker pull ghcr.io/crazyqin/nas-os:v2.106.0
docker stop nasd
docker rm nasd
docker run -d --name nasd ... ghcr.io/crazyqin/nas-os:v2.106.0

# 二进制方式
wget https://github.com/crazyqin/nas-os/releases/download/v2.106.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd
sudo systemctl restart nasd
```

### 配置迁移

无需特殊配置迁移，直接升级即可。

---

## 已知问题

无

---

## 贡献者

- **礼部**：文档更新
- **吏部**：版本同步

---

## 下载

- **GitHub Release**: https://github.com/crazyqin/nas-os/releases/tag/v2.106.0
- **Docker 镜像**: `ghcr.io/crazyqin/nas-os:v2.106.0`

---

*发布日期: 2026-03-16*