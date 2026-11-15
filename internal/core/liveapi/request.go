// Package liveapi parses, executes, and formats explicit HTTP requests in notes.
package liveapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/http/httpguts"
)

type Format string

const (
	FormatAuto Format = "auto"
	FormatAI   Format = "ai"
	FormatRaw  Format = "raw"
)

type Destination string

const (
	DestinationEditor  Destination = "editor"
	DestinationPreview Destination = "preview"
)

// Options holds the presentation and destination specified by an api fence.
type Options struct {
	Title     string
	Format    Format
	Out       Destination
	Collapsed bool
	Bare      bool
}

type Header struct {
	Name  string
	Value string
}

// Request is a user-authored note request. Parsing does not resolve placeholders.
type Request struct {
	Method  string
	URL     string
	Headers []Header
	Body    string
	Options Options
}

var errRequestURL = errors.New("Only HTTP and HTTPS URLs with a host are supported")

// ParseOptions recognizes an api request fence by its full Markdown info string.
func ParseOptions(info string) (Options, bool) {
	name, settings, hasOptions := strings.Cut(strings.TrimSpace(info), "[")
	if !strings.EqualFold(strings.TrimSpace(name), "api") {
		return Options{}, false
	}
	options := Options{Format: FormatAuto, Out: DestinationEditor}
	if !hasOptions {
		return options, true
	}
	settings, _, found := strings.Cut(settings, "]")
	if !found {
		return options, true
	}
	for entry := range strings.SplitSeq(settings, ",") {
		key, value, hasValue := strings.Cut(entry, ":")
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "title":
			options.Title = value
		case "format":
			if format := Format(strings.ToLower(value)); format == FormatAuto || format == FormatAI || format == FormatRaw {
				options.Format = format
			}
		case "out":
			if out := Destination(strings.ToLower(value)); out == DestinationEditor || out == DestinationPreview {
				options.Out = out
			}
		case "collapsed":
			options.Collapsed = options.Collapsed || !hasValue
		case "bare":
			options.Bare = options.Bare || !hasValue
		}
	}
	return options, true
}

// ParseRequest reads METHOD URL, ordered headers, and the body after a blank line.
func ParseRequest(info, source string) (Request, error) {
	options, ok := ParseOptions(info)
	if !ok {
		return Request{}, errors.New("Expected an api fence")
	}
	source = strings.TrimLeftFunc(normalizeLines(source), unicode.IsSpace)
	if strings.TrimSpace(source) == "" {
		return Request{}, errors.New("Empty request")
	}
	line, rest, _ := strings.Cut(source, "\n")
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return Request{}, errors.New("Invalid request line - expected: METHOD URL")
	}
	request := Request{Method: strings.ToUpper(parts[0]), URL: parts[1], Options: options}
	if !validMethod(request.Method) {
		return Request{}, fmt.Errorf("Unknown HTTP method: %s", parts[0])
	}
	var host string
	for rest != "" {
		line, rest, _ = strings.Cut(rest, "\n")
		if strings.TrimSpace(line) == "" {
			request.Body = strings.TrimSpace(rest)
			break
		}
		name, value, found := strings.Cut(line, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !found || name == "" {
			continue
		}
		if strings.EqualFold(name, "host") {
			host = value
		}
		request.Headers = append(request.Headers, Header{Name: name, Value: value})
	}
	if host != "" && !strings.Contains(request.URL, "://") {
		resolved, err := resolveRequestTarget(host, request.URL)
		if err != nil {
			return request, err
		}
		request.URL = resolved
	}
	if !hasPlaceholder(request.URL) {
		if _, err := parseRequestURL(request.URL); err != nil {
			return request, err
		}
	}
	return request, nil
}

// Validate checks whether the request can run before the caller asks for consent.
func (request Request) Validate() error {
	if hasPlaceholder(request.URL) || hasPlaceholder(request.Body) {
		return ErrUnresolvedPlaceholder
	}
	for _, header := range request.Headers {
		if hasPlaceholder(header.Name) || hasPlaceholder(header.Value) {
			return ErrUnresolvedPlaceholder
		}
		if !httpguts.ValidHeaderFieldName(header.Name) || !httpguts.ValidHeaderFieldValue(header.Value) {
			return errors.New("Invalid HTTP header")
		}
	}
	if !validMethod(strings.ToUpper(request.Method)) {
		return errors.New("Unknown HTTP method")
	}
	_, err := parseRequestURL(request.URL)
	return err
}

func resolveRequestTarget(host, target string) (string, error) {
	// Restrict Host to an authority. Its value does not override an explicit URL.
	if strings.ContainsAny(host, "/?#@\\") {
		return "", errRequestURL
	}
	base, err := parseRequestURL("https://" + host)
	if err != nil {
		if !hasPlaceholder(host) {
			return "", err
		}
		return "https://" + host + "/" + strings.TrimPrefix(target, "/"), nil
	}
	scheme := "https://"
	if strings.EqualFold(base.Hostname(), "localhost") || base.Hostname() == "127.0.0.1" || base.Hostname() == "::1" {
		scheme = "http://"
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	return scheme + host + target, nil
}

func parseRequestURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return nil, errRequestURL
	}
	return parsed, nil
}

func validMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func hasPlaceholder(source string) bool {
	_, rest, open := strings.Cut(source, "{{")
	return open && strings.Contains(rest, "}}")
}

func normalizeLines(source string) string {
	return strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n")
}
