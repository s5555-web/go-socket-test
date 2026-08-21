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
| POST | `/api/conversations/:id/attachments` | 上传设备端加密后的图片密文 |
| GET | `/api/conversations/:id/attachments/:attachment` | 下载会话内图片密文 |
| GET | `/ws?token=...` | 实时消息连接 |

## 加密设计

- 浏览器使用 WebCrypto 生成 P-256 ECDH 身份密钥；私钥以不可导出的 `CryptoKey` 保存在本机 IndexedDB，服务端只收到公钥。
- 每条消息通过 ECDH + HKDF-SHA-256 派生会话密钥，再以 AES-256-GCM 分别加密给每位成员。
- 明文加入随机填充并按区间扩展，降低依据密文长度推测消息内容的风险。
- 服务端拒绝明文消息、缺少成员密文或收件人不匹配的消息封装。
- 图片在设备端使用独立的随机 AES-256-GCM 密钥加密，服务端只保存密文；图片密钥、类型、名称和大小随消息封装分别端到端加密给每位成员。
- 图片上传和下载均校验登录身份与会话成员关系，单图上限为 8MB；未绑定消息的上传可安全撤销。
- 会话界面显示公钥安全码，成员可通过其他可信渠道比对。

生产环境必须使用 HTTPS/WSS，否则浏览器不会启用密钥模块。该实现尚不是完整 Signal Protocol：没有 Double Ratchet、一次性预密钥、离线密钥恢复或多设备同步，也未经独立密码学审计。服务器仍可观察账号、会话成员、发送时间和密文大小等元数据。

## 生产域名与手机推送

- 客户端：`https://msg.trip-vn.com`
- 管理后台：`https://msg.trip-vn.com:801`
- Android APK 使用 Trusted Web Activity，包名为 `com.tripvn.msg`。
- iOS 16.4 及以上可通过主屏幕 Web App 接收标准 Web Push；iOS 安装配置位于 `mobile/SignalWeb-iOS.mobileconfig`。
- 服务端使用 VAPID Web Push。推送载荷只包含发送者、会话编号和“端到端加密消息”提示，不包含消息正文。
- 正式原生 `.ipa` 仍需要 Apple Developer Team、签名证书、描述文件和 APNs 凭据。

## Socket 稳定性

- **断开与清理**：客户端断开时 `ReadPump` 会退出并调用 `Unregister`，从 Hub 移除并关闭 `Send` 通道，避免 goroutine 与通道泄漏。
- **心跳**：服务端按配置周期发送 Ping，客户端未在规定时间内回 Pong 则读超时并断开。
- **Panic 恢复**：Hub 主循环、每个连接的 ReadPump/WritePump 均带 `recover`，单连接异常不会拖垮整个服务。
- **优雅退出**：收到 SIGINT/SIGTERM 时先关闭 HTTP 监听、再停止 Hub，避免关闭已关闭的 channel。

## 扩展

- 新 HTTP 接口：在 `internal/api/router.go` 的 `routes()` 里挂路由，或新增 `internal/api/xxx/handler.go`。
- 新业务（如聊天室）：在 `internal/` 下增加 `service/`、`repository/` 等包，在 `cmd/server/main.go` 中注入并挂到路由。
- 新配置：在 `internal/config/config.go` 和 `configs/config.yaml` 中增加字段。
