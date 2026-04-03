ARG GO_VERSION=1.26.1

# ============================================
# Stage: WebUI 构建
# ============================================
FROM --platform=$BUILDPLATFORM node:20-alpine AS webui
WORKDIR /app
COPY webui/package.json ./
RUN npm install --no-audit --no-fund
COPY webui .
RUN npm run build

# ============================================
# Stage: Go 后端构建
# ============================================
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY *.go ./
COPY cmd ./cmd
COPY internal ./internal
COPY --from=webui /app/dist ./webui/dist
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0
ENV GOPROXY=https://goproxy.cn,direct
ARG BUILD_TIME
ARG BUILD_VERSION
ARG BUILD_COMMIT

# 获取版本信息并构建
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    BUILD_VERSION=${BUILD_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")} && \
    BUILD_COMMIT=${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -ldflags="-s -w -X minfo/internal/httpapi/handlers.BuildTime=${BUILD_TIME} -X minfo/internal/httpapi/handlers.BuildVersion=${BUILD_VERSION} -X minfo/internal/httpapi/handlers.BuildCommit=${BUILD_COMMIT}" -o /out/minfo ./cmd/minfo

# ============================================
# Stage: 最终镜像 - 直接使用原版镜像
# ============================================
FROM ghcr.io/mirrorb/minfo:latest AS runtime

# 安装 mkvmerge (mkvtoolnix)
RUN apk add --no-cache mkvtoolnix

# 复制截图脚本
COPY scripts/seedbox/ /usr/local/share/minfo/scripts/
RUN chmod +x /usr/local/share/minfo/scripts/*.sh

# 覆盖修改的二进制文件
COPY --from=build /out/minfo /usr/local/bin/minfo
RUN chmod +x /usr/local/bin/minfo
