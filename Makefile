# MediaInfoWebUI - Makefile
# 仅支持 Docker 构建，禁止本地直接编译运行 ./mediainfo
# Docker 构建：make docker-native

BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
PROJECT_VERSION := 1.5.6
BUILD_ARGS := --build-arg BUILD_TIME="$(BUILD_TIME)" --build-arg BUILD_VERSION="$(BUILD_VERSION)" --build-arg BUILD_COMMIT="$(BUILD_COMMIT)" --build-arg PROJECT_VERSION="$(PROJECT_VERSION)"
LDFLAGS := -s -w -X mediainfo/internal/httpapi/handlers.BuildTime=$(BUILD_TIME) -X mediainfo/internal/httpapi/handlers.BuildVersion=$(BUILD_VERSION) -X mediainfo/internal/httpapi/handlers.BuildCommit=$(BUILD_COMMIT) -X mediainfo/internal/version.Version=$(PROJECT_VERSION)

.PHONY: webui clean docker-native push-native

webui:
	cd webui && npm install --no-audit --no-fund && npm run build

clean:
	rm -f mediainfo
	rm -rf webui/dist

# Docker targets
docker-native:
	docker build --network=host $(BUILD_ARGS) --target native -t mediainfowebui:native .

# Push targets (to GHCR)
push-native:
	docker tag mediainfowebui:native ghcr.io/yeahzero/mediainfowebui:native
	docker push ghcr.io/yeahzero/mediainfowebui:native
	docker tag mediainfowebui:native ghcr.io/yeahzero/mediainfowebui:latest
	docker push ghcr.io/yeahzero/mediainfowebui:latest
