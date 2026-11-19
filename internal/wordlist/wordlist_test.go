package wordlist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var wantFirstSix = []string{"/admin", "/login", "/admin/login", "/login/admin", "/owner", "/master"}

func TestBuiltinHasExactly1800UniquePaths(t *testing.T) {
	list, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin() error: %v", err)
	}
	if got := len(list.Paths); got != 1800 {
		t.Fatalf("built-in wordlist has %d unique paths, want 1800", got)
	}

	// Builtin() de-duplicates, so also check the raw text itself: exactly
	// 1800 lines and no repeated line.
	raw := strings.Split(strings.TrimSpace(BuiltinText()), "\n")
	if len(raw) != 1800 {
		t.Fatalf("built-in wordlist text has %d lines, want 1800", len(raw))
	}
	seen := make(map[string]bool, len(raw))
	for _, line := range raw {
		if seen[line] {
			t.Errorf("duplicate line in built-in wordlist: %q", line)
		}
		seen[line] = true
	}
}

func TestBuiltinFirstSixLinesAreExact(t *testing.T) {
	raw := strings.Split(strings.TrimSpace(BuiltinText()), "\n")
	for i, want := range wantFirstSix {
		if raw[i] != want {
			t.Errorf("line %d of the built-in wordlist is %q, want %q", i+1, raw[i], want)
		}
	}

	list, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMandatory(list.Paths); err != nil {
		t.Errorf("mandatory six not first after loading: %v", err)
	}
}

func TestBuiltinPathsAreWellFormed(t *testing.T) {
	list, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list.Paths {
		if !strings.HasPrefix(p, "/") {
			t.Errorf("path %q does not start with /", p)
		}
		if strings.ContainsAny(p, " \t\r") {
			t.Errorf("path %q contains whitespace", p)
		}
	}
}

func TestLoadCustomWordlist(t *testing.T) {
	file := filepath.Join(t.TempDir(), "custom.txt")
	content := "x\n/login\n# a comment\n\n/admin\n/x\n/owner\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}

	// Comments and blank lines skipped, "x" normalised to "/x", the duplicate
	// "/x" dropped, and the mandatory paths moved to the front in their
	// required order.
	want := []string{"/admin", "/login", "/owner", "/x"}
	if len(list.Paths) != len(want) {
		t.Fatalf("got %v, want %v", list.Paths, want)
	}
	for i := range want {
		if list.Paths[i] != want[i] {
			t.Fatalf("got %v, want %v", list.Paths, want)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Fatal("expected an error for a missing wordlist file")
	}
}

func TestVerifyMandatory(t *testing.T) {
	if err := VerifyMandatory([]string{"/admin"}); err == nil {
		t.Error("expected an error for a list that is too short")
	}
	wrongOrder := []string{"/login", "/admin", "/admin/login", "/login/admin", "/owner", "/master"}
	if err := VerifyMandatory(wrongOrder); err == nil {
		t.Error("expected an error when the first six are out of order")
	}
	if err := VerifyMandatory(append([]string{}, wantFirstSix...)); err != nil {
		t.Errorf("unexpected error for the correct order: %v", err)
	}
}
