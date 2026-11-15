// spec: EV-UI-110 / EV-UI-111
package glow

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing"
)

func TestTrashFallbackPreservesBytesAndKeepsActivePath(t *testing.T) {
	env := glowFixture(t)
	const source = "Graph Terminal Probe/Graph Hub.md"
	const other = "Graph Hub.md"
	first, err := env.Vault.ReadDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	const second = "# Other hub\n"
	if _, err := env.Vault.CreateDocument(other, second); err != nil {
		t.Fatal(err)
	}
	env.TrashItem = func(string) error { return fs.ErrPermission }
	var session Session
	session.Submit("rm /", env)
	assertLastLine(t, &session, "refusing to trash the vault root", "")
	session.Submit("rm ../Canary.md", env)
	assertLastLine(t, &session, "path escapes the vault", "")
	if result := session.Submit(`rm this > "Trash Receipt"`, env); !result.VaultChanged || result.OpenPath != "" {
		t.Fatalf("trash redirect returned %+v", result)
	}
	receipt, err := env.Vault.ReadDocument("Trash Receipt.md")
	if err != nil || receipt.Text != source+" → .cwtrash/  (recoverable)\n" {
		t.Fatalf("trash redirect = %q, %v", receipt.Text, err)
	}
	if _, err := env.Vault.ReadDocument(source); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("source remains at %q: %v", source, err)
	}
	session.Submit("this", env)
	assertLastLine(t, &session, source, source)
	session.Submit("backlinks this", env)
	assertLastLine(t, &session, "no match", "")
	session.Submit(`rm "Graph Hub.md" ignored`, env)
	assertLastLine(t, &session, other+" → .cwtrash/  (recoverable)", "")
	entries, err := env.Vault.TerminalEntries(".cwtrash", 0)
	if err != nil || len(entries) != 2 {
		t.Fatalf("trash has %d files: %v", len(entries), err)
	}
	for _, entry := range entries {
		doc, err := env.Vault.ReadDocument(entry.Path)
		want := first.Text
		if entry.Name != path.Base(source) {
			prefix := strings.TrimSuffix(entry.Name, "-"+path.Base(source))
			if len(prefix) != 8 || strings.Trim(prefix, "0123456789ABCDEF") != "" {
				t.Fatalf("collision name = %q", entry.Name)
			}
			want = second
		}
		if err != nil || doc.Text != want {
			t.Fatalf("trash file %q = %q, %v", entry.Path, doc.Text, err)
		}
	}
	const recovered = ".cwtrash/Graph Hub.md"
	session.Submit(`rm ".cwtrash/Graph Hub.md"`, env)
	assertLastLine(t, &session, recovered+" → .cwtrash/  (recoverable)", "")
	if _, err := env.Vault.ReadDocument(recovered); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("trashing a recovery file left its original name: %v", err)
	}
}
