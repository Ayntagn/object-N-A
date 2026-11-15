package liveapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
)

const (
	maxResponseBytes = 4 << 20
	requestTimeout   = 20 * time.Second
)

var (
	ErrUnresolvedPlaceholder = errors.New("Request contains an unresolved credential placeholder")
	ErrResponseTooLarge      = errors.New("Response exceeds 4 MiB")
)

// Response preserves HTTP status and the bounded response body. Body is UTF-8
// text for the note, and RawBody retains bytes before charset decoding.
type Response struct {
	StatusCode  int
	StatusText  string
	Body        string
	ContentType string
	Headers     http.Header
	RawBody     []byte
	Duration    time.Duration
	Size        int64
}

var client = &http.Client{
	Timeout: requestTimeout,
	CheckRedirect: func(next *http.Request, previous []*http.Request) error {
		if len(previous) >= 10 || next.URL.User != nil || !sameOrigin(previous[0].URL, next.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// Run makes an explicitly requested HTTP call. A non-2xx status is returned
// with its body as an HTTP response. Callers own the user's confirmation.
func Run(ctx context.Context, request Request) (Response, error) {
	if err := request.Validate(); err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(request.Method), request.URL, strings.NewReader(request.Body))
	if err != nil {
		return Response{}, errors.New("Invalid HTTP request")
	}
	for _, header := range request.Headers {
		if !strings.EqualFold(header.Name, "host") {
			req.Header.Add(header.Name, header.Value)
		}
	}
	started := time.Now()
	result, err := client.Do(req)
	if err != nil {
		return Response{}, requestError(err)
	}
	defer result.Body.Close()
	response := Response{
		StatusCode: result.StatusCode, ContentType: result.Header.Get("Content-Type"), Headers: result.Header.Clone(),
	}
	_, response.StatusText, _ = strings.Cut(result.Status, " ")
	if result.ContentLength > maxResponseBytes && req.Method != http.MethodHead {
		response.Duration = time.Since(started)
		return response, ErrResponseTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(result.Body, maxResponseBytes+1))
	response.Duration = time.Since(started)
	if err != nil {
		return response, requestError(err)
	}
	if len(data) > maxResponseBytes {
		return response, ErrResponseTooLarge
	}
	response.RawBody, response.Size = data, int64(len(data))
	response.Body = decodeBody(data, response.ContentType)
	return response, nil
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

func requestError(err error) error {
	// net/http's URL wrapper can contain request data. It does not belong in a
	// persisted result note. Keep cancellation detectable by errors.Is.
	var requestErr *url.Error
	if errors.As(err, &requestErr) {
		return requestErr.Err
	}
	return err
}

func decodeBody(data []byte, contentType string) string {
	_, params, _ := mime.ParseMediaType(contentType)
	if label := params["charset"]; label != "" {
		if encoding, _ := charset.Lookup(label); encoding != nil {
			if decoded, err := encoding.NewDecoder().Bytes(data); err == nil {
				return strings.TrimPrefix(string(decoded), "\ufeff")
			}
		}
	}
	if utf8.Valid(data) {
		return strings.TrimPrefix(string(data), "\ufeff")
	}
	return strings.ToValidUTF8(string(data), "�")
}
