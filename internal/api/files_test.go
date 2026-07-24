package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/fake"
)

func newFileFormRequest(path string, fields map[string]string, fileData []byte, filename string) *http.Request {
	b, ct := newFileFormData(fields, fileData, filename)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", ct)
	return req
}

func newFileFormData(fields map[string]string, fileData []byte, filename string) ([]byte, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	if fileData != nil {
		fw, _ := w.CreateFormFile("file", filename)
		_, _ = fw.Write(fileData)
	}
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func doFileUpload(t *testing.T, handler http.Handler, purpose string, fileData []byte, filename string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := newFileFormRequest("/v1/files", map[string]string{"purpose": purpose}, fileData, filename)
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func doFileRequest(handler http.Handler, method, path string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestUploadFileOK(t *testing.T) {
	rr := doFileUpload(t, newTestHandler(fake.New()), "fine-tune", []byte("file-content"), "test.jsonl", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var fobj compat.FileObject
	if err := json.NewDecoder(rr.Body).Decode(&fobj); err != nil {
		t.Fatal(err)
	}
	if fobj.ID != "file_fake" || fobj.Purpose != "fine-tune" || fobj.Filename != "test.jsonl" {
		t.Fatalf("file=%#v", fobj)
	}
}

func TestUploadFileMissingPurpose(t *testing.T) {
	rr := doFileUpload(t, newTestHandler(fake.New()), "", []byte("data"), "test.jsonl", true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestUploadFileMissingFile(t *testing.T) {
	req := newFileFormRequest("/v1/files", map[string]string{"purpose": "fine-tune"}, nil, "")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	newTestHandler(fake.New()).ServeHTTP(rr, req)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestUploadFileRequiresAuth(t *testing.T) {
	rr := doFileUpload(t, newTestHandler(fake.New()), "fine-tune", []byte("data"), "test.jsonl", false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestListFilesOK(t *testing.T) {
	rr := doFileRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/files", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var list compat.FileList
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Object != "list" {
		t.Fatalf("object=%q", list.Object)
	}
}

func TestRetrieveFileOK(t *testing.T) {
	rr := doFileRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/files/file_fake", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var fobj compat.FileObject
	if err := json.NewDecoder(rr.Body).Decode(&fobj); err != nil {
		t.Fatal(err)
	}
	if fobj.ID != "file_fake" {
		t.Fatalf("file=%#v", fobj)
	}
}

func TestDeleteFileOK(t *testing.T) {
	rr := doFileRequest(newTestHandler(fake.New()), http.MethodDelete, "/v1/files/file_fake", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp compat.FileDeleteResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "file_fake" || !resp.Deleted {
		t.Fatalf("response=%#v", resp)
	}
}

func TestDownloadFileOK(t *testing.T) {
	rr := doFileRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/files/file_fake/content", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	data, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fake-file-content" {
		t.Fatalf("data=%q", string(data))
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Fatalf("content-type=%q", ct)
	}
}

func TestFilesMethodNotAllowed(t *testing.T) {
	rr := doFileRequest(newTestHandler(fake.New()), http.MethodPut, "/v1/files", true)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
}

func TestRetrieveFileRequiresAuth(t *testing.T) {
	rr := doFileRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/files/file_fake", false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestFilesWrongCapability(t *testing.T) {
	rr := doFileRequest(newTestHandler(fake.New()), http.MethodGet, "/v1/files", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
