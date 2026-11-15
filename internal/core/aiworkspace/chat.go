package aiworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
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
	// EV-UI-230. The connection test sends one short, non-streaming turn.
	response, err := postCompletion(ctx, config, completionRequest{
		Model: strings.TrimSpace(config.Model), Messages: []Message{{Role: "user", Content: "Hi"}}, MaxTokens: 5,
	})
	if err != nil {
		return "", connectionTestError(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", &StatusError{StatusCode: response.StatusCode}
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
	}
	return err
}

// Chat sends the conversation and returns the reply. If onDelta is supplied,
// Chat requests streaming and calls it synchronously in receive order on the
// caller's goroutine. An interrupted stream returns the text received so far
// along with an error. Cancel the context to stop a pending request.
func Chat(ctx context.Context, config Config, messages []Message, onDelta func(string)) (string, error) {
	// EV-UI-231. Workspace chat sends its supplied turns unchanged and omits
	// the connection probe's token limit.
	response, err := postCompletion(ctx, config, completionRequest{
		Model: strings.TrimSpace(config.Model), Messages: messages, Stream: onDelta != nil,
	})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType == "text/event-stream" {
		return readChatStream(ctx, response.Body, onDelta)
	}
	data, err := readResponse(ctx, response.Body)
	if err != nil {
		return "", err
	}
	choice, err := decodeChoice(data)
	if err != nil {
		return "", err
	}
	text := choice.Message.Content
	if text == "" {
		return "", errEmptyReply
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if onDelta != nil {
		onDelta(text)
	}
	return text, ctx.Err()
}

func postCompletion(ctx context.Context, config Config, request completionRequest) (*http.Response, error) {
	if request.Model == "" {
		return nil, errMissingModel
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, errInvalidResponse
	}
	accept := "application/json"
	if request.Stream {
		accept = "text/event-stream, application/json"
	}
	return openResponse(ctx, config, http.MethodPost, "chat/completions", accept, payload)
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

func readChatStream(ctx context.Context, source io.Reader, onDelta func(string)) (string, error) {
	var text strings.Builder
	finished := false
	err := readEvents(ctx, source, func(data string) (bool, error) {
		switch strings.TrimSpace(data) {
		case "":
			return false, nil
		case "[DONE]":
			finished = true
			return true, nil
		}
		choice, err := decodeChoice([]byte(data))
		if err != nil {
			return false, err
		}
		if chunk := choice.Delta.Content; chunk != "" {
			text.WriteString(chunk)
			if onDelta != nil {
				onDelta(chunk)
			}
		}
		finished = choice.FinishReason != ""
		return finished, nil
	})
	if err != nil {
		return text.String(), err
	}
	if !finished {
		return text.String(), errIncompleteResponse
	}
	if text.Len() == 0 {
		return "", errEmptyReply
	}
	return text.String(), nil
}
