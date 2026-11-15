package workspace

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

func readTOMLProperties(p Properties) (Properties, error) {
	p.toml = make(map[string]any)
	meta, err := toml.Decode(p.raw, &p.toml)
	if err != nil {
		return Properties{}, err
	}
	seen := make(map[string]bool)
	for _, key := range meta.Keys() {
		if name := key[0]; !seen[name] {
			seen[name] = true
			p.Rows = append(p.Rows, tomlProperty(name, p.toml[name]))
		}
	}
	return p, nil
}

func tomlProperty(key string, value any) Property {
	prop := Property{Key: key, Kind: PropertyNull, Text: fmt.Sprint(value)}
	switch value := value.(type) {
	case string:
		prop.Kind = PropertyText
	case int64, float64:
		prop.Kind = PropertyNumber
	case bool:
		prop.Kind = PropertyCheckbox
	case time.Time:
		if value.Year() == 0 {
			prop.Text = value.Format("15:04:05")
		} else {
			prop.Kind, prop.Text = PropertyDate, value.Format(time.DateOnly)
		}
	case []any:
		prop.Kind = PropertyList
		for _, item := range value {
			prop.Items = append(prop.Items, tomlProperty("", item))
		}
	case map[string]any:
		prop.Kind = PropertyObject
		for _, child := range slices.Sorted(maps.Keys(value)) {
			prop.Items = append(prop.Items, tomlProperty(child, value[child]))
		}
	case []map[string]any:
		prop.Kind = PropertyObject
		for i, item := range value {
			prop.Items = append(prop.Items, tomlProperty(fmt.Sprint(i+1), item))
		}
	}
	return prop
}

func (p Properties) editTOML(edit PropertyEdit) (string, int, error) {
	values := maps.Clone(p.toml)
	switch edit.Action {
	case RemoveProperty:
		delete(values, edit.Key)
	case AddPropertyItem, RemovePropertyItem:
		items := slices.Clone(values[edit.Key].([]any))
		if edit.Action == AddPropertyItem {
			items = append(items, edit.Text)
		} else {
			items = slices.Delete(items, edit.Item, edit.Item+1)
		}
		values[edit.Key] = items
	case AddProperty, SetProperty:
		value, err := propertyScalar(edit)
		if err != nil {
			return "", 0, err
		}
		if edit.Kind == PropertyDate {
			if existing, ok := values[edit.Key].(time.Time); ok {
				value = propertyDay(existing, value.(string))
			} else {
				var parsed struct{ Value time.Time }
				if _, err := toml.Decode("value = "+value.(string), &parsed); err != nil {
					return "", 0, err
				}
				value = parsed.Value
			}
		}
		values[edit.Key] = value
	default:
		return "", 0, errors.New("unknown property action")
	}
	var output strings.Builder
	encoder := toml.NewEncoder(&output)
	encoder.Indent = ""
	err := encoder.Encode(values)
	return output.String(), len(values), err
}
