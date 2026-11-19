package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetAcceptsValidValues(t *testing.T) {
	c := Default()

	got, err := c.Set("engines", "213")
	if err != nil || got.Engines != 213 {
		t.Errorf("engines 213: got %+v, err %v", got, err)
	}
	got, err = c.Set("timeout", "30")
	if err != nil || got.Timeout != 30 {
		t.Errorf("timeout 30: got %+v, err %v", got, err)
	}
	got, err = c.Set("rate", "2.5")
	if err != nil || got.Rate != 2.5 {
		t.Errorf("rate 2.5: got %+v, err %v", got, err)
	}
	got, err = c.Set("color", "off")
	if err != nil || got.Color {
		t.Errorf("color off: got %+v, err %v", got, err)
	}
}

func TestSetRejectsInvalidValues(t *testing.T) {
	tests := []struct{ key, value string }{
		{"engines", "0"},
		{"engines", "214"},
		{"engines", "many"},
		{"timeout", "0"},
		{"timeout", "301"},
		{"rate", "-1"},
		{"rate", "fast"},
		{"color", "maybe"},
		{"verbose", "1"},
	}
	for _, tc := range tests {
		if _, err := Default().Set(tc.key, tc.value); err == nil {
			t.Errorf("Set(%q, %q): expected an error", tc.key, tc.value)
		}
	}
}

func TestLoadWithoutFileReturnsDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != Default() {
		t.Errorf("got %+v, want defaults %+v", got, Default())
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := Default()
	want.Engines = 77
	want.Rate = 12.5
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadFillsMissingFieldsWithDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"engines": 50}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Engines != 50 {
		t.Errorf("engines = %d, want 50", got.Engines)
	}
	if got.Timeout != Default().Timeout {
		t.Errorf("a missing timeout must keep its default, got %d", got.Timeout)
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Error("expected an error for invalid JSON")
	}
}
