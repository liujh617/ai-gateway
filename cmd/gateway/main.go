package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/router"
)

func main() {
	addr := env("GATEWAY_ADDR", "127.0.0.1:8080")
	apiKey := env("GATEWAY_API_KEY", "test-gateway-key")

	fakeProvider := fake.New()
	modelRouter := router.NewModelRouter([]router.ModelRoute{{
		ExternalModel: "test-model",
		UpstreamModel: "test-model",
		Provider:      fakeProvider,
	}})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := api.NewServer(modelRouter, apiKey, logger)

	log.Printf("open-ai-gateway listening on http://%s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func env(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
