package tlnasp

import (
	"context"
	"testing"
)

// TestParse_TwoAnswerSets: the classic even negative loop has exactly two
// stable models, {p} and {q}.
func TestParse_TwoAnswerSets(t *testing.T) {
	prog, err := Parse(`p :- not q.  q :- not p.`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sets, err := New().Solve(context.Background(), prog)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if len(sets) != 2 {
		t.Fatalf("want 2 answer sets, got %d: %+v", len(sets), sets)
	}
}

// TestParse_WinGame: a → b → c (c terminal). b wins (moves to a loss), a loses
// (its only move is to a winning b), c loses (no moves). One stable model.
func TestParse_WinGame(t *testing.T) {
	prog, err := Parse(`
		move(a, b).
		move(b, c).
		win(X) :- move(X, Y), not win(Y).`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sets, err := New().Solve(context.Background(), prog)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("want 1 model, got %d: %+v", len(sets), sets)
	}
	if !sets[0].Has("win", "b") {
		t.Errorf("win(b) should hold (b moves to terminal c)")
	}
	if sets[0].Has("win", "a") {
		t.Errorf("win(a) should NOT hold (a only moves to winning b)")
	}
	if sets[0].Has("win", "c") {
		t.Errorf("win(c) should NOT hold (c is terminal)")
	}
}

// TestParse_Facts checks a plain EDB program yields the reified facts.
func TestParse_Facts(t *testing.T) {
	prog, err := Parse(`edge(1, 2). edge(2, 3).`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Rules) != 0 {
		t.Errorf("no rules expected, got %d", len(prog.Rules))
	}
	// two binary facts → four EAV cells
	if len(prog.Facts) != 4 {
		t.Fatalf("want 4 reified cells, got %d: %+v", len(prog.Facts), prog.Facts)
	}
}
