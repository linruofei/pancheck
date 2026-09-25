#!/usr/bin/env bash

# 进入脚本所在目录
cd "$(dirname "$0")" || exit 1

APP_NAME="PanCheck"
BIN_NAME="pancheck-server"
PID_FILE="pancheck.pid"
AUTO_PID_FILE="pancheck_autoupdate.pid"
VERSION_FILE=".version"

# 自动处理运行目录：如果当前目录下有名为 pancheck 的项目子目录，自动切换进去
if [ -d "./pancheck" ] && ([ -d "./pancheck/.git" ] || [ -f "./pancheck/go.mod" ]); then
    echo "[$APP_NAME] 检测到项目子目录 ./pancheck，自动切换至该目录运行..."
    cd "./pancheck" || exit 1
fi

# 查找可执行程序
find_bin() {
    if [ -f "./$BIN_NAME" ] && [ ! -d "./$BIN_NAME" ]; then
        echo "./$BIN_NAME"
    elif [ -f "./pancheck" ] && [ ! -d "./pancheck" ]; then
        echo "./pancheck"
    elif [ -f "./main" ] && [ ! -d "./main" ]; then
        echo "./main"
    else
        echo ""
    fi
}

# 默认 GitHub 仓库 (优先自动读取当前 Git 远程仓库)
DEFAULT_REPO="linruofei/pancheck"

# 获取仓库所有者和名称 (如 linruofei/pancheck)
get_repo() {
    if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        local remote_url
        remote_url=$(git config --get remote.origin.url 2>/dev/null)
        if [ -n "$remote_url" ]; then
            # 兼容 git@github.com:owner/repo.git 和 https://github.com/owner/repo.git
            echo "$remote_url" | sed -E 's/.*github\.com[:\/]([^\/]+\/[^\/\.]+)(\.git)?/\1/'
            return
        fi
    fi
    echo "$DEFAULT_REPO"
}

REPO=$(get_repo)

# 检测 CPU 架构
detect_arch() {
    local machine
    machine=$(uname -m)
    case "$machine" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            echo "amd64"
            ;;
    esac
}

ARCH=$(detect_arch)
TAR_NAME="pancheck-linux-${ARCH}.tar.gz"

# 优雅下载工具 (带直连与国内加速镜像自动切换)
download_file() {
    local raw_url="$1"
    local output_path="$2"

    # 国内加速镜像列表
    local mirrors=(
        ""
        "https://ghfast.top/"
        "https://ghproxy.net/"
    )

    for prefix in "${mirrors[@]}"; do
        local target_url="${prefix}${raw_url}"
        if command -v curl >/dev/null 2>&1; then
            if curl -sSfL --connect-timeout 8 --retry 2 "$target_url" -o "$output_path" 2>/dev/null; then
                if [ -s "$output_path" ]; then
                    return 0
                fi
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -q --timeout=8 --tries=2 "$target_url" -O "$output_path" 2>/dev/null; then
                if [ -s "$output_path" ]; then
                    return 0
                fi
            fi
        fi
    done

    return 1
}

# 检查服务是否正在运行
is_running() {
    if [ -f "$PID_FILE" ]; then
        local pid
        pid=$(cat "$PID_FILE")
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            return 0
        fi
    fi
    local pgrep_pid
    pgrep_pid=$(pgrep -f "(\./$BIN_NAME|\./pancheck\b|\./main)" 2>/dev/null | head -n 1)
    if [ -n "$pgrep_pid" ]; then
        echo "$pgrep_pid" > "$PID_FILE"
        return 0
    fi
    return 1
}

# 获取远端最新构建版本号
get_remote_version() {
    local ver_url="https://github.com/${REPO}/releases/download/latest/version.txt"
    local tmp_ver="/tmp/pancheck_ver_$$"
    if download_file "$ver_url" "$tmp_ver"; then
        local ver
        ver=$(head -n 1 "$tmp_ver" | tr -d '\r\n[:space:]')
        rm -f "$tmp_ver"
        echo "$ver"
    else
        rm -f "$tmp_ver"
        echo ""
    fi
}

