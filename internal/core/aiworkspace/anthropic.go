package aiworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/go-text/typesetting/segmenter"
)

const anthropicChatMaxTokens = 4096

var errAnthropicStream = errors.New("AI service reported a streaming error")

func encodeAnthropicCompletion(request completionRequest) ([]byte, error) {
	var system []string
	messages := make([]Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		switch message.Role {
		case "system":
			system = append(system, message.Content)
		case "user", "assistant":
			messages = append(messages, message)
		default:
			return nil, errInvalidResponse
		}
	}
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = anthropicChatMaxTokens
	}
	// EV-UI-287 / EV-UI-288. The default prompt leads the top-level system
	// string, followed by each retained system message with a blank line.
	return json.Marshal(struct {
		Model     string    `json:"model"`
		Messages  []Message `json:"messages"`
		System    string    `json:"system"`
		MaxTokens int       `json:"max_tokens"`
		Stream    bool      `json:"stream,omitempty"`
	}{request.Model, messages, strings.Join(system, "\n\n"), maxTokens, request.Stream})
}

func decodeAnthropicReply(data []byte) (string, error) {
	var response struct {
		Type    string `json:"type"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &response); err != nil || response.Type == "error" {
		return "", errInvalidResponse
	}
	// EV-UI-287. The ordinary non-streaming path reads the first content block.
	if len(response.Content) == 0 || response.Content[0].Text == "" {
		return "", errEmptyReply
	}
	return response.Content[0].Text, nil
}

func readAnthropicChatStream(ctx context.Context, source io.Reader, startedAt time.Time, onDelta func(string, StreamProgress)) (ChatReply, error) {
	var text strings.Builder
	var segments segmenter.Segmenter
	progress := StreamProgress{startedAt: startedAt}
	err := readEvents(ctx, source, func(data string) (bool, error) {
		if strings.TrimSpace(data) == "" {
			return false, nil
		}
		receivedAt := time.Now()
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return false, errInvalidResponse
		}
		switch event.Type {
		case "message_stop":
			return true, nil
		case "error":
			return false, errAnthropicStream
		case "content_block_delta":
			if chunk := event.Delta.Text; chunk != "" {
				text.WriteString(chunk)
				progress.observeText(&segments, chunk, receivedAt)
				if onDelta != nil {
					onDelta(chunk, progress)
				}
			}
		}
		return false, nil
	})
	reply := ChatReply{Text: text.String()}
	if err != nil {
		return reply, err
	}
	// EV-UI-287. A clean EOF also completes the native Claude stream.
	if reply.Text == "" {
		return reply, errEmptyReply
	}
	reply.Metrics = progress.MetricsAt(time.Now())
	return reply, nil
}
