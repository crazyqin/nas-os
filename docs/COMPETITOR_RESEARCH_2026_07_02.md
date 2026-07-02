# NAS 竞品调研与功能落地计划（2026-07-02）

## 调研来源
- TrueNAS 26 Beta / 文档：年度版本节奏、WebShare、TrueSearch、OpenZFS 2.4 混合池、LXC、SMB Stateful HA、FEC、重启原因记录、WebShell 审计。
- Synology DSM 7.4 / Computex 2026：本地 AI、DSM Agent、Cluster Manager、Mass Deployment、ActiveProtect 2.0、Log Center 可观测性。
- fnOS：易用媒体中心、海报墙、转码、P2P 远程访问、DIY x86 友好。

## 本轮落地功能
| 方向 | 对标 | nas-os 落地 |
|---|---|---|
| 网络可靠性 | TrueNAS 26 FEC | `internal/network` 新增 FEC 模式推荐、配置意图、审计意图 |
| 运维可观测 | TrueNAS 重启原因 / DSM Log Center | `internal/logcenter` 新增重启历史和原因分类 |
| 本地 AI 数据治理 | Synology DSM AI | `internal/semanticsearch` 新增 local-only governed search 与脱敏返回 |
| 容器工作负载迁移 | TrueNAS LXC / DSM Cluster | `internal/lxc` 新增可审计迁移计划与回滚步骤 |

## 六部任务分工
- 兵部：网络 FEC 设计与测试，提升 25G/100G 链路稳定性。
- 户部：迁移计划预估耗时/成本，避免无计划停机。
- 礼部：整理竞品调研，确保发布描述突出用户收益。
- 工部：重启历史、日志中心、Actions 状态优先核查。
- 吏部：LXC 迁移流程标准化，可审批可回滚。
- 刑部：本地 AI 查询治理，默认 local-only，可选脱敏与审计。
