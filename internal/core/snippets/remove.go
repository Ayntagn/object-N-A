package snippets

import (
	"encoding/json"
	"errors"
	"slices"
)

// Remove deletes a leaf by UUID and preserves all other library records.
func Remove(filename, id string) ([]Snippet, error) {
	if id == "" {
		return nil, errors.New("the snippet is no longer in the library")
	}
	collectionWriteMu.Lock()
	defer collectionWriteMu.Unlock()

	items, records, err := readCollection(filename)
	if err != nil {
		return nil, err
	}
	removed, err := removeSnippetRecord(&items, &records, id)
	if err != nil {
		return nil, err
	}
	if !removed {
		return nil, errors.New("the snippet is no longer in the library")
	}
	if err := writeCollection(filename, records); err != nil {
		return nil, err
	}
	return items, nil
}

func removeSnippetRecord(items *[]Snippet, records *[]json.RawMessage, id string) (bool, error) {
	for i := range *items {
		item := &(*items)[i]
		if item.ID == id {
			if item.IsFolder {
				return false, errors.New("the selection is a folder")
			}
			*items = slices.Delete(*items, i, i+1)
			*records = slices.Delete(*records, i, i+1)
			return true, nil
		}
		if len(item.Children) == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal((*records)[i], &fields); err != nil {
			return false, err
		}
		var children []json.RawMessage
		if err := json.Unmarshal(fields["children"], &children); err != nil {
			return false, err
		}
		removed, err := removeSnippetRecord(&item.Children, &children, id)
		if err != nil {
			return false, err
		}
		if !removed {
			continue
		}
		fields["children"], err = json.Marshal(children)
		if err != nil {
			return false, err
		}
		(*records)[i], err = json.Marshal(fields)
		return err == nil, err
	}
	return false, nil
}
