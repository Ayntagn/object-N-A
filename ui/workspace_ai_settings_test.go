// spec: EV-UI-226 / EV-UI-230
package ui

import (
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestAIConnectionForm(t *testing.T) {
	const address, model, selected, key = "http://127.0.0.1:1234/v1", "fixture-model", "selected-model", "fixture-key"
	var form workspaceAIConnectionForm
	var calls []string
	action := func(label string) func() {
		return func() {
			wantModel := selected
			if label == "Refresh" {
				wantModel = model
				form.Models = []string{model, selected}
			}
			if form.Draft.BaseURL != address || form.Draft.Model != wantModel || form.Draft.APIKey != key {
				t.Fatalf("%s did not receive the edited connection", label)
			}
			calls = append(calls, label)
		}
	}
	tester := mygoui.NewTester(func(c *mygoui.Context) {
		form.draw(c, action("Refresh"), action("Test"), action("Save"))
	}, 632, 320)
	control := func(role mygoui.Role, label string) mygoui.AccessInfo {
		t.Helper()
		item, ok := tester.AccessByRole(role, label)
		if !ok || item.Disabled {
			t.Fatalf("missing enabled %q control", label)
		}
		return item
	}
	click := func(item mygoui.AccessInfo) {
		tester.ClickAt(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2)
	}
	for _, field := range [...]struct{ label, value string }{
		{"Base URL", address}, {"Model", model}, {"API Key (optional)", key},
	} {
		click(control(mygoui.RoleTextField, field.label))
		tester.Type(field.value)
	}
	if field := control(mygoui.RoleTextField, "API Key (optional)"); !field.Password || field.Value != "" {
		t.Fatal("masked API key was exposed in accessibility")
	}
	click(control(mygoui.RoleButton, "Show"))
	if field := control(mygoui.RoleTextField, "API Key (optional)"); field.Password || field.Value != key {
		t.Fatal("Show did not reveal the current draft key")
	}
	click(control(mygoui.RoleButton, "Hide"))
	if field := control(mygoui.RoleTextField, "API Key (optional)"); !field.Password || field.Value != "" {
		t.Fatal("Hide did not restore password masking")
	}
	click(control(mygoui.RoleButton, "Refresh"))
	click(control(mygoui.RolePopUpButton, "Model"))
	tester.Key(0, mygoui.KeyEnd)
	tester.Key(0, mygoui.KeyEnter)
	for _, label := range [...]string{"Test", "Save"} {
		click(control(mygoui.RoleButton, label))
	}
	if !slices.Equal(calls, []string{"Refresh", "Test", "Save"}) {
		t.Fatalf("connection actions = %v", calls)
	}
}
