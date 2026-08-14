package tlnasp_test

import (
	"context"
	"testing"

	tlnasp "github.com/opentalon/tln-asp"
	fs "github.com/opentalon/tln-language/pkg/factstore"
)

func solve(t *testing.T, p tlnasp.Program) []tlnasp.AnswerSet {
	t.Helper()
	sets, err := tlnasp.New().Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	return sets
}

// The textbook two-answer-set program: p :- not q.  q :- not p.
func TestTwoAnswerSets(t *testing.T) {
	p := tlnasp.Program{Rules: []fs.Rule{
		{Name: "p", Body: []fs.Clause{&fs.Negation{Name: "q"}}},
		{Name: "q", Body: []fs.Clause{&fs.Negation{Name: "p"}}},
	}}
	sets := solve(t, p)
	if len(sets) != 2 {
		t.Fatalf("want 2 answer sets, got %d: %+v", len(sets), sets)
	}
	// One is {p}, the other {q} — mutually exclusive.
	got := map[bool]bool{} // key: has p
	for _, s := range sets {
		hp, hq := s.Has("p"), s.Has("q")
		if hp == hq {
			t.Fatalf("each model must have exactly one of p/q, got p=%v q=%v", hp, hq)
		}
		got[hp] = true
	}
	if !got[true] || !got[false] {
		t.Fatalf("expected both {p} and {q}")
	}
}

// win(X) :- move(X,Y), not win(Y). A 2-cycle a<->b has TWO stable models
// ({win(a)} and {win(b)}) — where the well-founded model leaves both undefined.
// This is the stable-vs-well-founded difference in one test.
func winRules() []fs.Rule {
	return []fs.Rule{{
		Name: "win",
		Args: []string{"?x"},
		Body: []fs.Clause{
			&fs.Pattern{Entity: fs.Var("e"), Attribute: ":edge/from", Value: fs.Var("x")},
			&fs.Pattern{Entity: fs.Var("e"), Attribute: ":edge/to", Value: fs.Var("y")},
			&fs.Negation{Name: "win", Args: []fs.Term{fs.Var("y")}},
		},
	}}
}

func edges(pairs ...[2]string) []fs.Fact {
	var f []fs.Fact
	for i, e := range pairs {
		id := string(rune('a'+i)) + "-edge"
		f = append(f,
			fs.Fact{RecordID: id, Attribute: ":edge/from", Value: e[0]},
			fs.Fact{RecordID: id, Attribute: ":edge/to", Value: e[1]},
		)
	}
	return f
}

func TestWinMoveTwoCycle(t *testing.T) {
	sets := solve(t, tlnasp.Program{Rules: winRules(), Facts: edges([2]string{"a", "b"}, [2]string{"b", "a"})})
	if len(sets) != 2 {
		t.Fatalf("2-cycle: want 2 stable models, got %d: %+v", len(sets), sets)
	}
	seen := map[bool]bool{}
	for _, s := range sets {
		wa, wb := s.Has("win", "a"), s.Has("win", "b")
		if wa == wb {
			t.Fatalf("each draw model picks exactly one winner, got win(a)=%v win(b)=%v", wa, wb)
		}
		seen[wa] = true
	}
	if !seen[true] || !seen[false] {
		t.Fatalf("expected both {win(a)} and {win(b)}")
	}
}

func TestWinMoveTerminal(t *testing.T) {
	// a -> b, b terminal: exactly one model, {win(a)} (b loses).
	sets := solve(t, tlnasp.Program{Rules: winRules(), Facts: edges([2]string{"a", "b"})})
	if len(sets) != 1 {
		t.Fatalf("terminal: want 1 model, got %d: %+v", len(sets), sets)
	}
	if !sets[0].Has("win", "a") || sets[0].Has("win", "b") {
		t.Fatalf("want {win(a)}, got %+v", sets[0])
	}
}

// p :- not p has NO stable model (odd loop through negation).
func TestUnsatisfiable(t *testing.T) {
	sets := solve(t, tlnasp.Program{Rules: []fs.Rule{
		{Name: "p", Body: []fs.Clause{&fs.Negation{Name: "p"}}},
	}})
	if len(sets) != 0 {
		t.Fatalf("want 0 stable models, got %d: %+v", len(sets), sets)
	}
}

// The empty program has exactly one stable model: the empty set.
func TestEmptyProgram(t *testing.T) {
	sets := solve(t, tlnasp.Program{})
	if len(sets) != 1 || len(sets[0].Atoms) != 0 {
		t.Fatalf("want one empty model, got %+v", sets)
	}
}

// A stratified program has a single stable model (agrees with well-founded).
// blocked(x) :- item(x), not exempt(x);  exempt bound from facts.
func TestStratifiedSingleModel(t *testing.T) {
	facts := []fs.Fact{
		{RecordID: "1", Attribute: ":item", Value: true},
		{RecordID: "1", Attribute: ":exempt", Value: true},
		{RecordID: "2", Attribute: ":item", Value: true},
	}
	rules := []fs.Rule{
		{Name: "exempt", Args: []string{"?x"}, Body: []fs.Clause{
			&fs.Pattern{Entity: fs.Var("x"), Attribute: ":exempt", Value: fs.Lit(true)},
		}},
		{Name: "blocked", Args: []string{"?x"}, Body: []fs.Clause{
			&fs.Pattern{Entity: fs.Var("x"), Attribute: ":item", Value: fs.Lit(true)},
			&fs.Negation{Name: "exempt", Args: []fs.Term{fs.Var("x")}},
		}},
	}
	sets := solve(t, tlnasp.Program{Rules: rules, Facts: facts})
	if len(sets) != 1 {
		t.Fatalf("stratified: want 1 model, got %d: %+v", len(sets), sets)
	}
	s := sets[0]
	if !s.Has("blocked", "2") || s.Has("blocked", "1") || !s.Has("exempt", "1") {
		t.Fatalf("want exempt(1), blocked(2), not blocked(1); got %+v", s)
	}
}

func TestMaxModels(t *testing.T) {
	p := tlnasp.Program{Rules: []fs.Rule{
		{Name: "p", Body: []fs.Clause{&fs.Negation{Name: "q"}}},
		{Name: "q", Body: []fs.Clause{&fs.Negation{Name: "p"}}},
	}}
	sets, err := tlnasp.New(tlnasp.WithMaxModels(1)).Solve(context.Background(), p)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("WithMaxModels(1): want 1, got %d", len(sets))
	}
}
