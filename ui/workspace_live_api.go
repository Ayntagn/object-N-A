package ui

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"cyberwriter/internal/core/liveapi"
	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const apiRequestLabel = "API Request"
const apiWeatherInfo = "api[title: Weather]"
const apiWeatherSource = "GET https://api.open-meteo.com/v1/forecast?latitude=40.71&longitude=-74.01&current=temperature_2m,weather_code,wind_speed_10m"
const apiWeatherTemplate = "```" + apiWeatherInfo + "\n" + apiWeatherSource + "\n```"

// EV-UI-163: the Source caret is placed before the newline in the opener.
func (app *workspaceWindow) insertAPIRequest() {
	if app.sourceEditing() {
		app.insertSourceSnippetAt(apiWeatherTemplate, len("```"+apiWeatherInfo))
		return
	}
	if app.displayedMode() == wysiwygMode && app.wys.editable(app) && app.wys.session.InsertAPIRequest(apiWeatherInfo, apiWeatherSource) {
		app.wys.sync(app)
		app.wys.focus = true
	}
}

type apiRequestTarget struct {
	snapshot     editor.Snapshot
	start, end   int
	resultStart  int
	resultEnd    int
	info, source string
}

func (app *workspaceWindow) confirmAPIRun(title, message, action string, destructive bool, run func()) {
	app.modalOpen = true
	options := mygo.MessageOptions{Parent: app.win, Title: title, Message: message, Type: mygo.MessageWarning,
		Buttons: []string{"Cancel", action}, DefaultButton: 0, CancelButton: 0}
	if destructive {
		options.DestructiveButtons = []int{1}
	}
	go func() {
		choice, err := mygo.Dialog.Message(options)
		app.win.Update(func() {
			app.modalOpen = false
			if app.win.IsDestroyed() {
				return
			}
			if err != nil {
				app.problem = err.Error()
			} else if choice.Button == 1 {
				run()
			}
		})
	}()
}

func apiRequestAt(snapshot editor.Snapshot, offset int) (apiRequestTarget, bool) {
	return findAPIRequest(snapshot, offset, -1)
}

// A target is associated with a specific fence occurrence in the saved
// draft, not just with request text that may occur more than once.
func findAPIRequest(snapshot editor.Snapshot, offset, ordinal int) (apiRequestTarget, bool) {
	prefix, _, body := workspace.SplitFrontmatter(snapshot.String())
	doc := parser.New(wysiwyg.Extensions)
	doc.Set(body)
	for i := range doc.Children(0) {
		block := doc.Block(i)
		if block.Kind != parser.CodeBlock || !block.Fenced || block.Open {
			continue
		}
		if _, ok := liveapi.ParseOptions(block.Info()); !ok {
			continue
		}
		start := len(prefix) + block.Start
		if ordinal != 0 && start != offset {
			if ordinal > 0 {
				ordinal--
			}
			continue
		}
		target := apiRequestTarget{snapshot: snapshot, start: start, end: len(prefix) + block.End,
			info: block.Info(), source: strings.TrimSuffix(doc.Lines(i), "\n")}
		if block.Next < doc.Len() {
			n := doc.Block(block.Next)
			if n.Kind == parser.CodeBlock && n.Parent == 0 && n.Fenced && !n.Open &&
				strings.EqualFold(n.Language(), "api-result") && strings.TrimSpace(body[block.End:n.Start]) == "" {
				target.resultStart = len(prefix) + strings.LastIndexByte(body[:n.Start], '\n') + 1
				target.resultEnd = len(prefix) + n.End
			}
		}
		return target, true
	}
	return apiRequestTarget{}, false
}

func (t apiRequestTarget) sameFence(snapshot editor.Snapshot) (apiRequestTarget, bool) {
	if snapshot.Version() == t.snapshot.Version() {
		return t, true
	}
	before, after := t.snapshot.String(), snapshot.String()
	fence := before[t.start:t.end]
	if strings.Count(before, fence) != strings.Count(after, fence) {
		return apiRequestTarget{}, false
	}
	end := max(t.end, t.resultEnd)
	offset := t.start
	if !strings.HasPrefix(after, before[:end]) {
		// A change entirely above this region can move it. Require the
		// full suffix, so identical request text elsewhere cannot match.
		if !strings.HasSuffix(after, before[t.start:]) {
			return apiRequestTarget{}, false
		}
		offset += len(after) - len(before)
	}
	next, ok := apiRequestAt(snapshot, offset)
	if !ok || next.info != t.info || next.source != t.source || (next.resultEnd == 0) != (t.resultEnd == 0) {
		return apiRequestTarget{}, false
	}
	if t.resultEnd != 0 && next.snapshot.Slice(next.resultStart, next.resultEnd) != t.snapshot.Slice(t.resultStart, t.resultEnd) {
		return apiRequestTarget{}, false
	}
	return next, true
}

