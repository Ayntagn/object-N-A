package aiworkspace

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Models returns the sorted IDs offered by the configured compatible endpoint.
// EV-UI-230 records the relative GET path and the menu ordering.
func Models(ctx context.Context, config Config) ([]string, error) {
	connection, err := normalize(config)
	if err != nil {
		return nil, err
	}
	models, err := connection.models(ctx)
	if len(models) > 0 || ctx.Err() != nil {
		return models, err
	}
	// EV-UI-288. An empty first result triggers a single /api fallback.
	fallback := *connection.base
	fallback.Path = strings.TrimRight(strings.ReplaceAll(fallback.Path, "/v1", ""), "/") + "/api"
	fallback.RawPath = ""
	connection.base = &fallback
	models, fallbackErr := connection.models(ctx)
	if len(models) == 0 && fallbackErr == nil {
		return models, err
	}
	return models, fallbackErr
}

func (c serviceConnection) models(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	response, err := c.openResponse(ctx, http.MethodGet, "models", "", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := readResponse(ctx, response.Body)
	if err != nil {
		return nil, err
	}
	models, err := decodeCompatibleModels(data)
	slices.Sort(models)
	return models, err
}

func decodeCompatibleModels(data []byte) ([]string, error) {
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &list) != nil || list.Data == nil {
		return nil, errInvalidResponse
	}
	models := make([]string, 0, len(list.Data))
	for _, entry := range list.Data {
		if entry.ID != "" {
			models = append(models, entry.ID)
		}
	}
	return models, nil
}
