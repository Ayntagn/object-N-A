//go:build !darwin

package ui

const sourceSpellAvailable = false

func requestSystemSourceSpelling(string, func([]sourceSpellingRange)) {}
