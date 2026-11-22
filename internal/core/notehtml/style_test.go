// spec: EV-MYGO-301
package notehtml

import "testing"

func TestCleanStyleKeepsNoteFormatting(t *testing.T) {
	raw := `FONT-FAMILY: "Iosevka; Notes", cursive; Color: rgb(32 60 80 / 75%); text-align: center; width: calc(100% - 2em); border: 1px solid var(--text-accent, #aaa); background: #eee; font-weight: bold !important;`
	want := `font-family: "Iosevka; Notes", cursive; color: rgb(32 60 80 / 75%); text-align: center; width: calc(100% - 2em); border: 1px solid var(--text-accent, #aaa); background-color: #eee; font-weight: bold !important`
	if got := cleanStyle(raw); got != want {
		t.Errorf("cleanStyle = %q, want %q", got, want)
	}
}

func TestCleanStyleKeepsPassiveNeighbors(t *testing.T) {
	raw := `color: red; background: url(example.png); list-style-image: var(--custom); position: fixed; --custom: #aaa; font-family: invalid/* ; color: blue; */; border-color: var(--color, URL(example.svg)); width: calc(1px + 2px); text-decoration: underline; color: black` + "\x00"
	want := `color: red; width: calc(1px + 2px); text-decoration: underline`
	if got := cleanStyle(raw); got != want {
		t.Errorf("cleanStyle = %q, want %q", got, want)
	}
}
