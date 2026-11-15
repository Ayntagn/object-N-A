// spec: FEAT-001 / EV-UI-241
package snippets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveSnippetKeepsFolderAndSibling(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "snippets.json")
	fixture := []byte(`[
		{"id":"folder","title":"Samples","tags":[],"isFolder":true,"extraFolder":42,"children":[
			{"id":"first","title":"Delete me","tags":[]},
			{"id":"second","title":"Keep me","tags":["swift"],"extraItem":true}
		]}
	]`)
	if err := os.WriteFile(filename, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Remove(filename, "first"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var folders []struct {
		Extra    int `json:"extraFolder"`
		Children []struct {
			Snippet
			Extra bool `json:"extraItem"`
		}
	}
	if err := json.Unmarshal(data, &folders); err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].Extra != 42 || len(folders[0].Children) != 1 {
		t.Fatalf("unexpected library after removing the selected snippet: %s", data)
	}
	sibling := folders[0].Children[0]
	if sibling.ID != "second" || !sibling.Extra || len(sibling.Tags) != 1 || sibling.Tags[0] != "swift" {
		t.Fatalf("the remaining snippet changed: %+v", sibling)
	}
}
