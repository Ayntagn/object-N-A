// spec: EV-UI-107 / EV-UI-108
package glow

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"cyberwriter/internal/core/vaultindex"
)

func TestDemoUsesVaultDataAndStopsWithoutExecutingInput(t *testing.T) {
	env := glowFixture(t)
	var session Session
	session.Submit("demo", env)
	assertLastLine(t, &session, "→ open the Example Vault   then run demo again.", ExampleVaultLink)
	if session.DemoRunning() {
		t.Fatal("demo started without the example notes")
	}
	for name, source := range map[string]string{
		"Marcus Kim.md":          "---\ntags: [person]\n---\n\n# Example\n",
		"System Architecture.md": "# Example\n\n```text\n #person #ff6b6b\n```\n" + strings.Repeat("thermal\n", 8),
	} {
		if _, err := env.Vault.CreateDocument(name, source); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	env.Entries, err = env.Vault.Entries()
	if err != nil {
		t.Fatal(err)
	}
	env.Index, err = vaultindex.Build(env.Vault, env.Entries)
	if err != nil {
		t.Fatal(err)
	}
	session.Submit("demo", env)
	if !session.DemoRunning() {
		t.Fatal("example notes did not start demo")
	}
	if output, _ := session.transcript(); !strings.Contains(output, "open the Example Vault") {
		t.Fatal("starting the guide removed earlier scrollback")
	}
	for i := 0; i < 5; i++ {
		for {
			_, delay := session.TickDemo(env)
			if delay == 0 {
				break
			}
		}
		if i < 4 {
			session.Submit("", env)
		}
	}
	crew, err := env.Vault.ReadDocument("_demo/crew.md")
	if err != nil || crew.Text != "#person  2 notes\n  [[Marcus Kim]]\n  [[System Architecture]]\n" {
		t.Fatalf("roster = %q, %v", crew.Text, err)
	}
	output, _ := session.transcript()
	colorIndexed := slices.ContainsFunc(env.Index.Graph().Tags, func(tag vaultindex.GraphTag) bool { return tag.Name == "ff6b6b" })
	if !strings.Contains(output, "\n5 matches\n") || colorIndexed {
		t.Fatal("demo should show five hits per note and ignore the color literal")
	}
	session.Submit(`new "Unintended Note"`, env)
	assertLastLine(t, &session, "- demo stopped · type help -", "")
	if session.DemoRunning() {
		t.Fatal("nonempty input did not stop the demo")
	}
	if _, err := env.Vault.ReadDocument("Unintended Note.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stop executed the input: %v", err)
	}
	if _, err := env.Vault.ReadDocument("_demo/crew.md"); err != nil {
		t.Fatalf("stop removed the roster: %v", err)
	}
}
