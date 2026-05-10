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
# Stage: 最终镜像
# ============================================
FROM ghcr.io/mirrorb/minfo:latest AS runtime

RUN apk add --no-cache mkvtoolnix

COPY scripts/seedbox/ /usr/local/share/mediainfo/scripts/
RUN chmod +x /usr/local/share/mediainfo/scripts/*.sh

COPY --from=build /out/mediainfo /usr/local/bin/mediainfo
RUN chmod +x /usr/local/bin/mediainfo

WORKDIR /app
ENV PORT=28888
ENV MEDIAINFO_BIN=/usr/bin/mediainfo
ENTRYPOINT ["/usr/local/bin/mediainfo"]
