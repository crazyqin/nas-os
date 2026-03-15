# NAS-OS v2.71.0 发布公告

**发布日期**: 2026-03-15  
**版本类型**: Stable  
**发布部门**: 礼部 (品牌营销与内容创作)

---

## 📢 版本亮点

NAS-OS v2.71.0 是一个测试代码修复版本，由兵部主导开发。本次更新修复了 disk 和 shares 模块的测试类型错误，提升了代码质量和可测试性。

## 🐛 问题修复

### 测试代码类型修复 (兵部)
- 修复 `internal/disk/handlers` 测试类型错误
- 修复 `internal/shares/handlers` 测试类型错误
- 新增 `internal/shares/interfaces.go` 接口定义，提升代码可测试性
- 简化测试代码结构，删除冗余代码约 444 行

## 🔄 变更内容

| 类别 | 内容 |
|------|------|
| 版本号 | v2.70.0 → v2.71.0 |
| README | 版本信息、下载链接、Docker 镜像标签同步更新 |
| CHANGELOG | 添加 v2.71.0 变更记录 |
| 发布文档 | 新增版本发布公告 |

## 📦 下载地址

### 二进制文件
- **AMD64 (x86_64)**: [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.71.0/nasd-linux-amd64)
- **ARM64**: [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.71.0/nasd-linux-arm64)
- **ARMv7**: [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.71.0/nasd-linux-armv7)

### Docker 镜像
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.71.0
```

## 🙏 致谢

感谢所有为 NAS-OS 项目贡献力量的成员！

---

**NAS-OS 团队**  
*让家用存储更简单*