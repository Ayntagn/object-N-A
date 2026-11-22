package ui

// spec: EV-MYGO-314 / EV-UI-052

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

func TestLocalAssetRoutesRequireGrantedPageAndOrigin(t *testing.T) {
	page, unrelated := &mygo.Page{}, &mygo.Page{}
	defer pageLocalAssets.RevokeOwner(page)
	token, ok := pageLocalAssets.AddText(page, "text/csv", "a,b\n1,2\n")
	if !ok {
		t.Fatal("register chart bytes")
	}
	call := func(owner *mygo.Page, origin, method, route, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, localAssetOrigin+"/"+route, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		response := httptest.NewRecorder()
		serveLocalAsset(response, req, owner)
		return response
	}
	if got := call(page, "null", http.MethodGet, token, ""); got.Code != 200 || got.Body.String() != "a,b\n1,2\n" ||
		got.Header().Get("Access-Control-Allow-Origin") != "null" ||
		got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("approved resource = %d, %q", got.Code, got.Body.String())
	}
	for _, got := range []*httptest.ResponseRecorder{
		call(unrelated, "null", http.MethodGet, token, ""),
		call(page, "https://example.com", http.MethodGet, token, ""),
		call(page, "null", http.MethodPost, token, ""),
	} {
		if got.Code != http.StatusNotFound {
			t.Fatalf("unapproved resource = %d", got.Code)
		}
	}

	upload, ok := pageLocalAssets.ReserveUpload(page, 64)
	if !ok {
		t.Fatal("register result upload")
	}
	contents := "%PDF-1.7\n"
	frame := binary.BigEndian.AppendUint32(nil, uint32(len(contents)))
	frame = binary.BigEndian.AppendUint32(frame, 0)
	if got := call(page, "file://", http.MethodPost, upload, string(append(frame, contents...))); got.Code != http.StatusNoContent {
		t.Fatalf("PDF result = %d", got.Code)
	}
	if data, received := pageLocalAssets.TakeUpload(page, upload); !received || string(data) != contents {
		t.Fatalf("PDF result bytes = %q", data)
	}
}

func TestPreviewVegaScopesRepeatedVaultData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "groups.tsv")
	prefix := "甲\t2\n"
	data := "Name\tScore\n" + strings.Repeat(prefix, localAssetThreshold/len(prefix)+1)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	observed, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	page := &mygo.Page{}
	defer pageLocalAssets.RevokeOwner(page)
	const source = `{"data":{"url":"groups.tsv"}}`
	origin := previewResourceOrigin{vault: vault, note: "Note.md"}
	assets := previewChartAssetFrame{page: page}
	render := func(frame *previewChartAssetFrame) (string, *previewChartFiles) {
		t.Helper()
		resources := origin.resources()
		resources.chartAssets = frame
		markup := previewMarkup{resources: resources, relative: origin.note}
		first := markup.preparedVega(source, "vega-lite")
		if first != markup.preparedVega(source, "vega-lite") {
			t.Fatal("identical datasets in one frame received different specifications")
		}
		var view struct {
			Data struct {
				URL    string
				Values string
				Format struct{ Type, Parse string }
			}
		}
		if err := json.Unmarshal([]byte(first), &view); err != nil {
			t.Fatal(err)
		}
		token, scoped := strings.CutPrefix(view.Data.URL, localAssetOrigin+"/")
		if frame != nil {
			if !scoped || view.Data.Values != "" {
				t.Fatal("live preview did not use a scoped reference")
			}
		} else if scoped || view.Data.Values != data {
			t.Fatal("self-contained render lost its embedded data")
		}
		if view.Data.Format.Type != "tsv" || view.Data.Format.Parse != "auto" {
			t.Fatalf("TSV format = %+v", view.Data.Format)
		}
		return token, resources.chartFiles
	}
	token, files := render(&assets)
	assets = previewChartAssetFrame{page: page, previous: assets.publish()}
	if again, _ := render(&assets); again != token {
		t.Fatal("unchanged chart did not reuse its resource")
	}
	render(nil)

	data = strings.Replace(data, prefix, "乙\t3\n", 1)
	if !vault.HasIndexedTextVersions() {
		data += prefix
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, observed.ModTime(), observed.ModTime()); err != nil {
		t.Fatal(err)
	}
	if !files.changed() {
		t.Fatal("updated dataset did not invalidate the chart")
	}
	assets = previewChartAssetFrame{page: page, previous: assets.publish()}
	updated, _ := render(&assets)
	assets.publish()
	if updated == token {
		t.Fatal("changing a dataset retained the old resource")
	}
	if _, active := pageLocalAssets.Read(page, token); active {
		t.Fatal("old dataset survived publication")
	}
	asset, active := pageLocalAssets.Read(page, updated)
	var current strings.Builder
	_, err = asset.WriteTo(&current)
	if !active || err != nil || current.String() != data {
		t.Fatal("new dataset did not contain the current file contents")
	}
}
