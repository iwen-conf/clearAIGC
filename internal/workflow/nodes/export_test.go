package nodes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

func TestDOCXRoundTrip(t *testing.T) {
	exporter := NewExporter()
	parser := NewParser()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.docx")
	input := "First paragraph.\n\nSecond paragraph with details."

	if err := exporter.Export(context.Background(), input, domain.FormatDOCX, path); err != nil {
		t.Fatalf("export docx: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat docx: %v", err)
	}

	output, err := parser.Parse(context.Background(), path, domain.FormatDOCX)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if !strings.Contains(output, "First paragraph.") || !strings.Contains(output, "Second paragraph with details.") {
		t.Fatalf("unexpected parsed output: %q", output)
	}
}
