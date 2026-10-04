// Command smartcontext is the singular SmartContext v1.1 binary (MCP + lifecycle + core).
package main

import (
	"context"
	"fmt"
	"os"

	scmcp "github.com/Cold104/smartcontext-core/mcp"
	"github.com/Cold104/smartcontext-core/version"
	"github.com/Cold104/smartcontext-core/winproc"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "mcp":
		// Drop any attached console so GUI hosts never leave a visible window.
		// Stdio MCP uses parent pipes; FreeConsole does not close those handles.
		winproc.DetachConsole()
		if err := scmcp.RunStdio(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "smartcontext mcp: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-V":
		fmt.Printf("smartcontext %s\n", version.Version)
	case "help", "-h", "--help":
		printUsage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w *os.File) {
	fmt.Fprintf(w, `smartcontext %s — singular Go binary (MCP primary)

Usage:
  smartcontext mcp       Run stdio MCP server (primary entry)
  smartcontext version   Print version
  smartcontext help      Show this help

Python CLI-first pack remains available in the legacy package (0.2.x);
this binary is the v1.1 development train.
`, version.Version)
}
