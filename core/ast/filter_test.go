package ast_test

import (
	"testing"

	"github.com/Cold104/smartcontext-core/core/ast"
)

func TestStubFilterNoGrammar(t *testing.T) {
	f := ast.Default()
	if f.GrammarAvailable() {
		t.Fatal("stub must not claim grammar")
	}
	r := f.Apply(ast.Request{})
	if r.Status != ast.StatusSkippedProposed {
		t.Fatalf("status=%s want %s", r.Status, ast.StatusSkippedProposed)
	}
	if r.Applied {
		t.Fatal("empty request must not apply")
	}
}

func TestStubFilterProposedWithoutGrammar(t *testing.T) {
	r := ast.StubFilter{}.Apply(ast.Request{
		Paths:    []string{"main.go"},
		Language: "go",
		Query:    "func",
	})
	if r.Status != ast.StatusProposed {
		t.Fatalf("status=%s want %s (no grammar → Proposed, tests skipped for real filter)", r.Status, ast.StatusProposed)
	}
	if r.Grammar {
		t.Fatal("grammar_available must be false")
	}
	if r.Applied {
		t.Fatal("stub must not apply structural filter")
	}
	if len(r.KeptPaths) != 1 || r.KeptPaths[0] != "main.go" {
		t.Fatalf("kept_paths=%v", r.KeptPaths)
	}
}

func TestRealGrammarFilterSkipped(t *testing.T) {
	// Tree-sitter not linked in this POC; keep a skip marker for future grammar tests.
	if ast.Default().GrammarAvailable() {
		t.Fatal("unexpected grammar")
	}
	t.Skip("Core AST grammar tests skipped: no tree-sitter grammar linked (label Proposed)")
}
