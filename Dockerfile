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
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/minfo ./cmd/minfo

# ============================================
# Stage: 最终镜像 - 直接使用原版镜像
# ============================================
FROM ghcr.io/mirrorb/minfo:latest

# 只覆盖修改的二进制文件
COPY --from=build /out/minfo /usr/local/bin/minfo
RUN chmod +x /usr/local/bin/minfo
