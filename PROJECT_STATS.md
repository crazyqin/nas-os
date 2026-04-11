# NAS-OS 项目统计

**统计日期:** 2026-04-11

## 代码规模

| 指标 | 数量 |
|------|------|
| Go 源文件 | 1,238 |
| Go 代码行数 | 689,996 |
| 前端文件 (ts/tsx/vue) | 22 |
| 项目总文件数 | 2,549 |

## 模块分布 (internal/)

| 模块 | 文件数 |
|------|--------|
| security | 69 |
| storage | 67 |
| reports | 57 |
| ai | 49 |
| auth | 34 |
| backup | 35 |
| quota | 30 |
| audit | 29 |
| media | 27 |
| network | 26 |
| photos | 25 |
| cluster | 23 |
| monitor | 24 |
| tunnel | 22 |
| search | 21 |
| cloudsync | 20 |
| container | 18 |
| docker | 18 |
| cost | 18 |
| project | 18 |
| snapshot | 16 |
| plugin | 15 |
| files | 15 |
| disk | 14 |
| smb | 14 |
| ai_classify | 7 |
| apps | 7 |
| budget | 7 |
| iscsi | 7 |
| perf | 7 |
| team | 7 |
| compress | 8 |
| connect | 10 |
| directplay | 9 |
| ftp | 8 |
| gpu | 9 |
| ldap | 9 |
| nfs | 8 |
| notification | 8 |
| replication | 8 |
| scheduler | 9 |
| sftp | 9 |
| webdav | 10 |
| api | 13 |
| dashboard | 13 |
| vm | 13 |
| web | 12 |
| billing | 12 |
| automation | 11 |
| performance | 10 |
| ha | 6 |
| dedup | 6 |
| websocket | 6 |
| cms | 5 |
| health | 5 |
| natpierce | 5 |
| office | 5 |
| versioning | 5 |
| cache | 4 |
| cloudfuse | 4 |
| concurrency | 4 |
| hardware | 4 |
| lock | 4 |
| notify | 4 |
| s3 | 4 |
| service | 4 |
| session | 4 |
| shares | 4 |
| trash | 4 |
| usbmount | 4 |
| appstore | 1 |
| cloud | 1 |
| face | 1 |
| fileactivity | 1 |
| fleet | 1 |
| haconfig | 1 |
| database | 2 |
| nat_tunnel | 2 |
| ransomware | 2 |
| system | 2 |
| version | 2 |
| users | 3 |
| album | 3 |
| compliance | 3 |
| logging | 3 |
| optimizer | 3 |
| prediction | 3 |
| tags | 3 |
| alerting | 2 |
| monitoring | 3 |
| nvmeof | 1 |
| services | 1 |
| zfs | 1 |

**总计:** 102 个子模块

## API 接口

- API 接口函数数量: **418**

## 依赖管理

- Go 版本: 1.26.1
- 直接依赖: 41 个
- 总依赖（含间接）: 269 个
- 安全更新: 无明显安全警告

## 项目结构

```
nas-os/
├── api/           # API 接口定义
├── internal/      # 核心业务逻辑 (102 模块)
├── web/           # 前端代码 (22 文件)
├── cmd/           # 命令行入口
└── go.mod         # 依赖管理
```

## 主要技术栈

- 后端: Go (Gin, WebSocket, OpenTelemetry)
- 前端: TypeScript/Vue
- 存储: SQLite, Redis, S3
- 监控: Prometheus, OpenTelemetry
- 文档: Swagger