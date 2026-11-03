package search

import (
	"strings"
	"testing"
)

func FuzzContains(f *testing.F) {
	for _, pair := range [][2]string{{"", ""}, {"postgres:5432", "5432"}, {strings.Repeat("x", 65) + "needle", "needle"}, {strings.Repeat("a", 128), "ab"}, {"привет мир", "мир"}} {
		f.Add(pair[0], pair[1])
	}
	f.Fuzz(func(t *testing.T, text, query string) {
		if got, want := Contains(text, query), strings.Contains(text, query); got != want {
			t.Fatalf("Contains(%q,%q)=%v want %v", text, query, got, want)
		}
	})
}
func BenchmarkContains(b *testing.B) {
	s := strings.Repeat("postgres container service ", 100)
	b.ReportAllocs()
	for b.Loop() {
		Contains(s, "missing")
	}
}
