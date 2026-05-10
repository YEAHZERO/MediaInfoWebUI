ARG BDINFO_REPO=https://github.com/mirrorb/BDInfoCLI.git
ARG BDINFO_REF=master
ARG BDINFO_CSPROJ=BDInfo/BDInfo.csproj
ARG GO_VERSION=1.26.1
ARG ALPINE_VERSION=edge
ARG ALPINE_EDGE_REPO=https://mirrors.aliyun.com/alpine/edge
ARG FFMPEG_PKG=ffmpeg

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
# Stage: BDInfo 构建 (.NET)
# ============================================
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/dotnet/sdk:9.0-alpine AS bdinfo-build
ARG BDINFO_REPO
ARG BDINFO_REF
ARG BDINFO_CSPROJ
ARG TARGETARCH
RUN apk add --no-cache git ca-certificates
RUN git clone --depth 1 --branch "$BDINFO_REF" "$BDINFO_REPO" /src/bdinfo
WORKDIR /src/bdinfo
RUN set -eux; \
    case "$TARGETARCH" in \
        amd64) rid="linux-musl-x64" ;; \
        arm64) rid="linux-musl-arm64" ;; \
        *) echo "unsupported TARGETARCH=$TARGETARCH" >&2; exit 1 ;; \
    esac; \
    dotnet restore "$BDINFO_CSPROJ"; \
    dotnet publish "$BDINFO_CSPROJ" -c Release -r "$rid" --self-contained true \
        -p:PublishSingleFile=true \
        -p:EnableCompressionInSingleFile=true \
        -p:DebugType=None \
        -p:DebugSymbols=false \
        -o /out/bdinfo; \
    exe=""; \
    for f in /out/bdinfo/*; do \
        [ -f "$f" ] || continue; \
        [ -x "$f" ] || continue; \
        case "${f##*.}" in \
            dll|json|pdb) continue ;; \
        esac; \
        exe="$f"; \
        break; \
    done; \
    if [ -n "$exe" ]; then \
        if [ "$exe" != "/out/bdinfo/BDInfo" ]; then \
            mv "$exe" /out/bdinfo/BDInfo; \
        fi; \
    else \
        echo "BDInfo executable not found" >&2; exit 1; \
    fi; \
    chmod +x /out/bdinfo/BDInfo; \
    find /out/bdinfo -type f \( -name '*.pdb' -o -name '*.xml' -o -name '*.dbg' \) -delete

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
COPY --from=bdinfo-build /out/bdinfo/BDInfo /usr/local/bin/bdinfo
COPY --from=media-helper-build /out/bdsub /usr/local/bin/bdsub
RUN chmod +x /usr/local/bin/mediainfo /usr/local/bin/bdinfo /usr/local/bin/bdsub

WORKDIR /app
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV PORT=28880
ENV MEDIAINFO_BIN=/usr/bin/mediainfo
ENV ENGINE_TYPE=script
ENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1
ENTRYPOINT ["/usr/local/bin/mediainfo"]

# ============================================
# Stage: 标准版镜像（含 mkvtoolnix，脚本引擎）
# ============================================
FROM alpine:${ALPINE_VERSION} AS runtime-standard
ARG ALPINE_EDGE_REPO

RUN set -eux; \
    printf '%s\n%s\n' "${ALPINE_EDGE_REPO}/main" "${ALPINE_EDGE_REPO}/community" > /etc/apk/repositories; \
    apk add --no-cache \
        ca-certificates \
        mediainfo \
        ffmpeg \
        mkvtoolnix \
        p7zip \
        udftools \
        kmod \
        util-linux \
        tzdata

COPY --from=build /out/mediainfo /usr/local/bin/mediainfo
COPY --from=bdinfo-build /out/bdinfo/BDInfo /usr/local/bin/bdinfo
COPY --from=media-helper-build /out/bdsub /usr/local/bin/bdsub
RUN chmod +x /usr/local/bin/mediainfo /usr/local/bin/bdinfo /usr/local/bin/bdsub

WORKDIR /app
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV PORT=28880
ENV MEDIAINFO_BIN=/usr/bin/mediainfo
ENV ENGINE_TYPE=script
ENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1
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
        mkvtoolnix \
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
COPY --from=bdinfo-build /out/bdinfo/BDInfo /usr/local/bin/bdinfo
COPY --from=media-helper-build /out/bdsub /usr/local/bin/bdsub
RUN chmod +x /usr/local/bin/mediainfo /usr/local/bin/bdinfo /usr/local/bin/bdsub

WORKDIR /app
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
ENV PORT=28880
ENV MEDIAINFO_BIN=/usr/bin/mediainfo
ENV ENGINE_TYPE=native
ENV ENABLE_NATIVE_ENGINE=1
ENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1
ENTRYPOINT ["/usr/local/bin/mediainfo"]

# ============================================
# 默认镜像为 native
# ============================================
FROM runtime-native AS final
