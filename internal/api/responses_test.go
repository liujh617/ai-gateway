package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/responsestore"
	"open-ai-gateway/internal/router"
)

func TestResponsesNonStreamOK(t *testing.T) {
	rr := doResponsesJSON(newTestHandler(fake.New()), `{"model":"test-model","input":"hello"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var got compat.Response
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Object != "response" || got.Model != "test-model" || len(got.Output) != 1 || got.Output[0].Content[0].Text != "Hello from open-ai-gateway." {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestResponsesNonStreamContinuesStoredResponse(t *testing.T) {
	p := &responseStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	first := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	var firstResponse compat.Response
	if err := json.NewDecoder(first.Body).Decode(&firstResponse); err != nil {
		t.Fatal(err)
	}
	if !firstResponse.Store {
		t.Fatalf("response should report stored: %#v", firstResponse)
	}

	second := doResponsesJSON(handler, `{"model":"test-model","input":"again","previous_response_id":"`+firstResponse.ID+`"}`, true)
	if second.Code != http.StatusOK {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	if len(p.requests) != 2 {
		t.Fatalf("requests=%d", len(p.requests))
	}
	messages := p.requests[1].Messages
	if len(messages) != 3 || messageText(messages[0]) != "hello" || messageText(messages[1]) != "answer-1" || messageText(messages[2]) != "again" {
		t.Fatalf("continuation messages=%#v", messages)
	}
}

func TestRetrieveStoredResponse(t *testing.T) {
	p := &responseStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello","store":true}`, true)
	if createdRecorder.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", createdRecorder.Code, createdRecorder.Body.String())
	}
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created response: %v", err)
	}

	rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodGet)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var retrieved compat.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &retrieved); err != nil {
		t.Fatalf("decode retrieved response: %v", err)
	}
	if !reflect.DeepEqual(retrieved, created) {
		t.Fatalf("retrieved=%#v created=%#v", retrieved, created)
	}
	if len(p.requests) != 1 {
		t.Fatalf("retrieve called provider: requests=%d", len(p.requests))
	}
}

func TestRetrieveStoredResponseHead(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodHead)
	if rr.Code != http.StatusOK || rr.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}
}

func TestRetrieveResponseNotFoundCases(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	assertResponseNotFound(t, retrieveResponse(handler, "resp_missing", testAPIKey, http.MethodGet))

	unstoredRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello","store":false}`, true)
	var unstored compat.Response
	if err := json.Unmarshal(unstoredRecorder.Body.Bytes(), &unstored); err != nil {
		t.Fatal(err)
	}
	assertResponseNotFound(t, retrieveResponse(handler, unstored.ID, testAPIKey, http.MethodGet))
	assertResponseNotFound(t, retrieveResponse(newTestHandler(fake.New()), "resp_missing", testAPIKey, http.MethodGet))
}

func TestRetrieveResponseIsClientIsolated(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newResponseStateIsolationHandler(&responseStateProvider{}, store)
	createdRecorder := doResponsesJSONWithKey(handler, `{"model":"test-model","input":"hello"}`, "alpha-secret")
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, "beta-secret", http.MethodGet))
}

func TestRetrieveResponseRequiresAuth(t *testing.T) {
	rr := retrieveResponse(newTestHandler(fake.New()), "resp_missing", "", http.MethodGet)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestRetrieveResponseMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(fake.New())
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		rr := retrieveResponse(handler, "resp_1", testAPIKey, method)
		assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
		if got := rr.Header().Get("Allow"); got != "GET, HEAD, DELETE" {
			t.Fatalf("%s Allow=%q", method, got)
		}
	}
}

func TestDeleteStoredResponse(t *testing.T) {
	p := &responseStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}
	var deleted compat.DeletedResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &deleted); err != nil {
		t.Fatal(err)
	}
	if deleted.ID != created.ID || deleted.Object != "response" || !deleted.Deleted {
		t.Fatalf("deleted=%#v", deleted)
	}
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, testAPIKey, http.MethodGet))
	if len(p.requests) != 1 {
		t.Fatalf("delete called provider: requests=%d", len(p.requests))
	}
}

