# 聊天记录显示修复（2026-10-30）

代码目录：`/Users/user/Documents/imadf/go-socket-test`。
线上域名：`https://msg.trip-vn.com/`。
Nginx：`/etc/nginx/sites-enabled/signal-web`，转发 `127.0.0.1:1802`。
运行目录：`/opt/signal-web`，服务 `signal-web.service`。

## 修改

- 普通会话先恢复本地加密历史和已信任成员公钥，再静默拉取服务端历史。
- 同一会话重新打开、离线消息同步时保留已有节点和图片；按消息 ID 合并并去重，保留同步期间收到的实时消息。
- 前台会话切换使用内存缓存；页面隐藏时仍释放解密显示状态和图片 URL。本地持久层只保存消息密文与元数据。
- 使用请求版本防止过期解密结果覆盖当前会话；撤回和清空使对应缓存失效。
- 客服端和游客浮窗新增本地密文缓存（最近 300 条），客服端显示日期分隔与时分，游客消息显示月日与时分。
- 游客关闭再打开浮窗时保留现有节点；后台返回时从密文缓存恢复。
- Service Worker shell 版本更新至 v28。

## 验证与部署

- `node --test frontend/tests/*.test.mjs`：10 项通过，覆盖节点复用、并发消息、请求过期、当前会话不清空以及缓存不包含解密正文。
- `node --check frontend/client/public/assets/support-widget.js`：通过。
- `npm --prefix frontend run build:client`：通过。
- 发布目录：`/opt/signal-web/releases/chat-cache-20261030`。
- 更新前备份：`/opt/signal-web/backups/client-before-chat-cache-20261030.tgz`。
- 保留旧版哈希资源，先更新 assets 与 sw.js，再原子替换 index.html。未修改数据库或重启服务。
- 线上首页引用 `index-DkWn-Ccj.js`；HTTPS 健康接口返回正常，服务为 active。线上 JS、客服浮窗与 sw.js 的 SHA-256 与本地一致。

本次未使用真实用户账号进行人工聊天验收。已打开的页面需要刷新或重开客户端以加载新版本。清除站点或应用数据会移除本机缓存；服务端历史的解密仍依赖设备密钥。

## 首次打开底部定位补充修复

修正缓存恢复误判已定位、空记录过早完成定位两个分支；有消息渲染后才完成首次底部定位，图片加载后继续跟随底部，主动上滑后停止跟随。13 项测试与生产构建通过。
发布目录 `/opt/signal-web/releases/chat-scroll-20261030`，备份 `/opt/signal-web/backups/client-before-chat-scroll-20261030.tgz`。线上首页引用 `index-CUBuHo8L.js`，SHA-256 与本地一致，健康接口正常，服务 active。Service Worker 更新至 v29。

## 移除新消息弱提示

移除普通聊天、客服消息的 Toast 以及离线消息接收成功提示，保留未读计数和系统通知。13 项测试和构建通过。发布目录 `/opt/signal-web/releases/no-message-toast-20261030`，备份 `/opt/signal-web/backups/client-before-no-message-toast-20261030.tgz`。Service Worker v30，新入口资源 `index-CEeU1ntn.js`。

## 群聊提及与会话头部

群聊输入 @ 后按昵称过滤成员，可点击或用方向键及 Enter/Tab 选择。点击群名称打开可搜索的成员列表，也可从列表提及成员。被提及的消息展示 @我 标记；成员 ID 和名字随消息正文加密，旧客户端仍可显示正文。
浏览器模式去除会话返回箭头、名称靠左；手机浏览器保留列表菜单入口，桌面客户端和安装模式保留返回导航。
17 项测试与生产构建通过。发布目录 `/opt/signal-web/releases/group-mentions-20261030`，备份 `/opt/signal-web/backups/client-before-group-mentions-20261030.tgz`。Service Worker v31，线上入口 `index-DJ_8zkQD.js` 与本地 SHA-256 一致；健康接口正常、服务 active。
