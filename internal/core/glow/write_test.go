// spec: EV-UI-102 / EV-UI-103
package glow

import "testing"

func TestSessionCreatesAndRedirectsWithinVault(t *testing.T) {
	env := glowFixture(t)
	var session Session
	const created = "Output Box/Child/Daily Log.md"
	const related = "Output Box/Related.md"

	session.Submit(`mkdir "Output Box/Child"`, env)
	assertLastLine(t, &session, "created Output Box/Child/", "")
	session.Submit(`mkdir "Output Box/Child"`, env)
	assertLastLine(t, &session, "created Output Box/Child/", "")
	session.Submit(`new "Output Box/Child/Daily Log" "Station Survey"`, env)
	assertLastLine(t, &session, "created "+created, created)
	session.Submit(`new "Output Box/Child/Daily Log.md" "Different Title"`, env)
	assertLastLine(t, &session, "already exists - "+created, created)

	session.Submit(`backlinks "Linked Note" > "Output Box/Related"`, env)
	assertLastLine(t, &session, "wrote 2 lines → "+related, related)
	session.Submit(`pwd > "Output Box/Related.md"`, env)
	assertLastLine(t, &session, "won't overwrite - "+related+"  (pick a new name, or use > this)", related)

	for path, want := range map[string]string{
		created: "# Station Survey\n\n",
		related: "1 → [[Graph Terminal Probe/Research/Linked Note]]\n  [[Graph Terminal Probe/Graph Hub]]\n",
	} {
		doc, err := env.Vault.ReadDocument(path)
		if err != nil || doc.Text != want {
			t.Errorf("read %q = %q, %v, want %q", path, doc.Text, err, want)
		}
	}
}
