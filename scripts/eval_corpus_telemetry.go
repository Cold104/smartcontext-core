// Corpus + telemetry eval driver (token/role fixtures, latency, local JSONL).
// Usage (from go/): go run ./scripts/eval_corpus_telemetry.go
//
// Writes docs/CORPUS_TELEMETRY_EVAL.md + corpus_telemetry_eval.json.
//
//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cold104/smartcontext-core/eval"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fatalf("getcwd: %v", err)
	}
	goRoot := wd
	if filepath.Base(wd) != "go" {
		cand := filepath.Join(wd, "go")
		if st, e := os.Stat(cand); e == nil && st.IsDir() {
			goRoot = cand
		}
	}

	rep, err := eval.Run(eval.Options{
		GoRoot:       goRoot,
		WriteDocs:    true,
		TelemetryLog: filepath.Join(goRoot, "docs", "corpus_telemetry_run.jsonl"),
	})
	if err != nil {
		fatalf("eval: %v", err)
	}

	fmt.Printf("Wrote %s\n", filepath.Join(goRoot, "docs", "CORPUS_TELEMETRY_EVAL.md"))
	fmt.Printf("Wrote %s\n", filepath.Join(goRoot, "docs", "corpus_telemetry_eval.json"))
	fmt.Printf("token: hit_rate=%.2f recall=%.2f median_reduction=%.1f%% pack_p50=%.3fms gate=%v\n",
		rep.TokenSuite.HitRate, rep.TokenSuite.MeanRecallAtK, rep.TokenSuite.MedianReduction*100,
		rep.TokenSuite.PackMSP50, rep.TokenSuite.GateOK)
	fmt.Printf("role:  hit_rate=%.2f recall=%.2f median_reduction=%.1f%% pack_p50=%.3fms gate=%v\n",
		rep.RoleSuite.HitRate, rep.RoleSuite.MeanRecallAtK, rep.RoleSuite.MedianReduction*100,
		rep.RoleSuite.PackMSP50, rep.RoleSuite.GateOK)
	fmt.Printf("telemetry: pack_events=%d raw_leak=%v mcp_pack_p50=%.3fms\n",
		rep.Telemetry.PackEventsWritten, rep.Telemetry.RawTaskLeaked, rep.MCPPackMSP50)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