func TestDeletedResponseCannotBeContinued(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete); rr.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rr.Code, rr.Body.String())
	}
	continued := doResponsesJSON(handler, `{"model":"test-model","input":"again","previous_response_id":"`+created.ID+`"}`, true)
	assertError(t, continued, http.StatusNotFound, "invalid_request_error")
}

func TestDeleteResponseDoesNotCascadeToDescendant(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	firstRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"first"}`, true)
	var first compat.Response
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	secondRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"second","previous_response_id":"`+first.ID+`"}`, true)
	var second compat.Response
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if rr := retrieveResponse(handler, first.ID, testAPIKey, http.MethodDelete); rr.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := retrieveResponse(handler, second.ID, testAPIKey, http.MethodGet); rr.Code != http.StatusOK {
		t.Fatalf("descendant GET status=%d body=%s", rr.Code, rr.Body.String())
	}
	continued := doResponsesJSON(handler, `{"model":"test-model","input":"third","previous_response_id":"`+second.ID+`"}`, true)
	if continued.Code != http.StatusOK {
		t.Fatalf("descendant continuation status=%d body=%s", continued.Code, continued.Body.String())
	}
}

func TestDeleteResponseNotFoundCases(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	assertResponseNotFound(t, retrieveResponse(handler, "resp_missing", testAPIKey, http.MethodDelete))
	assertResponseNotFound(t, retrieveResponse(newTestHandler(fake.New()), "resp_missing", testAPIKey, http.MethodDelete))

	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete); rr.Code != http.StatusOK {
		t.Fatalf("first delete status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete))
}

func TestDeleteResponseIsClientIsolated(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newResponseStateIsolationHandler(&responseStateProvider{}, store)
	createdRecorder := doResponsesJSONWithKey(handler, `{"model":"test-model","input":"hello"}`, "alpha-secret")
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, "beta-secret", http.MethodDelete))
	if rr := retrieveResponse(handler, created.ID, "alpha-secret", http.MethodGet); rr.Code != http.StatusOK {
		t.Fatalf("owner GET status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDeleteResponseRequiresAuth(t *testing.T) {
	rr := retrieveResponse(newTestHandler(fake.New()), "resp_missing", "", http.MethodDelete)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

type responseStoreClock struct{ now time.Time }

func (c *responseStoreClock) Now() time.Time { return c.now }

func TestDeleteExpiredResponseReturnsNotFound(t *testing.T) {
	clock := &responseStoreClock{now: time.Unix(100, 0)}
	store := responsestore.New(responsestore.Config{TTL: time.Minute, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, clock)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Minute)
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete))
}

func retrieveResponse(handler http.Handler, id, key, method string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/v1/responses/"+id, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func assertResponseNotFound(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var got compat.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Type != "invalid_request_error" || got.Error.Param == nil || *got.Error.Param != "response_id" {
		t.Fatalf("error=%#v", got)
	}
}

