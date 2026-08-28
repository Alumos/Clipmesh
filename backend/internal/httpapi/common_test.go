package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestDecodeJSONAcceptsOneObject(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"clip"}`))
	response := httptest.NewRecorder()
	var body struct {
		Name string `json:"name"`
	}

	if err := decodeJSON(response, request, 1024, &body); err != nil {
		t.Fatalf("decode valid JSON: %v", err)
	}
	if body.Name != "clip" {
		t.Fatalf("decoded name = %q, want clip", body.Name)
	}
}

func TestDecodeJSONRejectsUnknownAndTrailingValues(t *testing.T) {
	tests := []string{
		`{"name":"clip","unknown":true}`,
		`{"name":"clip"} {"name":"second"}`,
	}
	for _, input := range tests {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(input))
		response := httptest.NewRecorder()
		var body struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(response, request, 1024, &body); err == nil {
			t.Fatalf("decodeJSON accepted %q", input)
		}
	}
}

func TestHealthRejectsUnsupportedMethod(t *testing.T) {
	server := &Server{startedAt: time.Now(), version: "test"}
	request := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	response := httptest.NewRecorder()

	server.health(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", allow)
	}
}

func TestSafeFilenamePreservesUnicodeAndExtension(t *testing.T) {
	filename := safeFilename(`..\folder\` + strings.Repeat("剪", 100) + ".png")
	if !utf8.ValidString(filename) {
		t.Fatalf("filename is not valid UTF-8: %q", filename)
	}
	if len(filename) > 240 {
		t.Fatalf("filename uses %d bytes, want at most 240", len(filename))
	}
	if !strings.HasSuffix(filename, ".png") {
		t.Fatalf("filename %q lost its extension", filename)
	}
	if strings.ContainsAny(filename, `/\`) {
		t.Fatalf("filename still contains a path separator: %q", filename)
	}
}
