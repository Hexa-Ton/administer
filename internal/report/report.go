package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Report struct {
	Tool       string    `json:"tool"`
	Version    string    `json:"version"`
	Target     string    `json:"target"`
	Timestamp  time.Time `json:"timestamp"`
	Wordlist   string    `json:"wordlist"`
	Engines    int       `json:"engines"`
	Total      int64     `json:"total_paths"`
	Completed  int64     `json:"completed_paths"`
	Status     string    `json:"status"`
	Found      bool      `json:"found"`
	FoundURL   string    `json:"found_url,omitempty"`
	RedirectTo string    `json:"redirect_to,omitempty"`
	StatusCode int       `json:"status_code,omitempty"`
	DurationMS int64     `json:"duration_ms"`
}

// Write creates a new JSON report file at path. It refuses to
// overwrite an existing file — that is this tool's explicit,
// documented overwrite policy: none, by default.
func Write(path string, r Report) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("report file %q already exists; refusing to overwrite", path)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
