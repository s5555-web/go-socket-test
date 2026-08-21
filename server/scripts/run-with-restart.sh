#!/bin/sh
# 服务异常退出时自动重启（适合开发或简单部署）
cd "$(dirname "$0")/.."
while true; do
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] starting server..."
  go run ./cmd/server "$@"
  code=$?
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] server exited with code $code"
  [ "$code" = "0" ] && break
  sleep 2
done
