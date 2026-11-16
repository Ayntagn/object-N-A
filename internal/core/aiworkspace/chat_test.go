// spec: AI Workspace test, chat and SSE measurements, EV-UI-230 / EV-UI-231 / EV-UI-242 / EV-UI-257.
package aiworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestChatDeliversIncrementalSSEAndJSON(t *testing.T) {
	const first = "Cafe\u0301👩🏽‍💻"
	deltaDelivered := make(chan struct{})
	history := []Message{
		{Role: "system", Content: "fixture instructions"},
		{Role: "system", Content: "fixture context"},
		{Role: "user", Content: "本地回复"},
	}
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var prompt struct {
			Model     string
			Messages  []Message
			Stream    bool
			MaxTokens int `json:"max_tokens"`
		}
		payload, err := io.ReadAll(r.Body)
		var fields map[string]json.RawMessage
		if r.Method != http.MethodPost || r.URL.Path != "/local/v1/chat/completions" ||
			r.Header.Get("Content-Type") != "application/json" || err != nil ||
			json.Unmarshal(payload, &prompt) != nil || json.Unmarshal(payload, &fields) != nil ||
			fields["stream"] == nil || prompt.Model != "fixture" {
			t.Error("completion did not send the configured model and endpoint")
		}
		if prompt.MaxTokens != 0 {
			if len(fields) != 4 || prompt.MaxTokens != 5 || prompt.Stream || !slices.Equal(prompt.Messages, []Message{{Role: "user", Content: "Hi"}}) {
				t.Error("the connection test should send the short, non-streaming native probe")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"model":"service-reported-model"}`)
			return
		} else if len(fields) != 3 || !slices.Equal(prompt.Messages, history) {
			t.Error("chat did not send the expected conversation without extra fields")
		}
		if !prompt.Stream || r.URL.Query().Get("reply") == "json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"本地回复完成"}}],"usage":{"completion_tokens":128}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = io.WriteString(w, ": keepalive\n\ndata: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\""+first+"\"}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-deltaDelivered:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"abc\"}}]}\n\n"+
			"data: {\"choices\":[{\"delta\":{\"content\":\"de\"}}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"completion_tokens\":128}}\n\ndata: [DONE]\n\n")
	}))
	defer service.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	config := Config{BaseURL: service.URL + "/local/v1", Model: "fixture"}
	if model, err := Test(ctx, config); err != nil || model != "service-reported-model" {
		t.Fatalf("Test = %q, %v", model, err)
	}
	var deltas []string
	var lastProgress StreamProgress
	got, err := Chat(ctx, config, history, func(text string, progress StreamProgress) {
		deltas = append(deltas, text)
		lastProgress = progress
		if len(deltas) == 1 {
			close(deltaDelivered)
		}
	})
	if err != nil || got.Text != first+"abcde" || !slices.Equal(deltas, []string{first, "abc", "de"}) {
		t.Fatalf("streamed chat = %q, deltas %v, error %v", got.Text, deltas, err)
	}
	// Each of the three text deltas contributes one estimated token. Counting
	// Unicode scalars or measuring the joined reply would give a different rate.
	measured := lastProgress.MetricsAt(lastProgress.startedAt.Add(1500 * time.Millisecond))
	if measured.TokensPerSecond != 2 || measured.TimeToFirstToken <= 0 ||
		got.Metrics.TimeToFirstToken != measured.TimeToFirstToken || got.Metrics.TokensPerSecond <= 0 {
		t.Errorf("streamed chat measurements = %+v, final = %+v", measured, got.Metrics)
	}
	if got, err := Chat(ctx, config, history, nil); err != nil || got.Text != "本地回复完成" || got.Metrics != (ReplyMetrics{}) {
		t.Errorf("JSON chat = %+v, error %v", got, err)
	}
	config.BaseURL += "?reply=json"
	var jsonDelta string
	got, err = Chat(ctx, config, history, func(text string, progress StreamProgress) {
		jsonDelta += text
		if progress != (StreamProgress{}) {
			t.Errorf("JSON fallback reported stream progress: %+v", progress)
		}
	})
	if err != nil || got.Text != "本地回复完成" || jsonDelta != got.Text || got.Metrics != (ReplyMetrics{}) {
		t.Errorf("JSON fallback = %+v, delta %q, error %v", got, jsonDelta, err)
	}
}

func TestConnectionProbeFeedback(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "empty success", status: 200},
		{name: "success without model", status: 200, body: `{"choices":[]}`},
		{name: "created", status: 201, want: "HTTP 201"},
		{name: "no content", status: 204, want: "HTTP 204"},
		{name: "unauthorized", status: 401, want: "Authentication failed - check API key"},
		{name: "forbidden", status: 403, want: "Access denied - check API key permissions"},
		{name: "missing endpoint", status: 404, want: "Endpoint not found - check base URL and model name"},
		{name: "invalid request", status: 422, want: "Invalid request - model may not exist"},
		{name: "rate limited", status: 429, want: "Rate limited - try again later"},
		{name: "server error", status: 500, body: "private reply", want: "Server error (500)"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(scenario.status)
				_, _ = io.WriteString(w, scenario.body)
			}))
			defer service.Close()

			model, err := Test(t.Context(), Config{BaseURL: service.URL, Model: "fixture"})
			got := ""
			if err != nil {
				got = err.Error()
			}
			if model != "" || got != scenario.want {
				t.Errorf("unexpected model or feedback %q, %q", model, got)
			}
		})
	}
}

func TestChatCancellationAndStatusDoNotDiscloseCredentials(t *testing.T) {
	const syntheticKey = "fixture-token"
	var foreignHits atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { foreignHits.Add(1) }))
	defer foreign.Close()
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+syntheticKey || r.URL.Query().Get("probe") != "private-query" {
			t.Error("request did not keep the configured key in its header and preserve the base query")
		}
		switch r.URL.Path {
		case "/redirect/chat/completions":
			http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
		case "/failure/chat/completions":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, "private body echoed "+syntheticKey+r.URL.RawQuery)
		case "/stream/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	defer service.Close()

	ctx, timeout := context.WithTimeout(t.Context(), 5*time.Second)
	defer timeout()
	history := []Message{{Role: "user", Content: "probe"}}
	for prefix, status := range map[string]int{"redirect": 307, "failure": 401} {
		config := Config{BaseURL: service.URL + "/" + prefix + "?probe=private-query", Model: "fixture", APIKey: syntheticKey}
		_, err := Chat(ctx, config, history, nil)
		var result *StatusError
		if !errors.As(err, &result) || result.StatusCode != status || strings.Contains(result.Error(), syntheticKey) ||
			strings.Contains(result.Error(), "private-query") || strings.Contains(result.Error(), "private body") {
			t.Error("unexpected or unsanitized status error")
		}
	}
	if foreignHits.Load() != 0 {
		t.Error("completion followed a cross-origin redirect")
	}

	config := Config{BaseURL: service.URL + "/stream?probe=private-query", Model: "fixture", APIKey: syntheticKey}
	chatCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stopped ReplyMetrics
	reply, err := Chat(chatCtx, config, history, func(_ string, progress StreamProgress) {
		stopped = progress.MetricsAt(progress.startedAt.Add(time.Second))
		cancel()
	})
	if !errors.Is(err, context.Canceled) || reply.Text != "partial" || reply.Metrics != (ReplyMetrics{}) ||
		stopped.TokensPerSecond != 1 || stopped.TimeToFirstToken <= 0 {
		t.Errorf("canceled completion = %+v, %v, stopping snapshot = %+v", reply, err, stopped)
	}
}
