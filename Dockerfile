# ============================================
# MediaInfoWebUI Dockerfile
#
# 构建策略：
# 1. 从上一版 mediainfowebui 镜像提取所有依赖（ffmpeg、mediainfo、字体等）
# 2. 只覆盖修改的二进制文件（BDInfo、bdmv_subtitle_probe、mediainfo）
#
# 容器管理：
# - 启动新容器前务必先删除旧容器：docker rm -f mediainfo（非常重要！！！！）
# ============================================

ARG ALPINE_MIRROR=mirrors.aliyun.com

# ============================================
# Stage: Base - 从上一版镜像提取依赖（锁定 digest 确保构建一致性）
# ============================================
FROM ghcr.io/yeahzero/mediainfowebui@sha256:7fd1ba1b9aa59dbc4e48b45eb67ea5638f07cbc9675342314d8b35a23a3d9fe1 AS base
# 上一版镜像已包含：ffmpeg、mediainfo、字体、libplacebo 等所有依赖
# 无需重新安装，直接使用

# ============================================
# Stage: BDInfo - 从 minfo 镜像提取 BDInfo
# ============================================
FROM ghcr.io/mirrorb/minfo:latest AS bdinfo-source

# ============================================
# Stage: WebUI Build
# ============================================
FROM --platform=$BUILDPLATFORM node:20-alpine AS webui
ARG NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /app
COPY webui/package.json ./
RUN npm install --no-audit --no-fund --registry ${NPM_REGISTRY}
COPY webui .
RUN npm run build

# ============================================
# Stage: Go Backend Build (Native, with CGO)
# ============================================
FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS build-native
ARG ALPINE_MIRROR
RUN sed -i "s|dl-cdn.alpinelinux.org|${ALPINE_MIRROR}|g" /etc/apk/repositories

WORKDIR /src
COPY go.mod go.sum ./
COPY *.go ./
COPY cmd ./cmd
COPY internal ./internal
COPY --from=webui /app/dist ./webui/dist
COPY tools/bdmv_subtitle_probe.c ./tools/bdmv_subtitle_probe.c

ARG TARGETOS
ARG TARGETARCH
ARG BUILD_TIME
ARG BUILD_VERSION
ARG BUILD_COMMIT
ARG PROJECT_VERSION
ENV CGO_ENABLED=1
ENV GOPROXY=https://goproxy.cn,direct
ENV BUILD_TIME=${BUILD_TIME}
ENV BUILD_VERSION=${BUILD_VERSION}
ENV BUILD_COMMIT=${BUILD_COMMIT}
ENV PROJECT_VERSION=${PROJECT_VERSION}

RUN set -eux; \
    apk add --no-cache gcc musl-dev; \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -tags "native websocket" \
        -ldflags="-s -w \
            -X mediainfo/internal/httpapi/handlers.BuildTime=${BUILD_TIME} \
            -X mediainfo/internal/httpapi/handlers.BuildVersion=${BUILD_VERSION} \
            -X mediainfo/internal/httpapi/handlers.BuildCommit=${BUILD_COMMIT} \
            -X mediainfo/internal/version.Version=${PROJECT_VERSION}" \
        -o /out/mediainfo ./cmd/mediainfo; \
    gcc -O2 -static -o /out/bdmv_subtitle_probe ./tools/bdmv_subtitle_probe.c

# ============================================
# Stage: Final - 只覆盖修改的二进制文件
# ============================================
FROM base AS native
ARG ALPINE_MIRROR
RUN sed -i "s|dl-cdn.alpinelinux.org|${ALPINE_MIRROR}|g" /etc/apk/repositories
RUN apk upgrade --no-cache musl && apk add --no-cache mkvtoolnix

# BDInfo 从 ghcr.io/mirrorb/minfo:latest 镜像提取
COPY --from=bdinfo-source /usr/local/bin/bdinfo /opt/bdinfo/BDInfo
RUN chmod +x /opt/bdinfo/BDInfo

COPY --from=build-native /out/bdmv_subtitle_probe /usr/local/bin/bdmv_subtitle_probe
RUN chmod +x /usr/local/bin/bdmv_subtitle_probe

COPY --from=build-native /out/mediainfo /usr/local/bin/mediainfo
RUN chmod +x /usr/local/bin/mediainfo

ENV BDINFO_BIN=/opt/bdinfo/BDInfo
ENV ENGINE_TYPE=native
ENV ENABLE_NATIVE_ENGINE=1
ENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1
ENV PORT=28888
EXPOSE 28888
ENTRYPOINT ["/usr/local/bin/mediainfo"]