# Vendor 说明

本地化第三方前端库：NAS 常部署于局域网/离线环境，页面不依赖任何 CDN。

| 文件 | 包 | 版本 | 许可证 |
|---|---|---|---|
| chart.umd.min.js | chart.js | 4.5.1 | MIT |
| chartjs-adapter-date-fns.bundle.min.js | chartjs-adapter-date-fns（bundle 含 date-fns） | 3.0.0 | MIT |
| echarts.min.js | echarts | 5.4.3 | Apache-2.0 |
| swagger-ui.css / swagger-ui-bundle.js / swagger-ui-standalone-preset.js | swagger-ui-dist | 5.10.5 | Apache-2.0 |

升级：从 npm registry（或镜像）下载对应包 tgz，取 dist 文件替换，同步更新本表与 LICENSE-*.txt / NOTICE-*.txt。
来源：registry.npmmirror.com（2026-09-08 拉取）。
