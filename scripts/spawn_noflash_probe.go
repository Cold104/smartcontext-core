// Probe: spawn smartcontext.exe mcp with CREATE_NO_WINDOW and assert no console HWND.
// Usage (from go/): go run ./scripts/spawn_noflash_probe.go
//
//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Cold104/smartcontext-core/winproc"
)

func main() {
	if runtime.GOOS != "windows" {
		fmt.Println("SKIP non-windows")
		return
	}
	wd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	exe := filepath.Join(wd, "smartcontext.exe")
	if _, err := os.Stat(exe); err != nil {
		fatal(fmt.Errorf("missing %s — build first", exe))
	}

	cmd := exec.Command(exe, "mcp")
	cmd.Dir = wd
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fatal(err)
	}
	cmd.Stderr = os.Stderr
	winproc.HideConsole(cmd)

	if err := cmd.Start(); err != nil {
		fatal(err)
	}
	pid := cmd.Process.Pid
	time.Sleep(400 * time.Millisecond)

	classes := winproc.WindowClasses(pid)
	hasConsole := winproc.HasConsoleWindow(pid)

	_ = stdin.Close()
	_ = stdout.Close()
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	fmt.Printf("pid=%d window_classes=%v has_console_window=%v\n", pid, classes, hasConsole)
	if hasConsole {
		fmt.Fprintln(os.Stderr, "FAIL: MCP child has a visible console window")
		os.Exit(1)
	}
	fmt.Println("NOFLASH_OK")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
