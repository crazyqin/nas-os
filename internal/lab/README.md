# Lab（源码温室 · 留在本仓库）

`internal/lab` 是实验 / 未收编代码树，**不进生产 `nasd` 链接**（Core / Full 均禁止 `internal/web` import lab）。

## 与主模块的边界

本目录有**独立** `go.mod`（`module nas-os/internal/lab`），因此：

| 命令 | 是否包含 lab |
|------|----------------|
| 仓库根 `go test ./...` / `go list ./...` | **否**（嵌套模块被跳过） |
| `make build` / `make build-full` | **否** |
| `cd internal/lab && go test ./...` 或 `make test-lab` | **是** |

父模块依赖通过：

```text
require nas-os v0.0.0
replace nas-os => ../..
```

## 说明

Lab **保留在 nas-os 仓库内**，方便单仓浏览与演进；嵌套 `go.mod` 只是为了：

- 缩小根模块的 `go test` / IDE 默认索引面  
- 避免 lab 依赖污染主 `go.mod` 解析路径  

**不要**把 lab 包当成默认交付能力；毕业路径是 `extensions/` 或 recommended product，并经治理测试。
