package sweep

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"--version"}, strings.NewReader(""), &out, &errOut)

	if code != ExitOK {
		t.Fatalf("Run() code = %d, want %d", code, ExitOK)
	}
	if strings.TrimSpace(out.String()) != "Directory Sweeper ver 0.3.0" {
		t.Fatalf("stdout = %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestRunUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"--age", "-1"}, strings.NewReader(""), &out, &errOut)

	if code != ExitUsage {
		t.Fatalf("Run() code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(errOut.String(), "--age") {
		t.Fatalf("stderr = %q, want age error", errOut.String())
	}
}

func TestRunDeletesRealFiles(t *testing.T) {
	dir := t.TempDir()
	removePath := filepath.Join(dir, "remove.bak")
	keepPath := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(removePath, []byte("backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keepPath, []byte("normal"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(removePath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"--age", "1", dir}, strings.NewReader(""), &out, &errOut)

	if code != ExitOK {
		t.Fatalf("Run() code = %d, want %d; stderr=%q", code, ExitOK, errOut.String())
	}
	if _, err := os.Stat(removePath); !os.IsNotExist(err) {
		t.Fatalf("removed file still exists or unexpected error: %v", err)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("kept file stat error: %v", err)
	}
}

func TestRunDryRunKeepsRealFiles(t *testing.T) {
	dir := t.TempDir()
	removePath := filepath.Join(dir, "remove.bak")
	if err := os.WriteFile(removePath, []byte("backup"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"--dry-run", dir}, strings.NewReader(""), &out, &errOut)

	if code != ExitOK {
		t.Fatalf("Run() code = %d, want %d; stderr=%q", code, ExitOK, errOut.String())
	}
	if _, err := os.Stat(removePath); err != nil {
		t.Fatalf("dry-run removed file: %v", err)
	}
	if !strings.Contains(out.String(), "Would remove:") {
		t.Fatalf("stdout = %q, want dry-run output", out.String())
	}
}
