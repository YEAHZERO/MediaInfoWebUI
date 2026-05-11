# MediaInfoWebUI - Makefile
# 本地开发：make build
# Docker 构建：make docker-light / docker-standard / docker-native

BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X mediainfo/internal/httpapi/handlers.BuildTime=$(BUILD_TIME) -X mediainfo/internal/httpapi/handlers.BuildVersion=$(BUILD_VERSION) -X mediainfo/internal/httpapi/handlers.BuildCommit=$(BUILD_COMMIT)

.PHONY: build build-native webui clean

webui:
	cd webui && npm install --no-audit --no-fund && npm run build

build: webui
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o mediainfo ./cmd/mediainfo

build-native: webui
	CGO_ENABLED=1 go build -trimpath -tags native -ldflags="$(LDFLAGS)" -o mediainfo ./cmd/mediainfo

run: build
	./mediainfo

run-native: build-native
	./mediainfo

clean:
	rm -f mediainfo
	rm -rf webui/dist

docker-light:
	docker build --network=host --target runtime-light -t mediainfowebui:light .

docker-standard:
	docker build --network=host --target runtime-standard -t mediainfowebui:latest .

docker-native:
	docker build --network=host --target runtime-native -t mediainfowebui:native .

docker-all: docker-light docker-standard docker-native

push-light:
	docker tag mediainfowebui:light ghcr.io/yeahzero/mediainfowebui:light
	docker push ghcr.io/yeahzero/mediainfowebui:light

push-standard:
	docker tag mediainfowebui:latest ghcr.io/yeahzero/mediainfowebui:latest
	docker push ghcr.io/yeahzero/mediainfowebui:latest

push-native:
	docker tag mediainfowebui:native ghcr.io/yeahzero/mediainfowebui:native
	docker push ghcr.io/yeahzero/mediainfowebui:native

push-all: push-light push-standard push-native