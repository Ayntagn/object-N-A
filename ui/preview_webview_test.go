// spec: FEAT-001 / EV-UI-009, EV-UI-021, EV-UI-031
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewRendersVaultContentWithoutLoadingResources(t *testing.T) {
	src := `# Fixture

A <b>literal</b> tag, a [reference](https://example.invalid/reference), and
an [unsafe destination](javascript:void%280%29).

![alt text](https://example.invalid/image.png)

<iframe src="local-preview.html"></iframe>
`
	body, title := renderPreview(parser.New(parser.GFM), src, "fixture.md")
	if title != "Fixture" {
		t.Errorf("preview title = %q, want the document heading", title)
	}
	for _, want := range []string{
		`<h1 id="fixture">Fixture</h1>`,
		`<b>literal</b>`,
		`<a href="https://example.invalid/reference">reference</a>`,
		`an unsafe destination.`,
		`alt text</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview did not render %q in %s", want, body)
		}
	}
	for _, unwanted := range []string{`<iframe`, `<img`, `href="javascript:`, `image.png`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("preview emitted %q as active markup in %s", unwanted, body)
		}
	}
}

// spec: FEAT-001 / EV-UI-046
func TestPreviewPreservesFormulaBoundaries(t *testing.T) {
	source := "Price $5 and $12.00. Escaped \\$x^2$. Code `$x^2$`.\n\n" +
		"Inline $a < b$.\n\n$$\ny = x^2 + 1\n$$\n\n" +
		"```text\n$$ x^2 $$\n```\n\n$$\nunfinished"
	body, _ := renderPreview(parser.New(webPreviewExtensions), source, "Formula boundaries")
	for _, want := range []string{
		`Price $5 and $12.00. Escaped $x^2$. Code <code>$x^2$</code>.`,
		`<span class="math-inline">a &lt; b</span>`,
		`<div class="math-display">y = x^2 + 1</div>`,
		"<pre><code>$$ x^2 $$\n</code></pre>",
		"<div class=\"raw-markup\">$$\nunfinished</div>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview omitted %q in %s", want, body)
		}
	}
	if strings.Count(body, `class="math-inline"`)+strings.Count(body, `class="math-display"`) != 2 {
		t.Errorf("preview applied math outside the two closed formulas in %s", body)
	}
}

// spec: FEAT-001 / EV-MYGO-287
func TestPreviewRendersDigitLeadingInlineMath(t *testing.T) {
	source := `余额 $5 和 $12.00， $ 有正零和负零之分。错误仅发生在和的绝对值大于 $2^{n-1}$ 时。负数 $-N$ 的反码为 $\bar{N}=(2^{n}-1)-N$。系数 $2$，小数 $123.45$。`
	body, _ := renderPreview(parser.New(webPreviewExtensions), source, "Formula boundaries")
	want := `余额 $5 和 $12.00， $ 有正零和负零之分。错误仅发生在和的绝对值大于 <span class="math-inline">2^{n-1}</span> 时。负数 <span class="math-inline">-N</span> 的反码为 <span class="math-inline">\bar{N}=(2^{n}-1)-N</span>。系数 <span class="math-inline">2</span>，小数 <span class="math-inline">123.45</span>。`
	if !strings.Contains(body, want) {
		t.Fatalf("preview = %s, want %s", body, want)
	}
}

// spec: FEAT-001 / EV-UI-047
func TestPreviewRecognizesMermaidFences(t *testing.T) {
	source := "```MERMAID\nflowchart LR\n  A[开始] --> B[结束]\n```\n\n" +
		"```mermaid[bg:teal,background:none]\nflowchart LR\n  A[<br>] --> B\n```\n\n" +
		"```mermaid-example\nflowchart LR\n  A --> B\n```\n\n" +
		"```mermaid\nunfinished"
	preview, _ := renderPreviewResources(parser.New(webPreviewExtensions), source, "Mermaid", nil, "", true)
	for _, want := range []string{
		"<pre class=\"mermaid\" data-mermaid-background=\"\"><code>flowchart LR\n  A[开始] --&gt; B[结束]\n",
		"<pre class=\"mermaid\" data-mermaid-background=\"none\"><code>flowchart LR\n  A[&lt;br&gt;] --&gt; B\n",
		"<pre><code>flowchart LR\n  A --&gt; B\n",
		"<pre><code>unfinished\n</code></pre>",
	} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview omitted %q in %s", want, preview)
		}
	}
	if strings.Count(preview, mermaidBlockStart) != 2 {
		t.Errorf("preview recognized the wrong Mermaid fences in %s", preview)
	}
	static, _ := renderPreview(parser.New(webPreviewExtensions), source, "Mermaid")
	if strings.Contains(static, mermaidBlockStart) || strings.Count(static, "<pre><code>") != 4 {
		t.Errorf("static export changed the fenced source in %s", static)
	}
}

// spec: FEAT-001 / EV-UI-048
func TestPreviewVegaInlinesVaultData(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Vault")
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"data/quarterly.json": `{"rows":[{"name":"甲","value":2}]}`,
		"data/daily.csv":      "name,value\n乙,3\n",
		"data/lookup.tsv":     "name\tlabel\n甲\tQ1\n",
		"../outside.json":     `[{"name":"outside"}]`,
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	res := &previewResources{vault: vault, remaining: 12 << 20}
	src := `{"data":{"url":"data/quarterly.json","format":{"property":"rows"}},` +
		`"layer":[{"data":{"url":"daily.csv"}},{"data":{"url":"../../outside.json"}}],` +
		`"transform":[{"lookup":"name","from":{"data":{"url":"../data/lookup.tsv"},"key":"name"}}]}`
	preview, _ := renderPreviewResources(parser.New(webPreviewExtensions), "```VEGA-LITE\n"+src+"\n```\n", "Chart", res, "Notes/Chart.md", true)
	for _, want := range []string{
		`class="vega-lite" data-vega-preview`,
		`&#34;values&#34;:[{&#34;name&#34;:&#34;甲&#34;,&#34;value&#34;:2}]`,
		`&#34;parse&#34;:&#34;auto&#34;,&#34;type&#34;:&#34;csv&#34;`,
		`&#34;type&#34;:&#34;tsv&#34;`,
		`&#34;url&#34;:&#34;../../outside.json&#34;`,
	} {
		if !strings.Contains(preview, want) {
			t.Errorf("chart preview omitted %q in %s", want, preview)
		}
	}
	if strings.Contains(preview, `&#34;name&#34;:&#34;outside&#34;`) {
		t.Fatal("chart preview loaded data outside the Vault")
	}
}
