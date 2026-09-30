# Signal Web

基于 Go、MySQL 与 WebSocket 的 Signal 风格网页版即时通讯 MVP。

## 技术栈

- **HTTP**: [Gin](https://github.com/gin-gonic/gin)
- **WebSocket**: [gorilla/websocket](https://github.com/gorilla/websocket)
- **配置**: [Viper](https://github.com/spf13/viper)

## 项目结构

```
sket/
├── server/              # 完整 Go 后端（go.mod、cmd、internal、configs、scripts）
├── web/client/          # Web C 端（独立源码）
├── admin/               # Web 管理后台（独立源码）
├── desktop/             # Windows 端（独立源码）
├── mobile/android/      # Android 端（独立源码）
├── mobile/ios/          # iOS 安装端（独立源码）
├── deploy/              # Nginx 与 systemd 部署配置
└── go.work              # 根目录 Go 工作区入口
```

## 运行

```bash
cd server
go mod tidy
go run ./cmd/server
```

服务默认仅监听本机 `127.0.0.1:1802`（客户端）和 `127.0.0.1:1801`（管理后台），由 Nginx 统一通过 HTTPS/WSS 反向代理。复制 `server/configs/config.example.yaml` 为未跟踪的 `server/configs/config.yaml`，并在服务器的受限环境文件中设置数据库 DSN 和认证密钥：

```bash
SIGNAL_DATABASE_DSN='应用专用数据库用户:强随机密码@tcp(127.0.0.1:3306)/signal_web?charset=utf8mb4&parseTime=true&loc=Local'
SIGNAL_AUTH_SECRET='至少32字节的密码学随机值'
```

生产凭据、TLS 私钥、环境文件和真实服务器 IP 不得提交到 Git。服务也会拒绝空数据库 DSN、短于 32 字节或示例形式的认证密钥。

首次管理员可先注册普通账号，再执行：

```sql
UPDATE users SET is_admin=1 WHERE username='你的用户名';
```

**异常自动重启**：若希望进程崩溃后自动重启，可用脚本或进程管理器：

```bash
chmod +x server/scripts/run-with-restart.sh
./server/scripts/run-with-restart.sh
```

或使用 systemd / Docker 的 restart 策略（服务非 0 退出时会自动重启）。

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| POST | `/api/register` | 注册 |
| POST | `/api/login` | 登录 |
| GET | `/api/conversations` | 会话列表 |
| GET | `/api/friends` | 好友列表 |
| GET | `/api/friend-requests` | 收到的好友申请 |
| POST | `/api/friend-requests` | 发送好友申请 |
| PUT | `/api/friend-requests/:id/accept` | 接受好友申请 |
| DELETE | `/api/friends/:id` | 拒绝申请或删除好友 |
| POST | `/api/conversations/:id/messages` | 发送消息 |
| POST | `/api/conversations/:id/attachments` | 上传设备端加密后的图片密文 |
| GET | `/api/conversations/:id/attachments/:attachment` | 下载会话内图片密文 |
| POST | `/api/ws-ticket` | 使用登录态签发 30 秒、单次使用的实时连接凭证 |
| GET | `/ws?ticket=...` | 使用单次凭证建立实时消息连接 |

## 本地消息历史

- 在线消息通过实时连接投递；离线收件人的端到端加密密文会进入最多保留 30 天的临时队列，客户端成功写入本地并确认后立即从队列删除。
- “清除双方记录”会为离线会话成员保存清除指令；对方设备下次连接服务器时会先清空该会话的浏览器、Windows 本地历史，再确认并移除服务器指令，之后才接收清除操作之后产生的新消息。
- 每台设备按登录账号把收到的端到端加密消息保存在浏览器 IndexedDB 中；打开会话时先加载最近 40 条，上滑继续分页读取更早的本地记录。
- 本地记录不会跨浏览器或设备自动同步。设备上线时会补收尚未确认的离线密文；清除浏览器网站数据也会删除该设备上的历史。
- 加密图片文件仍由服务端保存，方便已收到消息的设备按权限下载；消息正文及其历史索引不在服务端持久化。

## 网页客服插件

客服插件是独立于原有私聊/群聊的业务通道：原有聊天仍使用端到端加密和设备本地历史；游客客服消息单独保存在服务端，以便游客刷新页面、客服换班或稍后上线后继续处理。

1. 管理员登录 `https://msg.trip-vn.com:801`，在“用户与客服账号”中把现有账号设为客服。
2. 在“客服插件站点”中新建站点并填写允许嵌入插件的完整来源，例如 `https://www.example.com`。
3. 复制后台生成的脚本到目标网站页面。客服账号重新登录现有 Chat 后会看到“客服”入口。

```html
<script src="https://msg.trip-vn.com/assets/support-widget.js"
        data-site-key="后台生成的站点密钥"
        data-title="在线客服"
        async></script>
```

可选参数 `data-position="left"` 可把悬浮按钮放到左下角；默认位于右下角。目标网站若配置了严格 CSP，需要在 `script-src` 和 `connect-src` 中允许 `https://msg.trip-vn.com`。

## 加密设计

- 浏览器使用 WebCrypto 生成 P-256 ECDH 身份密钥，并使用 NIST FIPS 203 的 ML-KEM-768 生成后量子 KEM 身份密钥。
- 当会话全部成员已完成升级时，新消息使用 P-256 ECDH 与 ML-KEM-768 的共享秘密共同经过 HKDF-SHA-256 派生 AES-256-GCM 密钥；任一算法仍安全时，消息内容仍保持机密。尚未全部升级的会话继续生成 v1 信封，历史 v1 消息保持可读。
- 私钥保存在本机 IndexedDB；跨设备恢复副本以用户密码经 PBKDF2-SHA-256 派生的 AES-256-GCM 密钥加密后再存到服务端。服务端无法直接读取私钥。
- 明文在加密前加入密码学随机填充，并扩展到固定的 512B、1KB、2KB、4KB 等长度桶，降低依据密文长度推测消息内容的风险。TLS/WSS 仍会暴露连接目标、时间和总流量，填充不能提供匿名性。
- WebSocket 只接受同源连接，并使用 30 秒内有效、消费后立即失效的随机连接凭证；长期登录令牌不会进入 WebSocket URL 或代理日志。
- 服务端严格校验 v1/v2 算法标识、AES-GCM 参数、ML-KEM-768 密文长度、成员覆盖和收件人匹配，拒绝明文或畸形信封。
- 图片在设备端使用独立的随机 AES-256-GCM 密钥加密，服务端只保存密文；图片密钥、类型、名称和大小随消息封装分别端到端加密给每位成员。
- 图片上传和下载均校验登录身份与会话成员关系，单图上限为 8MB；未绑定消息的上传可安全撤销。
- 会话界面显示包含经典公钥和后量子公钥的安全码；首次使用固定联系人密钥，后续密钥改变时阻止静默继续并要求用户确认。

生产环境必须使用 HTTPS/WSS，否则浏览器不会启用密钥模块。`web/crypto-build/` 固定并锁定浏览器密码组件版本，构建产物为 `web/client/assets/post-quantum.js`。

安全边界：当前 v2 能提高“现在收集、未来解密”攻击下的新消息机密性，但它不是 Signal Protocol、PQXDH、Double Ratchet 或 SPQR/Triple Ratchet 的实现，尚不具备逐消息前向保密和入侵后恢复能力，也未经独立密码学审计。浏览器 JavaScript 运行环境不能提供原生密钥隔离或严格常数时间保证。服务器仍可观察账号、会话成员、发送时间和密文大小等元数据；历史 v1 密文不会因为升级而自动获得后量子保护。

## 生产域名与手机推送

- 客户端：`https://msg.trip-vn.com`
- 管理后台：`https://msg.trip-vn.com:801`
- Windows 客户端：`https://msg.trip-vn.com/downloads/SignalWeb-Windows-x64-v1.2.0.zip`
- Android APK 使用 Trusted Web Activity，包名为 `com.tripvn.msg`。
- iOS 16.4 及以上可通过主屏幕 Web App 接收标准 Web Push；iOS 安装配置位于 `mobile/ios/SignalWeb-iOS.mobileconfig`。
- 服务端使用 VAPID Web Push。推送载荷只包含发送者、会话编号和“端到端加密消息”提示，不包含消息正文。
- 正式原生 `.ipa` 仍需要 Apple Developer Team、签名证书、描述文件和 APNs 凭据。

## Socket 稳定性

- **断开与清理**：客户端断开时 `ReadPump` 会退出并调用 `Unregister`，从 Hub 移除并关闭 `Send` 通道，避免 goroutine 与通道泄漏。
- **心跳**：服务端按配置周期发送 Ping，客户端未在规定时间内回 Pong 则读超时并断开。
- **Panic 恢复**：Hub 主循环、每个连接的 ReadPump/WritePump 均带 `recover`，单连接异常不会拖垮整个服务。
- **优雅退出**：收到 SIGINT/SIGTERM 时先关闭 HTTP 监听、再停止 Hub，避免关闭已关闭的 channel。

## 扩展

- 新 HTTP 接口：在 `server/internal/api/router.go` 挂路由，或在 `server/internal/api/` 增加处理器。
- 新业务（如聊天室）：在 `server/internal/` 下增加 `service/`、`repository/` 等包，在 `server/cmd/server/main.go` 中注入。
- 新配置：在 `server/internal/config/config.go` 和 `server/configs/config.yaml` 中增加字段。
