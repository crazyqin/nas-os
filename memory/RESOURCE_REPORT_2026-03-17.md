# NAS-OS 项目资源统计报告
**户部报告 | 日期: 2026-03-17**

---

## 一、代码规模统计

### 总体规模
| 指标 | 数量 |
|------|------|
| Go 源文件 | 701 个 |
| Go 代码行数 | **394,715 行** |
| Go 测试文件 | 247 个 |
| 测试代码行数 | 116,598 行 |
| 总文件数 | 1,353 个 |
| 项目总大小 | 212 MB |

### 前端资源 (webui)
| 指标 | 数量 |
|------|------|
| HTML/JS/CSS 行数 | 49,853 行 |

---

## 二、各模块代码规模 (Top 20)

| 模块 | 代码行数 | 占比 |
|------|----------|------|
| internal/reports | 35,589 | 9.0% |
| internal/security | 24,142 | 6.1% |
| internal/quota | 19,004 | 4.8% |
| internal/backup | 13,269 | 3.4% |
| internal/monitor | 12,642 | 3.2% |
| internal/auth | 10,514 | 2.7% |
| internal/billing | 10,191 | 2.6% |
| internal/cluster | 9,627 | 2.4% |
| internal/project | 9,594 | 2.4% |
| internal/media | 9,022 | 2.3% |
| internal/storage | 8,949 | 2.3% |
| internal/audit | 8,842 | 2.2% |
| internal/docker | 8,293 | 2.1% |
| internal/photos | 7,455 | 1.9% |
| internal/tiering | 5,611 | 1.4% |
| internal/cloudsync | 5,606 | 1.4% |
| internal/snapshot | 5,533 | 1.4% |
| internal/plugin | 5,529 | 1.4% |
| internal/automation | 5,327 | 1.3% |
| internal/container | 5,010 | 1.3% |

**Top 20 模块合计**: 206,953 行 (52.4%)

### 公共包 (pkg)
| 模块 | 代码行数 |
|------|----------|
| pkg/btrfs | 1,595 |
| pkg/safeguards | 791 |
| pkg/security | 307 |

---

## 三、测试覆盖率分布

### 总体覆盖率
**35.8%** (statements)

### 覆盖率分布
| 覆盖率范围 | 函数数量 |
|-----------|----------|
| 0% (未测试) | 4,950 |
| 1-49% | 306 |
| 50-69% | 500 |
| 70-89% | 1,126 |
| 90-100% | 2,330 |

### 测试覆盖率 Top 模块
| 模块 | 测试文件数 | 测试代码行数 |
|------|-----------|-------------|
| internal/security | 12 | 6,190 |
| internal/reports | 12 | 5,949 |
| internal/quota | 9 | 5,256 |
| internal/storage | 6 | 4,282 |
| internal/backup | 8 | 3,775 |
| internal/auth | 9 | 3,158 |
| internal/media | 7 | 3,110 |
| internal/nfs | 5 | 3,103 |
| internal/billing | 5 | 3,102 |
| internal/smb | 5 | 3,072 |

---

## 四、资源使用情况

### 编译产物
| 文件 | 大小 |
|------|------|
| nasd (主程序) | 73 MB |
| nasctl (CLI工具) | 6.2 MB |

### 待提交变更
- README.md (已修改)
- VERSION (已修改)
- docker-compose.yml (已修改)
- internal/version/version.go (已修改)

---

## 五、项目结构概览

```
nas-os/
├── api/           # API 处理器
├── cmd/           # 入口点 (1,984 行)
├── config/        # 配置
├── docs/          # 文档
├── internal/      # 核心业务逻辑 (68 个模块)
├── pkg/           # 公共库
├── webui/         # 前端界面
├── tests/         # 端到端测试
├── scripts/       # 构建脚本
├── monitoring/    # 监控配置
└── charts/        # Helm charts
```

---

## 六、总结

**项目规模**: 中大型项目，约 40 万行 Go 代码，68 个功能模块

**质量指标**:
- 测试覆盖率 35.8%，有提升空间
- 4950 个函数完全未测试 (需重点关注)
- 测试代码占比约 30% (116,598 / 394,715)

**建议**:
1. 优先提升核心模块 的测试覆盖率
2. 关注 0% 覆盖的函数，可能存在风险点
3. 内部模块占比高，架构合理

---

*户部统计完毕*