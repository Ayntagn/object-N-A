// Package aiworkspace connects the editor's AI workspace to a configured model service.
package aiworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http/httpguts"
)

// Config identifies the service, its chosen model and its API key.
type Config struct {
	BaseURL string
	Model   string
	APIKey  string
}

// Normalize trims form values and validates the HTTP service and API key.
// Model discovery does not require a selected model.
func Normalize(config Config) (Config, error) {
	connection, err := normalize(config)
	return connection.config, err
}

// Message is one turn of the conversation sent to the configured service.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const (
	maxResponseBytes    = 4 << 20
	anthropicHost       = "api.anthropic.com"
	anthropicAPIVersion = "2023-06-01"
)

type serviceConnection struct {
	config Config
	base   *url.URL
}

var (
	errInvalidURL         = errors.New("Enter a valid HTTP or HTTPS Base URL")
	errInvalidKey         = errors.New("Invalid API key")
	errServiceUnreachable = errors.New("Could not connect to the AI service")
	errInvalidResponse    = errors.New("Invalid response from the AI service")
	errResponseTooLarge   = errors.New("AI response exceeds 4 MiB")
)

// StatusError carries a non-success HTTP code without including the private
// request URL, credentials or error body.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("AI service returned HTTP %d", e.StatusCode)
}

var serviceClient = &http.Client{
	Timeout: 2 * time.Minute,
	CheckRedirect: func(next *http.Request, previous []*http.Request) error {
		if len(previous) >= 10 || next.URL.User != nil || !sameOrigin(previous[0].URL, next.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func sameOrigin(a, b *url.URL) bool {
	return a.Scheme == b.Scheme && strings.EqualFold(a.Hostname(), b.Hostname()) && originPort(a) == originPort(b)
}

func originPort(endpoint *url.URL) string {
	if port := endpoint.Port(); port != "" {
		return port
	}
	if endpoint.Scheme == "https" {
		return "443"
	}
	return "80"
}

func normalize(config Config) (serviceConnection, error) {
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	config.Model = strings.TrimSpace(config.Model)
	config.APIKey = strings.TrimSpace(config.APIKey)
	base, err := url.Parse(config.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" || base.User != nil || base.Fragment != "" {
		return serviceConnection{}, errInvalidURL
	}
	if !httpguts.ValidHeaderFieldValue(config.APIKey) {
		return serviceConnection{}, errInvalidKey
	}
	return serviceConnection{config: config, base: base}, nil
}

func (c serviceConnection) isAnthropic() bool {
	return strings.EqualFold(c.base.Hostname(), anthropicHost)
}

func (c serviceConnection) endpoint(route string) string {
	if c.isAnthropic() {
		return "https://" + anthropicHost + "/v1/" + route
	}
	return c.base.JoinPath(route).String()
}

func (c serviceConnection) completionRoute() string {
	if c.isAnthropic() {
		return "messages"
	}
	return "chat/completions"
}

func (c serviceConnection) openResponse(ctx context.Context, method, route, accept string, payload []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(route), bytes.NewReader(payload))
	if err != nil {
		return nil, errInvalidURL
	}
	if key := c.config.APIKey; key != "" {
		if c.isAnthropic() {
			req.Header.Set("x-api-key", key)
			req.Header.Set("anthropic-version", anthropicAPIVersion)
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	response, err := serviceClient.Do(req)
	if err != nil {
		return nil, transportError(ctx, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, &StatusError{StatusCode: response.StatusCode}
	}
	if response.ContentLength > maxResponseBytes {
		response.Body.Close()
		return nil, errResponseTooLarge
	}
	return response, nil
}

func transportError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return errServiceUnreachable
}

func readResponse(ctx context.Context, reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
	if err != nil {
		return nil, transportError(ctx, err)
	}
	if len(data) > maxResponseBytes {
		return nil, errResponseTooLarge
	}
	return data, nil
}
