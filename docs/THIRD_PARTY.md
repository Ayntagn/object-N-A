# 第三方运行时依赖

| 名称 | 版本 | 用途 | 许可证 | 来源 URL | 是否直接复用原应用内资源 | 替代方案 |
|---|---|---|---|---|---|---|
| MyGo | v0.8.3 | 跨平台窗口、原生控件、Vault 文件监听、Markdown 预览、Callout、图片和 Query Block 的 RichText 编辑、系统 emoji 字形和位图解码 | MIT | [MyGo v0.8.3](https://github.com/egoist/mygo/tree/v0.8.3) | 否 | 各平台原生工具包 |
| purego | v0.11.1 | 复用 MyGo 已引入的 Objective-C 接口，调用 macOS WebKit 视口 magnification 和 PDF 捕获、CoreText 字形、PDFKit 缩略图和附件查看器、系统菜单图标、AppKit 主窗快捷键和 PDF 标题操作、GLOW 浮窗焦点及 Foundation 文件名排序 | Apache-2.0 | [purego v0.11.1](https://github.com/ebitengine/purego/tree/v0.11.1) | 否 | MyGo、PDF.js、Lucide、Go `x/text/collate` |
| PDF.js 浏览器发行物 | 6.5.438 legacy | Windows/Linux 在隔离 WebView 中离线生成 PDF 缩略图并显示只读 PDF 附件 | Apache-2.0 | [Mozilla PDF.js v6.5.438](https://github.com/mozilla/pdf.js/releases/tag/v6.5.438) · [许可](../ui/webassets/pdfjs/LICENSE) | 否 | 平台 PDF 页面渲染 API |
| pdf-lib 浏览器发行物 | 1.17.1 | 裁切 WebView 的连续矢量 PDF，设置纸张底色、页码、页范围与链接 | MIT，发行物的间接依赖另有许可 | [pdf-lib v1.17.1](https://github.com/Hopding/pdf-lib/releases/tag/v1.17.1) · [许可](../ui/webassets/pdf-lib) | 否 | 平台 PDF 页面合成 API |
| go-text/typesetting segmenter | v0.3.5 | 使用 MyGo 已引入的字素簇分段器裁剪片段标题、计算 GLOW 字符数、匹配快速切换的文件名及检查 Vault Health 与 Unlinked Mentions 的短名称 | BSD-3-Clause 或 Unlicense | [go-text/typesetting v0.3.5](https://github.com/go-text/typesetting/tree/v0.3.5) | 否 | 其他 Unicode 字素簇分段器 |
| Go `x/image/colornames` | v0.46.0 | 使用 MyGo 已引入的 CSS 颜色名称表为行内公式解析颜色后缀 | BSD-3-Clause | [Go colornames](https://pkg.go.dev/golang.org/x/image@v0.46.0/colornames) | 否 | 系统 WebView 解析 CSS 颜色 |
| Go `x/net/html` | v0.59.0 | 使用 MyGo 已引入的 HTML 解析器保留文档结构、安装预览策略并校验独立导出的 Markdown HTML 格式块 | BSD-3-Clause | [Go HTML](https://pkg.go.dev/golang.org/x/net@v0.59.0/html) | 否 | WebView 内的标准 HTML 解析器 |
| Go `x/sys` | v0.48.0 | GLOW 文件和目录移动调用各平台的原子不覆盖重命名 | BSD-3-Clause | [Go x/sys v0.48.0](https://github.com/golang/sys/tree/v0.48.0) | 否 | 分别绑定系统重命名接口 |
| Go `x/text` | v0.43.0 | Unicode 大小写处理、规范化及排序，用于 GLOW 脚本断言、快速切换文件、Vault 查询及 Vault Health | BSD-3-Clause | [Go x/text v0.43.0](https://github.com/golang/text/tree/v0.43.0) | 否 | 其他 Unicode 规范化及排序实现 |
| Go YAML v3 | v3.0.5 | Properties 侧栏的有序 YAML 节点解析和改写，保留字段类型、flow 样式和注释 | MIT 与 Apache-2.0 | [go.yaml.in/yaml/v3](https://pkg.go.dev/go.yaml.in/yaml/v3@v3.0.5) | 否 | 旧的标签提取器只提供索引读取 |
| BurntSushi/toml | v1.6.0 | Properties 侧栏读写 TOML frontmatter | MIT | [BurntSushi/toml v1.6.0](https://github.com/BurntSushi/toml/tree/v1.6.0) | 否 | 其他纯 Go TOML 编解码器 |
| JetBrains Mono | 2.305 | 源文推荐字体及 PDF 导出可选字体 | SIL OFL 1.1 | [JetBrains 字库提交](https://github.com/JetBrains/JetBrainsMono/tree/02bb50b082dad9ef8a0f33ac393839202b760223/fonts/ttf) · [许可](../ui/fonts/JetBrainsMono.OFL.txt) | 否 | 系统等宽 |
| Fira Code | 6.2，TTF 6.002 | 源文推荐字体及 PDF 导出可选字体 | SIL OFL 1.1 | [Fira Code 6.2](https://github.com/tonsky/FiraCode/releases/tag/6.2) · [许可](../ui/fonts/FiraCode.OFL.txt) | 否 | 系统等宽 |
| Cascadia Code | 2404.23 | 源文推荐字体及 PDF 导出可选字体 | SIL OFL 1.1 | [Cascadia Code 2404.23](https://github.com/microsoft/cascadia-code/releases/tag/v2404.23) · [许可](../ui/fonts/CascadiaCode.OFL.txt) | 否 | 系统等宽 |
| Maple Mono | v7.9 | Markdown 编辑、预览和 PDF 使用的默认字体。优先使用系统安装的 NF CN，内置标准版有四种字面。 | SIL OFL 1.1 | [Maple Mono v7.9](https://github.com/subframe7536/maple-font/releases/tag/v7.9) · [许可](../ui/fonts/MapleMono.OFL.txt) | 否 | 系统等宽 |
| Space Grotesk | 2.000 | 源文推荐比例字体、快速切换的搜索框和文件名、PDF 导出可选字体、Blackout WYSIWYG 表格和块菜单 | SIL OFL 1.1 | [Google Fonts 字库](https://github.com/google/fonts/tree/2861cb7b12f90c0a294a12ed666e381e2211872f/ofl/spacegrotesk) · [许可](../ui/fonts/SpaceGrotesk.OFL.txt) | 否 | 系统字体 |
| Nunito Sans | 3.101 | 源文推荐比例字体，含正体及斜体，Apropos WYSIWYG 块菜单 | SIL OFL 1.1 | [Google Fonts 下载](https://fonts.google.com/specimen/Nunito+Sans) · [许可](../ui/fonts/NunitoSans.OFL.txt) | 否 | 系统字体 |
| Noto Sans | 2.015 | 源文推荐比例字体，含正体及斜体 | SIL OFL 1.1 | [Google Fonts 字库](https://github.com/google/fonts/tree/2984c575fdce412ee02b2baaba67672b9a9434d8/ofl/notosans) · [许可](../ui/fonts/NotoSans.OFL.txt) | 否 | 系统字体 |
| Lucide 图标子集 | 0.468.0 | 系统符号不可用时的工具栏图标 | ISC，继承自 Feather 的部分为 MIT | [Lucide 0.468.0](https://github.com/lucide-icons/lucide/releases/tag/0.468.0) · [许可证](https://github.com/lucide-icons/lucide/blob/0.468.0/LICENSE) | 否 | 操作系统提供的 SF Symbols |
| Highlight.js 样式色板 | 11.11.1，Highlightr `e05c9bf` 分发 | 源文主题与围栏代码色彩，独立 HTML 复用色板及 MyGo 分词 | BSD-3-Clause | [上游 CSS](https://github.com/raspu/Highlightr/tree/e05c9bf7c2252df65fda24039233247a8de57117/src/assets/styles) · [许可](../ui/source_syntax_styles.LICENSE) | 否 | MyGo 默认编辑器色板 |
| KaTeX 浏览器发行物 | 0.19.1 | WYSIWYG 的离线 TeX 排版，Markdown 预览、PDF 和独立 HTML 的 TeX 排版与 MathML | MIT，WOFF2 字体为 SIL OFL 1.1 | [KaTeX 0.19.1](https://github.com/KaTeX/KaTeX/releases/tag/v0.19.1) · [字体许可证](https://openfontlicense.org/open-font-license-official-text/) | 否 | 系统 WebView 原生 MathML |
| Mermaid 浏览器发行物 | 12.3.0 | Markdown 预览、WYSIWYG 图表卡片、PDF 和 Share HTML 的离线 SVG、DOCX 的 PNG 快照和 Map Vault 的 Dagre 布局 | MIT，附带组件保留上游许可与版权注释 | [Mermaid 12.3.0](https://github.com/mermaid-js/mermaid/releases/tag/mermaid%4012.3.0) · [许可文件](../ui/webassets/mermaid/LICENSE) | 否 | Markdown 围栏的源码与离线渲染 |
| Vega、Vega-Lite、Vega-Embed 浏览器发行物 | 5.33.1、5.23.0、6.29.0 | Markdown 预览、WYSIWYG、PDF 和独立 HTML 的 SVG 图表及表达式解释器 | BSD-3-Clause，保留发行物版权注释 | [Vega](https://github.com/vega/vega/releases/tag/v5.33.1) · [Vega-Lite](https://github.com/vega/vega-lite/releases/tag/v5.23.0) · [Vega-Embed](https://github.com/vega/vega-embed/tree/v6.29.0) | 否 | 现有 fenced code 预览 |
| d3-dsv | 3.0.1 | Vega 渲染的 CSV、TSV 和 DSV 逐行解析 | ISC | [d3-dsv 3.0.1](https://github.com/d3/d3-dsv/tree/v3.0.1) · [许可](../ui/webassets/vega/d3-dsv-LICENSE) | 否 | Go CSV 解析与自定义浏览器读入 |

运行依赖和校验由 [go.mod](../go.mod) 与 [go.sum](../go.sum) 固定。供嵌入的浏览器组件和字库由 `go run ./cmd/assets` 获取和核验。各资源目录保留对应许可证。

`go run ./cmd/assets` 同时通过 Go 模块工具下载 MyGo v0.8.3 源码，并应用本仓库的兼容源码。改动涵盖软折行、输入法、跨平台文件拖放、富文本粘贴及 Markdown 排版，按上游 MIT 许可分发。 [兼容源码与许可](../cmd/assets/mygo)
