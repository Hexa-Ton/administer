package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/YOUR_GITHUB_USERNAME/administer/internal/config"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/ui"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/version"
	"github.com/YOUR_GITHUB_USERNAME/administer/internal/wordlist"
)

// expectedBuiltinPaths is the size the built-in wordlist must have.
const expectedBuiltinPaths = 1800

// Run prints a diagnostic report about this installation.
func Run() {
	fmt.Println(ui.New().Brand() + " v" + version.Version + " doctor")
	fmt.Println()

	fmt.Printf("go runtime:     %s\n", runtime.Version())
	fmt.Printf("os/arch:        %s/%s\n", runtime.GOOS, runtime.GOARCH)

	if path, err := exec.LookPath("python3"); err == nil {
		fmt.Printf("python3:        found (%s) - used only by the optional helper scripts in python/\n", path)
	} else {
		fmt.Println("python3:        not found - only needed for the optional helper scripts in python/; scan and every built-in command work without it")
	}

	list, err := wordlist.Builtin()
	if err != nil {
		fmt.Printf("wordlist:       FAIL (%v)\n", err)
	} else {
		status := "OK"
		if len(list.Paths) != expectedBuiltinPaths {
			status = fmt.Sprintf("FAIL (expected %d)", expectedBuiltinPaths)
		}
		fmt.Printf("wordlist:       built-in, %d unique paths - %s\n", len(list.Paths), status)
		sum := sha256.Sum256([]byte(wordlist.BuiltinText()))
		fmt.Printf("wordlist hash:  sha256:%s\n", hex.EncodeToString(sum[:]))
		if verr := wordlist.VerifyMandatory(list.Paths); verr != nil {
			fmt.Printf("mandatory six:  FAIL (%v)\n", verr)
		} else {
			fmt.Println("mandatory six:  OK")
		}
	}

	cfgPath, pathErr := config.Path()
	if pathErr != nil {
		fmt.Printf("config:         FAIL (%v)\n", pathErr)
		return
	}
	if _, statErr := os.Stat(cfgPath); statErr != nil {
		fmt.Println("config:         not created (defaults in use)")
		return
	}
	if _, loadErr := config.Load(); loadErr != nil {
		fmt.Printf("config:         INVALID (%v)\n", loadErr)
		return
	}
	fmt.Printf("config:         %s (valid)\n", cfgPath)
}
