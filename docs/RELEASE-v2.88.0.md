# NAS-OS v2.88.0 发布说明

**发布日期**: 2026-03-16
**版本类型**: Stable

---

## 更新内容

### v2.88.0 版本更新 (礼部)

本次版本更新主要为版本号统一更新和文档完善。

#### 文档更新
- README.md 版本更新至 v2.88.0
- CHANGELOG.md 版本记录更新
- docs/ 目录文档版本号同步
- 创建 v2.88.0 发布说明

---

## v2.87.0 安全更新 (刑部)

v2.87.0 版本为安全审计版本，主要包含以下更新：

### 安全审计完成
- gosec 代码漏洞扫描完成 (443 文件，273,102 行代码)
- go vet 静态分析问题修复
- 依赖安全性检查完成

### 测试代码修复
- container_models_test.go 重复函数删除
- trigger_extended_test.go 字段引用修正
- manager_test.go 参数类型修正
- middleware_test.go 重复定义删除
- storage_handlers_test.go 字段引用修正

### 安全评估
- 整体风险等级: 低
- 审计结论: ✅ 通过

---

## 安装方式

### 方式一：下载二进制文件

```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.88.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.88.0/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7
wget https://github.com/crazyqin/nas-os/releases/download/v2.88.0/nasd-linux-armv7
chmod +x nasd-linux-armv7
sudo mv nasd-linux-armv7 /usr/local/bin/nasd
```

### 方式二：Docker 部署

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.88.0

docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.88.0
```

---

## 升级说明

从 v2.86.0 及更早版本升级：
1. 备份配置文件 (`/etc/nas-os/`)
2. 停止当前服务
3. 替换二进制文件或更新 Docker 镜像
4. 启动服务

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