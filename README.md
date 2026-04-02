## 项目介绍

`minfo` 是一个本地媒体信息检测 Web 工具，主要功能：

- 输出 MediaInfo 信息
- 输出 BDInfo 信息
- 使用 guyuan 截图脚本
- 支持图床链接生成

![minfo 截图](docs/images/screenshot.png)

## 本项目基于 [minfo](https://github.com/mirrorb/minfo) 进行了多项改进和优化：

### 功能增强

#### 1. 截图功能

- **字幕模式控制**：支持"挂载字幕"和"纯净截图"两种模式
- **预生成下载**：截图 ZIP 先生成并返回下载链接，支持浏览器原生下载
- **结构化日志**：返回脚本执行的详细日志，方便排查问题
- **移除 fast 变体**：简化为 PNG 和 JPG 两种模式

#### 2. BDInfo 优化

- **输出精简**：支持"精简报告"（提取 [code] 块）和"完整报告"两种模式
- **工作目录修复**：在源文件所在目录执行 BDInfo，解决相对路径问题

#### 3. BDInfo 高级功能 ✨ 新增

- **智能 Playlist 选择**：自动推荐时长 > 10 分钟的主片 Playlist
- **多种扫描模式**：支持自动选择、手动选择、整盘扫描三种模式
- **历史任务管理**：任务列表可重新查看，支持查看历史报告
- **实时进度推送**：通过 WebSocket 实时显示扫描进度和 ETA

### 前端体验改进

- **输出面板分离**：MediaInfo/BDInfo 文本输出和图床链接分别显示
- **图床链接管理**：支持链接预览、去重、删除、复制 BBCode
- **状态持久化**：使用 localStorage 保存用户配置，刷新页面不丢失
- **通知提示**：操作结果和错误通过右上角 toast 提示
- **响应式设计**：适配不同屏幕尺寸
- **BDInfo 面板** ✨ 新增：集成 Playlist 选择、任务进度、历史记录的统一面板

### 后端稳定性

- **ffprobe 增强**：双重 fallback（format → stream）和多行解析，支持更多格式
- **文件上传安全**：文件名清理和临时目录隔离，防止路径遍历攻击
- **脚本本地化**：截图脚本纳入版本控制，构建不再依赖外部网络
- **CJK 字体支持**：内置中文字体，确保字幕正确渲染
- **WebSocket 支持** ✨ 新增：实时推送任务状态和进度

### 部署与配置

- **多路径挂载**：支持挂载多个独立的媒体目录（/media_path1, /media_path2 等）
- **远程部署**：新增 run-remote-release.sh 脚本，一键部署到远程服务器
- **端口调整**：默认端口从 28080 改为 38080，避免冲突
- **构建代理**：支持配置 HTTP/HTTPS 代理用于 Docker 构建
- **网络优化** ✨ 新增：Docker 构建使用 `--network=host` 解决网络问题

## 部署方式

### 使用已发布镜像

🎉 **镜像已推送到 GitHub Container Registry！**

| 镜像 | 地址 | 大小 |
|------|------|------|
| v1.0.0 | `ghcr.io/yeahzero/mediainfowebui:v1.0.0` | 321MB (压缩后 98MB) |
| latest | `ghcr.io/yeahzero/mediainfowebui:latest` | 321MB (压缩后 98MB) |

### 快速部署

```bash
docker pull ghcr.io/yeahzero/mediainfowebui:latest

docker run -d \
  --name minfo \
  --privileged \
  -p 28080:28080 \
  -e WEB_USERNAME=admin \
  -e WEB_PASSWORD=your_password \
  -v /lib/modules:/lib/modules:ro \
  -v /your/media/path:/media_path1:ro \
  ghcr.io/yeahzero/mediainfowebui:latest
```

### 使用 docker-compose（推荐）

```yaml
services:
  minfo:
    image: ghcr.io/yeahzero/mediainfowebui:latest
    container_name: minfo
    privileged: true
    ports:
      - "28080:28080"
    environment:
      PORT: "28080"
      WEB_USERNAME: "admin"
      WEB_PASSWORD: "your_password"
      REQUEST_TIMEOUT: "20m"
    volumes:
      - /lib/modules:/lib/modules:ro
      - /path/to/your/media1:/media_path1:ro
      - /path/to/your/media2:/media_path2:ro
    restart: unless-stopped
```

