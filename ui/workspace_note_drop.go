package ui

import (
	"bytes"
	"fmt"
	"html"
	"image/png"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui/markdown/parser"
	xhtml "golang.org/x/net/html"
)

var noteDropOptions = transfer.DropOptions{
	Formats:    []transfer.Format{transfer.FileList, transfer.URIList, transfer.PNG, transfer.HTML, transfer.Text},
	Operations: transfer.Copy | transfer.Link,
}

type noteDropItem struct {
	file           string
	png            []byte
	address, label string
	remoteImage    bool
}

type downloadedNoteImage struct {
	data []byte
	err  error
}

// classifyNoteDrop keeps the order of mixed drops and retains PNG data as
// an alternative to a single file URL in the same native item.
func classifyNoteDrop(data transfer.Data) []noteDropItem {
	var items []noteDropItem
	seenFiles, seenLinks, seenImages := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, item := range data.Items() {
		value := transfer.New(item)
		if files, _ := value.Files(); len(files) > 0 {
			var alternative []byte
			if len(files) == 1 {
				alternative, _ = item.Read(transfer.PNG)
			}
			for _, file := range files {
				if !seenFiles[file] {
					seenFiles[file] = true
					items = append(items, noteDropItem{file: file, png: alternative})
				}
			}
			continue
		}
		// A PNG representation can be alongside the remote URL. An unreadable
		// header lets the explicit image address take over.
		raster, rasterErr := item.Read(transfer.PNG)
		if rasterErr == nil {
			if _, err := png.DecodeConfig(bytes.NewReader(raster)); err == nil {
				items = append(items, noteDropItem{png: raster})
				continue
			}
		}
		browserLink, browserImage := droppedBrowserContent(value)
		if browserImage.address != "" {
			if !seenImages[browserImage.address] {
				seenImages[browserImage.address] = true
				items = append(items, noteDropItem{address: browserImage.address, label: browserImage.label, remoteImage: true})
			}
			continue
		}
		var candidates []noteDropItem
		var hasRemoteImage bool
		for _, link := range droppedNoteLinks(value, browserLink) {
			remote := isDirectWebImage(link.address)
			hasRemoteImage = hasRemoteImage || remote
			candidates = append(candidates, noteDropItem{address: link.address, label: link.label, remoteImage: remote})
		}
		if rasterErr == nil && !hasRemoteImage {
			// Preserve the import error for malformed standalone PNG payloads.
			if raster == nil {
				raster = []byte{}
			}
			items = append(items, noteDropItem{png: raster})
			continue
		}
		for _, candidate := range candidates {
			seen := seenLinks
			if candidate.remoteImage {
				seen = seenImages
			}
			if !seen[candidate.address] {
				seen[candidate.address] = true
				items = append(items, candidate)
			}
		}
	}
	return items
}

func (app *workspaceWindow) noteDropItemsMarkdown(tab *workspaceTab, items []noteDropItem, downloaded map[string]downloadedNoteImage) (markdown string, hasImage bool) {
	vault := app.noteDropVault(tab)
	note := tab.document.Path
	var parts []string
	var problem string
	report := func(message string) {
		if problem == "" {
			problem = message
		}
	}
	addImage := func(label, caption string, load func() (string, error)) {
		if vault == nil || note == "" {
			report("Save the note before importing an image.")
			return
		}
		destination, err := load()
		if err != nil {
			report(fmt.Sprintf("Could not import the image. %v", err))
			return
		}
		parts = append(parts, importedImageMarkdown(note, destination, label, caption))
		hasImage = true
	}

	for _, item := range items {
		switch {
		case item.file != "":
			name := filepath.Base(item.file)
			rasterFile := workspace.IsRasterImagePath(name)
			if !rasterFile && len(item.png) == 0 {
				report(fmt.Sprintf("%q is not a supported image file.", name))
				continue
			}
			addImage(name, "", func() (string, error) {
				if !rasterFile {
					return vault.ImportPNG(note, item.png)
				}
				destination, err := vault.ImportImage(note, item.file)
				if err != nil && len(item.png) != 0 {
					if fallback, fallbackErr := vault.ImportPNG(note, item.png); fallbackErr == nil {
						return fallback, nil
					}
				}
				return destination, err
			})
		case item.png != nil:
			addImage("image.png", "", func() (string, error) { return vault.ImportPNG(note, item.png) })
		case item.remoteImage:
			addImage(item.label, item.label, func() (string, error) {
				result := downloaded[item.address]
				if result.err != nil {
					return "", result.err
				}
				if len(result.data) == 0 {
					return "", fmt.Errorf("no image data was downloaded")
				}
				return vault.ImportDownloadedImage(note, item.address, result.data)
			})
		case item.address != "":
			parts = append(parts, droppedMarkdownLink(item.label, item.address))
		}
	}
	if len(parts) > 0 || problem != "" {
		app.problem = problem
	}
	if hasImage {
		return strings.Join(parts, "\n\n"), true
	}
	return strings.Join(parts, " "), false
}

