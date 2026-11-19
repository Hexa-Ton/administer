package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/YOUR_GITHUB_USERNAME/administer/internal/config"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/engine"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/scheduler"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/version"
)

const (
	colorReset  = "\x1b[0m"
	colorGreen  = "\x1b[92m"
	colorCyan   = "\x1b[96m"
	colorRed    = "\x1b[91m"
	colorYellow = "\x1b[1;93m" // bold bright yellow, the brand color
)

// Printer writes terminal output, with or without ANSI colors.
type Printer struct {
	color bool
}

// New decides whether to use color. NO_COLOR always wins, then the saved
// `color` setting, then a TTY check using only the standard library:
// piped or redirected output falls back to plain text automatically.
func New() *Printer {
	if os.Getenv("NO_COLOR") != "" {
		return &Printer{color: false}
	}
	if saved, err := config.Load(); err == nil && !saved.Color {
		return &Printer{color: false}
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return &Printer{color: false}
	}
	return &Printer{color: (info.Mode() & os.ModeCharDevice) != 0}
}

func (p *Printer) colorize(code, s string) string {
	if !p.color {
		return s
	}
	return code + s + colorReset
}

// Brand returns "administer" in the tool's signature bold yellow,
// respecting the same NO_COLOR/TTY rules as everything else.
func (p *Printer) Brand() string {
	return p.colorize(colorYellow, "administer")
}

// Banner prints the scan header.
func (p *Printer) Banner(target, wordlistLabel string, total, engines int) {
	fmt.Println(p.Brand() + " v" + version.Version)
	fmt.Printf("target:   %s\n", target)
	fmt.Printf("wordlist: %s (%d paths)\n", wordlistLabel, total)
	fmt.Printf("engines:  %d\n", engines)
	fmt.Println(p.colorize(colorCyan, "scanning..."))
}

// Progress prints one live line per checked path.
func (p *Printer) Progress(r engine.Result) {
	if r.Err != nil {
		if errors.Is(r.Err, context.Canceled) {
			// Not a real error: this engine was mid-request when the scan
			// stopped (first hit or Ctrl+C) and everything still in flight
			// was cancelled. Nothing went wrong, so plain text, no red.
			fmt.Printf("[engine %03d] %-30s -> cancelled (scan already stopped)\n", r.EngineID, r.Path)
			return
		}
		fmt.Println(p.colorize(colorRed, fmt.Sprintf("[engine %03d] %-30s -> error: %v", r.EngineID, r.Path, r.Err)))
		return
	}
	if r.Redirect != "" {
		fmt.Printf("[engine %03d] %-30s -> %d -> %s (%s)\n", r.EngineID, r.Path, r.StatusCode, r.Redirect, r.Duration.Round(time.Millisecond))
		return
	}
	fmt.Printf("[engine %03d] %-30s -> %d (%s)\n", r.EngineID, r.Path, r.StatusCode, r.Duration.Round(time.Millisecond))
}

// Finish prints the final result. A discovery is shown as one clean path in
// neon green; an interrupted scan is never reported as "nothing found".
func (p *Printer) Finish(scan *scheduler.Scan, elapsed time.Duration) {
	fmt.Println()
	switch {
	case scan.Discovery != nil:
		// If the first hit was a redirect, the real page is the
		// destination (e.g. /admin/login), not the stub it was requested
		// through (e.g. /admin). The live line above already showed the
		// full redirect, so nothing is lost by showing one path here.
		shown := scan.Discovery.URL
		if scan.Discovery.Redirect != "" {
			shown = scan.Discovery.Redirect
		}
		fmt.Println(p.colorize(colorGreen, "[FOUND] "+shown))
	case scan.CancelReason == scheduler.ReasonUserInterrupt:
		fmt.Println(p.colorize(colorCyan, "scan interrupted before it finished; no candidate found so far"))
	default:
		fmt.Println(p.colorize(colorCyan, "no candidate found"))
	}
	fmt.Printf("checked %d/%d paths in %s\n", scan.Completed, scan.Total, elapsed.Round(time.Millisecond))
}
