package ui

import "testing"

func TestNoColorDisablesColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	p := New()
	if got := p.Brand(); got != "administer" {
		t.Errorf("Brand() = %q with NO_COLOR set, want plain \"administer\"", got)
	}
}

func TestColorizeAddsAndOmitsEscapeCodes(t *testing.T) {
	on := &Printer{color: true}
	if got := on.colorize(colorGreen, "x"); got != colorGreen+"x"+colorReset {
		t.Errorf("colorize with color on = %q", got)
	}
	off := &Printer{color: false}
	if got := off.colorize(colorGreen, "x"); got != "x" {
		t.Errorf("colorize with color off = %q, want plain text", got)
	}
}
