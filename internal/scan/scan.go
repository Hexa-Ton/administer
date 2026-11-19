package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/YOUR_GITHUB_USERNAME/administer/internal/classify"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/engine"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/netx"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/report"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/scheduler"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/ui"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/urlutil"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/version"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/wordlist"
)

// builtinLabel is shown (and recorded in reports) when no -w file is given.
const builtinLabel = "built-in"

// probePrefix starts the random path used for the reachability and
// wildcard probe that runs before the real scan.
const probePrefix = "administer-wildcard-check-"

// Outcome says how a scan ended; the CLI maps it to an exit code.
type Outcome int

const (
	// NotFound means every path was checked and none was a candidate.
	NotFound Outcome = iota
	// Found means a candidate was discovered and the scan stopped on it.
	Found
	// Interrupted means the user cancelled (Ctrl+C) before the scan finished.
	Interrupted
)

// String is the value recorded in the JSON report's "status" field.
func (o Outcome) String() string {
	switch o {
	case Found:
		return "found"
	case Interrupted:
		return "interrupted"
	default:
		return "not_found"
	}
}

// Config is everything a scan needs.
type Config struct {
	Target          *url.URL
	WordlistPath    string // empty means the built-in wordlist
	NumEngines      int
	Timeout         time.Duration
	RatePerSecond   float64
	Insecure        bool
	FollowRedirects bool
	MaxBodyBytes    int64
	ReportPath      string // empty means no report is written
}

func (c Config) wordlistLabel() string {
	if c.WordlistPath == "" {
		return builtinLabel
	}
	return c.WordlistPath
}

// Run performs one scan. A non-nil error means the scan could not run or a
// requested report could not be written; Outcome says how a scan that did
// run ended.
func Run(ctx context.Context, cfg Config) (Outcome, error) {
	words, err := loadWordlist(cfg.WordlistPath)
	if err != nil {
		return NotFound, fmt.Errorf("loading wordlist: %w", err)
	}

	client := netx.NewClient(netx.ClientConfig{
		Timeout:         cfg.Timeout,
		Insecure:        cfg.Insecure,
		FollowRedirects: cfg.FollowRedirects,
	})
	limiter := netx.NewRateLimiter(cfg.RatePerSecond)

	printer := ui.New()
	printer.Banner(cfg.Target.String(), cfg.wordlistLabel(), len(words.Paths), cfg.NumEngines)

	baseline, probeErr := establishBaseline(ctx, client, cfg.Target, cfg.MaxBodyBytes)
	if probeErr != nil {
		if ctx.Err() != nil {
			fmt.Println("scan interrupted before it started")
			return Interrupted, nil
		}
		return NotFound, fmt.Errorf("could not reach %s: %w\nthe target may not exist, be offline, or be blocking connections; nothing to scan", cfg.Target, probeErr)
	}

	engines := make([]*engine.Engine, cfg.NumEngines)
	for i := range engines {
		engines[i] = &engine.Engine{
			ID:      i + 1,
			Client:  client,
			Base:    cfg.Target,
			Limiter: limiter,
			MaxBody: cfg.MaxBodyBytes,
		}
	}

	start := time.Now()
	result := scheduler.Run(ctx, engines, words.Paths, baseline, printer.Progress)
	elapsed := time.Since(start)

	outcome := NotFound
	switch {
	case result.Discovery != nil:
		outcome = Found
	case result.CancelReason == scheduler.ReasonUserInterrupt:
		outcome = Interrupted
	}

	printer.Finish(result, elapsed)

	if cfg.ReportPath != "" {
		if err := writeReport(cfg, outcome, result, elapsed); err != nil {
			return outcome, err
		}
	}
	return outcome, nil
}

func loadWordlist(path string) (*wordlist.List, error) {
	if path == "" {
		return wordlist.Builtin()
	}
	return wordlist.Load(path)
}

// establishBaseline also doubles as the up-front reachability check:
// a non-nil error here means the connection itself failed (DNS,
// refused, timeout, TLS) before any HTTP response came back, which
// means the target is not reachable at all -- not that this one
// random path happens not to exist. The caller treats that as fatal
// and never starts the full scan.
func establishBaseline(ctx context.Context, client *http.Client, base *url.URL, maxBody int64) (classify.Baseline, error) {
	token := fmt.Sprintf("%s%d", probePrefix, rand.Int63())
	target := *base
	target.Path = urlutil.JoinPath(base.Path, "/"+token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return classify.Baseline{}, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return classify.Baseline{}, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBody)
	body, _ := io.ReadAll(limited)
	sum := sha256.Sum256(body)

	return classify.Baseline{
		Established: true,
		StatusCode:  resp.StatusCode,
		BodyHash:    hex.EncodeToString(sum[:]),
	}, nil
}

func writeReport(cfg Config, outcome Outcome, result *scheduler.Scan, elapsed time.Duration) error {
	rep := report.Report{
		Tool:       "administer",
		Version:    version.Version,
		Target:     cfg.Target.String(),
		Timestamp:  time.Now().UTC(),
		Wordlist:   cfg.wordlistLabel(),
		Engines:    cfg.NumEngines,
		Total:      result.Total,
		Completed:  result.Completed,
		Status:     outcome.String(),
		DurationMS: elapsed.Milliseconds(),
	}
	if result.Discovery != nil {
		rep.Found = true
		rep.FoundURL = result.Discovery.URL
		rep.RedirectTo = result.Discovery.Redirect
		rep.StatusCode = result.Discovery.StatusCode
	}
	return report.Write(cfg.ReportPath, rep)
}
