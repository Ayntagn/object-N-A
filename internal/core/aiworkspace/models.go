package aiworkspace

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
)

// Models returns the sorted IDs offered by the configured compatible endpoint.
// EV-UI-230 records the relative GET path and the menu ordering.
func Models(ctx context.Context, config Config) ([]string, error) {
	response, err := openResponse(ctx, config, http.MethodGet, "models", "", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := readResponse(ctx, response.Body)
	if err != nil {
		return nil, err
	}
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
	slices.Sort(models)
	return models, nil
}
