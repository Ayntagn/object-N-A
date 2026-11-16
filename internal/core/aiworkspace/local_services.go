package aiworkspace

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"time"
)

// LocalService is a compatible provider with a fixed local model route.
type LocalService struct {
	Name, Address, BaseURL, Icon string
	ollama                       bool
}

type LocalServiceResult struct {
	Service LocalService
	Online  bool
	Models  []string
}

var localServices = [...]LocalService{
	{Name: "Ollama", Address: "localhost:11434", BaseURL: "http://localhost:11434/v1", Icon: "hare", ollama: true},
	{Name: "LM Studio", Address: "localhost:1234", BaseURL: "http://localhost:1234/v1", Icon: "desktopcomputer"},
	{Name: "MLX (mlx-lm)", Address: "localhost:8080", BaseURL: "http://localhost:8080/v1", Icon: "apple.logo"},
	{Name: "vLLM", Address: "localhost:8000", BaseURL: "http://localhost:8000/v1", Icon: "bolt"},
}

func LocalServices() []LocalService { return slices.Clone(localServices[:]) }

// ScanLocalServices checks the four fixed loopback providers concurrently and
// reports each result as it arrives. EV-UI-280: each GET gets three seconds.
func ScanLocalServices(ctx context.Context, report func(LocalServiceResult)) {
	scanLocalServices(ctx, localServices[:], report)
}

func scanLocalServices(ctx context.Context, services []LocalService, report func(LocalServiceResult)) {
	completed := make(chan LocalServiceResult, len(services))
	for _, service := range services {
		go func() { completed <- service.probe(ctx) }()
	}
	for range services {
		report(<-completed)
	}
}

func (service LocalService) probe(ctx context.Context) LocalServiceResult {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result := LocalServiceResult{Service: service}
	base, route := service.BaseURL, "models"
	if service.ollama {
		base, route = "http://"+service.Address, "api/tags"
	}
	connection, err := normalize(Config{BaseURL: base})
	if err != nil {
		return result
	}
	response, err := connection.openResponse(ctx, http.MethodGet, route, "", nil)
	if err != nil {
		return result
	}
	defer response.Body.Close()
	data, err := readResponse(ctx, response.Body)
	if err != nil {
		return result
	}
	result.Online = true
	if service.ollama {
		var list struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.Unmarshal(data, &list) == nil {
			for _, model := range list.Models {
				if model.Name != "" {
					result.Models = append(result.Models, model.Name)
				}
			}
		}
	} else {
		result.Models, _ = decodeCompatibleModels(data)
	}
	return result
}
