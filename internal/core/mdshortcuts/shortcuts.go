package mdshortcuts

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Scope string

const (
	Any        Scope = ""
	Text       Scope = "text"
	Math       Scope = "math"
	InlineMath Scope = "inlineMath"
	BlockMath  Scope = "blockMath"
)

type Rule struct {
	Trigger     string  `json:"trigger"`
	Replacement string  `json:"replacement"`
	Description string  `json:"description,omitempty"`
	Scope       Scope   `json:"scope,omitempty"`
	Priority    float64 `json:"priority,omitempty"`
	Enabled     bool    `json:"enabled"`
	Automatic   bool    `json:"automatic,omitempty"`
	Regex       bool    `json:"regex,omitempty"`
	WholeWord   bool    `json:"wholeWord,omitempty"`
}

type Config struct {
	Enabled       bool   `json:"enabled"`
	Rules         []Rule `json:"rules,omitempty"`
	SnippetSource string `json:"snippets,omitempty"`

	cached *Program
}

// Program caches compiled rules per preferences value. Settings editing and
// document input use it on the UI thread.
func (c *Config) Program() (*Program, error) {
	if c.cached != nil && slices.Equal(c.Rules, c.cached.source) {
		return c.cached, nil
	}
	var err error
	c.cached, err = Compile(c.Rules)
	return c.cached, err
}

type Program struct {
	source []Rule
	rules  []compiledRule
}

type compiledRule struct {
	Rule
	pattern *regexp.Regexp
	parts   []part
	visual  bool
	stops   int
}

// Compile validates a batch before Settings accepts it.
func Compile(rules []Rule) (*Program, error) {
	p := &Program{source: slices.Clone(rules)}
	for i, rule := range rules {
		compiled, err := compileRule(rule)
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", i+1, err)
		}
		if compiled.Enabled {
			p.rules = append(p.rules, compiled)
		}
	}
	return p, nil
}

func compileRule(rule Rule) (compiledRule, error) {
	c := compiledRule{Rule: rule}
	if strings.TrimSpace(rule.Trigger) == "" || strings.ContainsAny(rule.Trigger, "\r\n") || !utf8.ValidString(rule.Trigger) {
		return c, errors.New("enter a non-blank trigger on one line")
	}
	switch rule.Scope {
	case Any, Text, Math, InlineMath, BlockMath:
	default:
		return c, fmt.Errorf("unsupported scope %q", rule.Scope)
	}
	if !utf8.ValidString(rule.Replacement) {
		return c, errors.New("the replacement must be valid UTF-8")
	}
	groups := 0
	if rule.Regex {
		var err error
		c.pattern, err = regexp.Compile("(?:" + rule.Trigger + ")\\z")
		if err != nil {
			return c, fmt.Errorf("invalid regular expression: %w", err)
		}
		groups = c.pattern.NumSubexp()
		if c.pattern.MatchString("") {
			return c, errors.New("a trigger must match at least one character")
		}
	}
	var err error
	c.parts, c.visual, c.stops, err = parseReplacement(rule.Replacement, rule.Regex, groups)
	if err != nil {
		return c, err
	}
	if c.visual && (rule.Regex || !rule.Automatic || utf8.RuneCountInString(rule.Trigger) != 1) {
		return c, errors.New("a ${VISUAL} rule needs one literal automatic trigger character")
	}
	return c, nil
}

type Input struct {
	Markdown   string
	Start, End int // UTF-8 byte offsets of the current selection
	Typed      string
}

type Expansion struct {
	Start, End int
	Template   string // TextMate syntax, with numbered stops in traversal order
	TabStops   int    // unique user stops before TextMate's final exit
}

// Match prefers higher priority, then the longest enabled suffix. The first
// saved rule wins ties. Typed is empty only for an explicit Tab request.
func (p *Program) Match(in Input) (Expansion, bool) {
	if p == nil || len(p.rules) == 0 || in.Start < 0 || in.Start > in.End || in.End > len(in.Markdown) {
		return Expansion{}, false
	}
	if !utf8Boundary(in.Markdown, in.Start) || !utf8Boundary(in.Markdown, in.End) || strings.ContainsAny(in.Typed, "\r\n") {
		return Expansion{}, false
	}
	before, selection := in.Markdown[:in.Start], in.Markdown[in.Start:in.End]
	lineStart := strings.LastIndexByte(before, '\n') + 1
	line := before[lineStart:] + in.Typed
	best := -1
	bestPriority := 0.0
	var result Expansion
	for _, rule := range p.rules {
		if (in.Typed != "") != rule.Automatic || (selection != "") != rule.visual {
			continue
		}
		start := in.Start
		var groups []string
		if rule.visual {
			if rule.Trigger != in.Typed {
				continue
			}
		} else {
			pos, matched := matchSuffix(rule, line)
			if pos < 0 || pos == len(line) {
				continue
			}
			start, groups = lineStart+pos, matched
		}
		length := len(before) + len(in.Typed) - start
		if best >= 0 && (rule.Priority < bestPriority || rule.Priority == bestPriority && length <= best) {
			continue
		}
		prefix := before[:min(start, len(before))]
		var retained string
		if start > len(before) {
			retained = in.Typed[:start-len(before)]
			prefix += retained
		}
		if !rule.Scope.allows(markdownMode(prefix)) || rule.WholeWord && !atWordEdges(prefix, in.Markdown[in.End:]) {
			continue
		}
		result = Expansion{
			Start: min(start, in.Start), End: in.End,
			Template: escapeTemplate.Replace(retained) + renderReplacement(rule.parts, selection, groups),
			TabStops: rule.stops,
		}
		best, bestPriority = length, rule.Priority
	}
	return result, best >= 0
}

func matchSuffix(rule compiledRule, line string) (int, []string) {
	if rule.pattern == nil {
		if strings.HasSuffix(line, rule.Trigger) {
			return len(line) - len(rule.Trigger), nil
		}
		return -1, nil
	}
	pairs := rule.pattern.FindStringSubmatchIndex(line)
	if pairs == nil {
		return -1, nil
	}
	groups := make([]string, len(pairs)/2-1)
	for i := range groups {
		if start, end := pairs[2*i+2], pairs[2*i+3]; start >= 0 {
			groups[i] = line[start:end]
		}
	}
	return pairs[0], groups
}

func atWordEdges(before, after string) bool {
	prev, _ := utf8.DecodeLastRuneInString(before)
	next, _ := utf8.DecodeRuneInString(after)
	word := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) }
	return !word(prev) && !word(next)
}

func utf8Boundary(text string, offset int) bool {
	return offset == len(text) || utf8.RuneStart(text[offset])
}
