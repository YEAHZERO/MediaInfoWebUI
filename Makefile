.PHONY: help webui-install webui-dev webui-build backend-dev test build run release-check docker-build docker-run docker-push docker-clean

ifneq (,$(wildcard ./.env))
include .env
export
endif

GO ?= go
NPM ?= npm
PORT ?= 28080
WEBUI_PORT ?= 28081
GOCACHE ?= $(CURDIR)/.gocache
WEBUI_DIR := webui
BINARY := ./bin/minfo
IMAGE_NAME ?= ghcr.io/yeahzero/mediainfowebui
IMAGE_TAG ?= latest

help:
	@printf '%s\n' \
		'make webui-install    # 安装前端依赖' \
		'make webui-dev        # 启动前端 Vite 开发服务器 (28081)' \
		'make backend-dev      # 启动 Go 后端 (28080)' \
		'make test             # 运行 Go 测试' \
		'make build            # 先构建前端，再构建 Go 二进制' \
		'make run              # 运行构建后的二进制' \
		'make release-check    # 发布前检查：前端构建 + Go 测试 + Go 构建' \
		'make docker-build     # 构建 Docker 镜像 (使用 host 网络)' \
		'make docker-run       # 运行 Docker 镜像' \
		'make docker-push      # 推送 Docker 镜像到 GitHub' \
		'make docker-clean     # 清理不需要的镜像'

webui-install:
	./scripts/bootstrap-webui.sh

webui-dev:
	cd $(WEBUI_DIR) && $(NPM) run dev -- --host 0.0.0.0 --port $(WEBUI_PORT)

backend-dev:
	mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) PORT=$(PORT) $(GO) run ./cmd/minfo

test:
	mkdir -p $(GOCACHE)
	GOCACHE=$(GOCACHE) $(GO) test ./...

build: webui-build
	mkdir -p $(GOCACHE) ./bin
	GOCACHE=$(GOCACHE) $(GO) build -trimpath -buildvcs=false -o $(BINARY) ./cmd/minfo

run: build
	PORT=$(PORT) $(BINARY)

release-check: test build

docker-build:
	docker build --network=host -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-run:
	docker run --rm -p $(PORT):$(PORT) --name minfo-test $(IMAGE_NAME):$(IMAGE_TAG)

docker-push:
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

docker-clean:
	@echo "清理悬空镜像..."
	docker image prune -f
	@echo "删除旧版本镜像..."
	docker rmi mediainfowebui:latest 2>/dev/null || true
	docker rmi mediainfowebui:v1.0.0 2>/dev/null || true
	docker rmi minfo:local 2>/dev/null || true
	@echo "完成！当前镜像列表："
	docker images | grep -E "mediainfowebui|minfo|REPOSITORY"
