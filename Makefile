GO ?= go
GATEWAY_ADDR ?= 127.0.0.1:8080
GATEWAY_API_KEY ?= test-gateway-key

.PHONY: fmt test race vet verify run smoke

fmt:
	$(GO)fmt -w cmd internal

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

verify: fmt test race vet

run:
	GATEWAY_ADDR=$(GATEWAY_ADDR) GATEWAY_API_KEY=$(GATEWAY_API_KEY) $(GO) run ./cmd/gateway

smoke:
	bash scripts/smoke-fake.sh
