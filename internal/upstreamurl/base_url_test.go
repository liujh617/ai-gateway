package upstreamurl

import (
	"strings"
	"testing"
)

func TestNormalizeHTTPBaseURL(t *testing.T) {
	t.Parallel()

	baseURL, err := NormalizeHTTPBaseURL("  https://api.example.com/v1///  ")
	if err != nil {
		t.Fatalf("normalize base URL: %v", err)
	}
	if baseURL != "https://api.example.com/v1" {
		t.Fatalf("baseURL = %q", baseURL)
	}
}

func TestNormalizeHTTPBaseURLRejectsInvalidURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name: "empty",
			want: "required",
		},
		{
			name:    "missing host",
			baseURL: "https:///v1",
			want:    "http or https",
		},
		{
			name:    "unsupported scheme",
			baseURL: "ftp://api.example.com/v1",
			want:    "http or https",
		},
		{
			name:    "query",
			baseURL: "https://api.example.com/v1?tenant=one",
			want:    "query or fragment",
		},
		{
			name:    "fragment",
			baseURL: "https://api.example.com/v1#models",
			want:    "query or fragment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeHTTPBaseURL(tt.baseURL)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err, tt.want)
			}
		})
	}
}
