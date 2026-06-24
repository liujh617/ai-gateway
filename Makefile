GO ?= go
GATEWAY_ADDR ?= 127.0.0.1:8080
GATEWAY_API_KEY ?= test-gateway-key
IMAGE ?= open-ai-gateway:local

.PHONY: fmt test race vet verify build run smoke docker-build docker-run

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
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/open-ai-gateway ./cmd/gateway

run:
	GATEWAY_ADDR=$(GATEWAY_ADDR) GATEWAY_API_KEY=$(GATEWAY_API_KEY) $(GO) run ./cmd/gateway

smoke:
	bash scripts/smoke-fake.sh

docker-build:
	docker build -t $(IMAGE) .

docker-run:
	docker run --rm -p 8080:8080 -e GATEWAY_ADDR=0.0.0.0:8080 $(IMAGE)
