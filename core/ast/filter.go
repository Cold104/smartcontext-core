// Package ast is the WS10 Core AST filter surface.
// Tree-sitter grammar integration is optional; without a grammar the stub
// filter reports Proposed / skipped and pack proceeds unchanged.
package ast

// Status labels for docs and probe hops.
const (
	StatusProposed        = "Proposed"
	StatusSkippedProposed = "skipped_proposed"
	StatusStub            = "stub"
)

// Request is an optional source-envelope filter request (ignored by Core pack
// unless SMARTCONTEXT_AST_FILTER is enabled at the orchestration/MCP layer).
type Request struct {
	// Paths are source files to structurally filter (not skill bodies).
	Paths []string `json:"paths,omitempty"`
	// Language hint (e.g. go, python). Empty → stub skip.
	Language string `json:"language,omitempty"`
	// Query is a future structural query; stub ignores content.
	Query string `json:"query,omitempty"`
}

// Result is the AST hop outcome for route probes / pack meta.
type Result struct {
	Status    string   `json:"status"` // Proposed | skipped_proposed | stub
	Applied   bool     `json:"applied"`
	Grammar   bool     `json:"grammar_available"`
	Note      string   `json:"note"`
	KeptPaths []string `json:"kept_paths,omitempty"`
}

// Filter is the Core AST structural filter interface.
type Filter interface {
	// Name identifies the implementation (stub, tree-sitter, …).
	Name() string
	// GrammarAvailable is false until a real parser/grammar is linked.
	GrammarAvailable() bool
	// Apply filters source envelopes. Must not replace skill/rule packing.
	Apply(req Request) Result
}

// StubFilter is the default WS10 implementation: no tree-sitter dependency.
type StubFilter struct{}

func (StubFilter) Name() string { return "stub" }

func (StubFilter) GrammarAvailable() bool { return false }

// Apply returns skipped_proposed when no grammar; never mutates pack selection.
func (StubFilter) Apply(req Request) Result {
	if len(req.Paths) == 0 && req.Language == "" && req.Query == "" {
		return Result{
			Status:  StatusSkippedProposed,
			Applied: false,
			Grammar: false,
			Note:    "Core AST hop skipped (no request); pack path unchanged",
		}
	}
	return Result{
		Status:    StatusProposed,
		Applied:   false,
		Grammar:   false,
		Note:      "Core AST stub: tree-sitter grammar not linked; label Proposed until tests",
		KeptPaths: append([]string(nil), req.Paths...),
	}
}

// Default returns the stub filter used when AST is feature-flagged on.
func Default() Filter { return StubFilter{} }
