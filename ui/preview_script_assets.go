package ui

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

// The hashed classic scripts retain the upstream bundles' global declarations.
// The API check also handles headings that become named window properties.
func previewAssetLoader(present, readyEvent string, bundles ...[]byte) func() (string, error) {
	return sync.OnceValues(func() (string, error) {
		var script strings.Builder
		fmt.Fprintf(&script, "if (!(%s)) { for (const source of [", present)
		for _, bundle := range bundles {
			literal, err := previewAssetLiteral(bundle)
			if err != nil {
				return "", err
			}
			script.Write(literal)
			script.WriteByte(',')
		}
		script.WriteString(`]) {
  const script = document.createElement("script");
  script.textContent = source;
  document.head.append(script);
  script.remove();
}}`)
		fmt.Fprintf(&script, `
if (!(%s)) throw new Error("Preview renderer did not load");
window.dispatchEvent(new Event(%q));
return true;`, present, readyEvent)
		return script.String(), nil
	})
}

func previewAssetLiteral(bundle []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	source, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(source))
}

func loadPreviewAsset(page *mygo.Page, name string, loader func() (string, error)) {
	script, err := loader()
	if err == nil {
		_, err = page.Eval(script)
	}
	if err != nil {
		log.Printf("Load Markdown Preview %s: %v", name, err)
	}
}