启动：
```bash
docker compose up -d
```

## 本地构建

```bash
# 快速构建（使用 host 网络）
make docker-build

# 运行
make docker-run

# 推送
make docker-push

# 清理旧镜像
make docker-clean
```

## API 端点

### 基础 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/mediainfo` | POST | MediaInfo 信息 |
| `/api/bdinfo` | POST | BDInfo 信息 |
| `/api/screenshots` | POST | 截图生成 |
| `/api/path` | GET | 路径浏览 |

### BDInfo 任务 API ✨ 新增

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/bdinfo/playlists` | POST | 获取 Playlist 列表和推荐 |
| `/api/bdinfo/jobs` | GET | 获取历史任务列表 |
| `/api/bdinfo/job/create` | POST | 创建扫描任务 |
| `/api/bdinfo/job` | GET | 获取任务详情 |
| `/api/bdinfo/report` | GET | 获取扫描报告 |
| `/api/bdinfo/ws` | GET | WebSocket 实时进度 |

## 技术架构

### 整体系统架构

```mermaid
graph TB
    subgraph "前端 Vue 3"
        UI[Web UI]
        PB[PathBrowser]
        AB[ActionButtons]
        BP[BDInfoPanel]
        OP[OutputPanel]
        IL[ImageLinksPanel]
    end
    
    subgraph "API 层"
        API[HTTP API]
        WS[WebSocket]
    end
    
    subgraph "后端 Go"
        MI[MediaInfo]
        BI[BDInfo]
        SS[ScreenshotService]
        JM[JobManager]
        WH[WebSocketHub]
    end
    
    subgraph "外部工具"
        MED[mediainfo CLI]
        BDI[bdinfo CLI]
        FFM[ffmpeg]
        SCP[截图脚本]
    end
    
    subgraph "存储"
        TMP[临时目录]
        REP[报告文件]
    end
    
    UI --> PB
    UI --> AB
    UI --> BP
    UI --> OP
    UI --> IL
    
    PB --> API
    AB --> API
    BP --> API
    BP --> WS
    OP --> API
    IL --> API
    
    API --> MI
    API --> BI
    API --> SS
    API --> JM
    WS --> WH
    
    MI --> MED
    BI --> BDI
    SS --> FFM
    SS --> SCP
    JM --> BDI
    
    WH --> BP
    JM --> WH
    JM --> REP
    SS --> TMP
```

### 核心模块架构

```mermaid
graph TD
    subgraph "前端层 Vue 3"
        subgraph "路径浏览模块"
            PB[PathBrowser.vue]
            UB[usePathBrowser.js]
        end
        subgraph "BDInfo 任务模块"
            BP[BDInfoPanel.vue]
            BJ[useBDInfoJobs.js]
        end
        subgraph "输出模块"
            OP[OutputPanel.vue]
            IL[ImageLinksPanel.vue]
        end
    end
    
    subgraph "API 层"
        subgraph "路径 API"
            PSH[PathSuggestHandler]
        end
        subgraph "BDInfo 任务 API"
            BJH[BDInfoCreateJobHandler]
            BJL[BDInfoListJobsHandler]
            WSH[BDInfoWebSocketHandler]
        end
    end
    
    subgraph "后端服务层"
        subgraph "媒体路径检测"
            MR[MediaRoots]
            DMR[detectMountedRoots]
            CFG["config.DefaultRoot=/media"]
        end
        subgraph "BDInfo 任务队列"
            JM[JobManager]
            SC[Scanner]
            WH[WebSocketHub]
        end
    end
    
    subgraph "外部工具"
        BDI[bdinfo CLI]
        PROC[/proc/self/mountinfo]
    end
    
    subgraph "存储"
        REP[报告文件]
        MEM[内存任务列表]
    end
    
    PB --> UB
    UB --> PSH
    PSH --> MR
    MR --> DMR
    DMR --> PROC
    CFG --> MR
    
    BP --> BJ
    BJ --> BJH
    BJ --> WSH
    BJH --> JM
    WSH --> WH
    JM --> SC
    SC --> BDI
    SC --> WH
    WH --> BJ
    JM --> MEM
    SC --> REP
    
    style PB fill:#f3e5f5,color:#7b1fa2
    style BP fill:#f3e5f5,color:#7b1fa2
    style PSH fill:#bbdefb,color:#0d47a1
    style BJH fill:#bbdefb,color:#0d47a1
    style WSH fill:#bbdefb,color:#0d47a1
    style MR fill:#c8e6c9,color:#1a5e20
    style JM fill:#c8e6c9,color:#1a5e20
    style SC fill:#c8e6c9,color:#1a5e20
    style WH fill:#c8e6c9,color:#1a5e20
    style CFG fill:#fff3e0,color:#e65100
