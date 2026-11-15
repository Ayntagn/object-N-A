// spec: EV-UI-112 / EV-UI-113
package glow

import (
	"strings"
	"testing"
)

func TestRunScriptUsesVaultSnapshotAndWritesRedirect(t *testing.T) {
	env := glowFixture(t)
	if err := env.Vault.EnsureFolder("Scripts"); err != nil {
		t.Fatal(err)
	}
	_, err := env.Vault.CreateDocument("Scripts/Station Log.md", ""+
		"~~~cw\nnew Skipped\n~~~\n"+
		"  ````CW\n# start\n#= start\n"+
		"mkdir Fresh\ncd Fresh\nnew Log Café Straße\ncat Log\n"+
		"#= CAFE\u0301 STRASSE\n#= CAFE STRASSE\n"+
		"this\nrun this\n#= scripts can't run scripts\n"+
		"````\nnew Also Skipped\n")
	if err != nil {
		t.Fatal(err)
	}
	var session Session
	input := `run "~/Scripts/Station Log" > "../Receipt"`
	result := session.Submit(input, env)
	if !result.VaultChanged || result.OpenPath != "" || session.Prompt(env.Name) != "Fresh>" {
		t.Fatalf("script redirect result = %+v, prompt %q", result, session.Prompt(env.Name))
	}
	assertLastLine(t, &session, "wrote 29 lines → Receipt.md", "Receipt.md")
	want := strings.Join([]string{
		"── run [[Scripts/Station Log]]", "# start", `  ✗ expected "start"`, " ",
		"» mkdir Fresh", "created Fresh/", " ", "» cd Fresh", "vault/Fresh/", " ",
		"» new Log Café Straße", "created [[Fresh/Log]]", " ", "» cat Log",
		"[[Fresh/Log]]", "# Café Straße", " ", " ", `  ✓ contains "CAFÉ STRASSE"`, `  ✗ expected "CAFE STRASSE"`, " ",
		"» this", "[[Graph Terminal Probe/Graph Hub]]", " ",
		"» run this", "scripts can't run scripts", `  ✓ contains "scripts can't run scripts"`, " ",
		"── done - 6 commands  ·  2 ✓  ·  2 ✗", "",
	}, "\n")
	doc, err := env.Vault.ReadDocument("Receipt.md")
	if err != nil || doc.Text != want {
		t.Fatalf("script receipt = %q, %v, want %q", doc.Text, err, want)
	}
	var cursor HistoryCursor
	if got := session.RecallFrom(&cursor, "", -1); got != input {
		t.Fatalf("history recalled %q, want %q", got, input)
	}
}

func TestRunScriptChecksTailUntilNextCommand(t *testing.T) {
	env := glowFixture(t)
	_, err := env.Vault.CreateDocument("Long.md", "```cw\n"+strings.Repeat("pwd\n", 200)+"# after cap\n#= vault/\ncd /missing\n# after stop\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	var session Session
	session.Submit("run Long", env)
	transcript, commands := session.transcript()
	want := "# after cap\n  ✓ contains \"vault/\"\n \n── done - 200 commands  ·  1 ✓  (stopped at the 200-command cap)"
	if commands != 1 || strings.Count(transcript, "» pwd\n") != 200 || !strings.HasSuffix(transcript, want) {
		t.Fatalf("commands = %d, transcript tail = %q", commands, transcript[max(0, len(transcript)-200):])
	}
}
