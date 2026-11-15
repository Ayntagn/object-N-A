package workspace

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

func readYAMLProperties(p Properties) (Properties, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(p.raw), &root); err != nil {
		return Properties{}, err
	}
	p.yaml = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if len(root.Content) > 0 {
		p.yaml = root.Content[0]
		if p.yaml.Kind != yaml.MappingNode {
			return Properties{}, errors.New("frontmatter must contain named properties")
		}
	}
	for i := 0; i < len(p.yaml.Content); i += 2 {
		key := p.yaml.Content[i]
		if key.Kind != yaml.ScalarNode {
			return Properties{}, errors.New("frontmatter contains a structured key")
		}
		p.Rows = append(p.Rows, yamlProperty(key.Value, p.yaml.Content[i+1]))
	}
	return p, nil
}

func yamlProperty(key string, value *yaml.Node) Property {
	prop := Property{Key: key, Text: value.Value, Kind: PropertyNull}
	switch value.Kind {
	case yaml.SequenceNode:
		prop.Kind = PropertyList
		for _, item := range value.Content {
			prop.Items = append(prop.Items, yamlProperty("", item))
		}
	case yaml.MappingNode:
		prop.Kind = PropertyObject
		for i := 0; i < len(value.Content); i += 2 {
			prop.Items = append(prop.Items, yamlProperty(value.Content[i].Value, value.Content[i+1]))
		}
	case yaml.AliasNode:
		prop.Text = "*" + value.Value
	case yaml.ScalarNode:
		switch value.ShortTag() {
		case "!!str":
			prop.Kind = PropertyText
		case "!!int", "!!float", "!!bool":
			var decoded any
			if value.Decode(&decoded) == nil {
				prop.Kind = PropertyNumber
				if value.ShortTag() == "!!bool" {
					prop.Kind = PropertyCheckbox
				}
				prop.Text = fmt.Sprint(decoded)
			}
		case "!!timestamp":
			var date time.Time
			if value.Decode(&date) == nil {
				prop.Kind, prop.Text = PropertyDate, date.Format(time.DateOnly)
			}
		}
	}
	return prop
}

func (p Properties) editYAML(index int, edit PropertyEdit) (string, int, error) {
	mapping := *p.yaml
	mapping.Content = slices.Clone(mapping.Content)
	i := index * 2
	switch edit.Action {
	case RemoveProperty:
		mapping.Content = slices.Delete(mapping.Content, i, i+2)
	case AddPropertyItem, RemovePropertyItem:
		value := *mapping.Content[i+1]
		value.Content = slices.Clone(value.Content)
		if edit.Action == AddPropertyItem {
			value.Content = append(value.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: edit.Text})
		} else {
			value.Content = slices.Delete(value.Content, edit.Item, edit.Item+1)
		}
		mapping.Content[i+1] = &value
	case AddProperty, SetProperty:
		scalar, err := propertyScalar(edit)
		if err != nil {
			return "", 0, err
		}
		var value yaml.Node
		if edit.Kind == PropertyDate {
			value = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: scalar.(string)}
			if edit.Action == SetProperty && len(mapping.Content[i+1].Value) > len(time.DateOnly) {
				var previous time.Time
				if mapping.Content[i+1].Decode(&previous) == nil {
					value.Value = propertyDay(previous, value.Value).Format(time.RFC3339Nano)
				}
			}
		} else if err := value.Encode(scalar); err != nil {
			return "", 0, err
		}
		if edit.Action == AddProperty {
			mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: edit.Key}, &value)
		} else {
			previous := mapping.Content[i+1]
			value.Anchor, value.HeadComment, value.LineComment, value.FootComment = previous.Anchor, previous.HeadComment, previous.LineComment, previous.FootComment
			if edit.Kind == PropertyText {
				value.Style = previous.Style
			}
			mapping.Content[i+1] = &value
		}
	default:
		return "", 0, errors.New("unknown property action")
	}
	count := len(mapping.Content) / 2
	if count == 0 {
		return "", count, nil
	}
	var output strings.Builder
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	encoder.CompactSeqIndent()
	if err := encoder.Encode(&mapping); err != nil {
		return "", 0, err
	}
	if err := encoder.Close(); err != nil {
		return "", 0, err
	}
	// Removing a property may also remove an anchor used by a different row.
	var check yaml.Node
	if err := yaml.Unmarshal([]byte(output.String()), &check); err != nil {
		return "", 0, fmt.Errorf("could not update the YAML properties: %w", err)
	}
	return output.String(), count, nil
}
