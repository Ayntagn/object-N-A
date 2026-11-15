package snippets

// FindByTitle resolves a reference through the collection's folders.
func FindByTitle(items []Snippet, title string) (Snippet, bool) {
	for _, item := range items {
		if !item.IsFolder && item.Title == title {
			return item, true
		}
		if child, found := FindByTitle(item.Children, title); found {
			return child, true
		}
	}
	return Snippet{}, false
}
