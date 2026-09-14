package templates

import "testing"

func TestNew_ParsesProductionTemplates(t *testing.T) {
	r := New("../../templates/*.html")
	if r == nil {
		t.Fatal("expected non-nil renderer")
	}
	if r.templates == nil {
		t.Fatal("failed to parse production templates; check template syntax errors above")
	}
}
