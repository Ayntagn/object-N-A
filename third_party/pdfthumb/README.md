# pdfthumb

`object-na.local/pdfthumb` 是 PDF 页图调度与缓存库。调用方提供 PDF 渲染器，并按视口顺序调用 `Thumbnail`。库按文档内容摘要、从 1 开始的页号和目标像素最长边标识栅格。缩略图数据使用不可变的字节切片。

```go
cache := pdfthumb.New(24<<20, 1)
key := pdfthumb.Key{Document: sha256.Sum256(pdf), Page: 1, Side: 360}
data, err := cache.Thumbnail(ctx, key, render)
```

同一键的并发调用共用渲染。取消最后一个等待者会取消渲染上下文。尚未开始的任务可以立即退出，已经进入渲染器的取消时机由渲染器决定。缓存以编码后的字节数计算，按最近使用顺序回收。

object-N/A 的 PDFKit 预览使用单个文档读者和按需请求。PDF.js 预览复用同一浏览器调度逻辑，并在相同 PDF.js 文档中渲染可见页及邻页。PDF.js 占位时使用首张页的尺寸，遇到不同页面尺寸时在渲染前修正当前页的占位大小。浏览器里的非可见页缓存另有预算，按解码后的近似像素字节计费。单双列布局的 360 像素缩略图可以在原页复用。单页渲染失败时可在离开该页再返回后重试一次。

验证命令为 `go test -race ./...`。模块使用 Go 1.24 及标准库。
