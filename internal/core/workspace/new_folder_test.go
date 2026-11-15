// spec: FEAT-VAULT-NEW-FOLDER / EV-UI-124 / EV-UI-125
package workspace_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestCreateNamedFolderFromPrompt(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "Research/Existing.md", "keep this note")
	vault := openFixture(t, root)

	for _, tt := range []struct {
		name    string
		created bool
		want    error
	}{
		{name: "  "},
		{name: "  New folder  ", created: true},
		{name: "New folder", want: fs.ErrExist},
		{name: "Existing.md", want: fs.ErrExist},
		{name: ".", want: fs.ErrExist},
		{name: "..", want: fs.ErrExist},
		{name: "Bad:Folder", want: workspace.ErrFolderNameSeparators},
		{name: "Bad/Folder", want: workspace.ErrFolderNameSeparators},
	} {
		created, err := vault.CreateNamedFolder("Research", tt.name)
		if created != tt.created || !errors.Is(err, tt.want) {
			t.Fatalf("create folder with name %q = %v, %v; want %v, %v", tt.name, created, err, tt.created, tt.want)
		}
	}
	parent := filepath.Join(root, "Research")
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 2 || entries[1].Name() != "New folder" || !entries[1].IsDir() {
		t.Fatalf("folder entries = %v, %v", entries, err)
	}
	children, err := os.ReadDir(filepath.Join(parent, "New folder"))
	if err != nil || len(children) != 0 {
		t.Fatalf("new folder contents = %v, %v", children, err)
	}
	existing, err := os.ReadFile(filepath.Join(parent, "Existing.md"))
	if err != nil || string(existing) != "keep this note" {
		t.Fatalf("existing file = %q, %v", existing, err)
	}
}
