package mdshortcuts

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ParseSnippets reads a static LaTeX Suite-style array and validates every
// rule with the same compiler used by the editor.
func ParseSnippets(source string) ([]Rule, error) {
	parser := snippetParser{source: source}
	if parser.atEnd() {
		return nil, nil
	}
	parser.expect('[', "expected a static snippet array starting with '['")
	var rules []Rule
	for parser.err == nil && !parser.take(']') {
		rules = append(rules, parser.rule())
		if parser.take(']') {
			break
		}
		parser.expect(',', "expected ',' or ']' after the snippet")
	}
	parser.take(';')
	if !parser.atEnd() {
		parser.fail(parser.pos, "unexpected content after the snippet array")
	}
	if parser.err != nil {
		return nil, parser.err
	}
	return rules, nil
}

func (p *snippetParser) rule() Rule {
	start := p.position()
	p.expect('{', "expected a snippet object starting with '{'")
	rule := Rule{Enabled: true}
	seen := make(map[string]bool, 6)
	var options, flags, literalFlags string
	optionsAt, flagsAt := start, start
	for p.err == nil && !p.take('}') {
		fieldAt := p.position()
		field := p.field()
		if seen[field] {
			p.fail(fieldAt, "duplicate field %q", field)
		}
		seen[field] = true
		p.expect(':', "expected ':' after the field name")
		if p.err != nil {
			break
		}
		switch field {
		case "trigger":
			if p.peek() == '/' {
				rule.Regex = true
				flagsAt = p.pos
				rule.Trigger, literalFlags = p.regularExpression()
			} else {
				rule.Trigger = p.quoted(field)
			}
		case "replacement":
			rule.Replacement = p.quoted(field)
		case "options":
			optionsAt = p.position()
			options = p.quoted(field)
		case "enabled":
			rule.Enabled = p.boolean()
		case "description":
			rule.Description = p.quoted(field)
		case "priority":
			rule.Priority = p.number()
		case "flags":
			flagsAt = p.position()
			flags = p.quoted(field)
		default:
			p.fail(fieldAt, "unsupported field %q", field)
		}
		if p.take('}') {
			break
		}
		p.expect(',', "expected ',' or '}' after the field value")
	}
	if p.err != nil {
		return Rule{}
	}
	if !seen["trigger"] || !seen["replacement"] {
		p.fail(start, "a snippet needs both trigger and replacement fields")
		return Rule{}
	}
	if err := applySnippetOptions(&rule, options); err != nil {
		p.fail(optionsAt, "%v", err)
		return Rule{}
	}
	if err := applySnippetFlags(&rule, literalFlags+flags); err != nil {
		p.fail(flagsAt, "%v", err)
		return Rule{}
	}
	if _, err := compileRule(rule); err != nil {
		p.fail(start, "%v", err)
	}
	return rule
}

func applySnippetOptions(rule *Rule, options string) error {
	const (
		plain = 1 << iota
		inline
		block
	)
	modes := 0
	visualOption := false
	for _, flag := range options {
		switch flag {
		case 't':
			modes |= plain
		case 'm':
			modes |= inline | block
		case 'M':
			modes |= block
		case 'n':
			modes |= inline
		case 'A':
			rule.Automatic = true
		case 'r':
			rule.Regex = true
		case 'w':
			rule.WholeWord = true
		case 'v':
			visualOption = true
		default:
			return fmt.Errorf("unsupported option %q", flag)
		}
	}
	switch modes {
	case 0, plain | inline | block:
		rule.Scope = Any
	case plain:
		rule.Scope = Text
	case inline | block:
		rule.Scope = Math
	case inline:
		rule.Scope = InlineMath
	case block:
		rule.Scope = BlockMath
	default:
		return fmt.Errorf("unsupported combination of text and one math mode in %q", options)
	}
	if visualOption || !rule.Automatic && strings.Contains(rule.Replacement, "${VISUAL}") {
		_, visual, _, err := parseReplacement(rule.Replacement, false, 0)
		if err != nil {
			return err
		}
		if visualOption && !visual {
			return fmt.Errorf("option 'v' requires ${VISUAL} in a quoted replacement")
		}
		rule.Automatic = rule.Automatic || visual
	}
	return nil
}

func applySnippetFlags(rule *Rule, flags string) error {
	if flags == "" {
		return nil
	}
	if !rule.Regex {
		return fmt.Errorf("regex flags require a regular expression trigger")
	}
	var seen strings.Builder
	for _, flag := range flags {
		if !strings.ContainsRune("ims", flag) {
			return fmt.Errorf("unsupported regular expression flag %q", flag)
		}
		if strings.ContainsRune(seen.String(), flag) {
			return fmt.Errorf("duplicate regular expression flag %q", flag)
		}
		seen.WriteRune(flag)
	}
	rule.Trigger = "(?" + flags + ")" + rule.Trigger
	return nil
}

// FormatSnippets builds editable source from compiled settings that predate
// SnippetSource. Settings can keep the original text for newer configurations.
func FormatSnippets(rules []Rule) string {
	if len(rules) == 0 {
		return "[]\n"
	}
	var out strings.Builder
	out.WriteString("[\n")
	for _, rule := range rules {
		fmt.Fprintf(&out, "  { trigger: %s, replacement: %s, options: %s",
			snippetQuote(rule.Trigger), snippetQuote(rule.Replacement), snippetQuote(snippetOptions(rule)))
		if !rule.Enabled {
			out.WriteString(", enabled: false")
		}
		if rule.Description != "" {
			fmt.Fprintf(&out, ", description: %s", snippetQuote(rule.Description))
		}
		if rule.Priority != 0 {
			fmt.Fprintf(&out, ", priority: %s", strconv.FormatFloat(rule.Priority, 'g', -1, 64))
		}
		out.WriteString(" },\n")
	}
	out.WriteString("]\n")
	return out.String()
}

func snippetQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func snippetOptions(rule Rule) string {
	var options strings.Builder
	switch rule.Scope {
	case Text:
		options.WriteByte('t')
	case Math:
		options.WriteByte('m')
	case InlineMath:
		options.WriteByte('n')
	case BlockMath:
		options.WriteByte('M')
	}
	if rule.Automatic {
		options.WriteByte('A')
	}
	if rule.Regex {
		options.WriteByte('r')
	}
	if rule.WholeWord {
		options.WriteByte('w')
	}
	return options.String()
}
