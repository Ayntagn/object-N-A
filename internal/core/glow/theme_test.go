// spec: EV-UI-106 / EV-UI-108
package glow

import "testing"

func TestThemeCommandUsesFirstArgumentAndRedirectsOnlyTheReply(t *testing.T) {
	env := glowFixture(t)
	var session Session
	if effect := session.Submit("theme MATRIX ignored", env); effect.Theme != "matrix" {
		t.Fatalf("theme effect = %q", effect.Theme)
	}
	assertLastLine(t, &session, "theme → matrix", "")
	session.Submit("theme", env)
	assertLastLine(t, &session, "themes → plato, mono, matrix", "")
	session.Submit("theme banana", env)
	assertLastLine(t, &session, "unknown theme: banana (plato, mono, matrix)", "")
	if effect := session.Submit(`theme mono > "Theme Receipt"`, env); effect.Theme != "" || !effect.VaultChanged {
		t.Fatalf("redirect effect = %+v", effect)
	}
	doc, err := env.Vault.ReadDocument("Theme Receipt.md")
	if err != nil || doc.Text != "theme → mono\n" {
		t.Fatalf("redirect file = %q, %v", doc.Text, err)
	}
}
