# 竞品调研报告 2026-06-29

## TrueNAS 26 (2026年4月 BETA)

### 核心更新
- **发布节奏**: 改为年度发布（之前半年一次），版本号简化为 "26.1" 格式
- **OpenZFS 2.4**: 混合池改进（flash+HDD）、物理块重写、动态gang header
- **Linux Kernel 6.18 LTS**: 新硬件支持
- **WebShare**: 浏览器文件分享，Dropbox式体验，FIPS 140加密传输，支持SMB/AD/NFSv4互操作
- **TrueSearch (Spotlight)**: 亚秒级搜索，支持10亿文件规模，SSD索引，macOS Spotlight集成
- **LXC容器**: 完全支持，低开销Linux工作负载部署
- **有状态SMB HA故障转移**: SMB会话状态跨控制器故障转移保持
- **SMB Spotlight搜索**: macOS客户端可直接Spotlight搜索SMB共享
- **勒索软件检测与防护**
- **400GbE网络支持**: V-Series硬件
- **API现代化**: JSON-RPC 2.0 WebSocket + SCRAM-SHA-512认证

### 对标差距
| 功能 | nas-os状态 | TrueNAS 26 |
|------|-----------|------------|
| WebShare | ✅ 已有WebShare Pro | ✅ WebShare + TrueSearch集成 |
| 全文搜索 | ⚠️ 基础搜索 | ✅ TrueSearch亚秒级/10亿文件 |
| LXC容器 | ✅ 已有 | ✅ 完全支持 |
| SMB HA有状态故障转移 | ⚠️ 基础HA | ✅ 有状态故障转移 |
| OpenZFS 2.4混合池 | ⚠️ 基础分层 | ✅ 混合池改进 |
| 勒索检测 | ✅ 已有ML检测 | ✅ 勒索检测防护 |
| API现代化 | ⚠️ REST API | ✅ WebSocket + SCRAM认证 |

## 群晖 DSM 7.3

### 核心更新
- **灵活数据分层**: 冷热数据自动分层，自定义规则（访问频率/时间）
- **共享标签**: 团队协作文件标签
- **文件请求**: 通过链接安全收集文件
- **文件锁定**: 防止协作同步错误
- **邮件审核**: 管理员审查敏感邮件
- **私有云AI**: 串接AI模型（云/本地LLM），MailPlus/Office AI辅助
- **AI去识别化**: Synology AI Console本地端遮蔽个人信息
- **弹性存储加密**: 密码解锁加密存储空间
- **灵活域控管**: 仅同步选定OU，最小权限原则

### 对标差距
| 功能 | nas-os状态 | DSM 7.3 |
|------|-----------|----------|
| 数据分层 | ✅ 已有Fusion Pool | ✅ 自动冷热分层+自定义规则 |
| 文件请求 | ❌ 无 | ✅ 链接收集文件 |
| 共享标签 | ⚠️ 基础 | ✅ 团队协作标签 |
| AI集成 | ✅ 已有AI模块 | ✅ 云+本地LLM，Office/Mail集成 |
| AI隐私脱敏 | ✅ 已有privacyproxy | ✅ AI Console去识别化 |
| 存储加密 | ✅ 已有 | ✅ 密码解锁弹性加密 |
| 邮件审核 | ❌ 无 | ✅ 敏感邮件审查 |

## 飞牛 fnOS v1.2.0012 (2026-06-25)

### 核心更新
- **Windows ACL权限**: 13种权限+"允许/拒绝"组合，父级继承至指定子级
- **预览加密PDF**: 预览时输入密码解锁
- **大文件读写优化**: 性能提升
- **ZFS快照目录适配**
- **RAID1/10写入性能提升**
- **RAID5/6初始化方式可选**
- **SMB日志优化**: 减少nobody无效探测日志
- **Docker稳定性**: 修复unless-stopped自启动、关机卡住问题
- **Gmail/Outlook邮件通知**: OAuth授权机制
- **CPU温度修复**

### 对标差距
| 功能 | nas-os状态 | fnOS v1.2 |
|------|-----------|-----------|
| Windows ACL | ✅ 已有19种权限 | ✅ 13种权限+继承 |
| PDF预览解锁 | ❌ 无 | ✅ 预览加密PDF |
| RAID性能优化 | ⚠️ 基础 | ✅ RAID1/10写入优化 |
| Docker稳定性 | ✅ 已有容器守护 | ✅ 修复多个Docker问题 |
| 邮件通知OAuth | ⚠️ 基础SMTP | ✅ Gmail/Outlook OAuth |

## 下一步建议（优先级排序）

### P0 - 快速跟进
1. **全文搜索引擎增强** - 对标TrueSearch，提升搜索性能到亚秒级
2. **文件请求功能** - 对标DSM，通过链接安全收集文件
3. **预览加密PDF** - 对标fnOS，预览时解锁

### P1 - 中期规划
4. **SMB有状态HA故障转移** - 对标TrueNAS，会话状态跨故障转移保持
5. **数据分层自定义规则** - 对标DSM，按访问频率自动分层
6. **邮件通知OAuth** - 对标fnOS，支持Gmail/Outlook OAuth
7. **API现代化** - 对标TrueNAS，WebSocket + SCRAM认证

### P2 - 长期规划
8. **混合池增强** - 对标OpenZFS 2.4，flash+HDD混合池改进
9. **存储弹性加密** - 对标DSM，密码解锁加密存储
10. **邮件审核机制** - 对标DSM，敏感邮件审查
