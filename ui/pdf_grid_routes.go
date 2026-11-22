package ui

import (
	"context"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"object-na.local/pdfthumb"
)

const pdfGridImageScheme = "cyberwriter-pdf-thumbnail"
const pdfGridImageOrigin = pdfGridImageScheme + "://localhost"

var pdfGridRoutes = struct {
	sync.RWMutex
	pages map[*mygo.Page]*pdfGridSource
}{pages: make(map[*mygo.Page]*pdfGridSource)}

var registerPDFGridProtocol = sync.OnceValue(func() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return mygo.Protocol.HandleFunc(pdfGridImageScheme, func(w http.ResponseWriter, r *http.Request) {
		servePDFGrid(w, r, mygo.CallerPage(r.Context()))
	})
})

func publishPDFGrid(page *mygo.Page, source *pdfGridSource) {
	pdfGridRoutes.Lock()
	defer pdfGridRoutes.Unlock()
	pdfGridRoutes.pages[page] = source
}

func unpublishPDFGrid(page *mygo.Page) {
	pdfGridRoutes.Lock()
	defer pdfGridRoutes.Unlock()
	delete(pdfGridRoutes.pages, page)
}

func servePDFGrid(w http.ResponseWriter, r *http.Request, caller *mygo.Page) {
	if caller == nil || r.Method != http.MethodGet || r.URL.Scheme != pdfGridImageScheme || r.URL.Host != "localhost" || r.URL.RawQuery != "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	pdfGridRoutes.RLock()
	source := pdfGridRoutes.pages[caller]
	pdfGridRoutes.RUnlock()
	if source == nil || source.ctx.Err() != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	index, found := strings.CutPrefix(r.URL.Path, source.route)
	page, err := strconv.Atoi(index)
	if !found || err != nil || page < 1 || page > source.count || strconv.Itoa(page) != index {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// A thumbnail retry must get a fresh response after a transient render failure.
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(source.ctx, cancel)
	defer stop()
	key := pdfthumb.Key{Document: source.digest, Page: page, Side: source.side}
	jpeg, err := source.cache.Thumbnail(ctx, key, func(ctx context.Context) ([]byte, error) {
		return source.render(ctx, page)
	})
	if err != nil || ctx.Err() != nil || source.ctx.Err() != nil || len(jpeg) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(jpeg)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(jpeg)
}
