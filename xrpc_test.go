package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestXrpcResponseBody(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
		body        []byte
		want        []byte
	}{
		{
			name:        "mp4",
			contentType: "video/mp4",
			body:        []byte("\x00\x00\x00\x18ftypmp42\x00\xff\x80"),
			want:        []byte("\x00\x00\x00\x18ftypmp42\x00\xff\x80"),
		},
		{
			name:        "text",
			contentType: "text/plain; charset=utf-8",
			body:        []byte("not JSON\n"),
			want:        []byte("not JSON\n"),
		},
		{
			name:        "empty binary",
			contentType: "application/octet-stream",
		},
		{
			name:        "json with parameters",
			contentType: "Application/JSON; charset=utf-8",
			body:        []byte(`{"ok":true}`),
			want:        []byte("{\n  \"ok\": true\n}\n"),
		},
		{
			name:        "json suffix",
			contentType: "application/vnd.example+json",
			body:        []byte(`{"ok":true}`),
			want:        []byte("{\n  \"ok\": true\n}\n"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()

			output, err := os.CreateTemp(t.TempDir(), "response-")
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			stdout := os.Stdout
			os.Stdout = output
			defer func() { os.Stdout = stdout }()

			if err := run([]string{"goat", "xrpc", "query", server.URL, "com.example.getThing"}); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("response body = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestXrpcOutputFile(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        []byte
		wantErr     bool
	}{
		{
			name:        "binary overwrites existing file",
			status:      http.StatusOK,
			contentType: "video/mp4",
			body:        []byte("\x00\x00\x00\x18ftypmp42\x00\xff\x80"),
		},
		{
			name:        "JSON saved without formatting",
			status:      http.StatusOK,
			contentType: "application/json",
			body:        []byte(`{"ok":true}`),
		},
		{
			name:        "HTTP error preserves existing file",
			status:      http.StatusForbidden,
			contentType: "application/json",
			body:        []byte(`{"error":"Forbidden","message":"denied"}`),
			wantErr:     true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()

			path := t.TempDir() + "/response"
			existing := bytes.Repeat([]byte("existing file contents"), 10)
			if err := os.WriteFile(path, existing, 0600); err != nil {
				t.Fatal(err)
			}
			err := run([]string{"goat", "xrpc", "-o", path, "query", server.URL, "com.example.getThing"})
			if (err != nil) != tc.wantErr {
				t.Fatalf("request error = %v, want error: %v", err, tc.wantErr)
			}
			want := tc.body
			if tc.wantErr {
				want = existing
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("output file = %q, want %q", got, want)
			}
		})
	}
}
