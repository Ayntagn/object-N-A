package wysiwyg

// spec: EV-EDITOR-004 / EV-EDITOR-007 / EV-MYGO-306 / EV-MYGO-326

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
)

func TestIncrementalParagraphKeepsSourceAndUndoSemantics(t *testing.T) {
	const prefix = "\ufeff---\ntitle: '原稿'\n---\n\n# 目录\n\n"
	const body = "**强调不变**\n\nA😀中B\n\n普通段落 e\u0301\n\n| A | B |\n| - | - |\n| 甲 | 乙 |"
	const original = prefix + body + "\n"
	draft := editor.NewBuffer(original)
	s := Open(draft)
	if s.Document == nil {
		t.Fatal("夹具应支持视觉编辑")
	}
	before, _, _ := strings.Cut(s.Document.Text().PlainText(), "A😀中B")
	at := len([]rune(before + "A😀"))
	s.Document.SetSelection(at, at+1)
	s.Document.InsertText("文呀")
	want := strings.Replace(original, "A😀中B", "A😀文呀B", 1)
	if !s.Sync(draft) || draft.String() != want {
		t.Fatalf("局部保存 = %q，期望 %q", draft.String(), want)
	}
	if s.incremental == nil || !s.incremental.dirty {
		t.Fatal("普通段落应使用局部写回")
	}
	a, z := s.Document.Selection()
	if a != at+2 || z != at+2 {
		t.Fatalf("输入后选区 = (%d, %d)", a, z)
	}

	s.Document.Undo()
	if !s.Sync(draft) || draft.String() != original || draft.Modified() {
		t.Fatalf("撤销后保存 = %q", draft.String())
	}
	s.Document.Redo()
	if !s.Sync(draft) || draft.String() != want {
		t.Fatalf("重做后保存 = %q", draft.String())
	}

	// The table follows the changed paragraph. Its source offsets must
	// remain usable even when the full source has not yet been materialized.
	table := s.EditTable(0)
	if table == nil || table.Rows()[1][0] != "甲" {
		t.Fatal("局部写回后表格应读取当前源文范围")
	}
	draft.Set("Source changed independently")
	s.Document.InsertText("旧稿")
	if s.Sync(draft) || draft.String() != "Source changed independently" {
		t.Fatal("过期的视觉草稿覆盖了更新的源文")
	}
}

func TestIncrementalFallbackUsesCurrentSourceForStructure(t *testing.T) {
	const original = "A😀中B\n\nA😀中B\n\n| A | B |\n| - | - |\n| 甲 | 乙 |\n"
	draft := editor.NewBuffer(original)
	s := Open(draft)
	s.Document.SetSelection(6, 6)
	s.Document.InsertText("增")
	want := strings.Replace(original, "\n\nA😀中B", "\n\nA增😀中B", 1)
	if !s.Sync(draft) || draft.String() != want || s.incremental == nil || !s.incremental.dirty {
		t.Fatalf("重复段落应写回第二处 = %q", draft.String())
	}
	s.Document.Edit(func(e *richtext.Edit) {
		e.ReplaceString(6, 6, "\n段落", richtext.Attributes{})
	})
	want = strings.Replace(want, "\n\nA增", "\n\nA\n\n段落增", 1)
	if !s.Sync(draft) || draft.String() != want {
		t.Fatalf("结构性变化后保存 = %q，期望 %q", draft.String(), want)
	}
	s.Document.Undo()
	if !s.Sync(draft) || !strings.Contains(draft.String(), "\n\nA增😀中B\n\n") {
		t.Fatalf("撤销断段 = %q", draft.String())
	}
}