func TestResponsesStoreFalseDoesNotCreatePreviousResponse(t *testing.T) {
	p := &responseStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	first := doResponsesJSON(handler, `{"model":"test-model","input":"hello","store":false}`, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	var response compat.Response
	if err := json.NewDecoder(first.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Store {
		t.Fatal("response unexpectedly stored")
	}
	second := doResponsesJSON(handler, `{"model":"test-model","input":"again","previous_response_id":"`+response.ID+`"}`, true)
	if second.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", second.Code, second.Body.String())
	}
	var errorResponse compat.ErrorResponse
	if err := json.NewDecoder(second.Body).Decode(&errorResponse); err != nil {
		t.Fatal(err)
	}
	if errorResponse.Error.Type != "invalid_request_error" || errorResponse.Error.Param == nil || *errorResponse.Error.Param != "previous_response_id" {
		t.Fatalf("error=%#v", errorResponse)
	}
}

func TestResponsesStoreFalseCanReadPreviousWithoutInheritingInstructions(t *testing.T) {
	p := &responseStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	first := doResponsesJSON(handler, `{"model":"test-model","instructions":"first-turn-only","input":"hello"}`, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	var firstResponse compat.Response
	if err := json.NewDecoder(first.Body).Decode(&firstResponse); err != nil {
		t.Fatal(err)
	}

	second := doResponsesJSON(handler, `{"model":"test-model","input":"again","store":false,"previous_response_id":"`+firstResponse.ID+`"}`, true)
	if second.Code != http.StatusOK {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	var secondResponse compat.Response
	if err := json.NewDecoder(second.Body).Decode(&secondResponse); err != nil {
		t.Fatal(err)
	}
	if secondResponse.Store {
		t.Fatal("store:false response unexpectedly stored")
	}
	for _, message := range p.requests[1].Messages {
		if message.Role == "developer" && messageText(message) == "first-turn-only" {
			t.Fatalf("instructions inherited: %#v", p.requests[1].Messages)
		}
	}
	third := doResponsesJSON(handler, `{"model":"test-model","input":"third","previous_response_id":"`+secondResponse.ID+`"}`, true)
	if third.Code != http.StatusNotFound {
		t.Fatalf("third status=%d body=%s", third.Code, third.Body.String())
	}
}

func TestResponsesPreviousResponseIsClientAndModelIsolated(t *testing.T) {
	p := &responseStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newResponseStateIsolationHandler(p, store)
	first := doResponsesJSONWithKey(handler, `{"model":"test-model","input":"hello"}`, "alpha-secret")
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	var response compat.Response
	if err := json.NewDecoder(first.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	otherClient := doResponsesJSONWithKey(handler, `{"model":"test-model","input":"again","previous_response_id":"`+response.ID+`"}`, "beta-secret")
	if otherClient.Code != http.StatusNotFound {
		t.Fatalf("client status=%d body=%s", otherClient.Code, otherClient.Body.String())
	}
	otherModel := doResponsesJSONWithKey(handler, `{"model":"other-model","input":"again","previous_response_id":"`+response.ID+`"}`, "alpha-secret")
	if otherModel.Code != http.StatusBadRequest || !strings.Contains(otherModel.Body.String(), `"param":"previous_response_id"`) {
		t.Fatalf("model status=%d body=%s", otherModel.Code, otherModel.Body.String())
	}
}

func TestResponsesContinuesFunctionCallOutputFromPreviousResponse(t *testing.T) {
	p := &responseFunctionStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	first := doResponsesJSON(handler, `{"model":"test-model","input":"weather","tools":[{"type":"function","name":"get_weather","parameters":{"type":"object"}}]}`, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	var firstResponse compat.Response
	if err := json.NewDecoder(first.Body).Decode(&firstResponse); err != nil {
		t.Fatal(err)
	}
	second := doResponsesJSON(handler, `{"model":"test-model","previous_response_id":"`+firstResponse.ID+`","input":[{"type":"function_call_output","call_id":"call_1","output":"sunny"}]}`, true)
	if second.Code != http.StatusOK {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	if len(p.requests) != 2 || len(p.requests[1].Messages) != 3 || p.requests[1].Messages[1].Extra["tool_calls"] == nil || p.requests[1].Messages[2].Role != "tool" {
		t.Fatalf("requests=%#v", p.requests)
	}
}

func TestResponsesRejectsPreviousResponseWhenStoreDisabled(t *testing.T) {
	rr := doResponsesJSON(newTestHandler(fake.New()), `{"model":"test-model","input":"next","previous_response_id":"resp_missing"}`, true)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "response store is disabled") {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestResponsesRejectsTranscriptOverStoreLimit(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 2, MaxTotalBytes: 100}, nil)
	rr := doResponsesJSON(newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store}), `{"model":"test-model","input":"hello"}`, true)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"param":"previous_response_id"`) {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

type responseFunctionStateProvider struct {
	requests []compat.ChatCompletionRequest
}

func (p *responseFunctionStateProvider) ListModels(context.Context) ([]compat.Model, error) {
	return nil, nil
}
func (p *responseFunctionStateProvider) CreateChatCompletion(_ context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	p.requests = append(p.requests, req)
	message := compat.ChatMessage{Role: "assistant", Content: json.RawMessage(`"done"`)}
	if len(p.requests) == 1 {
		message.Content = json.RawMessage("null")
		message.Extra = map[string]json.RawMessage{"tool_calls": json.RawMessage(`[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]`)}
	}
	return &compat.ChatCompletionResponse{Choices: []compat.ChatCompletionChoice{{Index: 0, Message: message, FinishReason: "stop"}}}, nil
}
func (p *responseFunctionStateProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("unused")
}
func (p *responseFunctionStateProvider) CreateCompletion(context.Context, compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	return nil, errors.New("unused")
}
func (p *responseFunctionStateProvider) StreamCompletion(context.Context, compat.CompletionsRequest) (provider.CompletionStream, error) {
	return nil, errors.New("unused")
}
func (p *responseFunctionStateProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("unused")
}

func (p *responseFunctionStateProvider) CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
	return nil, errors.New("not implemented")
}

func newResponseStateIsolationHandler(p provider.Provider, store *responsestore.Store) http.Handler {
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "upstream-test-model", ProviderName: "fake-provider", Provider: p},
		{ExternalModel: "other-model", UpstreamModel: "upstream-other-model", ProviderName: "fake-provider", Provider: p},
	})
	return api.NewServer(modelRouter, "", slog.New(slog.NewTextHandler(io.Discard, nil)), api.Options{
		ResponseStore: store,
		Credentials:   []middleware.AuthCredential{{Client: "alpha", APIKey: "alpha-secret"}, {Client: "beta", APIKey: "beta-secret"}},
	}).Handler()
}

func doResponsesJSONWithKey(handler http.Handler, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

type responseStateProvider struct {
	requests []compat.ChatCompletionRequest
}

func (p *responseStateProvider) ListModels(context.Context) ([]compat.Model, error) { return nil, nil }
func (p *responseStateProvider) CreateChatCompletion(_ context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	p.requests = append(p.requests, req)
	content, _ := json.Marshal(fmt.Sprintf("answer-%d", len(p.requests)))
	return &compat.ChatCompletionResponse{Choices: []compat.ChatCompletionChoice{{Index: 0, Message: compat.ChatMessage{Role: "assistant", Content: content}, FinishReason: "stop"}}}, nil
}
func (p *responseStateProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("unused")
}
func (p *responseStateProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("unused")
}
func (p *responseStateProvider) CreateCompletion(context.Context, compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	return nil, errors.New("unused")
}
func (p *responseStateProvider) StreamCompletion(context.Context, compat.CompletionsRequest) (provider.CompletionStream, error) {
	return nil, errors.New("unused")
}

func (p *responseStateProvider) CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
	return nil, errors.New("not implemented")
}

func messageText(message compat.ChatMessage) string {
	var text string
	_ = json.Unmarshal(message.Content, &text)
	return text
}

func TestResponsesRejectsUnsupportedField(t *testing.T) {
	rr := doResponsesJSON(newTestHandler(fake.New()), `{"model":"test-model","input":"hello","tools":[]}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestResponsesStreamOK(t *testing.T) {
	rr := doResponsesJSON(newTestHandler(fake.New()), `{"model":"test-model","input":"hello","stream":true}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type=%q", got)
	}
	text := rr.Body.String()
	wantOrder := []string{"response.created", "response.in_progress", "response.output_item.added", "response.content_part.added", "response.output_text.delta", "response.output_text.done", "response.content_part.done", "response.output_item.done", "response.completed"}
	last := -1
	for _, event := range wantOrder {
		index := strings.Index(text, "event: "+event+"\n")
		if index <= last {
			t.Fatalf("event %q missing or out of order: %s", event, text)
		}
		last = index
	}
	if strings.Contains(text, "[DONE]") {
		t.Fatalf("unexpected chat sentinel: %s", text)
	}
}

func TestResponsesCompletedStreamCanBeContinued(t *testing.T) {
	p := &responseStateStreamProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	streamed := doResponsesJSON(handler, `{"model":"test-model","input":"hello","stream":true}`, true)
	if streamed.Code != http.StatusOK {
		t.Fatalf("stream status=%d body=%s", streamed.Code, streamed.Body.String())
	}
	responseID := completedResponseID(t, streamed.Body.String())
	continued := doResponsesJSON(handler, `{"model":"test-model","input":"again","previous_response_id":"`+responseID+`"}`, true)
	if continued.Code != http.StatusOK {
		t.Fatalf("continued status=%d body=%s", continued.Code, continued.Body.String())
	}
	if len(p.requests) != 2 || len(p.requests[1].Messages) != 3 || messageText(p.requests[1].Messages[1]) != "stream-answer" {
		t.Fatalf("requests=%#v", p.requests)
	}
}

func TestRetrieveStoredStreamingResponse(t *testing.T) {
	p := &responseStateStreamProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	streamed := doResponsesJSON(handler, `{"model":"test-model","input":"hello","stream":true,"store":true}`, true)
	if streamed.Code != http.StatusOK {
		t.Fatalf("stream status=%d body=%s", streamed.Code, streamed.Body.String())
	}
	completed := completedResponseFromSSE(t, streamed.Body.String())

	rr := retrieveResponse(handler, completed.ID, testAPIKey, http.MethodGet)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var retrieved compat.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &retrieved); err != nil {
		t.Fatalf("decode retrieved response: %v", err)
	}
	if !reflect.DeepEqual(retrieved, completed) {
		t.Fatalf("retrieved=%#v completed=%#v", retrieved, completed)
	}
	if len(p.requests) != 1 {
		t.Fatalf("retrieve called provider: requests=%d", len(p.requests))
	}
}

func TestResponsesFailedStreamCannotBeContinued(t *testing.T) {
	p := &responseStateStreamProvider{streamErr: errors.New("stream failed")}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	streamed := doResponsesJSON(handler, `{"model":"test-model","input":"hello","stream":true}`, true)
	if strings.Contains(streamed.Body.String(), "response.completed") {
		t.Fatalf("unexpected completion: %s", streamed.Body.String())
	}
	if stats := store.Snapshot(); stats.Entries != 0 {
		t.Fatalf("stored failed stream: %+v", stats)
	}
}

func completedResponseID(t *testing.T, stream string) string {
	return completedResponseFromSSE(t, stream).ID
}

func completedResponseFromSSE(t *testing.T, stream string) compat.Response {
	t.Helper()
	for _, block := range strings.Split(stream, "\n\n") {
		if !strings.HasPrefix(block, "event: response.completed\n") {
			continue
		}
		line := strings.TrimPrefix(strings.SplitN(block, "\n", 2)[1], "data: ")
		var event struct {
			Response compat.Response `json:"response"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		return event.Response
	}
	t.Fatalf("missing response.completed: %s", stream)
	return compat.Response{}
}

type responseStateStreamProvider struct {
	requests  []compat.ChatCompletionRequest
	streamErr error
}

func (p *responseStateStreamProvider) ListModels(context.Context) ([]compat.Model, error) {
	return nil, nil
}
func (p *responseStateStreamProvider) CreateChatCompletion(_ context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	p.requests = append(p.requests, req)
	return &compat.ChatCompletionResponse{Choices: []compat.ChatCompletionChoice{{Index: 0, Message: compat.ChatMessage{Role: "assistant", Content: json.RawMessage(`"continued"`)}, FinishReason: "stop"}}}, nil
}
func (p *responseStateStreamProvider) StreamChatCompletion(_ context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	p.requests = append(p.requests, req)
	return &responseStateStream{err: p.streamErr}, nil
}
func (p *responseStateStreamProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("unused")
}
func (p *responseStateStreamProvider) CreateCompletion(context.Context, compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	return nil, errors.New("unused")
}
func (p *responseStateStreamProvider) StreamCompletion(context.Context, compat.CompletionsRequest) (provider.CompletionStream, error) {
	return nil, errors.New("unused")
}

func (p *responseStateStreamProvider) CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
	return nil, errors.New("not implemented")
}

type responseStateStream struct {
	sent bool
	err  error
}

func (s *responseStateStream) Next(context.Context) (*compat.ChatCompletionChunk, error) {
	if !s.sent {
		s.sent = true
		return &compat.ChatCompletionChunk{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Content: "stream-answer"}}}}, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, io.EOF
}
func (s *responseStateStream) Close() error { return nil }

func TestResponsesAuditUsesResponsesBodies(t *testing.T) {
	recorder := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: recorder})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"test-model","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set(audit.TraceIDHeader, "responses-trace")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	events := recorder.Events()
	if len(events) != 2 || events[0].Path != "/v1/responses" || events[1].Path != "/v1/responses" {
		t.Fatalf("events=%#v", events)
	}
	if !strings.Contains(string(events[0].Body), `"input":"hello"`) {
		t.Fatalf("request body=%s", events[0].Body)
	}
	if !strings.Contains(string(events[1].Body), `"object":"response"`) {
		t.Fatalf("response body=%s", events[1].Body)
	}
}