func (t apiRequestTarget) resultEdit(snapshot editor.Snapshot, markdown string) (editor.Edit, bool) {
	current, ok := t.sameFence(snapshot)
	if !ok {
		return editor.Edit{}, false
	}
	if current.resultEnd > 0 {
		return editor.Edit{Start: current.resultStart, End: current.resultEnd, Text: markdown}, true
	}
	tail := snapshot.Slice(current.end, snapshot.Len())
	separator := "\n\n"
	var after string
	switch {
	case strings.TrimSpace(tail) == "", strings.HasPrefix(tail, "\n\n"):
	case strings.HasPrefix(tail, "\n"):
		after = "\n"
	default:
		after = "\n\n"
	}
	return editor.Edit{Start: current.end, End: current.end, Text: separator + markdown + after}, true
}

func (app *workspaceWindow) runPreviewAPIRequest(offset int, version uint64) {
	if app.modalOpen || app.draft == nil || app.draft.Version() != version {
		return
	}
	if target, ok := apiRequestAt(app.draft.Snapshot(), offset); ok {
		app.runAPIRequest(target)
	}
}

func (app *workspaceWindow) runVisualAPIRequest(offset int, block *wysiwyg.APIBlock) {
	if !app.wys.editable(app) {
		return
	}
	app.wys.flush(app)
	ordinal := app.wys.session.APIRequestOrdinal(offset, block)
	if ordinal < 0 {
		return
	}
	if target, ok := findAPIRequest(app.draft.Snapshot(), -1, ordinal); ok && target.source == block.Source && target.info == block.Info {
		app.runAPIRequest(target)
	}
}

func (app *workspaceWindow) visualAPIRequestRunning(offset int, block *wysiwyg.APIBlock) bool {
	if len(app.apiRequests) == 0 {
		return false
	}
	ordinal := app.wys.session.APIRequestOrdinal(offset, block)
	if ordinal < 0 {
		return false
	}
	target, ok := findAPIRequest(app.draft.Snapshot(), -1, ordinal)
	return ok && app.apiIsRunning(target)
}

func (tab *workspaceTab) apiIsRunning(target apiRequestTarget) bool {
	for pending := range tab.apiRequests {
		if current, ok := pending.sameFence(target.snapshot); ok && current.start == target.start {
			return true
		}
	}
	return false
}

func (tab *workspaceTab) cancelAPIRequests() {
	for _, cancel := range tab.apiRequests {
		cancel()
	}
	tab.apiRequests = nil
}

