package templates

import (
	"errors"
	"html/template"
	"io"
	"log/slog"
)

type Renderer interface {
	Render(writer io.Writer, name string, data any) error
}

type BaseRenderer struct {
	templates *template.Template
}

func New(path string) *BaseRenderer {
	templates, err := template.New("").Funcs(funcMap()).ParseGlob(path)
	if err != nil {
		slog.Error("Failed to parse templates", "error", err, "path", path)
		return &BaseRenderer{templates: nil}
	}
	slog.Info("Templates parsed successfully", "path", path)
	return &BaseRenderer{templates: templates}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(content string) template.HTML {
			// content is sanitised in ai/chat.go via sanitise()
			// #nosec G203
			return template.HTML(content)
		},
	}
}

func (r *BaseRenderer) Render(writer io.Writer, name string, data any) error {
	if r.templates == nil {
		slog.Error("Cannot render with nil templates", "template", name)
		return errors.New("templates are nil")
	}
	return r.templates.ExecuteTemplate(writer, name, data)
}

type MockRenderer struct {
	ShouldFail bool
}

func (m *MockRenderer) Render(writer io.Writer, name string, data any) error {
	if m.ShouldFail {
		return errors.New("mock render error")
	}
	return nil
}
