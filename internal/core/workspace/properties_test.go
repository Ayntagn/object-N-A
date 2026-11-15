// spec: EV-UI-082
package workspace

import (
	"strings"
	"testing"
)

func TestPropertyEditPreservesBodyAndOtherYAMLValues(t *testing.T) {
	source := "\ufeff---\r\ntitle: \"001\" # keep the identifier\r\nprogress: 3\r\ntags: [alpha, beta]\r\nnested:\r\n  keep: true\r\n---\r\n\r\n# Body 中文\r\n"
	properties, err := ReadProperties(source)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := properties.Edit(PropertyEdit{Action: SetProperty, Key: "progress", Text: "4.5"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, body := SplitFrontmatter(updated)
	if body != "\r\n# Body 中文\r\n" || !strings.HasPrefix(updated, "\ufeff---\r\n") {
		t.Fatalf("changed the document outside the YAML block: %q", updated)
	}
	if !strings.Contains(updated, "title: \"001\" # keep the identifier\r\n") || !strings.Contains(updated, "tags: [alpha, beta]\r\n") {
		t.Fatalf("changed an unrelated YAML value: %s", updated)
	}
	properties, err = ReadProperties(updated)
	if err != nil || len(properties.Rows) != 4 || len(properties.Rows[3].Items) != 1 || properties.Rows[1].Kind != PropertyNumber || properties.Rows[1].Text != "4.5" || properties.Rows[3].Items[0].Text != "true" {
		t.Fatalf("unexpected properties after rewriting: %+v %v", properties, err)
	}
}

func TestTOMLPropertyEditKeepsFormatAndTable(t *testing.T) {
	source := "+++\ntitle = \"TOML field notes\"\ndraft = true\ntags = [\"gamma\", \"delta\"]\n[links]\nsource = \"local\"\n+++\n\n# TOML body\n"
	properties, err := ReadProperties(source)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := properties.Edit(PropertyEdit{Action: SetProperty, Key: "title", Text: "TOML revised"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(updated, "+++\n") || !strings.Contains(updated, "\n+++\n\n# TOML body\n") {
		t.Fatalf("changed the TOML boundary or body: %q", updated)
	}
	properties, err = ReadProperties(updated)
	if err != nil || !strings.Contains(updated, `title = "TOML revised"`) || !strings.Contains(updated, `source = "local"`) || len(properties.Rows) != 4 {
		t.Fatalf("unexpected TOML after rewriting: %s (%v)", updated, err)
	}
}
