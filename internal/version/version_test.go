package version

import "testing"

func TestUserAgent(t *testing.T) {
	oldVersion := Version
	t.Cleanup(func() {
		Version = oldVersion
	})

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "normal", in: "0.1.0", want: "open-ai-gateway/0.1.0"},
		{name: "empty", in: "  ", want: "open-ai-gateway/dev"},
		{name: "spaces", in: "release 1", want: "open-ai-gateway/release_1"},
		{name: "unicode", in: "版本1", want: "open-ai-gateway/__1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.in
			if got := UserAgent(); got != tt.want {
				t.Fatalf("UserAgent() = %q, want %q", got, tt.want)
			}
		})
	}
}
