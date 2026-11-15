package workspace

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type PropertyKind string

const (
	PropertyText     PropertyKind = "Text"
	PropertyNumber   PropertyKind = "Number"
	PropertyCheckbox PropertyKind = "Checkbox"
	PropertyDate     PropertyKind = "Date"
	PropertyList     PropertyKind = "List"
	PropertyObject   PropertyKind = "Object"
	PropertyNull     PropertyKind = "Null"
)

type Property struct {
	Key   string
	Kind  PropertyKind
	Text  string
	Items []Property
}

type PropertyAction uint8

const (
	SetProperty PropertyAction = iota
	AddProperty
	RemoveProperty
	AddPropertyItem
	RemovePropertyItem
)

type PropertyEdit struct {
	Action PropertyAction
	Key    string
	Kind   PropertyKind
	Text   string
	Item   int
}

// Properties is an ordered view of one draft's metadata. Editing produces a new
// source string, preserving the body and the format of the original block.
type Properties struct {
	Rows              []Property
	prefix, raw, body string
	yaml              *yaml.Node
	toml              map[string]any
}

func ReadProperties(source string) (Properties, error) {
	prefix, raw, body := SplitFrontmatter(source)
	p := Properties{prefix: prefix, raw: raw, body: body}
	if strings.HasPrefix(strings.TrimPrefix(prefix, "\ufeff"), "+++") {
		return readTOMLProperties(p)
	}
	return readYAMLProperties(p)
}

func (p Properties) Edit(edit PropertyEdit) (string, error) {
	index := -1
	for i, row := range p.Rows {
		if row.Key == edit.Key {
			if index >= 0 {
				return "", fmt.Errorf("the property %q occurs more than once", edit.Key)
			}
			index = i
		}
	}
	if edit.Action == AddProperty {
		if index >= 0 {
			return "", fmt.Errorf("the property %q already exists", edit.Key)
		}
		if strings.TrimSpace(edit.Key) == "" || strings.ContainsAny(edit.Key, "\r\n\x00") {
			return "", errors.New("enter a property name")
		}
	} else {
		if index < 0 {
			return "", fmt.Errorf("the property %q could not be found", edit.Key)
		}
		row := p.Rows[index]
		edit.Kind = row.Kind
		switch edit.Action {
		case SetProperty:
			if row.Text == edit.Text {
				return p.prefix + p.body, nil
			}
		case AddPropertyItem, RemovePropertyItem:
			if row.Kind != PropertyList || (edit.Action == RemovePropertyItem && (edit.Item < 0 || edit.Item >= len(row.Items))) {
				return "", errors.New("the list property has changed")
			}
			edit.Text = strings.TrimSpace(edit.Text)
			if edit.Action == AddPropertyItem && edit.Text == "" {
				return p.prefix + p.body, nil
			}
		}
	}
	var encoded string
	var count int
	var err error
	if p.toml != nil {
		encoded, count, err = p.editTOML(edit)
	} else {
		encoded, count, err = p.editYAML(index, edit)
	}
	if err != nil {
		return "", err
	}
	return p.join(encoded, count), nil
}

func (p Properties) join(encoded string, count int) string {
	if strings.TrimPrefix(p.prefix, "\ufeff") == "" {
		if count == 0 {
			return p.prefix + p.body
		}
		line := "\n"
		if i := strings.IndexByte(p.body, '\n'); i > 0 && p.body[i-1] == '\r' {
			line = "\r\n"
			encoded = strings.ReplaceAll(encoded, "\n", line)
		}
		separator := ""
		if p.body != "" && !strings.HasPrefix(p.body, "\n") && !strings.HasPrefix(p.body, "\r\n") {
			separator = line
		}
		return p.prefix + "---" + line + encoded + "---" + line + separator + p.body
	}
	if count == 0 {
		return p.prefix[:len(p.prefix)-len(strings.TrimPrefix(p.prefix, "\ufeff"))] + p.body
	}
	open, _, _ := strings.Cut(p.prefix, "\n")
	if strings.HasSuffix(open, "\r") {
		encoded = strings.ReplaceAll(encoded, "\n", "\r\n")
	}
	return open + "\n" + encoded + p.prefix[len(open)+1+len(p.raw):] + p.body
}

func propertyScalar(edit PropertyEdit) (any, error) {
	if edit.Action == AddProperty {
		switch edit.Kind {
		case PropertyText:
			return "", nil
		case PropertyNumber:
			return int64(0), nil
		case PropertyCheckbox:
			return false, nil
		case PropertyDate:
			return time.Now().Format(time.DateOnly), nil
		case PropertyList:
			return []any{}, nil
		}
	}
	switch edit.Kind {
	case PropertyText:
		return edit.Text, nil
	case PropertyCheckbox:
		return strconv.ParseBool(edit.Text)
	case PropertyDate:
		if _, err := time.Parse(time.DateOnly, edit.Text); err != nil {
			return nil, errors.New("enter a date in YYYY-MM-DD format")
		}
		return edit.Text, nil
	case PropertyNumber:
		text := strings.TrimSpace(edit.Text)
		if value, err := strconv.ParseInt(text, 10, 64); err == nil {
			return value, nil
		}
		if value, err := strconv.ParseFloat(text, 64); err == nil && !math.IsInf(value, 0) && !math.IsNaN(value) {
			return value, nil
		}
		return nil, errors.New("enter a valid number")
	default:
		return nil, errors.New("this property's value is read-only")
	}
}

func propertyDay(current time.Time, text string) time.Time {
	date, _ := time.Parse(time.DateOnly, text)
	year, month, day := date.Date()
	return time.Date(year, month, day, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
}
