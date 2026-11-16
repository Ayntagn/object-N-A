// spec: EV-UI-226 / EV-UI-230 / EV-UI-286 / EV-UI-288 / EV-MYGO-194
package ui

import (
	"slices"
	"strings"
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
		form.draw(c, languageEnglish, action("Refresh"), action("Test"), action("Save"))
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
	click(control(mygoui.RoleButton, "cURL"))
	for _, part := range []string{"curl -X POST", address + "/chat/completions", "Authorization: Bearer " + key, `"model":"fixture-model"`, `"content":"Hello"`, `"max_tokens":10`} {
		if !strings.Contains(tester.Clipboard(), part) {
			t.Fatalf("cURL did not copy the current draft's %q", part)
		}
	}
	if !tester.HasText("cURL copied to clipboard") {
		t.Fatal("cURL did not show copy feedback")
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

func TestAIConnectionProbesKeepFocusDuringBusy(t *testing.T) {
	form := workspaceAIConnectionForm{Status: "Connected - previous-model"}
	view := mygoui.NewTester(func(c *mygoui.Context) {
		form.draw(c, languageEnglish, func() { form.Refreshing = true }, func() { form.Testing = true }, nil)
	}, 632, 320)
	for _, probe := range []struct{ field, button string }{{"Base URL", "Refresh"}, {"Model", "Test"}} {
		field, _ := view.AccessByRole(mygoui.RoleTextField, probe.field)
		view.ClickAt(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
		if err := view.Click(probe.button); err != nil {
			t.Fatal(err)
		}
		button, ok := view.AccessByRole(mygoui.RoleButton, probe.button)
		if !ok || !button.Busy || !button.Disabled || !view.Focused(probe.field) {
			t.Fatalf("%s should be busy and disabled while %s keeps focus", probe.button, probe.field)
		}
		if _, ok := view.AccessByRole(mygoui.RoleStatus, "Connected - previous-model"); !ok {
			t.Fatal("the previous connection result disappeared during a request")
		}
	}
	form.Refreshing, form.Testing = false, false
	form.Status, form.StatusError = "Authentication failed - check API key", true
	view.Frame()
	if _, ok := view.AccessByRole(mygoui.RoleAlert, form.Status); !ok {
		t.Fatal("failed Test did not publish an accessible error")
	}
}
