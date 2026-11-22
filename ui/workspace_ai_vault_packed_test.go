// spec: EV-MYGO-195 / EV-MYGO-203 / EV-MYGO-214
package ui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cyberwriter/internal/core/vaultcontext"
)

func TestAIVaultPackedCacheReadsLegacyAndClearsBothFormats(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	index, err := vaultcontext.Build(context.Background(), []vaultcontext.Document{
		{Path: "Note.md", Text: "The original cache contains a passage about local vault embeddings."},
	}, vaultcontext.Quick)
	if err != nil {
		t.Fatal(err)
	}
	cache := aiVaultContextCache{Index: index, BuiltAt: time.Unix(1700000000, 0),
		Files: map[string]aiVaultFileStamp{"Note.md": {Modified: 42, Size: 100}}}
	legacy, err := aiVaultContextCacheFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := readAIVaultContextCache(root)
	if err != nil || old.Index == nil || !old.BuiltAt.Equal(cache.BuiltAt) {
		t.Fatalf("legacy reopen = %+v, %v", old, err)
	}
	cache.BuiltAt = cache.BuiltAt.Add(time.Hour)
	target, stage, err := stageAIVaultContextCache(root, cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(stage, target); err != nil {
		t.Fatal(err)
	}
	got, err := readAIVaultContextCache(root)
	if err != nil || got.Index == nil || !got.BuiltAt.Equal(cache.BuiltAt) || got.Files["Note.md"] != cache.Files["Note.md"] {
		t.Fatalf("packed reopen = %+v, %v", got, err)
	}
	if err := clearAIVaultContextCache(root, cache.Files, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{legacy, target} {
		if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cache file survived Clear: %s, %v", name, err)
		}
	}
}
