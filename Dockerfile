# syntax=docker/dockerfile:1

FROM golang:1.22-bookworm AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/open-ai-gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/open-ai-gateway /app/open-ai-gateway

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/open-ai-gateway"]

