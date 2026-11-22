// spec: EV-MYGO-316 / U108

package shortsearch

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkLookup(b *testing.B) {
	store := openStore(b, filepath.Join(b.TempDir(), "index.sqlite3"))
	random := rand.New(rand.NewPCG(20261122, 316))
	words := strings.Fields("窗口 索引 字段 条目 事务 目录 组合 标记 位置 标题 样式 摘要 字体 查询 边界 指针 模板 页表 读取 渲染 容器 回滚 同步 预览")
	documents := make([]Document, 192)
	for i := range documents {
		var text strings.Builder
		if i%4 == 0 {
			text.WriteString("系统调用。\n")
		}
		if i%53 == 0 {
			text.WriteString("岬湾港。\n")
		}
		for range 96 {
			fmt.Fprintf(&text, "%s与%s [%04x]。\n", words[random.IntN(len(words))], words[random.IntN(len(words))], random.Uint32()&0xffff)
		}
		documents[i] = Document{Path: fmt.Sprintf("Notes/%03d.md", i), Version: "1", Text: text.String()}
	}
	for start := 0; start < len(documents); start += 16 {
		if err := store.Update(b.Context(), documents[start:min(start+16, len(documents))], nil); err != nil {
			b.Fatal(err)
		}
	}
	for name, query := range map[string]string{"sparse": "岬湾", "common": "系统", "miss": "赣魟"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			var candidates CandidateSet
			for b.Loop() {
				var err error
				candidates, err = store.Lookup(b.Context(), query)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(candidates.Matches)), "candidates/op")
		})
	}
}
