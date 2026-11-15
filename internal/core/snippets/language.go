package snippets

import (
	"slices"
	"strings"
)

// EV-UI-054 records the detector's ordered content checks.
var languageMarkers = []struct {
	name    string
	markers []string
}{
	{"javascript", []string{"function ", "=>", "const ", "console.log", "document.", "let "}},
	{"typescript", []string{"interface ", ": string", ": number", "import type"}},
	{"cpp", []string{"#include", "std::", "int main"}},
	{"go", []string{"package main", "func ", ":= "}},
	{"rust", []string{"fn ", "let mut", "println!"}},
	{"java", []string{"public class", "system.out.print", "void main"}},
	{"html", []string{"<html", "<div", "</", "<!doctype"}},
	{"sql", []string{"select ", "insert into", "update ", "where ", "from "}},
	{"bash", []string{"#!/bin/", "echo ", "fi\n", "then\n"}},
}

func detectLanguage(content string) string {
	content = strings.ToLower(strings.TrimSpace(content))
	has := func(markers ...string) bool {
		return slices.ContainsFunc(markers, func(marker string) bool { return strings.Contains(content, marker) })
	}
	if strings.ContainsAny(content, "{}") && has("func ", "let ", "var ", "guard ", "import swiftui", "-> ") {
		return "swift"
	}
	if !strings.Contains(content, "{") && has("def ", "import ", "elif ", "print(", "__init__") {
		return "python"
	}
	for _, language := range languageMarkers {
		if has(language.markers...) {
			return language.name
		}
	}
	if strings.HasPrefix(content, "{") || strings.Contains(content, "{") && strings.Contains(content, `":`) {
		return "json"
	}
	if has("# ", "## ", "```", "**") {
		return "markdown"
	}
	return ""
}
