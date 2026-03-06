# sket

Go 服务端，为前端提供 HTTP API 与 WebSocket。当前以 Socket 服务为主，后续可扩展聊天室等。

## 技术栈

- **HTTP**: [Gin](https://github.com/gin-gonic/gin)
- **WebSocket**: [gorilla/websocket](https://github.com/gorilla/websocket)
- **配置**: [Viper](https://github.com/spf13/viper)

## 项目结构

```
sket/
├── cmd/server/          # 入口
├── configs/             # 配置文件
├── internal/
│   ├── api/             # 路由与 WebSocket
│   │   └── socket/      # Hub、Client、Handler
│   └── config/          # 配置加载
└── go.mod
```

## 运行

```bash
go mod tidy
go run ./cmd/server
```

默认监听 `:8080`。

**异常自动重启**：若希望进程崩溃后自动重启，可用脚本或进程管理器：

```bash
chmod +x scripts/run-with-restart.sh
./scripts/run-with-restart.sh
```

或使用 systemd / Docker 的 restart 策略（服务非 0 退出时会自动重启）。

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/ws/socket` | WebSocket 连接 |

## Socket 稳定性

- **断开与清理**：客户端断开时 `ReadPump` 会退出并调用 `Unregister`，从 Hub 移除并关闭 `Send` 通道，避免 goroutine 与通道泄漏。
- **心跳**：服务端按配置周期发送 Ping，客户端未在规定时间内回 Pong 则读超时并断开。
- **Panic 恢复**：Hub 主循环、每个连接的 ReadPump/WritePump 均带 `recover`，单连接异常不会拖垮整个服务。
- **优雅退出**：收到 SIGINT/SIGTERM 时先关闭 HTTP 监听、再停止 Hub，避免关闭已关闭的 channel。

## 扩展

- 新 HTTP 接口：在 `internal/api/router.go` 的 `routes()` 里挂路由，或新增 `internal/api/xxx/handler.go`。
- 新业务（如聊天室）：在 `internal/` 下增加 `service/`、`repository/` 等包，在 `cmd/server/main.go` 中注入并挂到路由。
- 新配置：在 `internal/config/config.go` 和 `configs/config.yaml` 中增加字段。
