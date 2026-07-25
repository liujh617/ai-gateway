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

func doFineTuningJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/fine_tuning/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func doFineTuningRequest(handler http.Handler, method, path string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestCreateFineTuningJobOK(t *testing.T) {
	rr := doFineTuningJSON(newTestHandler(fake.New()), `{"model":"gpt-4o-mini","training_file":"file_123"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var job compat.FineTuningJob
	if err := json.NewDecoder(rr.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.ID != "ftjob_fake" || job.Status != "validating_files" {
		t.Fatalf("job=%#v", job)
	}
}

func TestCreateFineTuningJobMissingModel(t *testing.T) {
	rr := doFineTuningJSON(newTestHandler(fake.New()), `{"training_file":"file_123"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCreateFineTuningJobMissingTrainingFile(t *testing.T) {
	rr := doFineTuningJSON(newTestHandler(fake.New()), `{"model":"gpt-4o-mini"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCreateFineTuningJobRequiresAuth(t *testing.T) {
	rr := doFineTuningJSON(newTestHandler(fake.New()), `{"model":"gpt-4o-mini","training_file":"file_123"}`, false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestListFineTuningJobsOK(t *testing.T) {
	rr := doFineTuningRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/fine_tuning/jobs", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var list compat.FineTuningJobList
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Object != "list" {
		t.Fatalf("object=%q", list.Object)
	}
}

func TestRetrieveFineTuningJobOK(t *testing.T) {
	rr := doFineTuningRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/fine_tuning/jobs/ftjob_fake", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var job compat.FineTuningJob
	if err := json.NewDecoder(rr.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.ID != "ftjob_fake" {
		t.Fatalf("job=%#v", job)
	}
}

func TestCancelFineTuningJobOK(t *testing.T) {
	rr := doFineTuningRequest(newTestHandler(fake.New()), http.MethodPost, "/v1/fine_tuning/jobs/ftjob_fake/cancel", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var job compat.FineTuningJob
	if err := json.NewDecoder(rr.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.Status != "cancelled" {
		t.Fatalf("status=%q", job.Status)
	}
}

func TestListFineTuningJobEventsOK(t *testing.T) {
	rr := doFineTuningRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/fine_tuning/jobs/ftjob_fake/events", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var events compat.FineTuningJobEventList
	if err := json.NewDecoder(rr.Body).Decode(&events); err != nil {
		t.Fatal(err)
	}
	if events.Object != "list" {
		t.Fatalf("object=%q", events.Object)
	}
}

func TestListFineTuningJobCheckpointsOK(t *testing.T) {
	rr := doFineTuningRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/fine_tuning/jobs/ftjob_fake/checkpoints", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var checkpoints compat.FineTuningJobCheckpointList
	if err := json.NewDecoder(rr.Body).Decode(&checkpoints); err != nil {
		t.Fatal(err)
	}
	if checkpoints.Object != "list" {
		t.Fatalf("object=%q", checkpoints.Object)
	}
}

func TestFineTuningJobsMethodNotAllowed(t *testing.T) {
	rr := doFineTuningRequest(newTestHandler(fake.New()), http.MethodPut, "/v1/fine_tuning/jobs", true)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
}