func TestIncrementalDefersProjectionAfterStructuralEdits(t *testing.T) {
	const original = "# 标题甲\n\n- 列表甲\n- 同一列表\n\nA😀中B\n"
	draft := editor.NewBuffer(original)
	s := Open(draft)
	if s.incremental == nil || s.incremental.ranges == nil {
		t.Fatal("夹具应支持局部写回")
	}
	want := original
	for _, target := range []string{"标题", "列表"} {
		for _, replacement := range []string{"乙", "丙"} {
			prefix, tail, ok := strings.Cut(s.Document.Text().PlainText(), target)
			if !ok {
				t.Fatalf("找不到编辑位置 %q", target)
			}
			at := utf8.RuneCountInString(prefix + target)
			s.Document.SetSelection(at, at+1)
			s.Document.InsertText(replacement)
			old, _ := utf8.DecodeRuneInString(tail)
			want = strings.Replace(want, target+string(old), target+replacement, 1)
			if !s.Sync(draft) || draft.String() != want {
				t.Fatalf("写回 %q 后源文 = %q，期望 %q", target, draft.String(), want)
			}
			if s.incremental.ranges != nil || s.incremental.dirty {
				t.Fatalf("写回 %q 后仍同步重建了段落映射", target)
			}
		}
	}
	prefix, _, _ := strings.Cut(s.Document.Text().PlainText(), "中B")
	at := utf8.RuneCountInString(prefix)
	s.Document.SetSelection(at, at+1)
	s.Document.InsertText("文呀")
	if !s.Sync(draft) || !s.incremental.dirty || draft.String() != strings.Replace(want, "A😀中B", "A😀文呀B", 1) {
		t.Fatalf("结构性编辑后未局部写回普通段落 = %q", draft.String())
	}
	s.Document.Undo()
	if !s.Sync(draft) || draft.String() != want {
		t.Fatalf("撤销普通段落编辑 = %q，期望 %q", draft.String(), want)
	}
}

func BenchmarkVisualParagraphSync(b *testing.B) {
	const paragraph = "MyGo 在一处修改时保留相邻段落和 Unicode 组合字形 e\u0301。长稿包含多段普通文字、图片注释和多语言说明 😀。这行是独立段落。\n\n"
	const start = "# 资料\n\n**强调**\n\n- 计划\n- 执行\n\n"
	const marker = "编辑位置😀甲 end"
	const end = "| A | B |\n| - | - |\n| 中 | 文 |\n"
	for _, size := range []struct {
		name  string
		bytes int
	}{
		{"100KiB", 100 << 10},
		{"1MiB", 1 << 20},
	} {
		half := strings.Repeat(paragraph, size.bytes/(2*len(paragraph)))
		source := start + half + marker + "\n\n" + half + end
		for _, implementation := range []struct {
			name        string
			incremental bool
		}{
			{"full", false},
			{"local", true},
		} {
			b.Run(size.name+"/"+implementation.name, func(b *testing.B) {
				draft := editor.NewBuffer(source)
				session := Open(draft)
				if session.Document == nil || session.incremental == nil || session.incremental.ranges == nil {
					b.Fatal("夹具必须能使用局部写回")
				}
				if !implementation.incremental {
					session.incremental = nil
				}
				before, _, _ := strings.Cut(session.Document.Text().PlainText(), marker)
				at := utf8.RuneCountInString(before + "编辑位置😀")
				updated := strings.Replace(source, marker, "编辑位置😀乙 end", 1)
				want := source
				alternate := false
				times := make([]time.Duration, 0, 512)
				b.ReportAllocs()
				for b.Loop() {
					started := time.Now()
					alternate = !alternate
					replacement := "甲"
					if alternate {
						replacement, want = "乙", updated
					} else {
						want = source
					}
					session.Document.Edit(func(edit *richtext.Edit) {
						edit.ReplaceString(at, at+1, replacement, richtext.Attributes{})
					})
					if !session.Sync(draft) {
						b.Fatal("视觉修改未同步")
					}
					if len(times) < cap(times) {
						times = append(times, time.Since(started))
					}
				}
				if draft.String() != want || implementation.incremental && !session.incremental.dirty {
					b.Fatal("写回内容或写回路径与基准不符")
				}
				slices.Sort(times)
				if n := len(times); n != 0 {
					b.ReportMetric(float64(times[(n*95+99)/100-1].Nanoseconds()), "p95-ns")
				}
			})
		}
	}
}
