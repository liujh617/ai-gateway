package main

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultAddr = "127.0.0.1:19090"
	apiVersion  = "2024-02-15-preview"
	testAPIKey  = "local-azure-test-key"
)

type modelRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func main() {
	addr := strings.TrimSpace(os.Getenv("AZURE_FAKE_ADDR"))
	if addr == "" {
		addr = defaultAddr
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("azure fake upstream listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("POST /openai/deployments/chat-deployment/chat/completions", handleChat)
	mux.HandleFunc("POST /openai/deployments/embedding-deployment/embeddings", handleEmbedding)
	return mux
}

func validateCommon(w http.ResponseWriter, r *http.Request, wantAccept string) bool {
	if r.URL.Query().Get("api-version") != apiVersion {
		http.Error(w, "api-version mismatch", http.StatusBadRequest)
		return false
	}
	if r.Header.Get("api-key") != testAPIKey {
		http.Error(w, "api-key mismatch", http.StatusUnauthorized)
		return false
	}
	if r.Header.Get("Authorization") != "" {
		http.Error(w, "Authorization must be absent", http.StatusBadRequest)
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return false
	}
	if r.Header.Get("Accept") != wantAccept {
		http.Error(w, "Accept mismatch", http.StatusBadRequest)
		return false
	}
	return true
}

func decodeModel(w http.ResponseWriter, r *http.Request, want string) (modelRequest, bool) {
	var req modelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return modelRequest{}, false
	}
	if req.Model != want {
		http.Error(w, "model mismatch", http.StatusBadRequest)
		return modelRequest{}, false
	}
	return req, true
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	var req modelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	wantAccept := "application/json"
	if req.Stream {
		wantAccept = "text/event-stream"
	}
	if !validateCommon(w, r, wantAccept) {
		return
	}
	if req.Model != "chat-deployment" {
		http.Error(w, "model mismatch", http.StatusBadRequest)
		return
	}
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"chunk_azure_smoke\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"chat-deployment\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"id":"chatcmpl_azure_smoke","object":"chat.completion","created":1,"model":"chat-deployment","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
}

func handleEmbedding(w http.ResponseWriter, r *http.Request) {
	if !validateCommon(w, r, "application/json") {
		return
	}
	if _, ok := decodeModel(w, r, "embedding-deployment"); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"object":"list","model":"embedding-deployment","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
}
