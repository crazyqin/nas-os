# 更新日志

所有重要的更改都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [v2.397.0] - 2026-04-05

### 🎯 六部协同开发第166轮 - 用户指南完善 + CHANGELOG维护 + 竞品对比更新

#### 📚 礼部文档交付
- **用户指南新增**：NVMe健康监控、磁盘电源管理、勒索防护三份完整用户指南
- **CHANGELOG维护**：第166轮更新记录
- **竞品对比更新**：2026Q2报告补充对标信息

#### 📖 新增用户指南
| 文档 | 路径 | 对标竞品 |
|------|------|----------|
| NVMe健康监控 | `docs/user-guide/nvme-health-guide.md` | TrueNAS 25.10 Disk界面 |
| 磁盘电源管理 | `docs/user-guide/disk-power-guide.md` | 飞牛fnOS按需唤醒 |
| 勒索防护教程 | `docs/user-guide/ransomware-protection-guide.md` | TrueNAS 26 Ransomware Defense |

#### 🔍 竞品对比更新要点
- TrueNAS 26 Ransomware Defense 与 nas-os WriteOnce 独家优势对比
- 飞牛 fnOS 按需唤醒安全设计分析（勒索监控盘永不休眠）
- NVMe SMART 监控对标 TrueNAS 25.10/群晖 DSM

#### 🎯 本轮重点
- 用户友好的功能使用教程（面向普通用户）
- 竞品功能差异化优势说明
- API参考与最佳实践整合

#### 🔄 六部协同任务
| 部门 | 状态 | 任务 |
|------|------|------|
| 兵部 | 进行中 | NVMe健康预测+磁盘电源管理 |
| 工部 | 进行中 | CI验证+Docker优化 |
| 刑部 | 进行中 | 安全审计Round166 |
| 户部 | 进行中 | 成本聚合+RAIDZ计算器 |
| 礼部 | ✅完成 | 用户指南+CHANGELOG+竞品对比 |
| 吏部 | 进行中 | 版本管理 |

---

## [v2.396.0] - 2026-04-05

### 🎯 六部协同开发第165轮 - 编译修复 + 竞品对标深化

#### 司礼监报告
- **编译错误修复**：enhanced_mfa_manager.go strings包未使用+maskUserID未定义
- **竞品调研深化**：TrueNAS 26/群晖DSM 7.3/飞牛fnOS功能对标
- **六部协同启动**：NVMe健康预测+磁盘电源管理+勒索防护增强

#### 🔧 修复内容
| 问题 | 修复 | 文件 |
|------|------|------|
| strings包未使用 | 移除import "strings" | `internal/auth/enhanced_mfa_manager.go` |
| maskUserID未定义 | 添加maskUserID辅助函数 | `internal/auth/enhanced_mfa_manager.go` |

#### 🔍 竞品对标发现
| 产品 | 核心特性 | nas-os对标状态 |
|------|----------|---------------|
| **TrueNAS 26 Goldeye** | WebShare+TrueSearch、Ransomware Defense、SMB Spotlight、NVMe-oF、RAIDZ Expansion | ✅ WebShare已实现 / 🎯 勒索防护原型 |
| **群晖 DSM 7.3** | Photos AI、Drive、Office、Hyper Backup、VMM | ✅ Photos+Office已有 / 📋 Drive规划 |
| **飞牛 fnOS** | 按需唤醒硬盘、核显加速AI、FN Connect云管理 | 🚧 按需唤醒本轮实现 |

#### 🎯 本轮重点
- NVMe健康预测完善（三级预警）
- 磁盘智能电源管理（对标飞牛）
- 勒索防护增强（对标TrueNAS）

#### 🔄 六部协同任务
| 部门 | 状态 | 任务 |
|------|------|------|
| 兵部 | 启动 | NVMe健康预测+磁盘电源管理 |
| 工部 | 启动 | CI验证+Docker优化+armv7排查 |
| 刑部 | 启动 | 安全审计Round165 |
| 户部 | 启动 | 成本聚合+RAIDZ计算器 |
| 礼部 | 启动 | 文档更新+CHANGELOG |
| 吏部 | ✅完成 | VERSION+ROADMAP更新 |

---

## [v2.395.0] - 2026-04-04

### 🎯 六部协同开发第164轮 - TrueNAS 26竞品对标 + 安全修复 + 六部协同

#### 司礼监报告
- TrueNAS 26 Goldeye竞品深度调研
- 安全审计修复：整数解析错误检查
- 六部协同任务执行（兵部、工部、刑部、礼部、户部）
- Actions状态：CI/CD失败（编译错误待修复）

