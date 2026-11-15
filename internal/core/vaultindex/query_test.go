// spec: FEAT-001 / EV-UI-160
package vaultindex

import (
	"reflect"
	"testing"
)

func TestVaultQueryClauses(t *testing.T) {
	index := buildFixture(t, map[string]string{
		"Projects 2026/Survey.md":          "---\nstatus: active\ntags: [person, engineering]\ntitle: 'Q1: Results'\npriority: 3\nrole: Architect\n---\n",
		"Projects 2026/Sub/Alpha.markdown": "---\nstatus: active\ntitle: 'Q1: Results'\npriority: 12\nrole: Developer\n---\n#person\n",
		"Projects 2026/Brief.md":           "---\nstatus: done\ntags: [person, engineering]\ntitle: Other\npriority: 8\nrole: Engineer\n---\n",
		"Outside.md":                       "---\nstatus: active\ntitle: 'Q1: Results'\npriority: 20\n---\n",
	})
	cases := []struct {
		source string
		want   QueryResult
	}{
		{
			source: "tAbLe role, priority\nFROM \"Projects 2026\"\nWHERE priority>=3 AND title = \"Q1: Results\"\nsOrT priority DESC\nLIMIT 1",
			want: QueryResult{Columns: []string{"role", "priority"}, Rows: []QueryRow{
				{Path: "Projects 2026/Sub/Alpha.markdown", Name: "Alpha", Cells: []string{"Developer", "12"}},
			}},
		},
		{
			source: "TABLE role\nFROM #person\nSORT role ASC",
			want: QueryResult{Columns: []string{"role"}, Rows: []QueryRow{
				{Path: "Projects 2026/Survey.md", Name: "Survey", Cells: []string{"Architect"}},
				{Path: "Projects 2026/Sub/Alpha.markdown", Name: "Alpha", Cells: []string{"Developer"}},
				{Path: "Projects 2026/Brief.md", Name: "Brief", Cells: []string{"Engineer"}},
			}},
		},
		{
			source: "LIST FROM #engineering\nWHERE status != \"active\" AND tags contains \"engineering\"",
			want:   QueryResult{Rows: []QueryRow{{Path: "Projects 2026/Brief.md", Name: "Brief"}}},
		},
	}
	for _, tc := range cases {
		got, err := index.Query(tc.source)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Query(%q) = %#v, %v, want %#v", tc.source, got, err, tc.want)
		}
	}
}

func TestVaultQueryFallbackAndNoMatches(t *testing.T) {
	index := buildFixture(t, map[string]string{"B/1-sample.md": "", "A/Sample.md": ""})
	got, err := index.Query("not a valid query")
	want := QueryResult{Rows: []QueryRow{{Path: "B/1-sample.md", Name: "1-sample"}, {Path: "A/Sample.md", Name: "Sample"}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("unrecognized input returned %#v, %v, want %#v", got, err, want)
	}
	got, err = index.Query("TABLE status\nFROM #absent")
	if err != nil || len(got.Rows) != 0 {
		t.Fatalf("empty match returned %#v, %v", got, err)
	}
	for _, source := range []string{"", "WHERE status", "LIMIT many", "WHERE status = active OR status = done"} {
		if _, err := index.Query(source); err != ErrInvalidQuery {
			t.Errorf("Query(%q) error = %v, want ErrInvalidQuery", source, err)
		}
	}
}

func TestVaultQueryMissingFields(t *testing.T) {
	index := buildFixture(t, map[string]string{
		"Z/Alpha.md": "---\nStatus: ready\n---\n",
		"Beta.md":    "---\nstatus: pending\n---\n",
		"Z/Gamma.md": "# Gamma\n",
	})
	got, err := index.Query(`TABLE Status WHERE status != "READY" SORT status ASC`)
	want := QueryResult{Columns: []string{"Status"}, Rows: []QueryRow{
		{Path: "Beta.md", Name: "Beta", Cells: []string{"pending"}},
		{Path: "Z/Gamma.md", Name: "Gamma", Cells: []string{""}},
	}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Query with missing fields = %#v, %v, want %#v", got, err, want)
	}
}
