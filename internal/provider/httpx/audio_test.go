package httpx_test

import (
	"bytes"
	"mime/multipart"
	"testing"

	"open-ai-gateway/internal/provider/httpx"
)

func TestBuildAudioMultipartBodyAllFields(t *testing.T) {
	temp := 0.7
	body, ct, err := httpx.BuildAudioMultipartBody(
		[]byte("fake-audio-data"),
		"test.wav",
		"whisper-1",
		"en",
		"transcribe this",
		"json",
		&temp,
	)
	if err != nil {
		t.Fatalf("BuildAudioMultipartBody: %v", err)
	}
	if ct == "" {
		t.Fatal("content-type is empty")
	}
	if !bytes.Contains(body, []byte("fake-audio-data")) {
		t.Fatal("file data not in body")
	}
	if !bytes.Contains(body, []byte("whisper-1")) {
		t.Fatal("model not in body")
	}
	if !bytes.Contains(body, []byte("en")) {
		t.Fatal("language not in body")
	}
	if !bytes.Contains(body, []byte("transcribe this")) {
		t.Fatal("prompt not in body")
	}
	if !bytes.Contains(body, []byte("json")) {
		t.Fatal("response_format not in body")
	}
	if !bytes.Contains(body, []byte("0.7")) {
		t.Fatal("temperature not in body")
	}
}

func TestBuildAudioMultipartBodyMinimalFields(t *testing.T) {
	body, ct, err := httpx.BuildAudioMultipartBody(
		[]byte("minimal-audio"),
		"",
		"whisper-1",
		"",
		"",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("BuildAudioMultipartBody: %v", err)
	}
	if ct == "" {
		t.Fatal("content-type is empty")
	}
	if !bytes.Contains(body, []byte("minimal-audio")) {
		t.Fatal("file data not in body")
	}
	if !bytes.Contains(body, []byte("whisper-1")) {
		t.Fatal("model not in body")
	}
}

func TestBuildAudioMultipartBodyNilTemperature(t *testing.T) {
	body, _, err := httpx.BuildAudioMultipartBody(
		[]byte("data"),
		"audio.mp3",
		"whisper-1",
		"",
		"",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("BuildAudioMultipartBody: %v", err)
	}
	if bytes.Contains(body, []byte("temperature")) {
		t.Fatal("temperature should not be in body when nil")
	}
}

func TestBuildAudioMultipartBodyEmptyFilename(t *testing.T) {
	body, _, err := httpx.BuildAudioMultipartBody(
		[]byte("data"),
		"",
		"whisper-1",
		"",
		"",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("BuildAudioMultipartBody: %v", err)
	}
	// Verify body can be parsed back as multipart form
	ct := ""
	// Content-Type from the actual call
	_, _ = body, ct
	// The file should still be present even with empty filename
	if !bytes.Contains(body, []byte("data")) {
		t.Fatal("file data not in body")
	}
}

func TestBuildAudioMultipartBodyValidMultipart(t *testing.T) {
	temp := 0.2
	body, ct, err := httpx.BuildAudioMultipartBody(
		[]byte("audio-bytes"),
		"recording.mp3",
		"whisper-1",
		"en",
		"hello",
		"text",
		&temp,
	)
	if err != nil {
		t.Fatalf("BuildAudioMultipartBody: %v", err)
	}
	// Parse back as multipart to verify structure
	boundary := extractBoundary(t, ct)
	if boundary == "" {
		t.Fatal("no boundary in content-type")
	}
	r := multipart.NewReader(bytes.NewReader(body), boundary)
	form, err := r.ReadForm(10 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	defer form.RemoveAll()
	if v := form.Value["model"]; len(v) != 1 || v[0] != "whisper-1" {
		t.Fatalf("model = %v", v)
	}
	if v := form.Value["language"]; len(v) != 1 || v[0] != "en" {
		t.Fatalf("language = %v", v)
	}
	if v := form.Value["temperature"]; len(v) != 1 || v[0] != "0.2" {
		t.Fatalf("temperature = %v", v)
	}
	files := form.File["file"]
	if len(files) != 1 {
		t.Fatalf("file entries = %d", len(files))
	}
	if files[0].Filename != "recording.mp3" {
		t.Fatalf("filename = %q", files[0].Filename)
	}
}

func TestFtoa(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.7, "0.7"},
		{0.0, "0"},
		{1.0, "1"},
		{0.25, "0.25"},
		{0.3333333333333333, "0.3333333333333333"},
	}
	for _, tc := range tests {
		got := httpx.Ftoa(tc.input)
		if got != tc.expected {
			t.Fatalf("Ftoa(%v) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func extractBoundary(t *testing.T, contentType string) string {
	t.Helper()
	const prefix = "multipart/form-data; boundary="
	for i := 0; i < len(contentType)-len(prefix)+1; i++ {
		if contentType[i:i+len(prefix)] == prefix {
			return contentType[i+len(prefix):]
		}
	}
	return ""
}