#### 🔍 竞品对标发现（TrueNAS 26 Goldeye）
| 功能 | 说明 | 对标计划 |
|------|------|----------|
| **WebShare + TrueSearch** | 浏览器文件访问+文件名/内容/类型搜索 | ✅ 已实现 |
| **Ransomware Defense** | 勒索软件实时防御+honeypot+行为分析+自动响应 | 🎯 原型开发 |
| **SMB Spotlight Search** | macOS Spotlight搜索SMB共享文件内容 | 📋 规划中 |
| **SMB Stateful Failover** | SMB会话状态HA故障转移 | 📋 规划中 |
| **LXC容器HA** | 容器故障转移支持Enterprise HA | ✅ 已有容器管理 |
| **OpenZFS 2.4** | hybrid pool+物理块重写+动态gang header | ✅ ZFS支持 |
| **Linux Kernel 6.18 LTS** | 新硬件支持+长期安全更新 | 📋 验证中 |

#### 🔒 安全修复
| 问题 | 修复 | 文件 |
|------|------|------|
| 整数解析忽略错误 | 添加错误检查+安全默认值(limit≤100, offset≤10000) | `internal/docker/app_handlers.go` |

#### 🔍 其他竞品摘要
| 产品 | 新特性 |
|------|------|
| **Synology DSM** | Photos AI、Drive、Office、Chat、MailPlus、Active Insight、Hyper Backup、VMM |
| **TerraMaster TOS6** | 文件管理、集中备份、CloudSync、TRAID、Terra Photos、AI NAS |

#### 🔄 六部协同成果
| 部门 | 状态 | 输出 |
|------|------|------|
| 兵部 | ✅完成 | WebShare/TrueSearch/Ransomware Defense/SMB Spotlight调研 |
| 工部 | 运行中 | CI/CD状态报告 |
| 刑部 | ✅完成 | 安全审计修复 |
| 礼部 | 运行中 | CHANGELOG更新 |
| 户部 | 运行中 | 资源统计 |

---

## [v2.394.0] - 2026-04-04

### 🛠️ 六部协同开发第163轮 - Actions修复 + 竞品学习 + 六部协同

#### 司礼监报告
- 修复v2.393.0 Actions编译失败（NVMe-oF文件位置错误、类型重复定义）
- 竞品学习深化：群晖DSM、TrueNAS 25.10、绿联NAS
- 项目资源统计：1192源文件 / 353测试文件 / 491K+行代码
- 六部协同任务执行（兵部、户部、礼部、工部、刑部）
- CI/CD恢复运行中，Security Scan/Compatibility Check成功

#### 🔧 修复内容
| 问题 | 修复 | 说明 |
|------|------|------|
| NVMe-oF包冲突 | `internal/storage/nvme-of.go` → `internal/storage/nvmeof/` | Go不允许同目录不同包名 |
| 类型重复定义 | 删除`nvme-of.go`，保留`manager.go` | TransportTCP/TransportRDMA已在manager.go定义 |
| Spotlight未使用变量 | 删除`parseDateRange`中未使用的`now`变量 | go build警告修复 |

#### 📊 竞品学习摘要
| 产品 | 特点 | 对标计划 |
|------|------|----------|
| **群晖DSM** | Photos AI、Office协同、Drive同步、Active Insight监控 | P1对标 |
| **TrueNAS 25.10** | NVMe-oF、RAIDZ Expansion、ZFS快照、LXC容器、多系统管理 | P0对标 |
| **绿联NAS** | AI相册、云影院、远程访问、应用中心 | P1对标 |

#### 📈 项目资源统计
- 源文件：1192个（非测试）
- 测试文件：353个
- 代码行数：491,363行
- 依赖数量：约175个（go.mod）

#### 🔄 六部协同成果
| 部门 | 状态 | 输出 |
|------|------|------|
| 兵部 | 运行中 | go vet检查、编译验证 |
| 户部 | ✅完成 | 资源统计报告 |
| 礼部 | 超时 | CHANGELOG准备中 |
| 工部 | 超时 | CI/CD状态报告 |
| 刑部 | 超时 | 安全审计启动 |
| 吏部 | 待执行 | 版本管理 |

---

## [v2.393.0] - 2026-04-04

### 🎯 六部协同开发第162轮 - 司礼监轮值！TrueNAS 25.10竞品对标 + NVMe-oF设计

#### 司礼监报告

[1674 more lines in file. Use offset=101 to continue.]