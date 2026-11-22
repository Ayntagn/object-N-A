// spec: EV-UI-188 / EV-MYGO-253 / EV-MYGO-309

package workspace_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"object-na.local/vaultdelta"
)

func BenchmarkVaultRefresh(b *testing.B) {
	b.Run("full", func(b *testing.B) {
		benchmarkVaultRefresh(b, func(vault *workspace.Vault, _ []workspace.Entry) ([]workspace.Entry, error) {
			return vault.SidebarEntries(false)
		})
	})
	b.Run("scoped", func(b *testing.B) {
		scope := vaultdelta.Hints("Folder-00/Section-00")
		benchmarkVaultRefresh(b, func(vault *workspace.Vault, previous []workspace.Entry) ([]workspace.Entry, error) {
			entries, used, err := vault.RefreshSidebarEntries(previous, false, scope)
			if err == nil && used.Full() {
				err = errors.New("leaf change unexpectedly needed a full scan")
			}
			return entries, err
		})
	})
}

func benchmarkVaultRefresh(b *testing.B, readEntries func(*workspace.Vault, []workspace.Entry) ([]workspace.Entry, error)) {
	for _, operation := range []string{"save", "rename"} {
		b.Run(operation, func(b *testing.B) {
			root := b.TempDir()
			makeRefreshFixture(b, root)
			vault, err := workspace.Open(root)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { _ = vault.Close() })
			entries, err := vault.SidebarEntries(false)
			if err != nil {
				b.Fatal(err)
			}
			var previous *vaultindex.Index
			previous, err = previous.RefreshMetadata(vault, entries)
			if err != nil {
				b.Fatal(err)
			}
			const source = "Folder-00/Section-00/Note-0000.md"
			first := filepath.Join(root, filepath.FromSlash(source))
			second := filepath.Join(filepath.Dir(first), "Renamed-0000.md")
			b.ReportAllocs()
			var round int
			var treeTime, indexTime time.Duration
			for b.Loop() {
				b.StopTimer()
				if operation == "save" {
					err = os.WriteFile(first, fmt.Appendf(nil, "# Updated\n\n[[Note-0001]] #round-%d\n", round), 0o600)
				} else {
					err = os.Rename(first, second)
					first, second = second, first
				}
				if err != nil {
					b.Fatal(err)
				}
				round++
				b.StartTimer()
				start := time.Now()
				entries, err = readEntries(vault, entries)
				if err != nil {
					b.Fatal(err)
				}
				treeTime += time.Since(start)
				start = time.Now()
				previous, err = previous.RefreshMetadata(vault, entries)
				if err != nil {
					b.Fatal(err)
				}
				indexTime += time.Since(start)
			}
			b.ReportMetric(float64(treeTime.Nanoseconds())/float64(b.N), "tree-ns/op")
			b.ReportMetric(float64(indexTime.Nanoseconds())/float64(b.N), "metadata-ns/op")
		})
	}
}

func makeRefreshFixture(b *testing.B, root string) {
	b.Helper()
	if err := os.WriteFile(filepath.Join(root, "Index.md"), []byte("# Synthetic index\n\n[[Note-0000]]\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	for folder := range 9 {
		for section := range 10 {
			directory := filepath.Join(root, fmt.Sprintf("Folder-%02d", folder), fmt.Sprintf("Section-%02d", section))
			if err := os.MkdirAll(directory, 0o700); err != nil {
				b.Fatal(err)
			}
			for note := range 50 {
				id := (folder*10+section)*50 + note
				filename := filepath.Join(directory, fmt.Sprintf("Note-%04d.md", id))
				text := fmt.Sprintf("---\ncategory: section-%02d\n---\n\n# Note %d\n\n[[Note-%04d]] #benchmark\n\n%s\n",
					section, id, (id+1)%4500, strings.Repeat("Synthetic Markdown content. ", 4))
				if err := os.WriteFile(filename, []byte(text), 0o600); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}
