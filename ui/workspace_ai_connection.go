package ui

import (
	"context"
	"errors"
	"fmt"

	"cyberwriter/internal/core/aiworkspace"
	"github.com/egoist/mygo"
)

const defaultAIBaseURL = "http://localhost:1234/v1"

var aiCredentials = mygo.Secrets.Service("dev.object-na.ai")

type workspaceAISavedConnection struct {
	BaseURL    string `json:"baseURL"`
	Model      string `json:"model"`
	Credential string `json:"credential,omitempty"`
}

type aiSettingsRequest struct {
	cancel context.CancelFunc
}

func (saved workspaceAISavedConnection) config() (aiworkspace.Config, error) {
	cfg := aiworkspace.Config{BaseURL: saved.BaseURL, Model: saved.Model}
	if saved.Credential == "" {
		return cfg, nil
	}
	key, err := aiCredentials.Get(saved.Credential)
	if err != nil {
		return cfg, errors.New("Could not read the API key from the system credential store.")
	}
	cfg.APIKey = string(key)
	clear(key)
	return cfg, nil
}

func (app *workspaceWindow) prepareAIConnectionSettings() {
	app.scanAILocalServices()
	if app.aiSettingsPrepared {
		return
	}
	app.aiSettingsPrepared = true
	cfg, err := app.preferences.AIConnection.config()
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultAIBaseURL
	}
	app.aiConnectionForm.Draft = cfg
	app.aiConnectionForm.ProfileID = app.preferences.AIActiveProfile
	if err != nil {
		app.aiConnectionForm.Status = err.Error()
		app.aiConnectionForm.StatusError = true
	}
}

func (app *workspaceWindow) closeAIConnectionSettings() {
	if request := app.aiServiceRequest; request != nil {
		request.cancel()
	}
	app.aiServiceRequest, app.aiServiceResults = nil, nil
	app.cancelAIConnectionRequests()
	app.aiSettingsPrepared = false
	app.aiConnectionForm = workspaceAIConnectionForm{}
}

func (app *workspaceWindow) cancelAIConnectionRequests() {
	for _, request := range []*aiSettingsRequest{app.aiModelRequest, app.aiTestRequest} {
		if request != nil {
			request.cancel()
		}
	}
	app.aiModelRequest, app.aiTestRequest = nil, nil
}

// EV-UI-280. Opening the AI page checks the local providers, and Rescan
// repeats the check with the same request lifetime as the Settings window.
func (app *workspaceWindow) scanAILocalServices() {
	if app.aiServiceRequest != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := &aiSettingsRequest{cancel: cancel}
	app.aiServiceRequest, app.aiServiceResults = request, nil
	go func() {
		defer cancel()
		aiworkspace.ScanLocalServices(ctx, func(result aiworkspace.LocalServiceResult) {
			mygo.RunOnMain(func() {
				if app.aiServiceRequest == request {
					app.aiServiceResults = append(app.aiServiceResults, result)
					app.invalidateAISettings()
				}
			})
		})
		mygo.RunOnMain(func() {
			if app.aiServiceRequest != request {
				return
			}
			app.aiServiceRequest = nil
			app.invalidateAISettings()
		})
	}()
}

func (app *workspaceWindow) useAILocalService(result aiworkspace.LocalServiceResult) {
	if !result.Online {
		return
	}
	config := aiworkspace.Config{BaseURL: result.Service.BaseURL}
	if len(result.Models) > 0 {
		config.Model = result.Models[0]
	}
	pending := *app.preferences
	pending.migrateAIProfiles()
	pending.AIConnection = workspaceAISavedConnection{BaseURL: config.BaseURL, Model: config.Model}
	pending.AIActiveProfile = ""
	if app.commitAIConnectionSettings(pending) {
		app.cancelAIConnectionRequests()
		app.aiConnectionForm = workspaceAIConnectionForm{Draft: config}
	}
}

// EV-UI-284. Choosing a cloud provider opens a new connection draft.
func (app *workspaceWindow) useAICloudService(baseURL, displayName string) {
	app.newAIConnection()
	app.aiConnectionForm.Draft.BaseURL = baseURL
	app.aiConnectionForm.Status = fmt.Sprintf(app.tr("Add your %s API key and model below, then Save.", "在下方填写 %s 的 API 密钥和模型，然后保存。"), displayName)
}

func (app *workspaceWindow) newAIConnection() {
	app.cancelAIConnectionRequests()
	app.aiConnectionForm = workspaceAIConnectionForm{}
}

func (app *workspaceWindow) refreshAIModels() {
	if app.aiModelRequest != nil {
		return
	}
	form := &app.aiConnectionForm
	draft := form.Draft
	ctx, cancel := context.WithCancel(context.Background())
	request := &aiSettingsRequest{cancel: cancel}
	app.aiModelRequest = request
	form.Refreshing = true
	go func() {
		defer cancel()
		// EV-UI-288. Refresh changes the model choices independently from the Test result.
		models, _ := aiworkspace.Models(ctx, draft)
		mygo.RunOnMain(func() {
			if app.aiModelRequest != request {
				return
			}
			app.aiModelRequest = nil
			form.Refreshing = false
			if form.Draft.BaseURL == draft.BaseURL && form.Draft.APIKey == draft.APIKey {
				form.Models = models
			}
			app.invalidateAISettings()
		})
	}()
}

func (app *workspaceWindow) testAIConnection() {
	if app.aiTestRequest != nil {
		return
	}
	form := &app.aiConnectionForm
	draft := form.Draft
	ctx, cancel := context.WithCancel(context.Background())
	request := &aiSettingsRequest{cancel: cancel}
	app.aiTestRequest = request
	form.Testing = true
	go func() {
		defer cancel()
		model, err := aiworkspace.Test(ctx, draft)
		mygo.RunOnMain(func() {
			if app.aiTestRequest != request {
				return
			}
			app.aiTestRequest = nil
			form.Testing = false
			if form.Draft == draft {
				form.Status, form.StatusError = "Connected", false
				if model != "" {
					form.Status += " - " + model
				}
				if err != nil {
					form.Status, form.StatusError = err.Error(), true
				}
			}
			app.invalidateAISettings()
		})
	}()
}

func (app *workspaceWindow) invalidateAISettings() {
	if app.settings != nil && !app.settings.IsDestroyed() {
		app.settings.Invalidate()
	}
}
