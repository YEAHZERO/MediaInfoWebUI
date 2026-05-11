ARG GO_VERSION=1.26.1
ARG ALPINE_VERSION=edge
ARG ALPINE_EDGE_REPO=https://mirrors.aliyun.com/alpine/edge
ARG FFMPEG_PKG=ffmpeg
ARG INCLUDE_BDINFO=false

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

RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    BUILD_VERSION=${BUILD_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")} && \
    BUILD_COMMIT=${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -ldflags="-s -w -X mediainfo/internal/httpapi/handlers.BuildTime=${BUILD_TIME} -X mediainfo/internal/httpapi/handlers.BuildVersion=${BUILD_VERSION} -X mediainfo/internal/httpapi/handlers.BuildCommit=${BUILD_COMMIT}" -o /out/mediainfo ./cmd/mediainfo

# ============================================
# Stage: Go 后端构建 (Native, with CGO)
# ============================================
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build-native
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

RUN apk add --no-cache gcc musl-dev && \
    BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    BUILD_VERSION=${BUILD_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")} && \
    BUILD_COMMIT=${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -tags native -ldflags="-s -w -X mediainfo/internal/httpapi/handlers.BuildTime=${BUILD_TIME} -X mediainfo/internal/httpapi/handlers.BuildVersion=${BUILD_VERSION} -X mediainfo/internal/httpapi/handlers.BuildCommit=${BUILD_COMMIT}" -o /out/mediainfo ./cmd/mediainfo

# ============================================
# Stage: BDInfo 构建 (.NET) - 默认跳过
# ============================================
FROM scratch AS bdinfo-build
# BDInfo 构建默认跳过，设置 INCLUDE_BDINFO=true 启用
# docker build --build-arg INCLUDE_BDINFO=true --target runtime-native -t mediainfowebui:native .

# ============================================
# Stage: BD 元数据 helper (C 工具)
# ============================================
FROM alpine:${ALPINE_VERSION} AS media-helper-build
WORKDIR /src
RUN apk add --no-cache build-base
COPY tools/bdmv_subtitle_probe.c ./tools/bdmv_subtitle_probe.c
RUN mkdir -p /out && \
    cc -O2 -Wall -Wextra -std=c11 ./tools/bdmv_subtitle_probe.c -o /out/bdsub

# ============================================
# Stage: 轻量版镜像（无 mkvtoolnix，脚本引擎）
# ============================================
FROM alpine:${ALPINE_VERSION} AS runtime-light
ARG ALPINE_EDGE_REPO

RUN set -eux; \
    printf '%s\n%s\n' "${ALPINE_EDGE_REPO}/main" "${ALPINE_EDGE_REPO}/community" > /etc/apk/repositories; \
    apk add --no-cache \
        ca-certificates \
        mediainfo \
        ffmpeg \
        p7zip \
        udftools \
        kmod \
        util-linux \
        tzdata

COPY --from=build /out/mediainfo /usr/local/bin/mediainfo
COPY --from=media-helper-build /out/bdsub /usr/local/bin/bdsub
RUN chmod +x /usr/local/bin/mediainfo /usr/local/bin/bdsub

WORKDIR /app
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV PORT=28888
ENV MEDIAINFO_BIN=/usr/bin/mediainfo
ENV ENGINE_TYPE=script
ENTRYPOINT ["/usr/local/bin/mediainfo"]

# ============================================
# Stage: 标准版镜像（无 mkvtoolnix，脚本引擎）
# ============================================
FROM alpine:${ALPINE_VERSION} AS runtime-standard
ARG ALPINE_EDGE_REPO

RUN set -eux; \
    printf '%s\n%s\n' "${ALPINE_EDGE_REPO}/main" "${ALPINE_EDGE_REPO}/community" > /etc/apk/repositories; \
    apk add --no-cache \
        ca-certificates \
        mediainfo \
        ffmpeg \
        p7zip \
        udftools \
        kmod \
        util-linux \
        tzdata

COPY --from=build /out/mediainfo /usr/local/bin/mediainfo
COPY --from=media-helper-build /out/bdsub /usr/local/bin/bdsub
RUN chmod +x /usr/local/bin/mediainfo /usr/local/bin/bdsub

WORKDIR /app
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV PORT=28888
ENV MEDIAINFO_BIN=/usr/bin/mediainfo
ENV ENGINE_TYPE=script
ENTRYPOINT ["/usr/local/bin/mediainfo"]

# ============================================
# Stage: Native 版镜像（原生引擎 + libplacebo）
# ============================================
FROM alpine:${ALPINE_VERSION} AS runtime-native
ARG ALPINE_EDGE_REPO

RUN set -eux; \
    printf '%s\n%s\n' "${ALPINE_EDGE_REPO}/main" "${ALPINE_EDGE_REPO}/community" > /etc/apk/repositories; \
    apk add --no-cache \
        ca-certificates \
        mediainfo \
        ffmpeg \
        p7zip \
        udftools \
        kmod \
        libgdiplus \
        libplacebo \
        vulkan-loader \
        mesa-vulkan-swrast \
        oxipng \
        pngquant \
        util-linux \
        tzdata

COPY --from=build-native /out/mediainfo /usr/local/bin/mediainfo
COPY --from=media-helper-build /out/bdsub /usr/local/bin/bdsub
RUN chmod +x /usr/local/bin/mediainfo /usr/local/bin/bdsub

WORKDIR /app
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV PORT=28888
ENV MEDIAINFO_BIN=/usr/bin/mediainfo
ENV ENGINE_TYPE=native
ENV ENABLE_NATIVE_ENGINE=1
ENTRYPOINT ["/usr/local/bin/mediainfo"]

# ============================================
# 默认镜像为 native
# ============================================
FROM runtime-native AS final