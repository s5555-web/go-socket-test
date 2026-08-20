# Signal Web

基于 Go、MySQL 与 WebSocket 的 Signal 风格网页版即时通讯 MVP。

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

客户端默认监听 `:802`，管理后台默认监听 `:801`。启动前请创建 `signal_web` 数据库并修改 `configs/config.yaml` 中的数据库 DSN 和认证密钥。

首次管理员可先注册普通账号，再执行：

```sql
UPDATE users SET is_admin=1 WHERE username='你的用户名';
```

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
| POST | `/api/register` | 注册 |
| POST | `/api/login` | 登录 |
| GET | `/api/conversations` | 会话列表 |
| POST | `/api/conversations/:id/messages` | 发送消息 |
| GET | `/ws?token=...` | 实时消息连接 |

> 当前版本提供 TLS 部署条件下的传输加密能力，但尚未实现 Signal Protocol 端到端加密，不应把它用于高敏感通讯。

## Socket 稳定性

- **断开与清理**：客户端断开时 `ReadPump` 会退出并调用 `Unregister`，从 Hub 移除并关闭 `Send` 通道，避免 goroutine 与通道泄漏。
- **心跳**：服务端按配置周期发送 Ping，客户端未在规定时间内回 Pong 则读超时并断开。
- **Panic 恢复**：Hub 主循环、每个连接的 ReadPump/WritePump 均带 `recover`，单连接异常不会拖垮整个服务。
- **优雅退出**：收到 SIGINT/SIGTERM 时先关闭 HTTP 监听、再停止 Hub，避免关闭已关闭的 channel。

## 扩展

- 新 HTTP 接口：在 `internal/api/router.go` 的 `routes()` 里挂路由，或新增 `internal/api/xxx/handler.go`。
- 新业务（如聊天室）：在 `internal/` 下增加 `service/`、`repository/` 等包，在 `cmd/server/main.go` 中注入并挂到路由。
- 新配置：在 `internal/config/config.go` 和 `configs/config.yaml` 中增加字段。
