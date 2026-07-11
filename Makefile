GO ?= go
GATEWAY_ADDR ?= 127.0.0.1:8080
GATEWAY_API_KEY ?= test-gateway-key
IMAGE ?= open-ai-gateway:local
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X open-ai-gateway/internal/version.Version=$(VERSION) -X open-ai-gateway/internal/version.Commit=$(COMMIT) -X open-ai-gateway/internal/version.BuildTime=$(BUILD_TIME)

.PHONY: fmt test race vet verify build run check-config check-config-examples smoke smoke-rate-limit smoke-azure smoke-deepseek smoke-deepseek-skip release-check docker-build docker-run

fmt:
	$(GO)fmt -w cmd internal

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

verify: fmt test race vet

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/open-ai-gateway ./cmd/gateway

run:
	GATEWAY_ADDR=$(GATEWAY_ADDR) GATEWAY_API_KEY=$(GATEWAY_API_KEY) $(GO) run ./cmd/gateway

check-config:
	GATEWAY_CHECK_CONFIG=1 $(GO) run ./cmd/gateway

check-config-examples:
	bash scripts/check-config-examples.sh

smoke:
	bash scripts/smoke-fake.sh

smoke-rate-limit:
	bash scripts/smoke-rate-limit.sh

smoke-azure:
	bash scripts/smoke-azure.sh

smoke-deepseek:
	bash scripts/smoke-deepseek.sh

smoke-deepseek-skip:
	DEEPSEEK_API_KEY= bash scripts/smoke-deepseek.sh

release-check: verify check-config check-config-examples build smoke smoke-rate-limit smoke-azure smoke-deepseek-skip

docker-build:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_TIME=$(BUILD_TIME) -t $(IMAGE) .

docker-run:
	docker run --rm -p 8080:8080 -e GATEWAY_ADDR=0.0.0.0:8080 $(IMAGE)