func TestResponsesAuditRecordsPreviousResponseID(t *testing.T) {
	recorder := &memoryAuditRecorder{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{Audit: recorder, ResponseStore: store})
	first := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var response compat.Response
	if err := json.NewDecoder(first.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	second := doResponsesJSON(handler, `{"model":"test-model","input":"again","previous_response_id":"`+response.ID+`"}`, true)
	if second.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", second.Code, second.Body.String())
	}
	events := recorder.Events()
	found := false
	for _, event := range events {
		if event.Event == audit.EventRequest && event.PreviousResponseID == response.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("previous response id missing from audit: %#v", events)
	}
}

func TestResponsesAccessLogDoesNotRecordPreviousResponseID(t *testing.T) {
	var logs bytes.Buffer
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithLogger(&responseStateProvider{}, slog.New(slog.NewJSONHandler(&logs, nil)), api.Options{ResponseStore: store})
	first := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var response compat.Response
	if err := json.NewDecoder(first.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	logs.Reset()
	second := doResponsesJSON(handler, `{"model":"test-model","input":"again","previous_response_id":"`+response.ID+`"}`, true)
	if second.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(logs.String(), `"previous_response":true`) || strings.Contains(logs.String(), response.ID) {
		t.Fatalf("unsafe access log: %s", logs.String())
	}
}

func TestResponsesMetricsUseResponsesPath(t *testing.T) {
	handler := newTestHandler(fake.New())
	rr := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertMetricsContains(t, handler, `open_ai_gateway_tokens_total{path="/v1/responses",model="test-model",provider="fake-provider",type="total",client="default"} 2`)
}

func TestResponsesStreamAuditsTypedEvents(t *testing.T) {
	recorder := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: recorder})
	rr := doResponsesJSON(handler, `{"model":"test-model","input":"hello","stream":true}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	foundDelta := false
	for _, event := range recorder.Events() {
		if event.Event == audit.EventStreamChunk && strings.Contains(string(event.Body), `"type":"response.output_text.delta"`) {
			foundDelta = true
		}
	}
	if !foundDelta {
		t.Fatalf("typed delta not audited: %#v", recorder.Events())
	}
}

func TestResponsesStreamFunctionCall(t *testing.T) {
	p := &functionStreamProvider{}
	rr := doResponsesJSON(newTestHandler(p), `{"model":"test-model","input":"weather","stream":true,"tools":[{"type":"function","name":"get_weather","parameters":{"type":"object"}}]}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	text := rr.Body.String()
	for _, event := range []string{"response.output_item.added", "response.function_call_arguments.delta", "response.function_call_arguments.done", "response.output_item.done", "response.completed"} {
		if !strings.Contains(text, "event: "+event+"\n") {
			t.Fatalf("missing %s: %s", event, text)
		}
	}
	if !strings.Contains(text, `"arguments":"{\"location\":\"Paris\"}"`) || !strings.Contains(text, `"call_id":"call_1"`) {
		t.Fatalf("stream=%s", text)
	}
	if !p.closed {
		t.Fatal("stream not closed")
	}
}

