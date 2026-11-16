package aiworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-text/typesetting/segmenter"
)

var (
	errMissingModel       = errors.New("Enter a model name")
	errEmptyReply         = errors.New("AI service returned no reply text")
	errIncompleteResponse = errors.New("AI stream ended before completing the reply")
)

type completionRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

// Test sends the short connection probe and returns the model identified by
// the service, if provided.
func Test(ctx context.Context, config Config) (string, error) {
	connection, err := normalize(config)
	if err != nil {
		return "", err
	}
	// EV-UI-230 and EV-UI-288. Claude's short probe also has a system prompt.
	messages := []Message{{Role: "user", Content: "Hi"}}
	if connection.isAnthropic() {
		messages = append([]Message{{Role: "system", Content: "Respond with just 'ok'."}}, messages...)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := connection.postCompletion(ctx, completionRequest{
		Model: connection.config.Model, Messages: messages, MaxTokens: 5,
	})
	if err != nil {
		return "", connectionTestError(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", connectionTestError(&StatusError{StatusCode: response.StatusCode})
	}
	data, err := readResponse(ctx, response.Body)
	if err != nil {
		return "", err
	}
	var reply struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(data, &reply) != nil {
		return "", ctx.Err()
	}
	return reply.Model, ctx.Err()
}

func connectionTestError(err error) error {
	var result *StatusError
	if errors.As(err, &result) {
		switch result.StatusCode {
		case http.StatusUnauthorized:
			return errors.New("Authentication failed - check API key")
		case http.StatusForbidden:
			return errors.New("Access denied - check API key permissions")
		case http.StatusNotFound:
			return errors.New("Endpoint not found - check base URL and model name")
		case http.StatusUnprocessableEntity:
			return errors.New("Invalid request - model may not exist")
		case http.StatusTooManyRequests:
			return errors.New("Rate limited - try again later")
		}
		if result.StatusCode >= 500 && result.StatusCode <= 599 {
			return fmt.Errorf("Server error (%d)", result.StatusCode)
		}
		return fmt.Errorf("HTTP %d", result.StatusCode)
	}
	return err
}

// ChatReply contains the text and, for a completed SSE stream, its metrics.
type ChatReply struct {
	Text    string
	Metrics ReplyMetrics
}

// Chat sends the conversation. If onDelta is supplied, Chat requests streaming
// and calls it synchronously in receive order on the caller's goroutine. An
// interrupted stream returns the text received so far along with an error.
func Chat(ctx context.Context, config Config, messages []Message, onDelta func(string, StreamProgress)) (ChatReply, error) {
	if strings.TrimSpace(config.Model) == "" {
		return ChatReply{}, errMissingModel
	}
	connection, err := normalize(config)
	if err != nil {
		return ChatReply{}, err
	}
	startedAt := time.Now()
	response, err := connection.postCompletion(ctx, completionRequest{
		Model: connection.config.Model, Messages: messages, Stream: onDelta != nil,
	})
	if err != nil {
		return ChatReply{}, err
	}
	defer response.Body.Close()

	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType == "text/event-stream" {
		if connection.isAnthropic() {
			return readAnthropicChatStream(ctx, response.Body, startedAt, onDelta)
		}
		return readChatStream(ctx, response.Body, startedAt, onDelta)
	}
	data, err := readResponse(ctx, response.Body)
	if err != nil {
		return ChatReply{}, err
	}
	var text string
	if connection.isAnthropic() {
		text, err = decodeAnthropicReply(data)
	} else {
		var choice completionChoice
		choice, err = decodeChoice(data)
		text = choice.Message.Content
	}
	if err != nil {
		return ChatReply{}, err
	}
	if text == "" {
		return ChatReply{}, errEmptyReply
	}
	if err := ctx.Err(); err != nil {
		return ChatReply{}, err
	}
	if onDelta != nil {
		onDelta(text, StreamProgress{})
	}
	return ChatReply{Text: text}, ctx.Err()
}

func (c serviceConnection) completionJSON(request completionRequest) ([]byte, error) {
	if c.isAnthropic() {
		return encodeAnthropicCompletion(request)
	}
	return json.Marshal(request)
}

func (c serviceConnection) postCompletion(ctx context.Context, request completionRequest) (*http.Response, error) {
	payload, err := c.completionJSON(request)
	if err != nil {
		return nil, errInvalidResponse
	}
	accept := "application/json"
	if request.Stream {
		accept = "text/event-stream, application/json"
	}
	return c.openResponse(ctx, http.MethodPost, c.completionRoute(), accept, payload)
}

type completionChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	Delta        Message `json:"delta"`
	FinishReason string  `json:"finish_reason"`
}

func decodeChoice(data []byte) (completionChoice, error) {
	var response struct {
		Choices []completionChoice `json:"choices"`
		Error   *json.RawMessage   `json:"error"`
	}
	if json.Unmarshal(data, &response) != nil || response.Error != nil {
		return completionChoice{}, errInvalidResponse
	}
	for _, choice := range response.Choices {
		if choice.Index == 0 {
			return choice, nil
		}
	}
	return completionChoice{}, nil
}

func readChatStream(ctx context.Context, source io.Reader, startedAt time.Time, onDelta func(string, StreamProgress)) (ChatReply, error) {
	var text strings.Builder
	var segments segmenter.Segmenter
	var endedAt time.Time
	progress := StreamProgress{startedAt: startedAt}
	err := readEvents(ctx, source, func(data string) (bool, error) {
		receivedAt := time.Now()
		switch strings.TrimSpace(data) {
		case "":
			return false, nil
		case "[DONE]":
			endedAt = receivedAt
			return true, nil
		}
		choice, err := decodeChoice([]byte(data))
		if err != nil {
			return false, err
		}
		if chunk := choice.Delta.Content; chunk != "" {
			text.WriteString(chunk)
			progress.observeText(&segments, chunk, receivedAt)
			if onDelta != nil {
				onDelta(chunk, progress)
			}
		}
		if choice.FinishReason != "" {
			endedAt = receivedAt
			return true, nil
		}
		return false, nil
	})
	reply := ChatReply{Text: text.String()}
	if err != nil {
		return reply, err
	}
	if endedAt.IsZero() {
		return reply, errIncompleteResponse
	}
	if text.Len() == 0 {
		return reply, errEmptyReply
	}
	reply.Metrics = progress.MetricsAt(endedAt)
	return reply, nil
}
