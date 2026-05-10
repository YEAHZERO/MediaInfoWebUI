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
# Stage: Go 后端构建（标准版本，无 CGO）
# ============================================
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build-base
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

RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    BUILD_VERSION=${BUILD_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")} && \
    BUILD_COMMIT=${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -ldflags="-s -w -X mediainfo/internal/httpapi/handlers.BuildTime=${BUILD_TIME} -X mediainfo/internal/httpapi/handlers.BuildVersion=${BUILD_VERSION} -X mediainfo/internal/httpapi/handlers.BuildCommit=${BUILD_COMMIT}" -o /out/mediainfo ./cmd/mediainfo

# ============================================
# Stage: Go 后端构建（Native 版本，含 libplacebo）
# ============================================
FROM --platform=$BUILDPLATFORM ghcr.io/mirrorb/minfo-build:latest AS build-native
WORKDIR /src
COPY go.mod go.sum ./
COPY *.go ./
COPY cmd ./cmd
COPY internal ./internal
COPY --from=webui /app/dist ./webui/dist
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=1
ENV GOPROXY=https://goproxy.cn,direct
ARG BUILD_TIME
ARG BUILD_VERSION
ARG BUILD_COMMIT

RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    BUILD_VERSION=${BUILD_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")} && \
    BUILD_COMMIT=${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -tags native -ldflags="-s -w -X mediainfo/internal/httpapi/handlers.BuildTime=${BUILD_TIME} -X mediainfo/internal/httpapi/handlers.BuildVersion=${BUILD_VERSION} -X mediainfo/internal/httpapi/handlers.BuildCommit=${BUILD_COMMIT}" -o /out/mediainfo-native ./cmd/mediainfo

# ============================================
# Stage: 最终镜像 - 全量安装
# ============================================
FROM ghcr.io/mirrorb/minfo:latest AS runtime

# 安装额外依赖（mkvtoolnix 等）
RUN apk add --no-cache mkvtoolnix

# 复制脚本目录
COPY scripts/seedbox/ /usr/local/share/mediainfo/scripts/
RUN chmod +x /usr/local/share/mediainfo/scripts/*.sh

# 复制标准版本二进制
COPY --from=build-base /out/mediainfo /usr/local/bin/mediainfo-base
RUN chmod +x /usr/local/bin/mediainfo-base

# 复制 Native 版本二进制（全量功能）
COPY --from=build-native /out/mediainfo-native /usr/local/bin/mediainfo-native
RUN chmod +x /usr/local/bin/mediainfo-native

# 创建启动脚本，根据 ENGINE_TYPE 选择引擎
RUN echo '#!/bin/sh\n\
if [ "$ENGINE_TYPE" = "native" ] || [ "$ENABLE_NATIVE_ENGINE" = "1" ]; then\n\
    exec /usr/local/bin/mediainfo-native "$@"\n\
else\n\
    exec /usr/local/bin/mediainfo-base "$@"\n\
fi' > /usr/local/bin/mediainfo && chmod +x /usr/local/bin/mediainfo

# 默认引擎类型：script（轻量）
ENV ENGINE_TYPE=script

# 工作目录
WORKDIR /app

# 启动命令
CMD ["mediainfo"]