func TestResponsesCompletedFunctionStreamCanBeContinued(t *testing.T) {
	p := &functionStreamProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	streamed := doResponsesJSON(handler, `{"model":"test-model","input":"weather","stream":true,"tools":[{"type":"function","name":"get_weather","parameters":{"type":"object"}}]}`, true)
	if streamed.Code != http.StatusOK {
		t.Fatalf("stream status=%d body=%s", streamed.Code, streamed.Body.String())
	}
	responseID := completedResponseID(t, streamed.Body.String())
	continued := doResponsesJSON(handler, `{"model":"test-model","previous_response_id":"`+responseID+`","input":[{"type":"function_call_output","call_id":"call_1","output":"sunny"}]}`, true)
	if continued.Code != http.StatusOK {
		t.Fatalf("continued status=%d body=%s", continued.Code, continued.Body.String())
	}
	if len(p.requests) != 2 || len(p.requests[1].Messages) != 3 || p.requests[1].Messages[2].Role != "tool" {
		t.Fatalf("requests=%#v", p.requests)
	}
}

func doResponsesJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

type functionStreamProvider struct {
	closed   bool
	requests []compat.ChatCompletionRequest
}

func (p *functionStreamProvider) ListModels(context.Context) ([]compat.Model, error) { return nil, nil }
func (p *functionStreamProvider) CreateChatCompletion(_ context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	p.requests = append(p.requests, req)
	return &compat.ChatCompletionResponse{Choices: []compat.ChatCompletionChoice{{Index: 0, Message: compat.ChatMessage{Role: "assistant", Content: json.RawMessage(`"done"`)}, FinishReason: "stop"}}}, nil
}
func (p *functionStreamProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("unused")
}
func (p *functionStreamProvider) CreateCompletion(context.Context, compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	return nil, errors.New("unused")
}
func (p *functionStreamProvider) StreamCompletion(context.Context, compat.CompletionsRequest) (provider.CompletionStream, error) {
	return nil, errors.New("unused")
}
func (p *functionStreamProvider) CreateImage(ctx context.Context, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
	return nil, errors.New("not implemented")
}
func (p *functionStreamProvider) StreamChatCompletion(_ context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	p.requests = append(p.requests, req)
	return &functionStream{p: p}, nil
}

type functionStream struct {
	p     *functionStreamProvider
	index int
}

func (s *functionStream) Next(context.Context) (*compat.ChatCompletionChunk, error) {
	if s.index >= 2 {
		return nil, io.EOF
	}
	arguments := `{"location":`
	if s.index == 1 {
		arguments = `"Paris"}`
	}
	extra, _ := json.Marshal([]map[string]any{{"index": 0, "id": "call_1", "type": "function", "function": map[string]string{"name": "get_weather", "arguments": arguments}}})
	s.index++
	return &compat.ChatCompletionChunk{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{"tool_calls": extra}}}}}, nil
}
func (s *functionStream) Close() error { s.p.closed = true; return nil }
