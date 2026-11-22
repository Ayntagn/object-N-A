package ui

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

const previewImageScheme = "cyberwriter-preview-image"

var previewImageOrigin = func() string {
	if runtime.GOOS == "windows" {
		// WebView2 serves MyGo protocols via its intercepted localhost origin.
		return "http://" + previewImageScheme + ".localhost"
	}
	return previewImageScheme + "://localhost"
}()

type previewLocalImage struct {
	vault         *workspace.Vault
	relative      string
	route         string
	stamp         workspace.IndexedNoteStamp
	digest        [sha256.Size]byte
	bytes         int64
	width, height int
}

// previewImageFrame records only authorized file versions and their geometry.
// The preview never holds the encoded attachment bytes between renders.
type previewImageFrame struct {
	previous map[string]previewLocalImage
	files    map[string]previewLocalImage
	routes   map[string]previewLocalImage
}

func (frame *previewImageFrame) add(vault *workspace.Vault, relative string, limit int64) (previewLocalImage, bool) {
	previous := frame.previous[relative]
	stamp := vault.PreviewImageStamp(relative)
	if previous.vault == vault && stamp != (workspace.IndexedNoteStamp{}) && previous.stamp == stamp {
		if previous.bytes > limit {
			return previewLocalImage{}, false
		}
		frame.remember(previous)
		return previous, true
	}

	data, _, err := vault.ReadPreviewImage(relative, limit)
	if err != nil {
		return previewLocalImage{}, false
	}
	if stamp != (workspace.IndexedNoteStamp{}) && vault.PreviewImageStamp(relative) != stamp {
		stamp = workspace.IndexedNoteStamp{}
	}
	digest := sha256.Sum256(data)
	if previous.vault == vault && previous.digest == digest {
		previous.stamp = stamp
		frame.remember(previous)
		return previous, true
	}
	width, height, _ := previewRasterDimensions(func() io.Reader { return bytes.NewReader(data) })
	asset := previewLocalImage{
		vault: vault, relative: relative, route: "/" + rand.Text(), stamp: stamp, digest: digest,
		bytes: int64(len(data)), width: width, height: height,
	}
	frame.remember(asset)
	return asset, true
}

func (frame *previewImageFrame) remember(asset previewLocalImage) {
	if frame.files == nil {
		frame.files = make(map[string]previewLocalImage)
		frame.routes = make(map[string]previewLocalImage)
	}
	frame.files[asset.relative] = asset
	frame.routes[asset.route] = asset
}

type previewImageRegistry struct {
	mu    sync.RWMutex
	pages map[*mygo.Page]map[string]previewLocalImage
}

var livePreviewImages = previewImageRegistry{pages: make(map[*mygo.Page]map[string]previewLocalImage)}

func registerPreviewImageProtocol() error {
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		return mygo.Protocol.HandleFunc(previewImageScheme, func(w http.ResponseWriter, r *http.Request) {
			livePreviewImages.serve(w, r, mygo.CallerPage(r.Context()))
		})
	default:
		return nil
	}
}

func (registry *previewImageRegistry) publish(page *mygo.Page, routes map[string]previewLocalImage) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if len(routes) == 0 {
		delete(registry.pages, page)
	} else {
		registry.pages[page] = routes
	}
}

func (registry *previewImageRegistry) serve(w http.ResponseWriter, r *http.Request, page *mygo.Page) {
	if page == nil || r.Method != http.MethodGet || r.URL.Host != "localhost" || r.URL.RawQuery != "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	registry.mu.RLock()
	asset, ok := registry.pages[page][r.URL.Path]
	registry.mu.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data, mime, err := asset.vault.ReadPreviewImage(asset.relative, asset.bytes)
	if err != nil || sha256.Sum256(data) != asset.digest {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func (p *workspacePreview) clearResources() {
	if p.localCharts != nil {
		if p.view != nil {
			pageLocalAssets.RevokeOwner(p.view.Page())
		}
		p.localCharts = nil
	}
	if p.localImages == nil {
		return
	}
	p.localImages = nil
	if p.view != nil {
		livePreviewImages.publish(p.view.Page(), nil)
	}
}

func (m *previewMarkup) imageDimensions(src string) (width, height int, ok bool) {
	if route, owned := strings.CutPrefix(src, previewImageOrigin); owned && m.resources != nil && m.resources.fileImages != nil {
		asset := m.resources.fileImages.routes[route]
		return asset.width, asset.height, asset.width > 0 && asset.height > 0
	}
	return previewImageDimensions(src)
}
