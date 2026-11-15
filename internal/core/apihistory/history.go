package apihistory

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const limit = 200

// Item holds the display-safe request and response for one completed API call.
// Sent, when present, contains the resolved request with secrets masked.
type Item struct {
	ID         string        `json:"id"`
	Method     string        `json:"method"`
	URL        string        `json:"url"`
	Request    string        `json:"request"`
	Response   string        `json:"response"`
	Sent       string        `json:"sent,omitempty"`
	StatusCode int           `json:"statusCode"`
	Duration   time.Duration `json:"duration"`
	Bytes      int64         `json:"bytes"`
	Timestamp  time.Time     `json:"timestamp"`
}

var historyMu sync.Mutex

// Load returns the newest recorded calls first.
func Load(filename string) ([]Item, error) {
	historyMu.Lock()
	defer historyMu.Unlock()
	return load(filename)
}

func load(filename string) ([]Item, error) {
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := json.NewDecoder(file)
	opening, err := reader.Token()
	if err != nil {
		return nil, err
	}
	if opening != json.Delim('[') {
		return nil, errors.New("expected an API history array")
	}
	items := make([]Item, 0, limit)
	for reader.More() {
		var item Item
		if err := reader.Decode(&item); err != nil {
			return nil, err
		}
		if len(items) < limit {
			items = append(items, item)
		}
	}
	if _, err := reader.Token(); err != nil {
		return nil, err
	}
	if _, err := reader.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected data after the API history array")
		}
		return nil, err
	}
	return items, nil
}

// Record prepends an item while retaining at most 200 entries and returns the stored list.
func Record(filename string, item Item) ([]Item, error) {
	historyMu.Lock()
	defer historyMu.Unlock()
	items, err := load(filename)
	if err != nil {
		return nil, err
	}
	item.ID = rand.Text()
	if item.Timestamp.IsZero() {
		item.Timestamp = time.Now()
	}
	records := make([]Item, min(len(items)+1, limit))
	records[0] = item
	copy(records[1:], items)
	if err := replace(filename, records); err != nil {
		return nil, err
	}
	return records, nil
}

// Clear removes all recorded API calls.
func Clear(filename string) error {
	historyMu.Lock()
	defer historyMu.Unlock()
	return replace(filename, []Item{})
}

func replace(filename string, items []Item) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	pending, err := os.CreateTemp(dir, ".relay-history-*")
	if err != nil {
		return err
	}
	defer os.Remove(pending.Name())
	writeErr := encodeHistory(pending, items)
	if writeErr == nil {
		writeErr = pending.Sync()
	}
	if err := errors.Join(writeErr, pending.Close()); err != nil {
		return err
	}
	return os.Rename(pending.Name(), filename)
}

func encodeHistory(output io.Writer, items []Item) error {
	if _, err := io.WriteString(output, "[\n"); err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	for i, item := range items {
		if i > 0 {
			if _, err := io.WriteString(output, ",\n"); err != nil {
				return err
			}
		}
		if err := encoder.Encode(item); err != nil {
			return err
		}
	}
	_, err := io.WriteString(output, "]\n")
	return err
}
