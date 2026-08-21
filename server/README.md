# Go server

该目录包含完整的共享 Go 后端：

- `cmd/server/`：客户端 API、WebSocket 与管理 API 的启动入口；
- `cmd/vapid/`：Web Push VAPID 工具；
- `internal/`：API、鉴权、配置、数据库与实时连接实现；
- `configs/`：本地配置模板；
- `scripts/`：开发运行与异常重启脚本；
- `go.mod`、`go.sum`：独立 Go 模块依赖。

从仓库根目录执行 `go test ./server/...`，或进入本目录执行 `go test ./...`。