# 检查并自动拉取更新
check_and_update() {
    local silent="${1:-false}"
    
    if [ "$silent" != "true" ]; then
        echo "[$APP_NAME] 正在检查 GitHub 最新版本..."
    fi

    local remote_ver
    remote_ver=$(get_remote_version)

    if [ -z "$remote_ver" ]; then
        if [ "$silent" != "true" ]; then
            echo "[$APP_NAME] 无法连接到 GitHub Releases 获取版本信息，跳过更新检查"
        fi
        return 1
    fi

    local local_ver=""
    if [ -f "$VERSION_FILE" ]; then
        local_ver=$(head -n 1 "$VERSION_FILE" | tr -d '\r\n[:space:]')
    fi

    local current_bin
    current_bin=$(find_bin)

    # 如果二进制文件不存在，或者版本号不一致，则触发拉取
    if [ -z "$current_bin" ] || [ ! -d "./static" ] || [ "$remote_ver" != "$local_ver" ]; then
        echo "[$APP_NAME] 检测到新版本构建: ${remote_ver} (当前本地: ${local_ver:-none})"
        echo "[$APP_NAME] 正在从 GitHub 拉取 ${ARCH} 编译产物包: ${TAR_NAME} ..."

        # 如果在 git 仓库内，同步最新脚本和配置文件
        if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
            git pull --ff-only 2>/dev/null || true
        fi

        local download_url="https://github.com/${REPO}/releases/download/latest/${TAR_NAME}"
        local tmp_tar="/tmp/${TAR_NAME}_$$"

        if download_file "$download_url" "$tmp_tar"; then
            echo "[$APP_NAME] 下载完成，正在解压更新文件..."
            if ! tar -xzf "$tmp_tar" -C .; then
                echo "[$APP_NAME] 解压更新文件失败！"
                rm -f "$tmp_tar"
                return 1
            fi
            rm -f "$tmp_tar"

            current_bin=$(find_bin)
            if [ -n "$current_bin" ]; then
                chmod +x "$current_bin" "./start_pancheck.sh" 2>/dev/null || true
            fi
            echo "$remote_ver" > "$VERSION_FILE"
            echo "[$APP_NAME] 成功更新至新版本: ${remote_ver}！"

            # 如果服务原本在运行，则重启
            if is_running; then
                echo "[$APP_NAME] 服务正在运行中，正在重启载入新版本..."
                restart
            fi
            return 0
        else
            echo "[$APP_NAME] 下载失败，请检查网络或稍后重试！"
            rm -f "$tmp_tar"
            return 1
        fi
    else
        if [ "$silent" != "true" ]; then
            echo "[$APP_NAME] 当前已是最新版本 (${local_ver})"
        fi
        return 0
    fi
}

# 启动服务
start() {
    local target_bin
    target_bin=$(find_bin)

    # 启动前检查并拉取最新版本（如果不存在可执行文件则强制拉取）
    if [ -z "$target_bin" ]; then
        echo "[$APP_NAME] 未检测到可执行程序，尝试从 GitHub 自动拉取最新构建..."
        check_and_update false
        target_bin=$(find_bin)
    else
        # 检查是否有更新，有则拉取并更新
        check_and_update false
        target_bin=$(find_bin)
    fi

    if [ -z "$target_bin" ]; then
        echo "[$APP_NAME] 错误：未找到可执行程序 $BIN_NAME，启动终止！"
        exit 1
    fi

    chmod +x "$target_bin"

    if is_running; then
        echo "[$APP_NAME] 服务已在运行中 (PID: $(cat "$PID_FILE"))"
        return 0
    fi

    echo "[$APP_NAME] 正在启动后台服务..."
    nohup "$target_bin" >/dev/null 2>&1 < /dev/null &
    local pid=$!
    echo "$pid" > "$PID_FILE"
    disown "$pid" 2>/dev/null || true

    sleep 1
    if kill -0 "$pid" 2>/dev/null; then
        echo "[$APP_NAME] 启动成功 (PID: $pid)"
        echo "[$APP_NAME] 请访问: http://localhost:6080"
    else
        echo "[$APP_NAME] 启动失败，进程未正常运行"
        exit 1
    fi
}

