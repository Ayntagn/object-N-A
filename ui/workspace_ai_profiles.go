package ui

import (
	"crypto/rand"
	"net/url"
	"slices"

	"cyberwriter/internal/core/aiworkspace"
)

type workspaceAIProfile struct {
	ID           string                     `json:"id"`
	Connection   workspaceAISavedConnection `json:"connection"`
	SystemPrompt *string                    `json:"systemPrompt,omitempty"`
}

func (p *workspacePreferences) migrateAIProfiles() {
	if p.AIProfiles != nil {
		return
	}
	p.AIProfiles = []workspaceAIProfile{}
	if p.AIConnection == (workspaceAISavedConnection{}) {
		return
	}
	const migrated = "legacy-connection"
	p.AIProfiles = append(p.AIProfiles, workspaceAIProfile{ID: migrated, Connection: p.AIConnection, SystemPrompt: p.AISystemPrompt})
	p.AIActiveProfile = migrated
}

func (profile workspaceAIProfile) label() string {
	parsed, _ := url.Parse(profile.Connection.BaseURL)
	if parsed != nil && parsed.Host != "" {
		return profile.Connection.Model + "@" + parsed.Host
	}
	return profile.Connection.Model
}

func (p *workspacePreferences) profileIndex(id string) int {
	if id == "" {
		return -1
	}
	return slices.IndexFunc(p.AIProfiles, func(profile workspaceAIProfile) bool { return profile.ID == id })
}

// EV-UI-283. Save reuses an identical profile. A changed connection or system
// prompt creates another entry, preserving the previous entry and its key.
func (app *workspaceWindow) saveAIConnection() {
	form := &app.aiConnectionForm
	config, err := aiworkspace.Normalize(form.Draft)
	if err != nil {
		form.Status, form.StatusError = err.Error(), true
		return
	}
	pending := *app.preferences
	pending.migrateAIProfiles()
	index := slices.IndexFunc(pending.AIProfiles, func(profile workspaceAIProfile) bool {
		prompt := workspaceAIWritingInstructions
		if profile.SystemPrompt != nil {
			prompt = *profile.SystemPrompt
		}
		if profile.Connection.BaseURL != config.BaseURL || profile.Connection.Model != config.Model || prompt != pending.aiSystemPrompt() {
			return false
		}
		if profile.Connection.Credential == "" {
			return config.APIKey == ""
		}
		prior, err := profile.Connection.config()
		return err == nil && prior.APIKey == config.APIKey
	})
	var profile workspaceAIProfile
	var newCredential string
	if index >= 0 {
		profile = pending.AIProfiles[index]
	} else {
		connection := workspaceAISavedConnection{BaseURL: config.BaseURL, Model: config.Model}
		if config.APIKey != "" {
			newCredential = rand.Text()
			if err := aiCredentials.Set(newCredential, []byte(config.APIKey)); err != nil {
				form.Status, form.StatusError = "Could not save the API key in the system credential store.", true
				return
			}
			connection.Credential = newCredential
		}
		profile = workspaceAIProfile{ID: rand.Text(), Connection: connection, SystemPrompt: pending.AISystemPrompt}
		pending.AIProfiles = append(slices.Clone(pending.AIProfiles), profile)
	}
	pending.AIActiveProfile, pending.AIConnection = profile.ID, profile.Connection
	if !app.commitAIConnectionSettings(pending) {
		if newCredential != "" {
			_ = aiCredentials.Delete(newCredential)
		}
		return
	}
	form.Draft, form.ProfileID = config, profile.ID
	form.Status, form.StatusError = "Settings saved", false
}

func (app *workspaceWindow) selectAIProfile(id string) {
	pending := *app.preferences
	index := pending.profileIndex(id)
	if index < 0 {
		app.newAIConnection()
		return
	}
	profile := pending.AIProfiles[index]
	config, err := profile.Connection.config()
	if err != nil {
		app.aiConnectionForm.Status, app.aiConnectionForm.StatusError = err.Error(), true
		return
	}
	pending.AIActiveProfile, pending.AIConnection = id, profile.Connection
	pending.AISystemPrompt = profile.SystemPrompt
	if app.commitAIConnectionSettings(pending) {
		app.cancelAIConnectionRequests()
		app.aiConnectionForm = workspaceAIConnectionForm{Draft: config, ProfileID: id}
	}
}

// EV-UI-283. Removing a saved profile clears its selection. The live
// connection remains available, including its key and system prompt.
func (app *workspaceWindow) deleteAIProfile(id string) {
	pending := *app.preferences
	index := pending.profileIndex(id)
	if index < 0 {
		return
	}
	credential := pending.AIProfiles[index].Connection.Credential
	pending.AIProfiles = slices.Delete(slices.Clone(pending.AIProfiles), index, index+1)
	if pending.AIActiveProfile == id {
		pending.AIActiveProfile = ""
	}
	if app.commitAIConnectionSettings(pending) {
		pending.releaseUnusedAICredential(credential)
		app.newAIConnection()
	}
}

func (app *workspaceWindow) commitAIConnectionSettings(pending workspacePreferences) bool {
	if err := pending.write(); err != nil {
		app.aiConnectionForm.Status, app.aiConnectionForm.StatusError = "Could not save the AI connection.", true
		return false
	}
	oldCredential := app.preferences.AIConnection.Credential
	*app.preferences = pending
	pending.releaseUnusedAICredential(oldCredential)
	if app.application != nil {
		for _, window := range app.application.windows {
			if window.win != nil && !window.win.IsDestroyed() {
				window.win.Invalidate()
			}
			window.invalidateAISettings()
		}
	}
	return true
}

func (p *workspacePreferences) releaseUnusedAICredential(id string) {
	if id == "" || id == p.AIConnection.Credential {
		return
	}
	if !slices.ContainsFunc(p.AIProfiles, func(profile workspaceAIProfile) bool { return profile.Connection.Credential == id }) {
		_ = aiCredentials.Delete(id)
	}
}