func (app *workspaceWindow) runAPIRequest(target apiRequestTarget) {
	tab, draft := app.workspaceTab, app.draft
	if app.modalOpen || app.win == nil || app.win.IsDestroyed() || tab.apiIsRunning(target) {
		return
	}
	request, err := liveapi.ParseRequest(target.info, target.source)
	if err == nil {
		err = request.Validate()
	}
	if err != nil {
		app.finishAPIRequest(tab, draft, &target, request, liveapi.Response{}, err)
		return
	}
	endpoint, _ := url.Parse(request.URL)
	host := strings.ToLower(endpoint.Hostname())
	send := func() {
		if app.win.IsDestroyed() || !slices.Contains(app.tabs, tab) || tab.draft != draft {
			return
		}
		if tab == app.workspaceTab {
			app.wys.flush(app)
		}
		current, ok := target.sameFence(draft.Snapshot())
		if !ok || tab.apiIsRunning(current) {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		if tab.apiRequests == nil {
			tab.apiRequests = make(map[*apiRequestTarget]context.CancelFunc)
		}
		tab.apiRequests[&target] = cancel
		app.problem = "Running API request…"
		go func() {
			defer cancel()
			response, err := liveapi.Run(ctx, request)
			app.win.Update(func() {
				if _, active := tab.apiRequests[&target]; active {
					delete(tab.apiRequests, &target)
					app.finishAPIRequest(tab, draft, &target, request, response, err)
				}
			})
		}()
	}
	approveWrite := func() {
		switch request.Method {
		case "POST", "PUT", "PATCH", "DELETE":
			app.confirmAPIRun("Send a "+request.Method+" request?",
				request.Method+" can create, change, or delete data on the server. This isn't undoable from cyberWriter.\n\n"+request.URL,
				"Send "+request.Method, true, send)
		default:
			send()
		}
	}
	if slices.Contains(app.preferences.APIAllowedHosts, host) {
		approveWrite()
		return
	}
	app.confirmAPIRun("Allow requests to "+host+"?",
		fmt.Sprintf("This document wants to send a %s request to:\n%s\n\ncyberWriter will remember your choice for this host. You can remove it later in Settings → API.", request.Method, request.URL),
		"Allow", false, func() {
			if !slices.Contains(app.preferences.APIAllowedHosts, host) {
				app.preferences.APIAllowedHosts = append(app.preferences.APIAllowedHosts, host)
				slices.Sort(app.preferences.APIAllowedHosts)
				app.writePreferences()
			}
			approveWrite()
		})
}

func (app *workspaceWindow) finishAPIRequest(tab *workspaceTab, draft *editor.Buffer, target *apiRequestTarget,
	request liveapi.Request, response liveapi.Response, requestErr error) {
	finished := time.Now()
	result := liveapi.MakeResult(response, requestErr, finished)
	if request.Options.Format == liveapi.FormatRaw {
		result.Format = liveapi.FormatRaw
	}
	requestText, responseText, address := apiHistoryDisplay(request.URL, target.source, result.Body)
	app.recordAPIRun(apiHistoryItem{Method: request.Method, URL: address, Request: requestText,
		Response: responseText, StatusCode: result.StatusCode, Duration: response.Duration, Bytes: response.Size, Timestamp: finished})
	owner := app
	for _, window := range app.documentWindows() {
		if slices.Contains(window.tabs, tab) && !window.win.IsDestroyed() {
			owner = window
			break
		}
	}
	if owner.win == nil || owner.win.IsDestroyed() || !slices.Contains(owner.tabs, tab) || tab.draft != draft {
		return
	}
	// Sync unflushed visual edits before comparing source offsets.
	if tab == owner.workspaceTab {
		owner.wys.flush(owner)
	}
	snapshot := draft.Snapshot()
	current, ok := target.sameFence(snapshot)
	if !ok {
		owner.problem = "The request block changed. Its response is available in API History."
		return
	}
	// EV-UI-164/165: the observed dispatcher writes the result to the
	// source fence, including a transport error with out: preview.
	if edit, ok := current.resultEdit(snapshot, result.Markdown()); ok {
		draft.Edit(edit)
	}
	owner.problem = result.Metadata
	if tab == owner.workspaceTab {
		owner.syncDocumentEdited()
	}
	owner.win.Invalidate()
}

func apiHistoryDisplay(address, source, response string) (string, string, string) {
	var values []string
	sensitive := func(key string) bool {
		switch strings.ToLower(strings.ReplaceAll(key, "_", "-")) {
		case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "api-key", "token",
			"access-token", "secret", "client-secret", "password":
			return true
		}
		return false
	}
	first, headers, _ := strings.Cut(strings.TrimSpace(source), "\n")
	if address == "" {
		if fields := strings.Fields(first); len(fields) >= 2 {
			address = fields[1]
		}
	}
	for line := range strings.SplitSeq(headers, "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		name, value, _ := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		if !sensitive(strings.TrimSpace(name)) || value == "" {
			continue
		}
		values = append(values, value)
		if strings.Contains(strings.ToLower(name), "authorization") {
			if _, token, ok := strings.Cut(value, " "); ok && token != "" {
				values = append(values, token)
			}
		}
	}
	if endpoint, err := url.Parse(address); err == nil {
		params := endpoint.Query()
		changed := endpoint.User != nil
		if endpoint.User != nil {
			if username := endpoint.User.Username(); username != "" {
				values = append(values, username, url.PathEscape(username))
			}
			if password, ok := endpoint.User.Password(); ok && password != "" {
				values = append(values, password, url.PathEscape(password))
			}
			endpoint.User = nil
		}
		for name, entries := range params {
			if !sensitive(name) {
				continue
			}
			for _, value := range entries {
				if value != "" {
					values = append(values, value, url.QueryEscape(value), url.PathEscape(value))
				}
			}
			params.Set(name, "••••")
			changed = true
		}
		if changed {
			endpoint.RawQuery = params.Encode()
			address = endpoint.String()
		}
	}
	for _, value := range values {
		source = strings.ReplaceAll(source, value, "••••")
		response = strings.ReplaceAll(response, value, "••••")
		address = strings.ReplaceAll(address, value, "••••")
	}
	return source, response, address
}

func (app *workspaceWindow) apiSettings(c *mygoui.Context) {
	mygoui.Text(c, "API Response Formatting").FontSize(17).Bold().Role(mygoui.RoleHeading)
	mygoui.Text(c, "Built-in tables (no AI)").FontSize(13)
	mygoui.Text(c, "Allowed Hosts").FontSize(17).Bold().Role(mygoui.RoleHeading).Margin(16, 0, 0)
	if len(app.preferences.APIAllowedHosts) == 0 {
		mygoui.Text(c, "No hosts allowed yet.").FontSize(13).TextColor(c.Theme().TextMuted)
	}
	for _, host := range app.preferences.APIAllowedHosts {
		mygoui.Row(c.Key(host)).FillWidth().Gap(8).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, host).Grow(1).MinWidth(0).Wrap()
			mygoui.Button(c, "Remove").Label("Remove " + host).OnClick(func() {
				app.preferences.APIAllowedHosts = slices.DeleteFunc(app.preferences.APIAllowedHosts, func(value string) bool { return value == host })
				app.writePreferences()
			})
		})
	}
}