# 停止服务
stop() {
    # 如果有自动更新守护进程，一并停止
    stop_autoupdate

    if is_running; then
        local pid
        pid=$(cat "$PID_FILE")
        echo "[$APP_NAME] 正在停止服务 (PID: $pid)..."
        kill "$pid" 2>/dev/null
        for i in 1 2 3 4 5 6 7 8 9 10; do
            if kill -0 "$pid" 2>/dev/null; then
                sleep 0.5
            else
                break
            fi
        done
        if kill -0 "$pid" 2>/dev/null; then
            echo "[$APP_NAME] 优雅退出超时，强制杀死进程..."
            kill -9 "$pid" 2>/dev/null
        fi
        rm -f "$PID_FILE"
        echo "[$APP_NAME] 服务已停止"
    else
        echo "[$APP_NAME] 服务未在运行"
        rm -f "$PID_FILE"
    fi
}

# 重启服务
restart() {
    stop
    sleep 1
    start
}

# 查看运行状态
status() {
    if is_running; then
        local ver="none"
        if [ -f "$VERSION_FILE" ]; then
            ver=$(head -n 1 "$VERSION_FILE" | tr -d '\r\n[:space:]')
        fi
        echo "[$APP_NAME] 状态: 运行中"
        echo "  - 进程 PID: $(cat "$PID_FILE")"
        echo "  - 当前版本: $ver"
        echo "  - 访问地址: http://localhost:6080"
    else
        echo "[$APP_NAME] 状态: 未运行"
    fi

    if [ -f "$AUTO_PID_FILE" ] && kill -0 "$(cat "$AUTO_PID_FILE")" 2>/dev/null; then
        echo "[$APP_NAME] 自动更新守护: 运行中 (PID: $(cat "$AUTO_PID_FILE"))"
    else
        echo "[$APP_NAME] 自动更新守护: 未运行"
    fi
}

# 后台自动更新守护循环
start_autoupdate() {
    local interval="${2:-300}" # 默认每5分钟检查一次
    if [ -f "$AUTO_PID_FILE" ] && kill -0 "$(cat "$AUTO_PID_FILE")" 2>/dev/null; then
        echo "[$APP_NAME] 自动更新守护已经在运行 (PID: $(cat "$AUTO_PID_FILE"))"
        return 0
    fi

    echo "[$APP_NAME] 启动自动更新守护进程 (检查间隔: ${interval}秒)..."
    (
        while true; do
            sleep "$interval"
            check_and_update true >/dev/null 2>&1
        done
    ) >/dev/null 2>&1 < /dev/null &
    local auto_pid=$!
    echo "$auto_pid" > "$AUTO_PID_FILE"
    disown "$auto_pid" 2>/dev/null || true
    echo "[$APP_NAME] 自动更新守护已在后台运行 (PID: $auto_pid)"
}

stop_autoupdate() {
    if [ -f "$AUTO_PID_FILE" ]; then
        local auto_pid
        auto_pid=$(cat "$AUTO_PID_FILE")
        if [ -n "$auto_pid" ] && kill -0 "$auto_pid" 2>/dev/null; then
            kill "$auto_pid" 2>/dev/null
        fi
        rm -f "$AUTO_PID_FILE"
    fi
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
    update)
        check_and_update false
        ;;
    auto-update)
        start_autoupdate "$@"
        ;;
    *)
        echo "用法: $0 {start|stop|restart|status|update|auto-update [间隔秒数]}"
        exit 1
        ;;
esac
