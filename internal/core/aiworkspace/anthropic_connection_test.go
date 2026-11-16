// spec: EV-UI-287 / EV-UI-288. Claude uses its native Models and Messages routes.
package aiworkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAnthropicConnectionUsesNativeAPI(t *testing.T) {
	const key = "fixture-api-key"
	var modelRequests atomic.Int32
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != anthropicHost || r.Header.Get("x-api-key") != key ||
			r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("Authorization") != "" {
			t.Error("Claude request did not use the original host and native authentication headers")
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/models":
			if modelRequests.Add(1) == 1 {
				_, _ = io.WriteString(w, `{"data":[]}`)
			} else {
				_, _ = io.WriteString(w, `{"data":[{"id":"fixture-zeta"},{"id":"fixture-alpha"}]}`)
			}
		case "POST /v1/messages":
			var body struct {
				Model     string    `json:"model"`
				System    string    `json:"system"`
				Messages  []Message `json:"messages"`
				MaxTokens int       `json:"max_tokens"`
				Stream    *bool     `json:"stream"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error("Claude request was not JSON", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.MaxTokens == 5 {
				if body.Model != "" || body.Stream != nil || body.System != "Respond with just 'ok'." ||
					!slices.Equal(body.Messages, []Message{{Role: "user", Content: "Hi"}}) {
					t.Error("Claude Test did not send the short probe with an empty model field")
				}
				_, _ = io.WriteString(w, `{"model":"fixture-tested"}`)
				return
			}
			if body.Model != "fixture-alpha" || body.MaxTokens != 4096 || body.Stream == nil || !*body.Stream ||
				body.System != "Write briefly.\n\nDocument marker." ||
				!slices.Equal(body.Messages, []Message{{Role: "user", Content: "Rewrite"}}) {
				t.Error("Claude Chat did not split its system instructions from the conversation")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Fixture reply\"}}\n\n"+
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		default:
			t.Errorf("unexpected Claude method and path %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(fixture.Close)
	previous := serviceClient
	client := *previous
	transport := fixture.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "127.0.0.1"
	dialer := net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != anthropicHost+":443" {
			return nil, fmt.Errorf("non-fixture dial blocked: %s", address)
		}
		return dialer.DialContext(ctx, network, fixture.Listener.Addr().String())
	}
	client.Transport = transport
	serviceClient = &client
	t.Cleanup(func() { serviceClient = previous; transport.CloseIdleConnections() })

	config := Config{BaseURL: " https://API.ANTHROPIC.COM/v1/ ", APIKey: key}
	models, err := Models(t.Context(), config)
	if err != nil || !slices.Equal(models, []string{"fixture-alpha", "fixture-zeta"}) || modelRequests.Load() != 2 {
		t.Fatalf("Claude model refresh = %v, %v (%d requests)", models, err, modelRequests.Load())
	}
	if model, err := Test(t.Context(), config); err != nil || model != "fixture-tested" {
		t.Fatalf("Claude Test = %q, %v", model, err)
	}
	config.Model = models[0]
	var streamed strings.Builder
	reply, err := Chat(t.Context(), config, []Message{
		{Role: "system", Content: "Write briefly."}, {Role: "system", Content: "Document marker."},
		{Role: "user", Content: "Rewrite"},
	}, func(delta string, _ StreamProgress) { streamed.WriteString(delta) })
	if err != nil || reply.Text != "Fixture reply" || streamed.String() != reply.Text {
		t.Fatalf("Claude Chat = %+v, %v; stream %q", reply, err, streamed.String())
	}
}
