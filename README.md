## 项目介绍

`minfo` 是一个本地媒体信息检测 Web 工具，主要功能：
- 输出 MediaInfo 信息
- 输出 BDInfo 信息
- 使用 guyuan 截图脚本

![minfo 截图](docs/images/screenshot.png)

## 部署方式

直接使用已发布镜像 `ghcr.io/mirrorb/minfo:latest`。

示例 `docker-compose.yml`：

```yaml
services:
  minfo:
    image: ghcr.io/mirrorb/minfo:latest
    container_name: minfo
    privileged: true
    ports:
      - "28080:28080"
    environment:
      PORT: "28080"
      WEB_USERNAME: "admin"
      WEB_PASSWORD: "passpass" # 请修改默认用户名密码
      REQUEST_TIMEOUT: "20m"
    volumes:
      - /lib/modules:/lib/modules:ro # 用于挂载ISO
      - /your/media/path:/media:ro # 媒体文件目录
    restart: unless-stopped
```

启动：

```bash
docker compose up -d
```

## 媒体路径配置说明

**重要**：minfo 使用 `/media` 作为媒体根目录，请确保 Docker 挂载路径为 `/media`。

### 媒体路径确定方式

minfo 的源代码中，媒体根目录通过以下方式确定：

1. **默认路径**：`/media`（定义在 `internal/config/config.go`）
2. **自动检测**：自动检测容器内的顶层挂载点

### 正确的挂载方式

```bash
docker run -d \
    --name minfo \
    --restart unless-stopped \
    --privileged \
    --network host \
    -v /lib/modules:/lib/modules:ro \
    -v /path/to/your/media:/media:ro \
    -e TZ=Asia/Shanghai \
    -e PORT="28080" \
    -e WEB_USERNAME="admin" \
    -e WEB_PASSWORD="admin123" \
    -e REQUEST_TIMEOUT="20m" \
    ghcr.io/mirrorb/minfo:latest
```

### 多个媒体目录

如果需要挂载多个媒体目录，可以创建一个父目录，将所有媒体目录放在其中：

```bash
# 目录结构
/media/
├── movies/
├── tv/
└── downloads/

# 挂载方式
-v /path/to/media:/media:ro
```

### 常见问题

**问题**：Web 界面显示"读取路径失败"

**原因**：挂载路径不是 `/media`，minfo 无法识别

**解决**：将挂载路径改为 `/media`

**问题**：容器启动但无法访问媒体文件

**原因**：挂载路径错误或目录权限问题

**解决**：
1. 确认挂载路径为 `/media`
2. 检查宿主机目录权限：`ls -la /path/to/media`
3. 确保容器有读取权限（使用 `:ro` 只读挂载）
