// spec: EV-UI-179

package workspace

import (
	"reflect"
	"testing"
)

func TestParseDelimitedPreviewPreservesPhysicalRows(t *testing.T) {
	tests := []struct {
		filename string
		source   string
		want     [][]string
	}{
		{
			filename: "Probe Table.csv",
			source:   "Item,Count,Notes\nAlpha,12,\"comma, retained\"\nBeta,3,\"multi\nline\"\n中文,0,\n",
			want: [][]string{
				{"Item", "Count", "Notes"},
				{"Alpha", "12", `"comma`, ` retained"`},
				{"Beta", "3", `"multi`},
				{`line"`},
				{"中文", "0", ""},
			},
		},
		{
			filename: "Probe Tabular.tsv",
			source:   "Item\tCount\tNotes\nDelta\t2\tTabbed\n中文\t7\tLine\n",
			want: [][]string{
				{"Item", "Count", "Notes"},
				{"Delta", "2", "Tabbed"},
				{"中文", "7", "Line"},
			},
		},
		{
			filename: "Physical.TSV",
			source:   "\r\n\t\t\u2028 \u0085",
			want:     [][]string{{"", "", ""}, {" "}},
		},
		{
			filename: "Graphemes.csv",
			source:   "a,\u0301b,c\n\u0600,x,z",
			want:     [][]string{{"a,\u0301b", "c"}, {"\u0600,x", "z"}},
		},
		{filename: "Empty.csv", source: "\r\n\u2028"},
		{filename: "Note.txt", source: "Item,Count,Notes\n"},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := ParseDelimitedPreview(tt.source, tt.filename); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("rows = %#v, want %#v", got, tt.want)
			}
		})
	}
}
