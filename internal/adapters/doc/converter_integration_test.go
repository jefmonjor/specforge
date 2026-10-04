//go:build integration

package doc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentConverterConvertsCSV(t *testing.T) {
	if _, err := exec.LookPath("markitdown"); err != nil {
		t.Skip("markitdown not installed")
	}
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "reconciliation.csv")
	if err := os.WriteFile(csvFile, []byte("Account,Amount\nES123456,150.00\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := NewDocumentConverter().ConvertFile(csvFile, filepath.Join(tmpDir, "reconciliation.md"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("markdown not written: %v", err)
	}
	if !strings.Contains(string(data), "ES123456") {
		t.Errorf("markdown lacks source data: %s", data)
	}
}
