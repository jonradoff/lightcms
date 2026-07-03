package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadStdinOrFile covers the file, missing-file, and stdin branches.
func TestReadStdinOrFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.html")
	if err := os.WriteFile(path, []byte("<div>hi</div>"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	data, err := readStdinOrFile(path)
	if err != nil {
		t.Fatalf("readStdinOrFile(file): %v", err)
	}
	if string(data) != "<div>hi</div>" {
		t.Errorf("unexpected content: %q", data)
	}

	if _, err := readStdinOrFile(filepath.Join(dir, "missing.html")); err == nil {
		t.Error("expected error for missing file")
	}

	// Stdin branch: under `go test` stdin is /dev/null, so this reads empty
	// input. We only assert it doesn't panic; the error depends on platform.
	_, _ = readStdinOrFile("-")
}
