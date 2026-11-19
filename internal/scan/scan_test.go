package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YOUR_GITHUB_USERNAME/administer/internal/report"
)

// adminServer answers /admin with a redirect to /admin/login and everything
// else with 404: the same shape as a typical panel behind a login page.
func adminServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	}))
}

func testConfig(t *testing.T, target string, words []string) Config {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	wordlistPath := filepath.Join(t.TempDir(), "words.txt")
	if err := os.WriteFile(wordlistPath, []byte(strings.Join(words, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Config{
		Target:       u,
		WordlistPath: wordlistPath,
		NumEngines:   5,
		Timeout:      5 * time.Second,
		MaxBodyBytes: 1 << 20,
	}
}

func readReport(t *testing.T, path string) report.Report {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	var rep report.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}
	return rep
}

func TestRunFindsTheAdminPathAndWritesAReport(t *testing.T) {
	srv := adminServer()
	defer srv.Close()

	cfg := testConfig(t, srv.URL, []string{"/nothing", "/admin", "/other"})
	cfg.ReportPath = filepath.Join(t.TempDir(), "report.json")

	outcome, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if outcome != Found {
		t.Errorf("outcome = %v, want found", outcome)
	}

	rep := readReport(t, cfg.ReportPath)
	if rep.Tool != "administer" {
		t.Errorf("tool = %q", rep.Tool)
	}
	if rep.Status != "found" || !rep.Found {
		t.Errorf("status = %q, found = %v; want found", rep.Status, rep.Found)
	}
	if rep.FoundURL != srv.URL+"/admin" {
		t.Errorf("found_url = %q, want %q", rep.FoundURL, srv.URL+"/admin")
	}
	if rep.RedirectTo != srv.URL+"/admin/login" {
		t.Errorf("redirect_to = %q, want %q", rep.RedirectTo, srv.URL+"/admin/login")
	}
	if rep.StatusCode != http.StatusFound {
		t.Errorf("status_code = %d, want 302", rep.StatusCode)
	}
	if rep.Total != 3 {
		t.Errorf("total_paths = %d, want 3", rep.Total)
	}
	if rep.Wordlist != cfg.WordlistPath {
		t.Errorf("wordlist = %q, want the custom path", rep.Wordlist)
	}
}

func TestRunWritesNoFileUnlessAReportIsRequested(t *testing.T) {
	srv := adminServer()
	defer srv.Close()

	cfg := testConfig(t, srv.URL, []string{"/admin"})

	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a scan without -o created %d file(s); it must create none", len(entries))
	}
}

func TestRunRefusesToOverwriteAnExistingReport(t *testing.T) {
	srv := adminServer()
	defer srv.Close()

	cfg := testConfig(t, srv.URL, []string{"/admin"})
	cfg.ReportPath = filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(cfg.ReportPath, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(context.Background(), cfg); err == nil {
		t.Fatal("expected an error when the report file already exists")
	}
	data, err := os.ReadFile(cfg.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep me" {
		t.Errorf("the existing report was modified: %q", data)
	}
}

func TestRunUsesTheBuiltInWordlistByDefault(t *testing.T) {
	srv := adminServer()
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Target:       u,
		NumEngines:   5,
		Timeout:      5 * time.Second,
		MaxBodyBytes: 1 << 20,
		ReportPath:   filepath.Join(t.TempDir(), "report.json"),
	}

	outcome, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if outcome != Found {
		t.Errorf("outcome = %v, want found", outcome)
	}
	rep := readReport(t, cfg.ReportPath)
	if rep.Wordlist != "built-in" {
		t.Errorf("wordlist = %q, want built-in", rep.Wordlist)
	}
	if rep.Total != 1800 {
		t.Errorf("total_paths = %d, want 1800", rep.Total)
	}
	if rep.FoundURL != srv.URL+"/admin" {
		t.Errorf("found_url = %q, want %q", rep.FoundURL, srv.URL+"/admin")
	}
}

func TestRunReportsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	cfg := testConfig(t, srv.URL, []string{"/a", "/b", "/c"})
	cfg.ReportPath = filepath.Join(t.TempDir(), "report.json")

	outcome, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if outcome != NotFound {
		t.Errorf("outcome = %v, want not_found", outcome)
	}
	rep := readReport(t, cfg.ReportPath)
	if rep.Status != "not_found" || rep.Found {
		t.Errorf("status = %q, found = %v; want not_found", rep.Status, rep.Found)
	}
	if rep.Completed != 3 {
		t.Errorf("completed_paths = %d, want 3", rep.Completed)
	}
}

func TestRunReportsAnInterruptedScanAsInterrupted(t *testing.T) {
	// The probe path answers at once; every wordlist path is slow, so the
	// cancel below lands in the middle of the scan itself.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, probePrefix) {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-r.Context().Done():
			}
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	words := make([]string, 200)
	for i := range words {
		words[i] = fmt.Sprintf("/p%03d", i)
	}
	cfg := testConfig(t, srv.URL, words)
	cfg.ReportPath = filepath.Join(t.TempDir(), "report.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)

	outcome, err := Run(ctx, cfg)
	if err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if outcome != Interrupted {
		t.Errorf("outcome = %v, want interrupted", outcome)
	}
	rep := readReport(t, cfg.ReportPath)
	if rep.Status != "interrupted" || rep.Found {
		t.Errorf("status = %q, found = %v; want interrupted and not found", rep.Status, rep.Found)
	}
	if rep.Completed >= rep.Total {
		t.Errorf("completed %d of %d: an interrupted scan must not look finished", rep.Completed, rep.Total)
	}
}

func TestRunFailsFastWhenTheTargetIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	target := srv.URL
	srv.Close() // nothing is listening on that address any more

	cfg := testConfig(t, target, []string{"/admin", "/login"})
	cfg.Timeout = 2 * time.Second

	start := time.Now()
	_, err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected an error for an unreachable target")
	}
	if !strings.Contains(err.Error(), "could not reach") {
		t.Errorf("error = %q, want it to say the target could not be reached", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("took %v; an unreachable target must fail after one probe", elapsed)
	}
}
