package vaultindex

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
)

type queryScope uint8

const (
	queryAll queryScope = iota
	queryFolder
	queryTag
)

type querySpec struct {
	columns    []string
	scope      queryScope
	source     string
	conditions []queryCondition
	sortField  string
	descending bool
	limit      int
}

type queryCondition struct {
	field string
	op    string
	value workspace.Property
}

type queryToken struct {
	text   string
	quoted bool
}

func (token queryToken) keyword() string {
	if token.quoted {
		return ""
	}
	return strings.ToLower(token.text)
}

func parseQuery(source string) (querySpec, error) {
	tokens := tokenizeQuery(source)
	if len(tokens) == 0 {
		return querySpec{}, ErrInvalidQuery
	}
	query := querySpec{limit: -1}
	// The output selector is recognized at the start of the source.
	switch tokens[0].keyword() {
	case "list":
		tokens = tokens[1:]
	case "table":
		query.columns = []string{}
		tokens = tokens[1:]
		for len(tokens) != 0 && !isQueryClause(tokens[0].keyword()) {
			if tokens[0].keyword() != "," {
				query.columns = append(query.columns, tokens[0].text)
			}
			tokens = tokens[1:]
		}
	}
	for len(tokens) != 0 {
		clause := tokens[0].keyword()
		tokens = tokens[1:]
		switch clause {
		case "from", "sort", "limit":
			if len(tokens) == 0 {
				return querySpec{}, ErrInvalidQuery
			}
			value := tokens[0]
			tokens = tokens[1:]
			switch clause {
			case "from":
				query.scope, query.source = queryFolder, value.text
				if tag, ok := strings.CutPrefix(value.text, "#"); !value.quoted && ok {
					query.scope, query.source = queryTag, strings.ToLower(tag)
				}
			case "sort":
				query.sortField = strings.ToLower(value.text)
				if len(tokens) != 0 {
					switch direction := tokens[0].keyword(); direction {
					case "asc", "desc":
						query.descending = direction == "desc"
						tokens = tokens[1:]
					}
				}
			case "limit":
				limit, err := strconv.Atoi(value.text)
				if value.quoted || err != nil {
					return querySpec{}, ErrInvalidQuery
				}
				query.limit = limit
			}
		case "where":
			// Conditions consume triples up to the native SORT/LIMIT boundary.
			for len(tokens) != 0 {
				keyword := tokens[0].keyword()
				if keyword == "sort" || keyword == "limit" {
					break
				}
				if keyword == "and" {
					tokens = tokens[1:]
					continue
				}
				if len(tokens) < 3 {
					return querySpec{}, ErrInvalidQuery
				}
				op := tokens[1].keyword()
				switch op {
				case "=", "==", "!=", ">", "<", ">=", "<=", "contains":
				default:
					return querySpec{}, ErrInvalidQuery
				}
				query.conditions = append(query.conditions, queryCondition{
					field: strings.ToLower(tokens[0].text), op: op, value: tokens[2].property(),
				})
				tokens = tokens[3:]
			}
		}
	}
	return query, nil
}

func isQueryClause(keyword string) bool {
	switch keyword {
	case "table", "list", "from", "where", "sort", "limit":
		return true
	}
	return false
}

func (token queryToken) property() workspace.Property {
	value := workspace.Property{Kind: workspace.PropertyText, Text: token.text}
	if token.quoted {
		return value
	}
	switch strings.ToLower(token.text) {
	case "true", "false":
		value.Kind = workspace.PropertyCheckbox
		value.Text = strings.ToLower(token.text)
	default:
		if _, err := strconv.ParseFloat(token.text, 64); err == nil && !strings.ContainsRune(token.text, '_') {
			value.Kind = workspace.PropertyNumber
		}
	}
	return value
}

func tokenizeQuery(source string) []queryToken {
	var tokens []queryToken
	for source != "" {
		char, width := utf8.DecodeRuneInString(source)
		switch {
		case unicode.IsSpace(char):
			source = source[width:]
		case char == '"' || char == '\'':
			source = source[width:]
			text, rest, closed := strings.Cut(source, string(char))
			if closed || text != "" {
				tokens = append(tokens, queryToken{text: text, quoted: closed})
			}
			source = rest
		case char == ',':
			tokens = append(tokens, queryToken{text: ","})
			source = source[width:]
		case queryOperatorRune(char):
			end := strings.IndexFunc(source, func(r rune) bool { return !queryOperatorRune(r) })
			if end < 0 {
				end = len(source)
			}
			tokens = append(tokens, queryToken{text: source[:end]})
			source = source[end:]
		default:
			end := strings.IndexFunc(source, func(r rune) bool {
				return unicode.IsSpace(r) || r == ',' || r == '"' || r == '\'' || queryOperatorRune(r)
			})
			if end < 0 {
				end = len(source)
			}
			tokens = append(tokens, queryToken{text: source[:end]})
			source = source[end:]
		}
	}
	return tokens
}

func queryOperatorRune(char rune) bool {
	return char == '=' || char == '<' || char == '>' || char == '!'
}
