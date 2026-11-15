package ui

import _ "embed"

const mermaidBlockStart = `<pre class="mermaid" data-mermaid-background="`
const mermaidScriptHash = "sha256-M234A9h6OLcKAec25mbI92AdLOQvXkfI+v5dchB0uaM="

//go:embed webassets/mermaid/mermaid.min.js.gz
var mermaidBundle []byte

//go:embed preview_mermaid.js
var previewMermaidJS string

//go:embed preview_mermaid.css
var previewMermaidCSS string

var mermaidLoader = previewAssetLoader(`typeof window.mermaid?.render === "function"`, "cyberwriter:mermaid-ready", mermaidBundle)
