// spec: AI Workspace compatible service discovery, EV-UI-230.
package aiworkspace

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestModelsUseTheConfiguredBasePath(t *testing.T) {
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/compatible/v1/models" || r.Header.Get("Authorization") != "" {
			t.Error("discovery did not perform an unauthenticated GET at the configured base path")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"fixture-beta"},{"id":"fixture-alpha"}]}`)
	}))
	defer service.Close()

	result, err := Models(t.Context(), Config{BaseURL: " " + service.URL + "/compatible/v1/ "})
	if err != nil || !slices.Equal(result, []string{"fixture-alpha", "fixture-beta"}) {
		t.Fatalf("Models = %v, %v", result, err)
	}
}
