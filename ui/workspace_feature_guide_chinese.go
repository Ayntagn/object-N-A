package ui

var featureGuideChineseCopy = map[string]featureGuideCopy{
	"headings": {
		title: "标题", summary: "用标题组织笔记章节，生成侧栏大纲和目录。",
		hints: []string{"⌘ ⌃ 1 / 2 / 3", "在 Insert 菜单选择 Headings", "# 章节标题"},
	},
	"callouts": {
		title: "提示块", summary: "用独立的提示块突出说明、技巧或警告。",
		hints: []string{"源文工具栏 → Alert", "> [!NOTE]"},
	},
	"tables": {
		title: "表格", summary: "用行和列整理相关数据。第一行是表头。",
		hints:  []string{"在 Insert 菜单选择 Table", "在所见即所得模式中使用表格网格", "用竖线分隔列"},
		points: []string{"在分隔行的列末尾加冒号可右对齐该列。", "使用表格编辑器更改单元格内容。"},
	},
	"tasks": {
		title: "任务列表", summary: "用 Markdown 复选框跟踪任务。",
		hints: []string{"Insert → Lists → Task List", "- [ ] 待办任务"},
	},
	"lists": {
		title: "列表", summary: "用项目符号归类相关条目，或用数字标记步骤的先后顺序。",
		hints: []string{"在源文工具栏插入无序列表", "在源文工具栏插入有序列表", "增加缩进可创建子项"},
	},
	"code": {
		title: "语法高亮", summary: "在代码围栏的起始标记后注明语言，让代码示例更易读。",
		hints: []string{"Insert → Code Block", "View → Disable Syntax Highlighting"},
	},
	"mermaid": {
		title: "Mermaid 图表", summary: "在 Markdown 围栏中描述图表。预览和所见即所得卡片可以离线渲染。",
		hints: []string{"Insert → Diagrams → Flowchart", "⌘ / → Flowchart", "在所见即所得卡片中选择 Edit source"},
	},
	"vega": {
		title: "图表（Vega-Lite）", summary: "用视觉编码呈现数据。示例柱状图的数据直接写在笔记里。",
		hints:  []string{"用 vega-lite 围栏开始", "用 data.values 写入内嵌数据", "用 data.url 引用本地知识库文件", "打开 Markdown 预览"},
		points: []string{"CSV、TSV 和 JSON 数据源可使用相对此笔记或知识库的路径。"},
	},
	"math": {
		title: "LaTeX 公式", summary: "简短表达式可以写在正文行内，长公式可以独立成行。",
		hints: []string{"行内用 $表达式$", "独立公式的两端用单独一行 $$", "View → Preview Only"},
	},
	"footnotes": {
		title: "脚注", summary: "给正文添加参考说明，保持句子连贯。",
		hints: []string{"Insert → Footnote", "引用与脚注定义使用同一个标签"},
	},
	"highlight": {
		title: "强调与行内代码", summary: "突出一段文字，或用等宽字体显示简短的命令。",
		hints: []string{"⌘ B", "Insert → Inline Code", "先选中要格式化的文字"},
	},
	"figures": {
		title: "插图与图注", summary: "给插图加上说明，并使用便于迁移的相对图片路径。",
		hints: []string{"Insert → Image", "用替代文字描述图片"},
	},
	"wikilinks": {
		title: "双链", summary: "按名称连接笔记，使用别名作为链接文字，也可以直接链接到标题。",
		hints: []string{"[[笔记标题]]", "在知识库文件菜单选择 Copy Wikilink"},
		points: []string{
			"笔记重名时可用 [[文件夹/笔记]]。添加竖线和别名可更改预览中的链接文字，仍打开同一份笔记。",
			"在 # 后填入目标笔记中的标题，就能直接打开该段。",
			"在方括号前添加 ! 可嵌入图片。路径相对于当前笔记或知识库。",
			"点击 Markdown 预览里的链接可以打开目标笔记，原笔记仍可切回。",
			"有重名笔记时，右键知识库里的文件并选择 Copy Wikilink 可取得准确路径。",
			"本指南用示例笔记展示链接渲染后的文字。",
		},
	},
	"tags": {
		title: "标签", summary: "在正文或文档头（frontmatter）添加标签，方便找到相关笔记。",
		hints: []string{"#project/survey", "在 Properties 中查看 Tags"},
	},
	"css": {
		title: "行内 CSS 样式", summary: "使用 HTML 的 style 属性设置颜色或对齐方式。示例只为一句文字设置样式。",
		hints: []string{"用 span 设置短语的样式", "用 div 设置整块内容的样式", "用 HTML 预览检查结果"},
	},
	"pagebreak": {
		title: "分页符", summary: "在 PDF 和 Word 导出中，从独立的分页符开始新的一页。",
		hints:  []string{"Insert → Page Break", "GLOW → /page-break"},
		points: []string{"让分页标记独占一行。大纲和目录仍按文档标题生成。"},
	},
	"mermaid-bg": {
		title: "Mermaid 卡片背景", summary: "直接调整 Mermaid 卡片的背景色，保留图表内容。",
		hints:  []string{"在 Mermaid 卡片中选择 Background", "mermaid[bg:#24354a]", "mermaid[bg:none]"},
		points: []string{"选择 Theme 恢复主题背景。选择 None 让卡片融入页面背景。"},
	},
	"api": {
		title: "实时 API 数据", summary: "在笔记中发起 HTTP 请求并显示响应。",
		hints:  []string{"Add → Live Data → API Request", "所见即所得 → /api", "⌘ / → API History"},
		points: []string{"点 Run 发送请求。响应显示在代码块下方，并随笔记一起保存。", "内置格式化可把 JSON 记录显示为表格。可以在 API History 查看或复制过去的请求。"},
	},
	"query": {
		title: "查询块", summary: "列出符合条件的笔记，或用表格显示它们的属性。",
		hints:  []string{"⌘ / → Query Block", "Add → Live Data → Query Block", "所见即所得 → /query"},
		points: []string{"按文件夹、标签或属性过滤。用 SORT 排序，用 LIMIT 限制结果数量。", "点击结果可打开对应笔记。在所见即所得卡片中点 Edit source 可编辑查询。"},
	},
	"fold": {
		title: "折叠文本", summary: "在源文模式中折叠选中的行，收起较长的段落。",
		hints:  []string{"Edit → Fold Selection", "⌘ .", "⌘ / → Fold Selection"},
		points: []string{"选中多行进行折叠。点击折叠标记可展开这些行。"},
	},
	"embeds": {
		title: "嵌入媒体", summary: "在双链前加 ! 即可嵌入知识库图片或笔记。原版也支持媒体和 CSV 等表格。",
		hints:  []string{"![[知识库内的路径]]", "Insert → Image", "使用相对文件路径"},
		points: []string{"在 Markdown 预览中查看知识库图片和嵌入的笔记内容。"},
	},
	"export": {
		title: "导出 PDF、DOCX 与 HTML", summary: "导出当前草稿，并选择格式、纸张设置和保存位置。",
		hints:  []string{"⌘ E：快速导出 PDF", "⌘ ⇧ E：导出选项", "预览工具栏 → Share"},
		points: []string{"PDF 支持按页范围导出并采用当前主题。Word 格式保留标题、段落和列表。", "HTML 可包含 Mermaid 图表，图表背景由文档主题提供。"},
	},
	"custom-css": {
		title: "自定义样式表", summary: "为 Markdown 预览和导出应用文档样式。可先使用内置主题。",
		hints:  []string{"设置 → 主题", "自定义样式表"},
		points: []string{"在设置中编辑样式表，或从该区域打开它的文件。", "开关控制是否应用 CSS，关闭后样式表文件仍可编辑。", "PDF 按可打印宽度使用屏幕样式。用外部编辑器改动样式表后，重新打开笔记可使快速 PDF 导出采用最新样式。"},
	},
	"themes": {
		title: "主题", summary: "选择应用窗口的外观和文档预览的样式。",
		hints:  []string{"设置 → 主题", "标题栏 → Preview Theme"},
		points: []string{"用应用外观调整窗口，用预览主题调整笔记的渲染效果。更改默认主题时，示例会随之更新。"},
	},
	"toc": {
		title: "目录", summary: "从当前笔记的各级标题生成目录链接。",
		hints: []string{"⌘ ⇧ T", "Insert → Table of Contents", "GLOW → /table-of-contents"},
	},
	"slides": {
		title: "幻灯片", summary: "在同一窗口演示笔记，以标题或分隔符划分幻灯片。",
		hints:  []string{"编辑器工具栏 → Slideshow", "选择 Play as Slides"},
		points: []string{"演示时可以切换布局和缩放。幻灯片支持笔记原有的代码、公式和表格。"},
	},
	"flashcards": {
		title: "闪卡", summary: "回想提示的答案，显示答案后评估自己的记忆。",
		hints:  []string{"编辑器工具栏 → Slideshow", "选择 Flash Cards"},
		points: []string{"显示答案后再评分。复习进度按笔记保存。"},
	},
	"focus": {
		title: "专注模式", summary: "收起编辑控件，专注当前文档。",
		hints:  []string{"⌘ ⇧ F", "标题栏 → Focus"},
		points: []string{"再次使用快捷键或点击退出按钮即可恢复编辑工具栏。"},
	},
	"terminal": {
		title: "GLOW 终端", summary: "用本地命令查看知识库并运行编辑器操作。",
		hints:  []string{"底栏 → GLOW Terminal", "⌃ `", "输入 / 查看应用命令"},
		points: []string{"终端可停靠在工作区、从标题栏下展开，或作为浮窗。可在本地命令中用双链指定笔记路径。"},
	},
	"wikibuilder": {
		title: "未链接提及与知识库检查", summary: "找出可转成双链的提及，并检查已有链接能否解析。",
		hints:  []string{"⌘ / → Vault Health Report", "GLOW → /health"},
		points: []string{"Vault Health.md 列出失效链接、孤立笔记、未链接提及和重名标题。"},
	},
	"drawings": {
		title: "绘图（Excalidraw）", summary: "在画布上勾画图示，并将它保存为 Excalidraw 场景。",
		hints:  []string{"标题栏 → Drawing → New Drawing", "Open Drawing…", "Map Vault"},
		points: []string{"绘图会在独立标签中打开。保存后，形状、标签和连线随绘图一起存入知识库。"},
	},
	"properties": {
		title: "属性与文档头", summary: "在侧栏编辑文档元数据，数据以 YAML 或 TOML 保存在笔记里。",
		hints:  []string{"知识库侧栏 → Properties", "在笔记开头编辑文档头"},
		points: []string{"添加属性时选择字段类型。保存后的键和值仍是 Markdown 文件的一部分。"},
	},
	"graph": {
		title: "知识图谱", summary: "查看笔记如何通过双链连接，并从图谱节点打开笔记。",
		hints:  []string{"知识库侧栏 → Graph", "Open Graph in Window"},
		points: []string{"可选择三维布局的形状或使用二维模式。搜索或应用标签来缩小图中的节点范围。"},
	},
	"concept-search": {
		title: "与知识库对话", summary: "原版可以在对话中引用知识库的相关段落。",
		hints:  []string{"可按文本搜索知识库的笔记", "打开结果核对来源"},
		points: []string{"模型驱动的知识库对话仍待实现。"},
	},
	"vault-embeddings": {
		title: "知识库向量索引", summary: "向量索引按语义相似度归类文本，可用于搜索和模型上下文。",
		hints:  []string{"使用知识库的普通搜索找文字", "Properties → Tags"},
		points: []string{"本地全文索引已可用。嵌入模型和语义索引仍待实现。"},
	},
	"backlinks": {
		title: "反向链接", summary: "查看哪些笔记通过双链指向当前文档。",
		hints:  []string{"打开工作区的反向链接区"},
		points: []string{"点入链可打开来源笔记。在同一文档上下文也能查看出链。"},
	},
	"quick-switcher": {
		title: "快速切换", summary: "按文件名或所在文件夹快速打开笔记和绘图。",
		hints:  []string{"⌘ P", "输入文件名后按 Return"},
		points: []string{"查看结果中的路径可区分同名的笔记。"},
	},
	"outline": {
		title: "大纲侧栏", summary: "通过当前笔记的标题浏览结构，并跳到指定章节。",
		hints:  []string{"⌘ ⇧ O", "View → Toggle Outline"},
		points: []string{"选择大纲中的标题即可在当前文档中定位。"},
	},
	"smartpaste": {
		title: "智能粘贴", summary: "粘贴纯文本或带格式的内容，然后检查编辑器生成的 Markdown。",
		hints:  []string{"⌘ V", "切到所见即所得模式处理富文本剪贴板", "粘贴后检查源文"},
		points: []string{"当剪贴板中的表格格式需要修正时，可使用表格编辑器。"},
	},
	"selectiontoolbar": {
		title: "选区工具栏", summary: "原版在选中的文字旁显示格式工具。",
		hints:  []string{"先选中文字", "使用编辑器工具栏设置格式"},
		points: []string{"普通工具栏支持粗体、斜体、链接和表格。浮动的选区工具栏仍待实现。"},
	},
	"hex": {
		title: "十六进制色块", summary: "十六进制颜色用三位或六位代码表示色值。",
		hints:  []string{"输入 #4f86d9", "在图表卡片中使用颜色选择器"},
		points: []string{"原版显示行内色块。object-N/A 能在支持的卡片中调整颜色。"},
	},
	"wysiwyg": {
		title: "所见即所得编辑", summary: "用可视化方式编辑标题、列表、链接和 Mermaid 卡片，同时保留 Markdown。",
		hints:  []string{"⌘ 4", "View → WYSIWYG", "切回源文查看 Markdown"},
		points: []string{"用表格编辑器更改单元格。需要编辑可视化模式尚不支持的结构时，切回源文。"},
	},
	"templates": {
		title: "模板", summary: "从模板开始写笔记，也可将整篇文档保存为模板。",
		hints:  []string{"File → New from Template", "⌘ / → Templates", "File → Save as Template…"},
		points: []string{"文件菜单中的入口打开新草稿。在命令面板选择模板则把它插入当前选区。"},
	},
	"palette": {
		title: "命令面板", summary: "用键盘搜索可用操作并执行选中的命令。",
		hints:  []string{"⌘ /", "输入命令后按 Return"},
		points: []string{"用方向键浏览匹配项。按 Escape 关闭面板并返回编辑器。"},
	},
	"quick-actions": {
		title: "AI 快速操作", summary: "原版可以让模型改写或解释选中的文字。",
		hints:  []string{"选中一段文字", "原版编辑器 → AI Quick Actions"},
		points: []string{"模型驱动的选区操作仍待实现。"},
	},
	"ai-panel": {
		title: "AI 工作区", summary: "在编辑器旁的对话中询问模型有关笔记的问题。",
		hints:  []string{"标题栏 → AI", "设置 → AI"},
		points: []string{"附上当前笔记或选区，选择是否将回复插入编辑器，再用历史按钮打开过去的对话。Aa 按钮在格式化回复与 Markdown 源文之间切换。"},
	},
	"voice": {
		title: "语音笔记与听写", summary: "原版编辑器可录制附件、转写录音或插入听写文字。",
		hints:  []string{"原版源文工具栏 → 麦克风", "录制语音笔记", "开始听写"},
		points: []string{"录音和转写仍待实现。"},
	},
}
