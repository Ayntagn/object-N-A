package ui

import (
	"context"
	"crypto/rand"
	"errors"

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
	if app.aiSettingsPrepared {
		return
	}
	app.aiSettingsPrepared = true
	cfg, err := app.preferences.AIConnection.config()
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultAIBaseURL
	}
	app.aiConnectionForm.Draft = cfg
	if err != nil {
		app.aiConnectionForm.Status = err.Error()
		app.aiConnectionForm.StatusError = true
	}
}

func (app *workspaceWindow) closeAIConnectionSettings() {
	for _, request := range []*aiSettingsRequest{app.aiModelRequest, app.aiTestRequest} {
		if request != nil {
			request.cancel()
		}
	}
	app.aiModelRequest, app.aiTestRequest = nil, nil
	app.aiSettingsPrepared = false
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
		models, err := aiworkspace.Models(ctx, draft)
		mygo.RunOnMain(func() {
			if app.aiModelRequest != request {
				return
			}
			app.aiModelRequest = nil
			form.Refreshing = false
			if form.Draft.BaseURL == draft.BaseURL && form.Draft.APIKey == draft.APIKey {
				form.Models = models
				if err != nil {
					form.Status, form.StatusError = err.Error(), true
				} else {
					form.Status, form.StatusError = "", false
				}
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
	form.Status, form.StatusError = "", false
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

func (app *workspaceWindow) saveAIConnection() {
	form := &app.aiConnectionForm
	cfg, err := aiworkspace.Normalize(form.Draft)
	if err != nil {
		form.Status, form.StatusError = err.Error(), true
		return
	}
	saved := workspaceAISavedConnection{BaseURL: cfg.BaseURL, Model: cfg.Model}
	if cfg.APIKey != "" {
		saved.Credential = rand.Text()
		if err := aiCredentials.Set(saved.Credential, []byte(cfg.APIKey)); err != nil {
			form.Status, form.StatusError = "Could not save the API key in the system credential store.", true
			return
		}
	}
	old := app.preferences.AIConnection
	pending := *app.preferences
	pending.AIConnection = saved
	if err := pending.write(); err != nil {
		if saved.Credential != "" {
			_ = aiCredentials.Delete(saved.Credential)
		}
		form.Status, form.StatusError = "Could not save the AI connection.", true
		return
	}
	app.preferences.AIConnection = saved
	if old.Credential != "" {
		_ = aiCredentials.Delete(old.Credential)
	}
	form.Draft, form.Status, form.StatusError = cfg, "Settings saved", false
	if app.application != nil {
		for _, window := range app.application.windows {
			window.win.Invalidate()
		}
	}
}

func (app *workspaceWindow) invalidateAISettings() {
	if app.settings != nil && !app.settings.IsDestroyed() {
		app.settings.Invalidate()
	}
}
