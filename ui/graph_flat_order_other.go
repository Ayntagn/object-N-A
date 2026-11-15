//go:build !darwin

package ui

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func withGraphNameComparer(sortPaths func(compare func(string, string) int)) {
	sortPaths(collate.New(language.Und, collate.IgnoreCase).CompareString)
}
