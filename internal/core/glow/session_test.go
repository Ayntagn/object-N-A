// spec: FEAT-001 / EV-UI-067 / EV-UI-072 / EV-UI-094 / EV-UI-099
package glow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
)

func TestSessionFollowsVaultWikilinksAndActiveNote(t *testing.T) {
	env := glowFixture(t)
	var session Session
	session.Submit(`cd "Graph Terminal Probe/Research"`, env)
	if got := session.WorkingDirectory(env.Name); got != "vault/Graph Terminal Probe/Research/" {
		t.Fatalf("working directory = %q", got)
	}
	session.Submit("links this", env)
	assertLastLine(t, &session, "  Graph Terminal Probe/Research/Linked Note.md", "Graph Terminal Probe/Research/Linked Note.md")
	env.Active = "Graph Terminal Probe/Research/Linked Note.md"
	session.Submit("backlinks this", env)
	assertLastLine(t, &session, "  Graph Terminal Probe/Graph Hub.md", "Graph Terminal Probe/Graph Hub.md")
	if opened := session.Submit("open this", env).OpenPath; opened != env.Active {
		t.Errorf("open this = %q, want %q", opened, env.Active)
	}
	session.Submit("clear", env)
	if len(session.Lines()) != 0 || session.Prompt(env.Name) != "Graph Terminal Probe/Research>" {
		t.Fatalf("clear left %d lines and prompt %q", len(session.Lines()), session.Prompt(env.Name))
	}
	var cursor HistoryCursor
	if got := session.RecallFrom(&cursor, "draft", -1); got != "clear" {
		t.Errorf("clear command recall = %q", got)
	}
}

func TestSessionRetainsRelativeDirectoryAndHistoryWhenVaultChanges(t *testing.T) {
	first := glowFixture(t)
	secondVault, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { secondVault.Close() })
	if _, err := secondVault.CreateDocument("Second.md", "# Second\n"); err != nil {
		t.Fatal(err)
	}
	entries, err := secondVault.Entries()
	if err != nil {
		t.Fatal(err)
	}
	second := Environment{Vault: secondVault, Name: "B Vault", Entries: entries}
	var session Session
	session.Submit(`cd "Graph Terminal Probe/Research"`, first)
	session.Submit("ls", first)
	session.Start(second)
	assertLastLine(t, &session, "Linked Note.md", "Graph Terminal Probe/Research/Linked Note.md")
	var cursor HistoryCursor
	if recalled := session.RecallFrom(&cursor, "unsent", -1); recalled != "ls" || session.RecallFrom(&cursor, recalled, 1) != "unsent" {
		t.Fatalf("history and draft were lost on switching Vaults")
	}
	session.Submit("pwd", second)
	assertLastLine(t, &session, "B Vault/Graph Terminal Probe/Research/", "")
	session.Submit("ls", second)
	assertLastLine(t, &session, "no such folder: .", "")
	session.Submit("ls /", second)
	assertLastLine(t, &session, "Second.md", "Second.md")
	session.Submit("this", second)
	assertLastLine(t, &session, "no current note - open one first", "")
}

func TestCompletionAndDraftHistory(t *testing.T) {
	env := glowFixture(t)
	var session Session
	for input, want := range map[string]string{
		"backl":                        "backlinks ",
		"cd Gra":                       `cd "Graph Terminal Probe/`,
		`cat "Graph Terminal Probe/Gr`: `cat "Graph Terminal Probe/Graph Hub.md" `,
		"cat @Iso":                     "cat @Iso",
		"cat @AX-B":                    "cat @AX-Backend-FYWFO4 ",
		"cat @thi":                     "cat @this ",
	} {
		if got := session.Complete(input, env); got != want {
			t.Errorf("Complete(%q) = %q, want %q", input, got, want)
		}
	}
	partial := `cd "Graph Terminal Probe/`
	if got := session.Complete(partial, env); got != partial {
		t.Errorf("ambiguous directory completion = %q, want %q", got, partial)
	}
	assertLastLine(t, &session, "Graph Terminal Probe/Graph Hub.md   Graph Terminal Probe/Isolated Note.md   Graph Terminal Probe/Research/", "Graph Terminal Probe/Isolated Note.md")
	for _, command := range []string{"pwd", "pwd", "help"} {
		session.Submit(command, env)
	}
	input := "draft not submitted"
	var cursor HistoryCursor
	for _, step := range []struct {
		direction int
		want      string
	}{{-1, "help"}, {-1, "pwd"}, {1, "help"}, {1, input}} {
		input = session.RecallFrom(&cursor, input, step.direction)
		if input != step.want {
			t.Fatalf("history move %d = %q, want %q", step.direction, input, step.want)
		}
	}
	session.RecallFrom(&cursor, input, -1)
	cursor = HistoryCursor{}
	if got := session.RecallFrom(&cursor, "new pane", 1); got != "new pane" {
		t.Errorf("new pane retained previous draft %q", got)
	}
}

func TestHistoryRecallKeepsEachPanesDraft(t *testing.T) {
	env := glowFixture(t)
	var session Session
	var orbit, anchor HistoryCursor
	session.Submit("pwd", env)
	session.Submit("this", env)
	orbitLine := session.RecallFrom(&orbit, "orbit draft", -1)
	anchorLine := session.RecallFrom(&anchor, "anchor draft", -1)
	if orbitLine != "this" || anchorLine != "this" {
		t.Fatalf("new history cursors recalled %q and %q", orbitLine, anchorLine)
	}
	if got := session.RecallFrom(&anchor, anchorLine, -1); got != "pwd" {
		t.Fatalf("anchor previous line = %q", got)
	}
	if got := session.RecallFrom(&orbit, orbitLine, 1); got != "orbit draft" {
		t.Fatalf("orbit recovered %q after anchor moved its cursor", got)
	}
	anchorLine = session.RecallFrom(&anchor, "pwd", 1)
	if got := session.RecallFrom(&anchor, anchorLine, 1); got != "anchor draft" {
		t.Fatalf("anchor recovered %q", got)
	}
}

func assertLastLine(t *testing.T, session *Session, text, link string) {
	t.Helper()
	lines := session.Lines()
	if len(lines) == 0 {
		t.Fatal("empty output")
	}
	var got strings.Builder
	var linked string
	for _, span := range lines[len(lines)-1] {
		got.WriteString(span.Text)
		if span.Path != "" {
			linked = span.Path
		}
	}
	if got.String() != text || linked != link {
		t.Errorf("last line = %q with link %q, want %q with link %q", got.String(), linked, text, link)
	}
}

func glowFixture(t *testing.T) Environment {
	t.Helper()
	root := t.TempDir()
	for relative, content := range map[string]string{
		"Graph Terminal Probe/Graph Hub.md":            "# Graph Hub\n\n[[Research/Linked Note]]\n[Isolated](Isolated%20Note.md)\n#graph-probe\n",
		"Graph Terminal Probe/Research/Linked Note.md": "# Linked Note\n\nNeighbor of the hub.\n",
		"Graph Terminal Probe/Isolated Note.md":        "# Isolated\n",
		"AX-Backend-FYWFO4.md":                         "# Backend\n",
	} {
		filename := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	entries, err := vault.Entries()
	if err != nil {
		t.Fatal(err)
	}
	index, err := vaultindex.Build(vault, entries)
	if err != nil {
		t.Fatal(err)
	}
	return Environment{Vault: vault, Name: "vault", Entries: entries, Index: index, Active: "Graph Terminal Probe/Graph Hub.md"}
}