```

**业务流程**：

| 模块 | 流程 |
|------|------|
| **路径检测** | `DefaultRoot=/media` → `MediaRoots()` → 读取 `/proc/self/mountinfo` → 返回可用路径 |
| **BDInfo 任务** | 前端创建任务 → `JobManager` 入队 → `Scanner` 执行 BDInfo CLI → `WebSocketHub` 广播进度 |
| **实时通信** | 前端连接 WebSocket → 接收任务更新/进度 → 显示实时状态 |

### WebSocket 实时通信架构 ✨ 新增

```mermaid
sequenceDiagram
    participant F as 前端
    participant W as WebSocket Handler
    participant H as WebSocketHub
    participant S as Scanner
    
    F->>W: 连接 /api/bdinfo/ws
    W->>H: Register(connection)
    H->>F: 发送现有任务列表
    
    loop 扫描过程
        S->>H: BroadcastJobUpdate(job)
        H->>F: {"type":"job_update","data":{...}}
        S->>H: BroadcastProgress(jobID, 45%, 120s)
        H->>F: {"type":"progress","data":{...}}
    end
    
    S->>H: BroadcastJobUpdate(job完成)
    H->>F: {"type":"job_update","data":{status:"done"}}
```

**WebSocket 消息类型**：

| type | 说明 | data |
|------|------|------|
| `job_update` | 任务状态更新 | Job 对象 |
| `progress` | 进度更新 | `{jobId, progress, etaSec}` |
| `ping` | 心跳检测 | 时间戳 |

### 新增依赖

| 包 | 版本 | 说明 |
|---|------|------|
| github.com/gorilla/websocket | v1.5.1 | WebSocket 支持 |

### 新增文件

```
internal/bdinfo/
├── playlist.go      # Playlist 列表和推荐算法
├── job.go           # 任务队列管理
├── scanner.go       # BDInfo 扫描执行器
└── websocket.go     # WebSocket Hub

internal/httpapi/handlers/
└── bdinfo_jobs.go   # BDInfo 任务 API 处理器

webui/src/
├── api/media.js              # API 函数（含 BDInfo 任务）
├── composables/useBDInfoJobs.js  # BDInfo 任务管理
└── components/
    ├── BDInfoPanel.vue       # BDInfo 统一面板
    ├── BDInfoPlaylistPicker.vue  # Playlist 选择器
    ├── BDInfoJobProgress.vue # 任务进度显示
    └── BDInfoJobHistory.vue  # 历史任务列表
```

## 常见问题

**问题**：Web 界面显示"读取路径失败"

**解决**：
1. 检查挂载路径是否正确
2. 检查宿主机目录权限：`ls -la /path/to/media`
3. 确保容器有读取权限（使用 `:ro` 只读挂载）

**问题**：截图中字幕显示为方块

**解决**：使用最新镜像，已内置 CJK 字体

**问题**：Docker 构建网络超时

**解决**：使用 `--network=host` 参数
```bash
docker build --network=host -t minfo:local .
```

## 更新日志

### [v1.0.0] - 2024-04-02

#### 新增功能

- **BDInfo 高级功能**
  - 智能 Playlist 选择（自动推荐时长 > 10min）
  - 整盘扫描支持
  - 历史任务管理
  - WebSocket 实时进度推送
- **截图数量自定义**：支持 1-10 张截图数量自定义
- **BDMV 字幕探测**：新增 bdsub 工具
- **多路径挂载**：支持多个独立媒体目录
- **构建代理**：支持 HTTP/HTTPS 代理

#### 变更

- 移除 FAST 截图变体，简化为 PNG 和 JPG
- 优化 Dockerfile，基于原版镜像只覆盖修改文件
- Docker 构建使用 `--network=host` 解决网络问题

#### 修复

- 修复截图数量固定限制
- 改进多路径挂载文档
- WebSocket 连接稳定性优化
