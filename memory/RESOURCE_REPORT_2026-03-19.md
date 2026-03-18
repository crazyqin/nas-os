# nas-os 项目资源统计报告

**统计日期**: 2026-03-19  
**统计者**: 户部

---

## 一、代码行数统计

### 1.1 总体代码量

| 类型 | 行数 |
|------|------|
| Go 源码 | 279,632 行 |
| Go 测试码 | 120,969 行 |
| Web UI (JS/HTML/CSS) | 49,853 行 |
| **总计** | **450,454 行** |

### 1.2 按目录分布

| 目录 | 源码行数 | 占比 |
|------|----------|------|
| internal/ | 256,716 | 91.8% |
| webui/ | 49,853 | - |
| api/ | 6,186 | 2.2% |
| cmd/ | 1,991 | 0.7% |
| pkg/ | 1,627 | 0.6% |

### 1.3 internal/ 主要模块代码量 (Top 15)

| 模块 | 代码行数 | 测试代码行数 |
|------|----------|--------------|
| reports | 29,116 | 5,949 |
| security | 18,184 | 6,194 |
| quota | 13,780 | 5,256 |
| monitor | 10,442 | 2,233 |
| backup | 9,521 | 3,775 |
| cluster | 8,477 | - |
| project | 8,395 | - |
| audit | 7,634 | - |
| auth | 7,479 | 3,158 |
| billing | 7,151 | 3,105 |
| media | 5,923 | 3,110 |
| docker | 5,495 | 2,862 |
| photos | 5,197 | - |
| storage | 4,727 | 4,282 |
| budget | 4,299 | - |

---

## 二、测试覆盖率统计

### 2.1 整体覆盖率

- **总体覆盖率**: 19.5% (基于 coverage.out)
- **测试文件数量**: 252 个
- **测试/源码比**: 120,969 / 279,632 ≈ 43.2%

### 2.2 覆盖率高于 60% 的模块 (测试覆盖良好) ✅

| 模块 | 覆盖率 |
|------|--------|
| version | 100.0% |
| notify | 90.7% |
| automation/api | 86.8% |
| transfer | 86.1% |
| trash | 83.1% |
| billing/cost_analysis | 82.0% |
| dashboard | 77.3% |
| database | 76.0% |
| smb | 73.4% |
| nfs | 72.6% |
| versioning | 71.6% |
| iscsi | 68.0% |
| replication | 63.9% |
| health | 63.8% |
| pkg/btrfs | 63.6% |
| prediction | 63.6% |
| billing | 61.8% |
| quota/optimizer | 61.3% |
| rbac | 60.7% |
| concurrency | 60.5% |

### 2.3 覆盖率低于 30% 的模块 (需改进) ⚠️

| 模块 | 覆盖率 | 建议 |
|------|--------|------|
| cmd/backup | 0.0% | 添加单元测试 |
| cmd/nasctl | 0.0% | 添加单元测试 |
| cmd/nasd | 0.0% | 添加单元测试 |
| internal/security/cmdsec | 0.0% | 添加安全测试 |
| internal/downloader | 18.1% | 提升测试覆盖 |
| internal/compress | 19.8% | 提升测试覆盖 |
| internal/container | 20.6% | 提升测试覆盖 |
| internal/photos | 20.6% | 提升测试覆盖 |
| internal/project | 20.4% | 提升测试覆盖 |
| internal/quota | 20.5% | 提升测试覆盖 |
| pkg/safeguards | 20.9% | 提升测试覆盖 |
| internal/office | 20.9% | 提升测试覆盖 |
| internal/budget | 22.5% | 提升测试覆盖 |
| internal/logging | 22.5% | 提升测试覆盖 |
| internal/network | 22.1% | 提升测试覆盖 |
| internal/ftp | 21.7% | 提升测试覆盖 |
| internal/perf | 22.7% | 提升测试覆盖 |
| internal/service | 22.7% | 提升测试覆盖 |
| internal/web | 23.4% | 提升测试覆盖 |
| internal/cloudsync | 25.8% | 提升测试覆盖 |
| internal/monitor | 25.7% | 提升测试覆盖 |
| internal/notification | 25.0% | 提升测试覆盖 |
| internal/dashboard/health | 25.3% | 提升测试覆盖 |
| internal/cluster | 27.9% | 提升测试覆盖 |
| internal/webdav | 27.4% | 提升测试覆盖 |
| internal/security/v2 | 27.2% | 提升测试覆盖 |
| internal/disk | 28.5% | 提升测试覆盖 |
| internal/automation/action | 29.7% | 接近阈值 |
| internal/ldap | 29.4% | 接近阈值 |
| internal/security/scanner | 29.4% | 接近阈值 |

---

## 三、依赖统计

### 3.1 依赖数量

| 类型 | 数量 |
|------|------|
| 直接依赖 (go.mod) | 161 个 |
| 总依赖 (go.sum) | 475 个 |
| 间接依赖 | 314 个 |

### 3.2 依赖来源分布

| 来源 | 数量 |
|------|------|
| GitHub | 127 |
| golang.org | 13 |
| go.opentelemetry.io | 9 |
| google.golang.org | 4 |
| 其他 | 12 |

### 3.3 主要直接依赖

- **Web 框架**: gin-gonic/gin
- **数据库**: go-redis/redis
- **云存储**: aws/aws-sdk-go-v2 (S3)
- **搜索**: blevesearch/bleve
- **监控**: prometheus/client_golang
- **认证**: go-ldap/ldap, pquerna/otp
- **文档**: swaggo (Swagger)
- **CLI**: spf13/cobra

---

## 四、编译产物

| 产物 | 大小说明 |
|------|----------|
| nasd (后端服务) | 73 MB |
| nasctl (CLI 工具) | 6.2 MB |

---

## 五、文件统计

| 类型 | 数量 |
|------|------|
| Go 源文件 | 455 个 |
| Go 测试文件 | 252 个 |
| Web UI 文件 | 62 个 |
| 文档文件 (MD) | 398 个 |
| 配置文件 | 109 个 |

---

## 六、改进建议

### 6.1 高优先级

1. **cmd/ 目录测试覆盖率为 0%** - 入口命令应有基本测试
2. **security/cmdsec 无测试** - 安全模块必须有完整测试
3. **downloader (18.1%)** - 文件下载是核心功能
4. **container (20.6%)** - 容器管理是核心功能

### 6.2 中优先级

5. **project (20.4%)** - 项目管理功能
6. **quota (20.5%)** - 配额管理
7. **cloudsync (25.8%)** - 云同步功能

### 6.3 测试资源分配建议

| 模块 | 当前覆盖率 | 建议目标 | 测试工作量估计 |
|------|------------|----------|----------------|
| cmd/* | 0% | 30% | 中 |
| security/cmdsec | 0% | 50%+ | 高 |
| downloader | 18.1% | 40% | 中 |
| container | 20.6% | 50% | 高 |

---

**报告生成时间**: 2026-03-19 01:45