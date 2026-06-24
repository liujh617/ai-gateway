# syntax=docker/dockerfile:1

FROM golang:1.22-bookworm AS build

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X open-ai-gateway/internal/version.Version=${VERSION} -X open-ai-gateway/internal/version.Commit=${COMMIT} -X open-ai-gateway/internal/version.BuildTime=${BUILD_TIME}" \
  -o /out/open-ai-gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/open-ai-gateway /app/open-ai-gateway

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/open-ai-gateway"]
