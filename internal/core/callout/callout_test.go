// spec: FEAT-WYS-CALLOUT / EV-UI-155 / EV-MYGO-274.
package callout

import (
	"errors"
	"testing"
)

func TestStyleOverridesAndAliases(t *testing.T) {
	warning, err := Normalize(Style{Kind: " WARNING ", Color: "#123ABC", Icon: "✦"})
	if err != nil {
		t.Fatal(err)
	}
	styles := []Style{warning, {Kind: "lab-note", Name: "实验记录", Icon: "🧪"}}
	if got := Resolve("CAUTION", styles); got.Kind != "caution" || got.Color != "#123abc" || got.Icon != "✦" {
		t.Fatalf("style of caution = %+v", got)
	}
	menu := Menu(styles)
	if len(menu) != 12 || menu[3].Kind != "warning" || menu[11].Kind != "lab-note" || menu[11].Name != "实验记录" || menu[11].Color != Resolve("note", nil).Color {
		t.Fatalf("menu = %+v", menu)
	}
	if _, err := Normalize(Style{Kind: "note", Color: "#123456; display: none"}); !errors.Is(err, ErrColor) {
		t.Fatalf("invalid accent accepted: %v", err)
	}
}
