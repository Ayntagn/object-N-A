package aiworkspace

// spec: EV-UI-287 / EV-UI-288 / EV-MYGO-193

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAnthropicCompletionWireFormat(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		request  completionRequest
		system   string
		turns    []Message
		max      int
		keyCount int
	}{
		{
			name: "ordinary chat",
			request: completionRequest{Model: "claude-local", Stream: true, Messages: []Message{
				{Role: "system", Content: "Writing instructions"},
				{Role: "user", Content: "Earlier turn"},
				{Role: "assistant", Content: "Earlier reply"},
				{Role: "system", Content: ""},
				{Role: "system", Content: "Selected β 🧭  \n"},
				{Role: "user", Content: "Current turn"},
			}},
			system: "Writing instructions\n\n\n\nSelected β 🧭  \n",
			turns: []Message{
				{Role: "user", Content: "Earlier turn"},
				{Role: "assistant", Content: "Earlier reply"},
				{Role: "user", Content: "Current turn"},
			},
			max: 4096, keyCount: 5,
		},
		{
			name: "test connection",
			request: completionRequest{Model: "claude-local", MaxTokens: 5, Messages: []Message{
				{Role: "system", Content: "Respond with just 'ok'."},
				{Role: "user", Content: "Hi"},
			}},
			system: "Respond with just 'ok'.",
			turns:  []Message{{Role: "user", Content: "Hi"}},
			max:    5, keyCount: 4,
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			payload, err := encodeAnthropicCompletion(scenario.request)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Model    string    `json:"model"`
				Messages []Message `json:"messages"`
				System   string    `json:"system"`
				Max      int       `json:"max_tokens"`
				Stream   *bool     `json:"stream"`
			}
			if err := json.Unmarshal(payload, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.Model != scenario.request.Model || wire.Max != scenario.max || wire.System != scenario.system || !reflect.DeepEqual(wire.Messages, scenario.turns) {
				t.Fatalf("unexpected request body %s", payload)
			}
			if scenario.request.Stream && (wire.Stream == nil || !*wire.Stream) || !scenario.request.Stream && wire.Stream != nil {
				t.Fatalf("unexpected stream field in %s", payload)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(payload, &fields); err != nil || len(fields) != scenario.keyCount {
				t.Fatalf("unexpected request fields in %s (%v)", payload, err)
			}
		})
	}
}

func TestAnthropicResponseBoundaries(t *testing.T) {
	const first = "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi β\"}}\n\n"
	const second = "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" 🧭\"}}\n\n"
	const stopping = "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n"
	const done = "data: {\"type\":\"message_stop\"}\n\n"
	const serviceError = "data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"fixture service details\"}}\n\n"
	const metadata = "data: {\"type\":\"ping\"}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"omit\"}}\n\n"

	text, err := decodeAnthropicReply([]byte(`{"type":"message","content":[{"type":"text","text":"Hi β 🧭"},{"type":"text","text":"Later block"}]}`))
	if err != nil || text != "Hi β 🧭" {
		t.Fatalf("single reply = %q, %v", text, err)
	}
	startedAt := time.Now().Add(-time.Second)
	var updates []string
	reply, err := readAnthropicChatStream(context.Background(), strings.NewReader(metadata+first+second+stopping+done+serviceError), startedAt, func(delta string, progress StreamProgress) {
		updates = append(updates, delta)
		if progress.MetricsAt(time.Now()).TokensPerSecond <= 0 {
			t.Fatal("missing progress metrics")
		}
	})
	if err != nil || reply.Text != "Hi β 🧭" || reply.Metrics.TokensPerSecond <= 0 || !reflect.DeepEqual(updates, []string{"Hi β", " 🧭"}) {
		t.Fatalf("completed stream = %#v, %v; updates %q", reply, err, updates)
	}
	reply, err = readAnthropicChatStream(context.Background(), strings.NewReader(first+stopping+second), startedAt, nil)
	if err != nil || reply.Text != "Hi β 🧭" || reply.Metrics.TokensPerSecond <= 0 {
		t.Fatalf("clean EOF stream = %#v, %v", reply, err)
	}
	for _, scenario := range []struct {
		label, wire string
		cancel      bool
		wantError   error
	}{
		{label: "server error", wire: first + serviceError, wantError: errAnthropicStream},
		{label: "user cancellation", wire: first + second + done, cancel: true, wantError: context.Canceled},
	} {
		t.Run(scenario.label, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reply, err := readAnthropicChatStream(ctx, strings.NewReader(scenario.wire), startedAt, func(string, StreamProgress) {
				if scenario.cancel {
					cancel()
				}
			})
			if !errors.Is(err, scenario.wantError) || reply.Text != "Hi β" {
				t.Fatalf("partial stream = %#v, %v; want first delta with %v", reply, err, scenario.wantError)
			}
		})
	}
}
