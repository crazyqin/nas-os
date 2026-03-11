# 报表系统 (Reports)

企业级报表生成和管理模块，支持自定义报表、定时调度和多种导出格式。

## 功能特性

### 1. 自定义报表生成
- 基于模板快速生成报表
- 支持自定义字段、过滤、排序和聚合
- 支持多数据源查询
- 支持参数化报表（运行时传参）

### 2. 定时报表
- 支持 hourly/daily/weekly/monthly 预设频率
- 支持自定义 cron 表达式
- 支持邮件和 Webhook 通知
- 支持报表保留策略（自动清理旧文件）
- 执行历史记录和状态追踪

### 3. 导出格式
- **JSON** - 数据交换格式，适合程序处理
- **CSV** - 逗号分隔值，适合表格软件导入
- **HTML** - 网页格式，支持浏览器查看和打印
- **PDF** - 便携文档格式（需要 wkhtmltopdf）
- **Excel** - 电子表格格式（需要 excelize 库）

### 4. 报表模板
- 内置默认模板：配额报表、存储报表、用户报表、系统报表
- 支持自定义模板创建和管理
- 模板字段类型支持：string/number/percent/bytes/date/datetime/duration/boolean/list
- 模板支持过滤器、排序、聚合和分组

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Report Service                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Template   │  │   Generator  │  │   Schedule   │       │
│  │   Manager    │  │              │  │   Manager    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│         │                 │                  │               │
│         └────────┬────────┴──────────────────┘               │
│                  │                                           │
│         ┌────────▼────────┐                                  │
│         │    Exporter     │                                  │
│         └────────┬────────┘                                  │
│                  │                                           │
│    ┌─────────────┼─────────────┐                             │
│    │             │             │                             │
│    ▼             ▼             ▼                             │
│  JSON    CSV/Excel/HTML    PDF                               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 使用示例

### 1. 从模板生成报表

```go
// 初始化服务
service, _ := reports.NewService(reports.Config{
    DataDir: "/var/lib/nas-os/reports",
})

// 获取模板
template := service.TemplateManager.GetDefaultTemplate(reports.TemplateTypeQuota)

// 生成报表
report, _ := service.Generator.GenerateFromTemplate(template.ID, map[string]interface{}{
    "volume_name": "data",
}, nil)

// 导出为 HTML
result, _ := service.Exporter.Export(report, reports.ExportHTML, "", reports.ExportOptions{
    Title: "配额使用报表",
})
```

### 2. 创建定时报表

```go
// 创建定时报表：每天 9:00 生成配额报表并发送邮件
schedule, _ := service.ScheduleManager.CreateSchedule(reports.ScheduledReportInput{
    Name:         "每日配额报表",
    TemplateID:   "quota-template-id",
    Frequency:    reports.FrequencyDaily,
    ExportFormat: reports.ExportPDF,
    NotifyEmail:  []string{"admin@example.com"},
    Retention:    30, // 保留 30 天
    Enabled:      true,
}, "admin")

// 启动调度器
service.Start()
```

### 3. 创建自定义报表

```go
// 创建自定义报表
customReport, _ := service.Generator.CreateCustomReport(reports.CustomReportInput{
    Name:       "用户存储使用详情",
    DataSource: "quota",
    Fields: []reports.TemplateField{
        {Name: "username", Label: "用户名", Type: reports.FieldTypeString, Source: "quota.target_name"},
        {Name: "used", Label: "已使用", Type: reports.FieldTypeBytes, Source: "usage.used_bytes"},
        {Name: "percent", Label: "使用率", Type: reports.FieldTypePercent, Source: "usage.usage_percent"},
    },
    Sorts: []reports.TemplateSort{
        {Field: "percent", Order: "desc"},
    },
    Limit: 50,
}, "admin")

// 生成报表
report, _ := service.Generator.GenerateFromCustomReport(customReport.ID, nil, nil)
```

### 4. 注册数据源

```go
// 注册配额数据源
quotaSource := reports.NewQuotaDataSource(quotaManager)
service.RegisterDataSource(quotaSource)

// 注册存储数据源
storageSource := reports.NewStorageDataSource(storageManager)
service.RegisterDataSource(storageSource)
```

## API 端点

### 模板管理
- `GET /api/report-templates` - 列出模板
- `POST /api/report-templates` - 创建模板
- `GET /api/report-templates/:id` - 获取模板
- `PUT /api/report-templates/:id` - 更新模板
- `DELETE /api/report-templates/:id` - 删除模板
- `POST /api/report-templates/:id/generate` - 从模板生成报表

### 自定义报表
- `GET /api/custom-reports` - 列出自定义报表
- `POST /api/custom-reports` - 创建自定义报表
- `GET /api/custom-reports/:id` - 获取自定义报表
- `PUT /api/custom-reports/:id` - 更新自定义报表
- `DELETE /api/custom-reports/:id` - 删除自定义报表
- `POST /api/custom-reports/:id/generate` - 生成报表
- `POST /api/custom-reports/:id/preview` - 预览报表

### 定时报表
- `GET /api/scheduled-reports` - 列出定时任务
- `POST /api/scheduled-reports` - 创建定时任务
- `GET /api/scheduled-reports/:id` - 获取定时任务
- `PUT /api/scheduled-reports/:id` - 更新定时任务
- `DELETE /api/scheduled-reports/:id` - 删除定时任务
- `POST /api/scheduled-reports/:id/enable` - 启用任务
- `POST /api/scheduled-reports/:id/disable` - 禁用任务
- `POST /api/scheduled-reports/:id/run` - 立即执行
- `GET /api/scheduled-reports/:id/executions` - 获取执行记录

### 导出
- `POST /api/export` - 导出报表
- `POST /api/export/batch` - 批量导出多种格式
- `GET /api/export/formats` - 获取支持的格式

### 数据源
- `GET /api/data-sources` - 列出数据源
- `GET /api/data-sources/:name/fields` - 获取数据源可用字段

### 快速报表
- `POST /api/quick-reports/generate` - 快速生成报表

## 数据存储

```
/var/lib/nas-os/reports/
├── templates/           # 模板定义
│   ├── xxx.json
│   └── ...
├── custom/              # 自定义报表定义
│   ├── custom_xxx.json
│   └── ...
├── schedules/           # 定时任务定义
│   ├── schedule_xxx.json
│   └── ...
└── outputs/             # 导出文件
    └── schedule_xxx/    # 按任务ID分组
        ├── report_20260311_090000.pdf
        └── ...
```

## 依赖

- Go 1.26+
- github.com/robfig/cron/v3 - 定时任务调度
- github.com/google/uuid - UUID 生成
- github.com/gin-gonic/gin - HTTP 框架

可选依赖：
- wkhtmltopdf - PDF 导出
- github.com/xuri/excelize/v2 - Excel 导出