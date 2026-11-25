package eval_test

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points SMARTCONTEXT_HOME at a throwaway dir so tests that do not
// set it themselves never write sticky/telemetry/session state into the
// developer's real ~/.smartcontext. Tests may still t.Setenv their own home.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "smartcontext-test-home-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: mkdtemp:", err)
		os.Exit(1)
	}
	_ = os.Setenv("SMARTCONTEXT_HOME", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
