package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/go-text/typesetting/segmenter"
)

type workspaceAIReference struct {
	name       string
	content    string
	characters int
}

type workspaceAIReferenceLoad struct {
	ctx     context.Context
	cancel  context.CancelFunc
	waiting []string
}

// EV-UI-277. The paperclip opens the platform's multiple-file chooser.
func (app *workspaceWindow) pickAIReferences() {
	win, chat := app.win, &app.ai
	if win == nil || win.IsDestroyed() || app.modalOpen || chat.referenceLoad != nil {
		return
	}
	load := chat.beginAIReferenceLoad()
	app.modalOpen = true
	options := mygo.OpenDialogOptions{
		Parent: win, Title: "Open", Message: "Select files to attach as context",
		DefaultPath: app.root, Multiple: true,
	}
	go func() {
		paths, err := mygo.Dialog.Open(options)
		win.Update(func() {
			app.modalOpen = false
			if chat.referenceLoad != load {
				return
			}
			if err != nil {
				app.problem = fmt.Sprintf("Could not open the attachment picker. %v", err)
			}
			if err != nil || len(paths) == 0 {
				chat.completeAIReferenceBatch(load, nil)
				return
			}
			app.readAIReferenceBatch(load, paths)
		})
	}()
}

func (s *workspaceAI) beginAIReferenceLoad() *workspaceAIReferenceLoad {
	ctx, cancel := context.WithCancel(context.Background())
	load := &workspaceAIReferenceLoad{ctx: ctx, cancel: cancel}
	s.referenceLoad = load
	return load
}

func (app *workspaceWindow) readAIReferenceBatch(load *workspaceAIReferenceLoad, paths []string) {
	chat, win := &app.ai, app.win
	attached := make(map[string]bool, len(chat.references))
	for _, reference := range chat.references {
		attached[reference.name] = true
	}
	go func() {
		references := loadAIReferenceFiles(load.ctx, paths, attached)
		win.Update(func() {
			if next := chat.completeAIReferenceBatch(load, references); len(next) > 0 {
				app.readAIReferenceBatch(load, next)
			}
		})
	}()
}

func (s *workspaceAI) completeAIReferenceBatch(load *workspaceAIReferenceLoad, references []workspaceAIReference) []string {
	if s.referenceLoad != load {
		return nil
	}
	s.addAIReferences(references)
	next := load.waiting
	load.waiting = nil
	if len(next) == 0 {
		load.cancel()
		s.referenceLoad = nil
	}
	return next
}

func loadAIReferenceFiles(ctx context.Context, paths []string, attached map[string]bool) []workspaceAIReference {
	var references []workspaceAIReference
	for _, filename := range paths {
		if ctx.Err() != nil {
			return nil
		}
		if attached[filepath.Base(filename)] {
			continue
		}
		if reference, ok := loadAIReference(ctx, filename); ok {
			references = append(references, reference)
			attached[reference.name] = true
		}
	}
	return references
}

func loadAIReference(ctx context.Context, filename string) (workspaceAIReference, bool) {
	name := filepath.Base(filename)
	if strings.EqualFold(filepath.Ext(name), ".pdf") {
		content, err := aiReferencePDFText(ctx, filename)
		return workspaceAIReference{name: name, content: content}, err == nil && strings.TrimSpace(content) != ""
	}
	listed, err := os.Stat(filename)
	if err != nil || !listed.Mode().IsRegular() {
		return workspaceAIReference{}, false
	}
	file, err := os.Open(filename)
	if err != nil {
		return workspaceAIReference{}, false
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(listed, opened) {
		return workspaceAIReference{}, false
	}
	data, err := io.ReadAll(file)
	if err != nil || !utf8.Valid(data) {
		return workspaceAIReference{}, false
	}
	return workspaceAIReference{name: name, content: strings.TrimPrefix(string(data), "\uFEFF")}, true
}

func (s *workspaceAI) addAIReferences(references []workspaceAIReference) {
	for _, reference := range references {
		if !slices.ContainsFunc(s.references, func(prior workspaceAIReference) bool { return prior.name == reference.name }) {
			var segments segmenter.Segmenter
			segments.InitWithString(reference.content)
			for graphemes := segments.GraphemeIterator(); graphemes.Next(); {
				reference.characters++
			}
			s.references = append(s.references, reference)
		}
	}
}

func (s *workspaceAI) clearAIReferences() {
	s.references = nil
	if load := s.referenceLoad; load != nil {
		load.cancel()
		s.referenceLoad = nil
	}
}

func (s *workspaceAI) drawAIReferences(c *mygoui.Context, colors workspaceAIColors) {
	if len(s.references) == 0 {
		return
	}
	mygoui.Row(c.Key("AI file attachments")).Width(vaultPanelWidth-32).Shrink(0).
		Margin(0, 16).Gap(4).Wrap().Label("Attached files").Children(func() {
		for _, reference := range s.references {
			mygoui.Row(c.Key(reference.name)).MaxWidth(212).Height(18).Shrink(0).Gap(4).Padding(0, 5).
				AlignItems(mygoui.Center).Radius(5).Background(colors.fill).Children(func() {
				aiWorkspaceIcon(c, "doc.text", "", 11, colors.accent)
				mygoui.Text(c, reference.name).FontSize(11).TextColor(colors.text).SingleLine().
					Ellipsis("…").MinWidth(0).Tooltip(reference.name)
				count := strconv.Itoa(reference.characters)
				if reference.characters >= 1000 {
					count = fmt.Sprintf("%.1fk", float64(reference.characters)/1000)
				}
				mygoui.Text(c, count).Font("monospace").FontSize(10).TextColor(colors.muted).Shrink(0)
				mygoui.ButtonBase(c.Key("remove attached file")).Size(14, 14).Shrink(0).Center().KeepFocus().
					Label("Remove " + reference.name).Tooltip("Remove attachment").OnClick(func() {
					s.references = slices.DeleteFunc(s.references, func(item workspaceAIReference) bool {
						return item.name == reference.name
					})
				}).Children(func() {
					aiWorkspaceIcon(c, "xmark", "", 7, colors.muted)
				})
			})
		}
	})
}
