package aiworkspace

import "strings"

// ConnectionCommand returns the short manual probe copied by Settings.
// EV-UI-286. The command is written for a POSIX shell.
func ConnectionCommand(config Config) (string, error) {
	connection, err := normalize(config)
	if err != nil {
		return "", err
	}
	endpoint := connection.endpoint(connection.completionRoute())
	const contentType = "  -H 'Content-Type: application/json'"
	const hello = "Hello"
	request := completionRequest{Model: connection.config.Model, Messages: []Message{{Role: "user", Content: hello}}, MaxTokens: 10}
	key := connection.config.APIKey
	var headers []string
	if connection.isAnthropic() {
		request.Messages = append([]Message{{Role: "system", Content: "Respond briefly."}}, request.Messages...)
		if key != "" {
			headers = append(headers, "  -H "+shellQuote("x-api-key: "+key), "  -H "+shellQuote("anthropic-version: "+anthropicAPIVersion))
		}
	} else if key != "" {
		headers = append(headers, "  -H "+shellQuote("Authorization: Bearer "+key))
	}
	payload, err := connection.completionJSON(request)
	if err != nil {
		return "", err
	}
	parts := []string{"curl -X POST", "  " + shellQuote(endpoint), contentType}
	parts = append(parts, headers...)
	parts = append(parts, "  -d "+shellQuote(string(payload)))
	return strings.Join(parts, " \\\n"), nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
