package liveapi

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Result holds the first line of an api-result fence and its saved content.
type Result struct {
	Metadata   string
	Body       string
	Timestamp  string
	StatusCode int
	Error      bool
	Format     Format
}

const timestampLayout = "2006-01-02 15:04"

var (
	resultTimestamp = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}`)
	resultStatus    = regexp.MustCompile(`\b[1-5][0-9]{2}\b`)
)

// ParseResult reads the contents of an existing api-result fence.
func ParseResult(source string) Result {
	metadata, body, _ := strings.Cut(normalizeLines(source), "\n")
	metadata = strings.TrimSpace(metadata)
	result := Result{
		Metadata:  metadata,
		Body:      body,
		Timestamp: resultTimestamp.FindString(metadata),
		Error:     metadata == "Error" || strings.HasPrefix(metadata, "Error ·"),
		Format:    FormatAuto,
	}
	if !result.Error {
		result.StatusCode, _ = strconv.Atoi(resultStatus.FindString(metadata))
		if strings.Contains(metadata, "format:ai") {
			result.Format = FormatAI
		}
	}
	return result
}

// MakeResult creates a result from a completed HTTP response or request error.
func MakeResult(response Response, requestErr error, at time.Time) Result {
	stamp := at.Format(timestampLayout)
	if requestErr != nil {
		return Result{
			Metadata: "Error · " + stamp, Body: normalizeLines(requestErr.Error()), Timestamp: stamp,
			Error: true, Format: FormatAuto,
		}
	}
	status := strings.Join(strings.Fields(response.StatusText), " ")
	if status == "" {
		status = http.StatusText(response.StatusCode)
	}
	statusLine := "HTTP " + strconv.Itoa(response.StatusCode)
	if status != "" {
		statusLine += " " + status
	}
	size := response.Size
	if size == 0 && response.Body != "" {
		size = int64(len(response.Body))
	}
	return Result{
		Metadata: fmt.Sprintf("%s · %d ms · %s · %s", statusLine, response.Duration.Milliseconds(), SizeLabel(size), stamp),
		Body:     normalizeLines(response.Body), Timestamp: stamp, StatusCode: response.StatusCode, Format: FormatAuto,
	}
}

// SizeLabel formats an API response byte count for display.
func SizeLabel(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
}

// Markdown returns a complete api-result fence without a trailing newline.
func (result Result) Markdown() string {
	metadata := strings.Join(strings.FieldsFunc(result.Metadata, func(r rune) bool { return r == '\r' || r == '\n' }), " ")
	if result.Format == FormatAI && !strings.Contains(metadata, "format:ai") {
		metadata += " · format:ai"
	}
	body := normalizeLines(result.Body)
	length := 3
	for line := range strings.SplitSeq(body, "\n") {
		candidate := strings.TrimSpace(line)
		if strings.Trim(candidate, "`") == "" {
			length = max(length, len(candidate)+1)
		}
	}
	fence := strings.Repeat("`", length)
	return fence + "api-result\n" + metadata + "\n" + body + "\n" + fence
}
