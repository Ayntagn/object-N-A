// spec: FEAT-001 / EV-UI-164 / EV-UI-165
package liveapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseRequestFence(t *testing.T) {
	request, err := ParseRequest("API[title: Noon: Weather, format: RAW, out: preview, collapsed, bare]",
		"\r\npost /weather?day=1\r\nHost: 127.0.0.1:8000\r\nX-Note: noon: test\r\nX-Test: {{token}}\r\nIgnored line\r\n\r\n{\r\n  \"today\": true\r\n}\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if request.Method != http.MethodPost || request.URL != "http://127.0.0.1:8000/weather?day=1" ||
		request.Body != "{\n  \"today\": true\n}" {
		t.Fatalf("request = %#v", request)
	}
	if want := (Options{Title: "Noon: Weather", Format: FormatRaw, Out: DestinationPreview, Collapsed: true, Bare: true}); request.Options != want {
		t.Errorf("options = %#v, want %#v", request.Options, want)
	}
	if want := []Header{{"Host", "127.0.0.1:8000"}, {"X-Note", "noon: test"}, {"X-Test", "{{token}}"}}; !reflect.DeepEqual(request.Headers, want) {
		t.Errorf("headers = %#v, want %#v", request.Headers, want)
	}
	if err := request.Validate(); !errors.Is(err, ErrUnresolvedPlaceholder) {
		t.Errorf("placeholder validation = %v", err)
	}
	if _, ok := ParseOptions("api-result"); ok {
		t.Error("a result fence was classified as a request")
	}
	for _, source := range []string{"", "GET", "TRACE http://127.0.0.1/", "GET file:///example"} {
		if _, err := ParseRequest("api", source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}

func TestAPIResultRoundTripAndTable(t *testing.T) {
	at := time.Date(2026, 11, 14, 12, 21, 0, 0, time.FixedZone("fixture", 8*60*60))
	response := Response{StatusCode: 200, StatusText: "OK", Duration: 12 * time.Millisecond,
		Body: `{"data":[{"name":"Ada","ready":true},{"name":"Bo","ready":false}]}`}
	response.Size = int64(len(response.Body))
	result := MakeResult(response, nil, at)
	wantHeader := fmt.Sprintf("HTTP 200 OK · 12 ms · %d B · 2026-11-14 12:21", len(response.Body))
	wantFence := "```api-result\n" + wantHeader + "\n" + response.Body + "\n```"
	if got := result.Markdown(); got != wantFence {
		t.Fatalf("Markdown() = %q, want %q", got, wantFence)
	}
	parsed := ParseResult(wantHeader + "\n" + response.Body)
	if parsed.StatusCode != 200 || parsed.Body != response.Body || parsed.Timestamp != "2026-11-14 12:21" || parsed.Error {
		t.Fatalf("parsed result = %#v", parsed)
	}
	table, ok := parsed.Table()
	if want := (Table{Columns: []string{"name", "ready"}, Rows: [][]string{{"Ada", "true"}, {"Bo", "false"}}}); !ok || !reflect.DeepEqual(table, want) {
		t.Errorf("table = %#v, %t, want %#v", table, ok, want)
	}
	transportErr := errors.New("The operation couldn’t be completed. Operation not permitted")
	failure := MakeResult(Response{}, transportErr, at)
	if got := failure.Markdown(); got != "```api-result\nError · 2026-11-14 12:21\n"+transportErr.Error()+"\n```" {
		t.Errorf("failure fence = %q", got)
	}
	if parsedFailure := ParseResult(failure.Metadata + "\n" + failure.Body); parsedFailure.StatusCode != 0 || !parsedFailure.Error {
		t.Errorf("failure classified as HTTP response: %#v", parsedFailure)
	}
	ai := ParseResult(wantHeader + " · format:ai\n| Name | State |\n| --- | --- |\n| Ada | `ready` |")
	table, ok = ai.Table()
	if want := (Table{Columns: []string{"Name", "State"}, Rows: [][]string{{"Ada", "ready"}}}); !ok || !reflect.DeepEqual(table, want) {
		t.Errorf("existing AI table = %#v, %t, want %#v", table, ok, want)
	}
	if fenced := (Result{Metadata: wantHeader, Body: "text\n```\nafter"}).Markdown(); !strings.HasPrefix(fenced, "````api-result\n") || !strings.HasSuffix(fenced, "\n````") {
		t.Errorf("response could close the result fence: %q", fenced)
	}
}

func TestRunLocalHTTP(t *testing.T) {
	var redirected, unexpected atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer other.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/result":
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || r.Header.Get("X-Fixture") != "local" || string(body) != "draft" {
				t.Errorf("request = %s %q %q", r.Method, r.Header.Get("X-Fixture"), body)
			}
			http.SetCookie(w, &http.Cookie{Name: "fixture", Value: "one", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"reason":"missing"}`)
		case "/redirect":
			if r.Header.Get("Cookie") != "" {
				t.Error("Run reused a response cookie")
			}
			http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
		case "/same":
			http.Redirect(w, r, "/options", http.StatusTemporaryRedirect)
		case "/options":
			if r.Method != http.MethodOptions {
				t.Errorf("redirected method = %q, want OPTIONS", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		case "/large":
			_, _ = io.WriteString(w, strings.Repeat("a", maxResponseBytes+1))
		case "/head":
			if r.Method != http.MethodHead {
				t.Errorf("metadata request = %q, want HEAD", r.Method)
			}
			w.Header().Set("Content-Length", fmt.Sprint(maxResponseBytes+1))
			w.WriteHeader(http.StatusOK)
		default:
			unexpected.Add(1)
		}
	}))
	defer local.Close()
	request, err := ParseRequest("api", "POST "+local.URL+"/result\nX-Fixture: local\n\ndraft")
	if err != nil {
		t.Fatal(err)
	}
	response, err := Run(t.Context(), request)
	if err != nil || response.StatusCode != http.StatusNotFound || response.StatusText != "Not Found" || response.Body != `{"reason":"missing"}` || string(response.RawBody) != response.Body {
		t.Errorf("HTTP error result = %#v, %v", response, err)
	}
	response, err = Run(t.Context(), Request{Method: http.MethodGet, URL: local.URL + "/redirect"})
	if err != nil || response.StatusCode != http.StatusTemporaryRedirect || redirected.Load() != 0 {
		t.Errorf("cross-origin redirect: status=%d sent=%d err=%v", response.StatusCode, redirected.Load(), err)
	}
	response, err = Run(t.Context(), Request{Method: http.MethodOptions, URL: local.URL + "/same"})
	if err != nil || response.StatusCode != http.StatusNoContent {
		t.Errorf("same-origin OPTIONS redirect: status=%d err=%v", response.StatusCode, err)
	}
	response, err = Run(t.Context(), Request{Method: http.MethodHead, URL: local.URL + "/head"})
	if err != nil || response.StatusCode != http.StatusOK || response.Size != 0 {
		t.Errorf("HEAD result: status=%d size=%d err=%v", response.StatusCode, response.Size, err)
	}
	request, err = ParseRequest("api", "HEAD "+local.URL+"\nHost: {{name}}")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Run(t.Context(), request)
	if !errors.Is(err, ErrUnresolvedPlaceholder) || unexpected.Load() != 0 {
		t.Errorf("unresolved placeholder: sent=%d err=%v", unexpected.Load(), err)
	}
	if _, err := Run(t.Context(), Request{Method: http.MethodGet, URL: local.URL + "/large"}); !errors.Is(err, ErrResponseTooLarge) {
		t.Errorf("oversized response error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Run(ctx, Request{Method: http.MethodGet, URL: local.URL}); !errors.Is(err, context.Canceled) || unexpected.Load() != 0 {
		t.Errorf("canceled request: sent=%d err=%v", unexpected.Load(), err)
	}
}
