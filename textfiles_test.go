package hh

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// zipOf returns an archive holding the named files.
func zipOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(files[n])
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// WalkText reads text files in folders, in archives, and in archives inside
// archives.
func TestWalkText(t *testing.T) {
	dir := t.TempDir()
	day := zipOf(t, map[string][]byte{"TableOne.txt": []byte(sampleShowdown), "notes.md": []byte("not a hand")})
	all := zipOf(t, map[string][]byte{"day1.zip": day, "day2/TableTwo.txt": []byte(sampleRunTwice)})
	if err := os.WriteFile(filepath.Join(dir, "all.zip"), all, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "loose.txt"), []byte(sampleShowdown), 0o644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	got := map[string]int{}
	err := WalkText([]string{dir}, 4, func(path string, data []byte) {
		st, _ := ParseText(data, func(*HandHistory) error { return nil }, nil)
		mu.Lock()
		got[RelText(dir, path)] = st.Hands
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"loose.txt": 1,
		filepath.Join("all", "day1", "TableOne.txt"): 1,
		filepath.Join("all", "day2", "TableTwo.txt"): 1,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%s: %d hands, want %d (got %v)", k, got[k], n, got)
		}
	}
}
