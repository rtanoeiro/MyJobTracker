package templates

import (
	"bytes"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNew_InvalidPath(t *testing.T) {
	r := New("/nonexistent/path/that/does/not/exist/*.html")
	if r == nil {
		t.Fatal("expected non-nil renderer even on parse error")
	}
	if r.templates != nil {
		t.Error("expected nil templates for invalid path")
	}
}

func TestNew_ValidPath(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "tmpl*.html")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(`{{define "greeting"}}Hello{{end}}`); err != nil {
		t.Fatalf("failed to write template content: %v", err)
	}
	errClose := f.Close()
	if errClose != nil {
		t.Error("error closing file, failing test")
	}

	r := New(filepath.Join(dir, "*.html"))
	if r.templates == nil {
		t.Error("expected templates to be loaded for valid path")
	}
}

func TestBaseRenderer_Render_NilTemplates(t *testing.T) {
	r := &BaseRenderer{templates: nil}

	err := r.Render(io.Discard, "any", nil)

	if err == nil {
		t.Error("expected error when templates are nil")
	}
}

func TestBaseRenderer_Render_Success(t *testing.T) {
	tmpl := template.Must(template.New("greeting").Parse("Hello {{.}}"))
	r := &BaseRenderer{templates: tmpl}
	var buf bytes.Buffer

	err := r.Render(&buf, "greeting", "World")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "Hello World" {
		t.Errorf("output = %q, want %q", buf.String(), "Hello World")
	}
}

func TestBaseRenderer_Render_TemplateNotFound(t *testing.T) {
	tmpl := template.Must(template.New("greeting").Parse("Hello"))
	r := &BaseRenderer{templates: tmpl}

	err := r.Render(io.Discard, "nonexistent", nil)

	if err == nil {
		t.Error("expected error for unknown template name")
	}
}
