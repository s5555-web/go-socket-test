package api

import "testing"

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		allowed string
		origin  string
		want    bool
	}{
		{name: "wildcard", allowed: "*", origin: "https://example.com", want: true},
		{name: "exact", allowed: "https://example.com", origin: "https://example.com", want: true},
		{name: "trailing slash", allowed: "https://example.com/", origin: "https://example.com", want: true},
		{name: "list", allowed: "https://one.example, https://two.example", origin: "https://two.example", want: true},
		{name: "reject different origin", allowed: "https://example.com", origin: "https://evil.example", want: false},
		{name: "server request without origin", allowed: "https://example.com", origin: "", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := originAllowed(test.allowed, test.origin); got != test.want {
				t.Fatalf("originAllowed(%q, %q) = %v, want %v", test.allowed, test.origin, got, test.want)
			}
		})
	}
}
