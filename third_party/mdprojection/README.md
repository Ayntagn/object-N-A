# mdprojection

`object-na.local/mdprojection` 是独立的 Go 1.24+ 模块。调用方提供经过校验的视觉 rune 区间和 Markdown UTF-8 字节区间，以及两份文本的版本号。库依靠长度增量移动编辑后续区间的坐标。

```go
import "object-na.local/mdprojection"

base := mdprojection.Version{Visual: 1, Source: 7}
mapping := mdprojection.New(base, []mdprojection.Span{
    {VisualStart: 0, VisualEnd: 3, SourceStart: 8, SourceEnd: 17},
})
patch, ok := mapping.Patch(base, mdprojection.Change{
    Start: 2, OldEnd: 3, NewEnd: 4,
})
```

`Patch` 返回编辑前的源区间和编辑后的视觉区间末端。调用方在原版本发布源文修改后，把新的视觉版本、源版本和该段的新字节长度交给 `Advance`。跨段修改、日志缺失或版本冲突可回到完整序列化流程，重新建立映射。一个映射由单个编辑会话持有。

object-N/A 先用现有序列化器校验打开时的源文。普通段落可以独立写回，周围可包含标题、列表、强调或表格。直接修改结构、格式或转义内容时沿用原始写回。源稿通过 MyGo 缓冲区进行版本匹配后再提交。

```sh
# 在模块目录验证区间移动和过期版本
CGO_ENABLED=0 go test ./...

# 在 object-N/A 根目录对照局部修改和完整写回
CGO_ENABLED=0 GOMAXPROCS=1 go test ./ui/wysiwyg -run '^$' \
  -bench '^BenchmarkVisualParagraphSync$' -benchmem -benchtime=250ms -count=5
```

基准包括富文本替换和草稿同步，使用约 100 KiB 或 1 MiB 的混排合成稿件。计时范围是进程内编辑和写回。