// Use an exact Vault path for imported images. A root file referenced from a
// nested note needs an explicit relative path to avoid a local same-name file.
func importedImageMarkdown(note, destination, label, caption string) string {
	relative, ok := workspace.ResolveImagePath(note, destination)
	rawDestination, _ := url.PathUnescape(destination)
	if path.Dir(relative) == "." && path.Dir(note) != "." {
		relative = rawDestination
	}
	if strings.TrimSpace(label) == "" {
		label = path.Base(rawDestination)
	}
	caption = strings.TrimSpace(caption)
	width, _ := strconv.Atoi(caption)
	if ok && strings.TrimSpace(relative) == relative && !strings.ContainsAny(relative, "\\|#^[]:\r\n") &&
		!strings.Contains(relative, "%%") && !strings.ContainsAny(caption, "\\|[]\r\n") && width <= 0 {
		_, embed := attachmentCopyLinks(relative)
		if caption != "" {
			embed = strings.TrimSuffix(embed, "]]") + "|" + caption + "]]"
		}
		doc := parser.New(webPreviewExtensions)
		doc.Set(embed)
		inlines := doc.Inlines(1)
		if len(inlines) == 1 && inlines[0].Kind == parser.Text && inlines[0].Start == 0 && inlines[0].End == len(embed) {
			return embed
		}
	}
	return "!" + droppedMarkdownLink(label, destination)
}

type droppedNoteLink struct {
	address, label string
}

func droppedNoteLinks(value transfer.Data, browser droppedNoteLink) []droppedNoteLink {
	urls, _ := value.URLs()
	rawText, _ := value.Read(transfer.Text)
	text := strings.TrimSpace(string(rawText))
	textURL := previewExternalURL(text)
	var links []droppedNoteLink
	for _, candidate := range urls {
		address := previewExternalURL(candidate)
		if address == "" {
			continue
		}
		var label string
		if browser.address == address {
			label = browser.label
		} else if len(urls) == 1 && textURL == "" {
			label = text
		}
		links = append(links, droppedNoteLink{address, label})
	}
	if len(links) > 0 {
		return links
	}
	if browser.address != "" {
		return []droppedNoteLink{browser}
	}
	if textURL != "" {
		return []droppedNoteLink{{address: textURL}}
	}
	return nil
}

func droppedBrowserContent(value transfer.Data) (link, image droppedNoteLink) {
	fragment, err := value.Read(transfer.HTML)
	if err != nil {
		return link, image
	}
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(fragment))
	var label strings.Builder
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			link.label = label.String()
			return link, image
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "a":
				if link.address == "" {
					link.address = previewExternalURL(strings.TrimSpace(droppedHTMLAttr(token, "href")))
				}
			case "img":
				if image.address == "" {
					address := previewExternalURL(strings.TrimSpace(droppedHTMLAttr(token, "src")))
					if droppedHTTPURL(address) != nil {
						image = droppedNoteLink{address: address, label: droppedHTMLAttr(token, "alt")}
					}
				}
			}
		case xhtml.TextToken:
			if link.address != "" {
				label.Write(tokenizer.Text())
			}
		case xhtml.EndTagToken:
			end, _ := tokenizer.TagName()
			if link.address != "" && string(end) == "a" {
				link.label = label.String()
				return link, image
			}
		}
	}
}

func droppedHTTPURL(address string) *url.URL {
	target, err := url.Parse(address)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") {
		return nil
	}
	return target
}

func isDirectWebImage(address string) bool {
	target := droppedHTTPURL(address)
	return target != nil && workspace.IsRasterImagePath(target.Path)
}

func droppedHTMLAttr(token xhtml.Token, key string) string {
	for _, attr := range token.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

var droppedLinkLabelEscapes = strings.NewReplacer(
	`\`, `\\`, "[", `\[`, "]", `\]`, "*", `\*`, "_", `\_`, "`", "\\`",
	"<", `\<`, ">", `\>`, "&", `\&`, "~", `\~`, "$", `\$`, "=", `\=`, "|", `\|`, "!", `\!`,
)

func droppedMarkdownLink(label, destination string) string {
	label = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, label)), " ")
	if label == "" {
		label = destination
	}
	if strings.ContainsAny(destination, " ()<>") {
		destination = "<" + html.EscapeString(destination) + ">"
	} else {
		destination = strings.ReplaceAll(destination, "&", "&amp;")
	}
	return "[" + droppedLinkLabelEscapes.Replace(label) + "](" + destination + ")"
}
