package debugdump

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestWriter_RotatesAndPrunes(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	line := []byte(strings.Repeat("x", 59)) // 60 bytes with newline: one line per file
	for i := 0; i < 5; i++ {
		if err := w.WriteLine(line); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond) // distinct rotation timestamps
	}

	names := listDir(t, dir)
	if len(names) != 3 {
		t.Fatalf("want active + 2 rotated files, got %v", names)
	}
	for _, n := range names {
		st, err := os.Stat(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		if st.Size() != 60 {
			t.Errorf("%s: size %d, want 60", n, st.Size())
		}
	}
}

func TestWriter_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir, 1000, 2)
	_ = w.WriteLine([]byte("a"))
	_ = w.Close()

	w, _ = NewWriter(dir, 1000, 2)
	_ = w.WriteLine([]byte("b"))
	_ = w.Close()

	data, _ := os.ReadFile(filepath.Join(dir, activeName))
	if string(data) != "a\nb\n" {
		t.Fatalf("got %q", data)
	}
}

func TestRedactHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer sk-ant-oat01-secret-abcd")
	h.Set("X-Api-Key", "short")
	h.Set("Anthropic-Beta", "claude-code-20250219")

	got := RedactHeaders(h)
	if v := got.Get("Authorization"); v != "Bearer ***abcd" {
		t.Errorf("authorization = %q", v)
	}
	if v := got.Get("X-Api-Key"); v != "***" {
		t.Errorf("x-api-key = %q", v)
	}
	if v := got.Get("Anthropic-Beta"); v != "claude-code-20250219" {
		t.Errorf("anthropic-beta = %q", v)
	}
	if h.Get("Authorization") != "Bearer sk-ant-oat01-secret-abcd" {
		t.Error("original header mutated")
	}
}
