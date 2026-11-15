// spec: EV-UI-163 / EV-UI-165
package apihistory

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRetainsLatestCallsAfterReopen(t *testing.T) {
	file := filepath.Join(t.TempDir(), "relay-history.json")
	start := time.Date(2026, time.November, 14, 12, 19, 0, 0, time.UTC)
	for i := range 201 {
		item := Item{
			Method: "GET", URL: fmt.Sprintf("https://example.invalid/item/%d", i),
			Request: "GET /fixture\nX-Fixture: data-test", Response: "fixture body\n",
			StatusCode: 200, Duration: 12 * time.Millisecond, Bytes: 13,
			Timestamp: start.Add(time.Duration(i) * time.Second),
		}
		if _, err := Record(file, item); err != nil {
			t.Fatal(err)
		}
	}
	items, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 200 || items[0].URL != "https://example.invalid/item/200" || items[199].URL != "https://example.invalid/item/1" {
		t.Fatalf("loaded %d calls, want latest 200 first", len(items))
	}
	if got := items[0]; got.ID == "" || got.Request != "GET /fixture\nX-Fixture: data-test" || got.Response != "fixture body\n" ||
		got.StatusCode != 200 || got.Duration != 12*time.Millisecond || got.Bytes != 13 || !got.Timestamp.Equal(start.Add(200*time.Second)) {
		t.Fatalf("reopened latest call lost detail: %#v", got)
	}
	if err := Clear(file); err != nil {
		t.Fatal(err)
	}
	if items, err := Load(file); len(items) != 0 || err != nil {
		t.Fatalf("reopened cleared history: %d calls, %v", len(items), err)
	}
}
