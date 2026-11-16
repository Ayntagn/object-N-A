// spec: EV-UI-280. Local AI Services probes the provider's model route.
package aiworkspace

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestScanLocalServices(t *testing.T) {
	services := LocalServices()
	fixtures := []struct {
		name, route, body string
		status            int
		models            []string
	}{
		{"Ollama", "/api/tags", `{"models":[{"name":"zeta:latest"},{"name":"alpha:8b"}]}`, http.StatusOK, []string{"zeta:latest", "alpha:8b"}},
		{"LM Studio", "/v1/models", `{"data":[{"id":"local-zeta"},{"id":"local-alpha"}]}`, http.StatusOK, []string{"local-zeta", "local-alpha"}},
		{"MLX (mlx-lm)", "/v1/models", `{"data":[]}`, http.StatusOK, nil},
		{"vLLM", "/v1/models", `{"error":"offline"}`, http.StatusServiceUnavailable, nil},
	}
	if len(services) != len(fixtures) {
		t.Fatalf("got %d local services", len(services))
	}
	for i, fixture := range fixtures {
		if services[i].Name != fixture.name {
			t.Fatalf("service %d = %q, want %q", i, services[i].Name, fixture.name)
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != fixture.route || r.Header.Get("Authorization") != "" {
				t.Errorf("%s did not use an unauthenticated GET at %s", fixture.name, fixture.route)
			}
			w.WriteHeader(fixture.status)
			_, _ = io.WriteString(w, fixture.body)
		}))
		t.Cleanup(server.Close)
		services[i].Address = server.Listener.Addr().String()
		services[i].BaseURL = server.URL + "/v1"
	}
	results := make(map[string]LocalServiceResult, len(services))
	scanLocalServices(t.Context(), services, func(result LocalServiceResult) {
		results[result.Service.Name] = result
	})
	for i, fixture := range fixtures {
		result := results[fixture.name]
		if result.Service != services[i] || result.Online != (fixture.status == http.StatusOK) || !slices.Equal(result.Models, fixture.models) {
			t.Errorf("%s: got %+v, expected online=%t models=%v", fixture.name, result, fixture.status == http.StatusOK, fixture.models)
		}
	}
}
