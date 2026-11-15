package ui

import (
	"cyberwriter/internal/core/quickswitcher"
	"cyberwriter/internal/core/workspace"
	"github.com/ebitengine/purego/objc"
)

func quickSwitcherFiles(entries []workspace.Entry) []quickswitcher.File {
	defer nativeAutoreleasePool()()
	names := make(map[string]objc.ID)
	stringForPath := func(path string) objc.ID {
		if name, ok := names[path]; ok {
			return name
		}
		name := nativeNSString(path)
		names[path] = name
		return name
	}
	compare := objc.RegisterName("localizedCaseInsensitiveCompare:")
	return quickswitcher.Files(entries, func(a, b string) int {
		return objc.Send[int](stringForPath(a), compare, stringForPath(b))
	})
}
