#!/usr/bin/env bash

# 进入脚本所在目录
cd "$(dirname "$0")" || exit 1

APP_NAME="PanCheck"
BIN_NAME="pancheck"
FALLBACK_BIN="./main"
PID_FILE="pancheck.pid"

# 查找可执行程序
find_bin() {
    if [ -f "./$BIN_NAME" ]; then
        echo "./$BIN_NAME"
    elif [ -f "$FALLBACK_BIN" ]; then
        echo "$FALLBACK_BIN"
    else
        echo ""
    fi
}

# 检查是否在运行
is_running() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
            return 0
        fi
    fi
    # 辅助兜底检查进程名
    PID=$(pgrep -f "(\./$BIN_NAME|\./main)" | head -n 1)
    if [ -n "$PID" ]; then
        echo "$PID" > "$PID_FILE"
        return 0
    fi
    return 1
}

# 启动服务
start() {
    if is_running; then
        echo "[$APP_NAME] 服务已在运行中 (PID: $(cat "$PID_FILE"))"
        return 0
    fi

    TARGET_BIN=$(find_bin)
    if [ -z "$TARGET_BIN" ]; then
        echo "[$APP_NAME] 未找到二进制文件 ($BIN_NAME 或 $FALLBACK_BIN)"
        if command -v go >/dev/null 2>&1; then
            echo "[$APP_NAME] 检测到 Go 环境，正在编译 ./cmd/api -> $BIN_NAME ..."
            go build -o "$BIN_NAME" ./cmd/api
            if [ $? -ne 0 ]; then
                echo "[$APP_NAME] 编译失败，请先手动编译或排查错误！"
                exit 1
            fi
            TARGET_BIN="./$BIN_NAME"
            chmod +x "$TARGET_BIN"
        else
            echo "[$APP_NAME] 未检测到 Go 编译器，请先编译生成 $BIN_NAME！"
            exit 1
        fi
    fi

    # 确保可执行权限
    chmod +x "$TARGET_BIN"

    # 启动后台进程并彻底脱离终端（不保留日志）
    nohup "$TARGET_BIN" >/dev/null 2>&1 < /dev/null &
    PID=$!
    echo "$PID" > "$PID_FILE"
    disown "$PID" 2>/dev/null || true

    sleep 1
    if kill -0 "$PID" 2>/dev/null; then
        echo "[$APP_NAME] 启动成功 (PID: $PID)"
    else
        echo "[$APP_NAME] 启动失败，进程未能正常常驻"
        exit 1
    fi
}

# 停止服务
stop() {
    if is_running; then
        PID=$(cat "$PID_FILE")
        echo "[$APP_NAME] 正在停止服务 (PID: $PID)..."
        kill "$PID" 2>/dev/null
        for i in 1 2 3 4 5 6 7 8 9 10; do
            if kill -0 "$PID" 2>/dev/null; then
                sleep 0.5
            else
                break
            fi
        done
        if kill -0 "$PID" 2>/dev/null; then
            echo "[$APP_NAME] 优雅退出超时，尝试强制停止..."
            kill -9 "$PID" 2>/dev/null
        fi
        rm -f "$PID_FILE"
        echo "[$APP_NAME] 服务已停止"
    else
        echo "[$APP_NAME] 服务未在运行"
        rm -f "$PID_FILE"
    fi
}

# 服务状态
status() {
    if is_running; then
        echo "[$APP_NAME] 运行中 (PID: $(cat "$PID_FILE"))"
    else
        echo "[$APP_NAME] 未运行"
    fi
}

# 重启服务
restart() {
    stop
    sleep 1
    start
}

case "${1:-start}" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    *)
        echo "用法: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac
