package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Hexa-Ton/administer/internal/config"
	"github.com/Hexa-Ton/administer/internal/doctor"
	"github.com/Hexa-Ton/administer/internal/scan"
	"github.com/Hexa-Ton/administer/internal/ui"
	"github.com/Hexa-Ton/administer/internal/urlutil"
	"github.com/Hexa-Ton/administer/internal/version"
	"github.com/Hexa-Ton/administer/internal/wordlist"
)

// Exit codes. Scripts can rely on these.
const (
	exitFound       = 0   // a candidate was found
	exitError       = 1   // bad input, or the scan could not run
	exitNotFound    = 2   // the scan finished without a candidate
	exitInterrupted = 130 // the user pressed Ctrl+C
)

// Run executes the CLI and returns the process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		printHelp()
		return exitError
	}

	switch args[0] {
	case "-h", "--help", "help":
		printHelp()
		return 0
	case "version", "--version", "-v":
		printVersion()
		return 0
	case "scan":
		return runScan(args[1:])
	case "wordlist":
		return runWordlist(args[1:])
	case "config":
		return runConfig(args[1:])
	case "doctor":
		doctor.Run()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		printHelp()
		return exitError
	}
}

func printVersion() {
	fmt.Printf("%s v%s (%s, %s/%s)\n", ui.New().Brand(), version.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Println(ui.New().Brand() + " v" + version.Version + " — web path discovery")
	fmt.Print(`
USAGE
  administer -h | --help
  administer scan <target> [options]
  administer wordlist <info|validate|export> [path]
  administer config <show|set> [key value]
  administer doctor
  administer version

SCAN OPTIONS
  -u <url>              target URL (same as a bare positional target)
  -d <domain>           target domain (same as a bare positional target)
  -w <path>             custom wordlist (default: built-in 1,800-path list)
  -engines <n>          concurrent engine instances, 1-213 (default: saved config, else 20)
  -timeout <duration>   per-request timeout, e.g. 10s (default: saved config, else 10s)
  -rate <n>             max requests/sec, 0 = unlimited (default: saved config, else 0)
  -k                    skip TLS certificate verification (explicit, warned)
  -follow-redirects     follow redirects within the same host (max 10 hops)
  -o <path>             write a JSON report (default: no report; never overwrites)

CONFIG KEYS
  engines, timeout (seconds), rate, color (on|off)

EXIT CODES
  0 found   1 error   2 finished without a find   130 interrupted (Ctrl+C)

EXAMPLES
  administer scan example.com
  administer scan https://example.com -engines 50
  administer scan -u https://example.com -o report.json
  administer wordlist export > paths.txt
  administer config set engines 50

BEHAVIOR
  Missing scheme defaults to HTTPS.
  The mandatory first six paths are dispatched before the rest.
  The scan stops, and cancels in-flight requests, on the first
  response classified as a real candidate — shown in neon green.
  Redirects are not followed by default; TLS is verified by default.
  No credential attempts and no automatic retries are made.

This tool is for targets you own or are explicitly authorized to
assess. Do not use it against systems without permission.
`)
}

// splitTargetFromFlags pulls the one bare positional target out of a
// scan subcommand's args, wherever it appears, and returns the
// remaining tokens untouched and in order for flag.Parse. It knows
// which flags consume a following value so it never mistakes a
// flag's argument (e.g. the path after -w) for the target.
func splitTargetFromFlags(args []string) (target string, flagArgs []string) {
	valueFlags := map[string]bool{
		"-u": true, "-d": true, "-w": true, "-engines": true,
		"-timeout": true, "-rate": true, "-o": true,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			if target == "" {
				target = a
			}
			continue
		}
		flagArgs = append(flagArgs, a)
		name := a
		if eq := strings.Index(a, "="); eq != -1 {
			name = a[:eq]
		} else if valueFlags[name] && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return target, flagArgs
}

func runScan(args []string) int {
	target, flagArgs := splitTargetFromFlags(args)

	saved, cfgErr := config.Load()
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring unreadable config (%v); using built-in defaults\n", cfgErr)
	}

	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.Usage = printHelp
	u := fs.String("u", "", "target URL")
	d := fs.String("d", "", "target domain")
	w := fs.String("w", "", "custom wordlist path (default: built-in list)")
	engines := fs.Int("engines", saved.Engines, "concurrent engine instances (1-213)")
	timeout := fs.Duration("timeout", time.Duration(saved.Timeout)*time.Second, "per-request timeout")
	rate := fs.Float64("rate", saved.Rate, "max requests/sec, 0 = unlimited")
	insecure := fs.Bool("k", false, "skip TLS certificate verification")
	followRedirects := fs.Bool("follow-redirects", false, "follow redirects within the same host")
	output := fs.String("o", "", "write a JSON report to this path")

	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return exitError
	}

	if *u != "" {
		target = *u
	} else if *d != "" {
		target = *d
	}
	if target == "" {
		fmt.Fprintln(os.Stderr, "error: provide a target, e.g. administer scan example.com")
		return exitError
	}

	parsed, err := urlutil.Normalize(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if *engines < 1 || *engines > config.MaxEngines {
		fmt.Fprintf(os.Stderr, "error: -engines must be between 1 and %d\n", config.MaxEngines)
		return exitError
	}
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "error: -timeout must be greater than zero")
		return exitError
	}
	if *rate < 0 {
		fmt.Fprintln(os.Stderr, "error: -rate cannot be negative")
		return exitError
	}
	if *insecure {
		fmt.Fprintln(os.Stderr, "warning: TLS certificate verification is disabled (-k). Only use this against targets you trust.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := scan.Config{
		Target:          parsed,
		WordlistPath:    *w,
		NumEngines:      *engines,
		Timeout:         *timeout,
		RatePerSecond:   *rate,
		Insecure:        *insecure,
		FollowRedirects: *followRedirects,
		MaxBodyBytes:    5 * 1024 * 1024,
		ReportPath:      *output,
	}

	outcome, err := scan.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	switch outcome {
	case scan.Found:
		return exitFound
	case scan.Interrupted:
		return exitInterrupted
	default:
		return exitNotFound
	}
}

func runWordlist(args []string) int {
	if len(args) == 0 {
		fmt.Println("usage: administer wordlist <info|validate|export> [path]")
		return exitError
	}
	switch args[0] {
	case "info":
		list, err := wordlist.Builtin()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		fmt.Println("built-in wordlist: embedded in the binary")
		fmt.Printf("unique paths:      %d\n", len(list.Paths))
		if err := wordlist.VerifyMandatory(list.Paths); err != nil {
			fmt.Printf("mandatory six:     FAIL (%v)\n", err)
			return exitError
		}
		fmt.Printf("mandatory six:     OK (%s)\n", strings.Join(wordlist.MandatorySix, " "))
		return 0
	case "validate":
		if len(args) < 2 {
			fmt.Println("usage: administer wordlist validate <path>")
			return exitError
		}
		list, err := wordlist.Load(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid: %v\n", err)
			return exitError
		}
		fmt.Printf("ok: %d unique paths\n", len(list.Paths))
		return 0
	case "export":
		if len(args) < 2 {
			fmt.Print(wordlist.BuiltinText())
			return 0
		}
		return exportWordlist(args[1])
	default:
		fmt.Printf("unknown wordlist subcommand %q\n", args[0])
		return exitError
	}
}

// exportWordlist copies the built-in wordlist to dest. It refuses to
// overwrite an existing file (O_EXCL makes that check atomic).
func exportWordlist(dest string) int {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			fmt.Fprintf(os.Stderr, "error: %s already exists; refusing to overwrite\n", dest)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return exitError
	}
	if _, err := f.WriteString(wordlist.BuiltinText()); err != nil {
		f.Close()
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	fmt.Printf("exported the built-in wordlist to %s\n", dest)
	return 0
}

func runConfig(args []string) int {
	if len(args) == 0 || args[0] == "show" {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		fmt.Printf("engines: %d\ntimeout: %ds\nrate:    %.1f/s\ncolor:   %v\n", cfg.Engines, cfg.Timeout, cfg.Rate, cfg.Color)
		if path, pathErr := config.Path(); pathErr == nil {
			fmt.Printf("\nconfig file: %s\n", path)
		}
		return 0
	}
	if args[0] == "set" {
		if len(args) != 3 {
			fmt.Println("usage: administer config set <engines|timeout|rate|color> <value>")
			return exitError
		}
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		updated, err := cfg.Set(args[1], args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		if err := config.Save(updated); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		fmt.Printf("saved: %s = %s\n", args[1], args[2])
		return 0
	}
	fmt.Println("usage: administer config <show|set> [key value]")
	return exitError
}
