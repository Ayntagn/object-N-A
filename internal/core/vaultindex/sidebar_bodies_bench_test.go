// spec: EV-UI-184 / EV-MYGO-316
package vaultindex

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"cyberwriter/internal/core/workspace"
)

func BenchmarkSidebarBodyIndex(b *testing.B) {
	root := b.TempDir()
	const files = 512
	mixedSidebarFixture(b, root, files, 128<<10)
	vault, err := workspace.Open(root)
	if err != nil {
		b.Fatal(err)
	}
	defer vault.Close()
	if !vault.HasIndexedTextVersions() {
		b.Skip("the fixture volume does not supply precise saved-text versions")
	}
	entries, err := vault.SidebarEntries(true)
	if err != nil {
		b.Fatal(err)
	}
	cache := filepath.Join(b.TempDir(), "search.sqlite")
	ctx := context.Background()
	started := time.Now()
	if err := RefreshSidebarBodyIndex(ctx, vault, entries, cache); err != nil {
		b.Fatal(err)
	}
	cold := time.Since(started)
	info, err := os.Stat(cache)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("%d notes; cold build %s; %d bytes at rest", files, cold, info.Size())
	var index *Index
	for _, probe := range []struct{ name, query string }{
		{"Rare", "潮汐"}, {"Common", "页面"}, {"Miss", "珊瑚"},
	} {
		scan := func() []SidebarSearchHit { return index.SearchSidebar(ctx, vault, entries, probe.query, "") }
		cached := func() []SidebarSearchHit {
			return index.SearchSidebarWithBodyIndex(ctx, vault, entries, probe.query, "", cache)
		}
		if a, z := scan(), cached(); !reflect.DeepEqual(a, z) {
			b.Fatalf("query %q changed ordered hits (%d versus %d)", probe.query, len(a), len(z))
		}
		for _, path := range []struct {
			name string
			run  func() []SidebarSearchHit
		}{{"Scan", scan}, {"Index", cached}} {
			b.Run(probe.name+"/"+path.name, func(b *testing.B) {
				times := make([]time.Duration, 0, 512)
				b.ReportAllocs()
				for b.Loop() {
					at := time.Now()
					_ = path.run()
					if len(times) < cap(times) {
						times = append(times, time.Since(at))
					}
				}
				slices.Sort(times)
				if n := len(times); n != 0 {
					b.ReportMetric(float64(times[(n*50+99)/100-1].Nanoseconds()), "p50-ns")
					b.ReportMetric(float64(times[(n*95+99)/100-1].Nanoseconds()), "p95-ns")
				}
				if path.name == "Index" {
					b.ReportMetric(float64(cold.Microseconds())/1000, "cold-ms")
					b.ReportMetric(float64(info.Size()), "db-bytes")
				}
			})
		}
	}
	b.Run("RefreshOne", func(b *testing.B) {
		file := filepath.Join(root, "Field Notes 0000.md")
		content, err := os.ReadFile(file)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			content[len(content)-1] ^= 1
			if err := os.WriteFile(file, content, 0o600); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			if err := RefreshSidebarBodyIndex(ctx, vault, entries, cache); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func mixedSidebarFixture(t testing.TB, root string, files, size int) {
	t.Helper()
	rng := rand.New(rand.NewSource(319))
	words := strings.Fields("内存 帧率 节点 渲染 存储 文本 检索 查询 词法 字段 数据 文档 标签 页码 缓存 预览 指标 更新 排序 快照")
	cities := strings.Fields("北京 苏州 大连 杭州 南京 成都 珠海 高雄 南宁 上海 香港 厦门")
	for i := range files {
		var text strings.Builder
		text.Grow(size + 256)
		if i%71 == 0 {
			text.WriteString("This note recorded the 潮汐 observations.\n")
		}
		if i%4 == 0 {
			text.WriteString("相关页面保存在这里。\n")
		}
		for text.Len() < size {
			fmt.Fprintf(&text, "### %s手记\n%s %s。Cafe\u0301 / résumé / id-%06x references note %03d.\n",
				cities[rng.Intn(len(cities))], words[rng.Intn(len(words))], words[rng.Intn(len(words))],
				rng.Intn(1<<24), rng.Intn(files))
		}
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("Field Notes %04d.md", i)), []byte(text.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
