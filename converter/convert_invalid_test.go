package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectsNonPDF(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "not.pdf")
	if err := os.WriteFile(in, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewConverter(nil).Convert(in, filepath.Join(dir, "out.pptx"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "不是有效的 PDF") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "out.pptx")); !os.IsNotExist(statErr) {
		t.Fatal("output should not be created")
	}
}

func TestEmptyPath(t *testing.T) {
	_, err := NewConverter(nil).Convert(filepath.Join(t.TempDir(), "missing.pdf"), filepath.Join(t.TempDir(), "out.pptx"))
	if err == nil || !strings.Contains(err.Error(), "无法读取文件") {
		t.Fatalf("got %v", err)
	}
}
