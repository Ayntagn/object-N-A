package ui

import "github.com/ebitengine/purego/objc"

func withGraphNameComparer(sortPaths func(compare func(string, string) int)) {
	defer nativeAutoreleasePool()()
	names := make(map[string]objc.ID)
	stringForName := func(name string) objc.ID {
		if native, ok := names[name]; ok {
			return native
		}
		native := nativeNSString(name)
		names[name] = native
		return native
	}
	compare := objc.RegisterName("localizedCaseInsensitiveCompare:")
	sortPaths(func(a, b string) int {
		return objc.Send[int](stringForName(a), compare, stringForName(b))
	})
}
