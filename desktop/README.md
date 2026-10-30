# Signal Web Desktop

Windows x64 桌面壳，安全加载生产 HTTPS 客户端，通过 Electron `setOverlayIcon` 在 Windows 任务栏显示未读数字，并把端到端加密的消息历史持久保存到 Windows 用户数据目录。

```bash
npm install
npm run package:win
```

产物位于 `dist/SignalWeb-win32-x64/`。发布压缩包为 `SignalWeb-Windows-x64-v1.2.1.zip`，解压后运行 `SignalWeb.exe`，无需安装。

启动会显示连接界面，失败自动重试一次并清理网页资源缓存；不会清理 IndexedDB、localStorage、登录 Cookie 或本地历史。重试失败时可再次连接或打开诊断日志目录。诊断日志仅记录启动状态和错误代码。
