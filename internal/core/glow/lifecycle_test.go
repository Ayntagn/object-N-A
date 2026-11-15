// spec: EV-UI-104 / EV-UI-105
package glow

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestMoveUsesExactPathsAndRetainsSourceExtension(t *testing.T) {
	env := glowFixture(t)
	var session Session
	const target = "Moved/Hub.md"
	session.Submit(`mv "Graph Terminal Probe/Graph Hub" "Moved/Hub"`, env)
	assertLastLine(t, &session, "no such file: Graph Terminal Probe/Graph Hub", "")
	session.Submit(`mv this "Moved/Hub"`, env)
	assertLastLine(t, &session, env.Active+" → "+target, target)
	session.Submit(`mv this "Moved/Other"`, env)
	assertLastLine(t, &session, "no such file: "+env.Active, "")
	session.Submit(`mv "Graph Terminal Probe/Research" Moved`, env)
	assertLastLine(t, &session, "Graph Terminal Probe/Research → Moved/Research", "Moved/Research")
	session.Submit(`mv "Moved/Research/Linked Note.md" "Moved/Hub.md"`, env)
	assertLastLine(t, &session, "destination exists: "+target, "")
	for _, filename := range []string{target, "Moved/Research/Linked Note.md"} {
		if _, err := env.Vault.ReadDocument(filename); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDumpPreservesTranscriptAndUsesUniqueRootFilename(t *testing.T) {
	env := glowFixture(t)
	env.Name = "Terminal Lifecycle Probe"
	const spare = "# Spare\n\n```text\na code fence is part of the note\n```\n"
	if _, err := env.Vault.CreateDocument("Spare.md", spare); err != nil {
		t.Fatal(err)
	}
	var session Session
	session.Submit("clear", env)
	session.Submit("pwd", env)
	session.Submit("cat Spare.md", env)
	result := session.Submit(`dump "Session Capture"`, env)
	if result.OpenPath != "Session Capture.md" {
		t.Fatalf("archive opened %q", result.OpenPath)
	}
	archive, err := env.Vault.ReadDocument(result.OpenPath)
	if err != nil {
		t.Fatal(err)
	}
	_, after, _ := strings.Cut(archive.Text, "created: ")
	date, _, _ := strings.Cut(after, "\n")
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		t.Fatalf("archive date %q: %v", date, err)
	}
	want := "---\ntags: [glow, session]\ncreated: " + date + "\n---\n\n" +
		"# GLOW session - Terminal Lifecycle Probe\n\n" +
		"*Saved from the GLOW terminal on " + date + " - every command and its output.*\n\n" +
		"````text\nTerminal Lifecycle Probe ▸ pwd\nTerminal Lifecycle Probe/\n\n" +
		"Terminal Lifecycle Probe ▸ cat Spare.md\n── Spare.md \n# Spare\n \n```text\n" +
		"a code fence is part of the note\n```\n \n\n" +
		"Terminal Lifecycle Probe ▸ dump \"Session Capture\"\n````"
	if archive.Text != want {
		t.Fatalf("archive = %q, want %q", archive.Text, want)
	}
	result = session.Submit(`dump "Session Capture" > "No Redirect.md"`, env)
	if result.OpenPath != "Session Capture 2.md" {
		t.Fatalf("second archive opened %q", result.OpenPath)
	}
	second, err := env.Vault.ReadDocument(result.OpenPath)
	if err != nil || !strings.Contains(second.Text, "\nnothing to write\n") {
		t.Fatalf("redirected archive = %q, %v", second.Text, err)
	}
	if _, err := env.Vault.ReadDocument("No Redirect.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unexpected redirect file: %v", err)
	}
}
