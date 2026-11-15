package wysiwyg

// spec: EV-UI-114 / EV-UI-116

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func TestVisualTemplateKeepsBashFencesThroughEditAndUndo(t *testing.T) {
	const original = "# Original\n\nTail.\n"
	const template = "# Project\n\n## Install\n\n" +
		"```bash\n# First line\n# Second line\n```\n\n## Run\n\n" +
		"```bash\n# Another command\n```\n\n## License\n\nMIT"
	draft := editor.NewBuffer(original)
	session := Open(draft)
	session.Document.SetSelection(0, 0)
	if !session.InsertMarkdown(template) || !session.Sync(draft) {
		t.Fatal("README template was not inserted")
	}
	inserted := draft.String()
	if strings.Count(inserted, "```bash\n") != 2 || !strings.Contains(inserted, "MITOriginal") {
		t.Fatalf("template Markdown = %q", inserted)
	}
	session.Document.Undo()
	if !session.Sync(draft) || draft.String() != original {
		t.Fatalf("Markdown after undoing insertion = %q", draft.String())
	}
	session.Document.Redo()
	if !session.Sync(draft) || draft.String() != inserted {
		t.Fatalf("Markdown after redoing insertion = %q", draft.String())
	}
	reopened := Open(draft)
	if reopened.Document == nil {
		t.Fatal("reopened template must be editable")
	}
	source := reopened.Document.Text().PlainText()
	offset := len([]rune(source[:strings.Index(source, "# First line")]))
	reopened.Document.SetSelection(offset, offset)
	reopened.Document.InsertText("# Added line\n")
	if !reopened.Sync(draft) || !strings.Contains(draft.String(), "```bash\n# Added line\n# First line") {
		t.Fatalf("edited Markdown = %q", draft.String())
	}
	reopened.Document.Undo()
	if !reopened.Sync(draft) || draft.String() != inserted {
		t.Fatalf("Markdown after undoing edit = %q", draft.String())
	}
}
