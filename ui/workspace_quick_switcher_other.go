//go:build !darwin

package ui

import (
	"cyberwriter/internal/core/quickswitcher"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func quickSwitcherFiles(entries []workspace.Entry) []quickswitcher.File {
	tag := language.Und
	if preferred := mygo.App.PreferredLanguages(); len(preferred) > 0 {
		tag = language.Make(preferred[0])
	}
	return quickswitcher.Files(entries, collate.New(tag, collate.IgnoreCase).CompareString)
}
