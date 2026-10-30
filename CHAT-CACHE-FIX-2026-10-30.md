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

## 所有模式移除对话框返回按钮

按后续要求，普通聊天与客服对话框均移除头部返回按钮以及手机浏览器列表按钮，不再区分浏览器、安装模式和桌面客户端。前端构建通过，Service Worker v32。发布目录 `/opt/signal-web/releases/remove-chat-back-20261030`，更新前备份 `/opt/signal-web/backups/client-before-remove-chat-back-20261030.tgz`。

## Windows 启动白屏恢复（v1.2.1）

用户反馈启动后直接空白。线上浏览器登录界面正常，检查已发布 v1.2.0 的 app.asar，确认与仓库一致，旧桌面壳只调用 loadURL，没有启动失败恢复或错误界面。尚未获得用户 Windows 日志，不能断言其具体失败原因。

v1.2.1 增加本地启动界面、25 秒超时、空 Vue 根节点检测、网络失败或渲染进程退出后的单次缓存恢复重试；失败后展示重试与诊断日志入口。仅清理 HTTP、Service Worker 和 Cache Storage 缓存，不清理 Cookie、IndexedDB、localStorage 或本地加密历史。诊断日志不记录消息、凭证或完整 URL。

23 项自动化测试通过，前端构建通过。使用原 v1.2.0 Windows 包替换 app.asar 生成 v1.2.1，逐项核对其他文件 CRC 不变并检查 ZIP 完整性；未在真实 Windows 环境启动验收。
下载 `/downloads/SignalWeb-Windows-x64-v1.2.1.zip`，SHA-256 `4b75487d9239194ae3626c51449f2fc52a64b7999985d6ae4e2f3763f8622737`。服务器 HTTPS 下载探测返回 206 application/zip。
前端发布 `/opt/signal-web/releases/windows-startup-20261030`，备份 `/opt/signal-web/backups/client-before-windows-startup-20261030.tgz`，首页引用 `index-BbqbO8yx.js`，健康检查正常。旧桌面壳需下载新版程序以获得启动恢复功能。
