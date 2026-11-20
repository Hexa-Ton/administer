package wordlist

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Hexa-Ton/administer/wordlists"
)

// MandatorySix is the required priority order for the first six
// entries of the built-in wordlist. They are always moved to the
// front of the job queue, in this exact order, regardless of where
// they appear in the source file.
var MandatorySix = []string{
	"/admin",
	"/login",
	"/admin/login",
	"/login/admin",
	"/owner",
	"/master",
}

// List is a parsed, de-duplicated wordlist in dispatch order.
type List struct {
	Paths []string
}

// BuiltinText returns the raw text of the wordlist embedded in the binary.
func BuiltinText() string {
	return wordlists.Builtin
}

// Builtin parses the wordlist embedded in the binary. It needs no files on disk.
func Builtin() (*List, error) {
	return parse(strings.NewReader(wordlists.Builtin), "built-in wordlist")
}

// Load reads a custom wordlist file. Blank lines and lines starting with #
// are skipped, duplicates are removed (first occurrence wins), and the
// mandatory six are reordered to the front if present.
func Load(path string) (*List, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening wordlist %q: %w", path, err)
	}
	defer f.Close()
	return parse(f, path)
}

func parse(r io.Reader, name string) (*List, error) {
	seen := make(map[string]bool)
	var rest []string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "/") {
			line = "/" + line
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		rest = append(rest, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading wordlist %q: %w", name, err)
	}

	ordered := make([]string, 0, len(rest))
	mandatorySet := make(map[string]bool, len(MandatorySix))
	for _, m := range MandatorySix {
		mandatorySet[m] = true
		if seen[m] {
			ordered = append(ordered, m)
		}
	}
	for _, p := range rest {
		if !mandatorySet[p] {
			ordered = append(ordered, p)
		}
	}

	return &List{Paths: ordered}, nil
}

// VerifyMandatory checks that the first six paths are exactly the
// mandatory priority paths, in order.
func VerifyMandatory(paths []string) error {
	if len(paths) < len(MandatorySix) {
		return fmt.Errorf("wordlist has fewer than %d entries", len(MandatorySix))
	}
	for i, want := range MandatorySix {
		if paths[i] != want {
			return fmt.Errorf("position %d is %q, want %q", i+1, paths[i], want)
		}
	}
	return nil
}
