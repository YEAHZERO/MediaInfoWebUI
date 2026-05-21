#!/bin/bash

# MediaInfoWebUI 自动部署脚本
# 自动识别环境并选择正确的媒体路径

set -e

# 颜色输出函数
info() {
    echo -e "\033[1;34m[INFO]\033[0m $1"
}

success() {
    echo -e "\033[1;32m[SUCCESS]\033[0m $1"
}

warning() {
    echo -e "\033[1;33m[WARNING]\033[0m $1"
}

error() {
    echo -e "\033[1;31m[ERROR]\033[0m $1"
}

# 检测环境检测函数
detect_environment() {
    echo "检测环境..."

    # 检测是否在 WSL 环境
    if grep -qi microsoft /proc/version 2>/dev/null; then
        IS_WSL=1
        info "检测到 WSL 环境"
    else
        IS_WSL=0
        info "检测到 Linux 服务器环境"
    fi

    # 查找存在的媒体路径
    local found_path=""
    found_path=$(smart_path_finder)

    # 如果没有找到，尝试自动创建
    if [ -z "$found_path" ]; then
        warning "未找到媒体路径，尝试自动创建..."
        
        if [ $IS_WSL -eq 1 ]; then
            found_path="/home/liveup/qbittorrent/downloads"
        else
            found_path="/home/live/qbittorrent/downloads"
        fi
        
        mkdir -p "$found_path"
        success "已创建媒体路径: $found_path"
    fi

    MEDIA_PATH="$found_path"
}

# 智能路径查找器
smart_path_finder() {
    # 第一优先级：查找特定用户路径
    local priority_paths=(
        "/home/liveup/qbittorrent/downloads"
        "/home/live/qbittorrent/downloads"
        "$HOME/qbittorrent/downloads"
    )
    
    for path in "${priority_paths[@]}"; do
        if [ -d "$path" ]; then
            echo "$path"
            return 0
        fi
    done
    
    # 第二优先级：智能查找 - 查找有 docker 或 qb 文件夹的上级目录下的 qbittorrent/downloads
    local parent_dirs=(
        "/home"
        "/"
        "$HOME"
        "/data"
        "/mnt"
    )
    
    for parent_dir in "${parent_dirs[@]}"; do
        if [ ! -d "$parent_dir" ]; then
            continue
        fi
        
        # 查找该目录下的用户文件夹
        local user_dirs=($(find "$parent_dir" -maxdepth 1 -type d 2>/dev/null | grep -E "^$parent_dir/[a-zA-Z0-9._-]+$" | head -20))
        
        for user_dir in "${user_dirs[@]}"; do
            # 检查是否有 docker 或 qb 相关文件夹
            local has_docker=false
            local has_qb=false
            
            if [ -d "$user_dir/docker" ] || [ -d "$user_dir/.docker" ]; then
                has_docker=true
            fi
            
            if [ -d "$user_dir/qbittorrent" ] || [ -d "$user_dir/qb" ]; then
                has_qb=true
            fi
            
            # 如果有相关文件夹，尝试查找 downloads
            if [ "$has_docker" = true ] || [ "$has_qb" = true ]; then
                local check_paths=(
                    "$user_dir/qbittorrent/downloads"
                    "$user_dir/qb/downloads"
                    "$user_dir/downloads"
                )
                
                for check_path in "${check_paths[@]}"; do
                    if [ -d "$check_path" ]; then
                        echo "$check_path"
                        return 0
                    fi
                done
            fi
        done
    done
    
    # 第三优先级：常见路径
    local fallback_paths=(
        "/data/qbittorrent/downloads"
        "/media/downloads"
        "/media"
    )
    
    for path in "${fallback_paths[@]}"; do
        if [ -d "$path" ]; then
            echo "$path"
            return 0
        fi
    done
    
    echo ""
    return 1
}

# 部署函数
deploy() {
    local image_tag="${1:-native}"
    local container_name="${2:-mediainfo}"
    local port="${3:-28888}"

    info "开始部署 MediaInfoWebUI"
    info "镜像: ghcr.io/yeahzero/mediainfowebui:$image_tag"
    info "容器名称: $container_name"
    info "端口: $port"
    info "媒体路径: $MEDIA_PATH"
    echo ""

    # 停止并删除旧容器
    info "清理旧容器..."
    docker rm -f "$container_name" 2>/dev/null || true

    # 根据环境选择网络模式
    if [ $IS_WSL -eq 1 ]; then
        info "使用端口映射模式 (WSL 环境)"
        NETWORK_ARGS="-p ${port}:28888"
    else
        info "使用 host 网络模式 (服务器环境)"
        NETWORK_ARGS="--network host"
    fi

    # 启动容器
    echo ""
    info "启动容器..."
    docker run -d \
        --name "$container_name" \
        --privileged \
        $NETWORK_ARGS \
        -e TZ=Asia/Shanghai \
        -e PORT="$port" \
        -e REQUEST_TIMEOUT=30m \
        -v /lib/modules:/lib/modules:ro \
        -v "$MEDIA_PATH:/media:ro" \
        --restart unless-stopped \
        "ghcr.io/yeahzero/mediainfowebui:$image_tag"

    sleep 2

    # 检查状态
    if docker ps --format '{{.Names}}' | grep -q "^$container_name$" >/dev/null; then
        success "部署成功！"
        echo ""
        echo "========================================"
        if [ $IS_WSL -eq 1 ]; then
            echo "WSL 本地访问: http://localhost:$port"
        else
            echo "服务器访问: http://$(hostname -I | awk '{print $1}'):$port"
        fi
        echo "媒体路径: $MEDIA_PATH"
        echo "========================================"
    else
        error "部署失败，请检查日志"
        docker logs "$container_name"
        exit 1
    fi
}

# 帮助信息
show_help() {
    cat << 'EOF'
MediaInfoWebUI 自动部署脚本

使用方法:
  ./deploy.sh [镜像标签] [容器名称] [端口]

参数:
  镜像标签: native | latest | light (默认 native)
  容器名称: mediainfo (默认)
  端口: 28888 (默认)

示例:
  ./deploy.sh
  ./deploy.sh native
  ./deploy.sh light mediainfo 28888

EOF
}

# 主函数
main() {
    if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
        show_help
        exit 0
    fi

    detect_environment
    deploy "$1" "$2" "$3"
}

main "$@"
