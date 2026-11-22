package ui

import (
	"errors"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"object-na.local/localassets"
)

const localAssetScheme = "cyberwriter-local-assets"
const localAssetOrigin = localAssetScheme + "://localhost"
const localAssetThreshold = 256 << 10
const localAssetUploadChunkBytes = 512 << 10

var pageLocalAssets = localassets.New[*mygo.Page](localassets.Limits{
	MaxBytes: 256 << 20, MaxOwnerBytes: 96 << 20, MaxRoutes: 512, MaxOwnerRoutes: 128,
	UploadChunkBytes: localAssetUploadChunkBytes,
})

func registerPageAssetProtocol() error {
	// The binary protocol is enabled with WKWebView, where loading the
	// preview shell and the temporary PDF document has been verified.
	if runtime.GOOS != "darwin" {
		return nil
	}
	return mygo.Protocol.HandleFunc(localAssetScheme, func(w http.ResponseWriter, r *http.Request) {
		serveLocalAsset(w, r, mygo.CallerPage(r.Context()))
	})
}

func pageAssetAvailable(page *mygo.Page) bool {
	return page != nil && runtime.GOOS == "darwin" && mygo.Protocol.IsHandled(localAssetScheme)
}

func localAssetConnectPolicy(policy string) string {
	if runtime.GOOS != "darwin" {
		return policy
	}
	return strings.Replace(policy, "connect-src https: http: data:", "connect-src https: http: data: "+localAssetOrigin, 1)
}

func serveLocalAsset(w http.ResponseWriter, r *http.Request, owner *mygo.Page) {
	origin := r.Header.Get("Origin")
	token, ok := strings.CutPrefix(r.URL.Path, "/")
	if owner == nil || r.URL.Scheme != localAssetScheme || r.URL.Host != "localhost" || r.URL.User != nil ||
		!ok || token == "" || strings.Contains(token, "/") || r.URL.RawPath != "" ||
		r.URL.RawQuery != "" || r.URL.ForceQuery || r.URL.Fragment != "" ||
		(origin != "null" && origin != "file://") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	allow := func() {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}
	switch r.Method {
	case http.MethodGet:
		asset, ok := pageLocalAssets.Read(owner, token)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		allow()
		w.Header().Set("Content-Type", asset.ContentType())
		w.Header().Set("Content-Length", strconv.Itoa(asset.Len()))
		_, _ = asset.WriteTo(w)
	case http.MethodPost:
		if err := pageLocalAssets.ReceiveUpload(owner, token, r.Body); err != nil {
			if errors.Is(err, localassets.ErrGrant) {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}
		allow()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
