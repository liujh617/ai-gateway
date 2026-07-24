package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/fake"
)

func TestCreateBatchOK(t *testing.T) {
	rr := doBatchesJSON(newTestHandler(fake.New()), `{"input_file_id":"file_123","endpoint":"/v1/chat/completions","completion_window":"24h"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var batch compat.Batch
	if err := json.NewDecoder(rr.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	if batch.ID != "batch_fake" || batch.Status != "validating" {
		t.Fatalf("batch=%#v", batch)
	}
}

func TestCreateBatchMissingInputFileID(t *testing.T) {
	rr := doBatchesJSON(newTestHandler(fake.New()), `{"endpoint":"/v1/chat/completions","completion_window":"24h"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCreateBatchMissingEndpoint(t *testing.T) {
	rr := doBatchesJSON(newTestHandler(fake.New()), `{"input_file_id":"file_123","completion_window":"24h"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCreateBatchRequiresAuth(t *testing.T) {
	rr := doBatchesJSON(newTestHandler(fake.New()), `{"input_file_id":"file_123","endpoint":"/v1/chat/completions","completion_window":"24h"}`, false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestListBatchesOK(t *testing.T) {
	rr := doBatchesRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/batches", "", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var list compat.BatchList
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Object != "list" {
		t.Fatalf("object=%q", list.Object)
	}
}

func TestRetrieveBatchOK(t *testing.T) {
	rr := doBatchesRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/batches/batch_fake", "", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var batch compat.Batch
	if err := json.NewDecoder(rr.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	if batch.ID != "batch_fake" {
		t.Fatalf("batch=%#v", batch)
	}
}

func TestCancelBatchOK(t *testing.T) {
	rr := doBatchesRequest(newTestHandler(fake.New()), http.MethodPost, "/v1/batches/batch_fake/cancel", "", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var batch compat.Batch
	if err := json.NewDecoder(rr.Body).Decode(&batch); err != nil {
		t.Fatal(err)
	}
	if batch.Status != "cancelling" {
		t.Fatalf("status=%q", batch.Status)
	}
}

func TestBatchesMethodNotAllowed(t *testing.T) {
	rr := doBatchesRequest(newTestHandler(fake.New()), http.MethodPut, "/v1/batches", "", true)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
}

func doBatchesJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	return doBatchesRequest(handler, http.MethodPost, "/v1/batches", body, auth)
}

func doBatchesRequest(handler http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}
