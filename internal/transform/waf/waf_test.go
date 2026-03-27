package waf

import (
	"encoding/json"
	"testing"
)

func TestTransformer_RequestURL(t *testing.T) {
	t.Parallel()

	var tr Transformer
	acl := "test-acl"

	tests := []struct {
		name        string
		line        string
		wantURL     string
		wantMissing bool
	}{
		{
			name: "full url with args default https",
			line: `{"action":"BLOCK","httpRequest":{"uri":"/api/foo","args":"a=1","headers":[{"name":"Host","value":"example.com"}]}}`,
			wantURL: "https://example.com/api/foo?a=1",
		},
		{
			name: "x-forwarded-proto http",
			line: `{"action":"BLOCK","httpRequest":{"uri":"/p","headers":[{"name":"Host","value":"h"},{"name":"X-Forwarded-Proto","value":"http"}]}}`,
			wantURL: "http://h/p",
		},
		{
			name: "no host path and query only",
			line: `{"action":"BLOCK","httpRequest":{"uri":"/path","args":"q=1","headers":[{"name":"X-Forwarded-Proto","value":"http"}]}}`,
			wantURL: "/path?q=1",
		},
		{
			name: "no host path only",
			line: `{"action":"BLOCK","httpRequest":{"uri":"/only"}}`,
			wantURL: "/only",
		},
		{
			name: "host case insensitive",
			line: `{"action":"BLOCK","httpRequest":{"uri":"/","headers":[{"name":"host","value":"UPPER.example"}]}}`,
			wantURL: "https://UPPER.example/",
		},
		{
			name: "uri without leading slash",
			line: `{"action":"BLOCK","httpRequest":{"uri":"api","headers":[{"name":"Host","value":"x"}]}}`,
			wantURL: "https://x/api",
		},
		{
			name: "invalid proto defaults https",
			line: `{"action":"BLOCK","httpRequest":{"uri":"/","headers":[{"name":"Host","value":"x"},{"name":"X-Forwarded-Proto","value":"grpc"}]}}`,
			wantURL: "https://x/",
		},
		{
			name:        "empty uri omits request_url",
			line:        `{"action":"BLOCK","httpRequest":{"uri":"","headers":[{"name":"Host","value":"x"}]}}`,
			wantMissing: true,
		},
		{
			name:        "whitespace uri omits request_url",
			line:        `{"action":"BLOCK","httpRequest":{"uri":"  ","headers":[{"name":"Host","value":"x"}]}}`,
			wantMissing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, keep := tr.Transform(tt.line, acl)
			if !keep {
				t.Fatal("expected keep true")
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(out), &obj); err != nil {
				t.Fatalf("unmarshal out: %v", err)
			}
			got, has := obj["request_url"].(string)
			if tt.wantMissing {
				if has {
					t.Fatalf("request_url present, got %q", got)
				}
				return
			}
			if !has || got != tt.wantURL {
				t.Fatalf("request_url = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestTransformer_NoHTTPRequest_NoPanic(t *testing.T) {
	t.Parallel()
	var tr Transformer
	out, keep := tr.Transform(`{"action":"BLOCK"}`, "acl")
	if !keep {
		t.Fatal("expected keep")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["request_url"]; ok {
		t.Fatal("unexpected request_url")
	}
}

func TestTransformer_MalformedHTTPRequest_NoPanic(t *testing.T) {
	t.Parallel()
	var tr Transformer
	out, keep := tr.Transform(`{"action":"BLOCK","httpRequest":"not-an-object"}`, "acl")
	if !keep {
		t.Fatal("expected keep")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["request_url"]; ok {
		t.Fatal("unexpected request_url")
	}
}
