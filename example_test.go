package tlnasp_test

import (
	"context"
	"fmt"
	"testing"

	tlnasp "github.com/opentalon/tln-asp"
	fs "github.com/opentalon/tln-language/pkg/factstore"
)

// ExampleGoSolver_Solve shows the two-answer-set program end to end.
func ExampleGoSolver_Solve() {
	prog := tlnasp.Program{Rules: []fs.Rule{
		{Name: "p", Body: []fs.Clause{&fs.Negation{Name: "q"}}},
		{Name: "q", Body: []fs.Clause{&fs.Negation{Name: "p"}}},
	}}
	sets, _ := tlnasp.New().Solve(context.Background(), prog)
	fmt.Println(len(sets), "answer sets")
	// Output: 2 answer sets
}

// TestAnswerSetsFeedBackAsFacts is the boundary in action: solve a program,
// convert each answer set to EAV facts, and assert them into a tln
// factstore.MemoryStore — answers flowing back into the store the host queries.
func TestAnswerSetsFeedBackAsFacts(t *testing.T) {
	// node 1 -> node 2 (2 terminal) — one model, {win(1)}. Numeric node ids so
	// the in-process MemoryStore (which keys entities by integer id) accepts the
	// answer-set facts; the Facts() boundary is store-agnostic, so a document
	// backend like tln-db takes string ids just the same.
	prog := tlnasp.Program{Rules: winRules(), Facts: edges([2]string{"1", "2"})}
	sets := solve(t, prog)
	if len(sets) != 1 {
		t.Fatalf("want 1 model, got %d", len(sets))
	}

	store := fs.NewMemoryStore()
	if err := store.Assert(context.Background(), sets[0].Facts()); err != nil {
		t.Fatalf("Assert answer-set facts: %v", err)
	}

	// Query the store for the winning positions the solver found.
	rows, err := store.Query(context.Background(), fs.Query{
		Find: []string{"?e"},
		Where: []fs.Clause{
			&fs.Pattern{Entity: fs.Var("e"), Attribute: ":asp/win", Value: fs.Lit(true)},
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 || fmt.Sprintf("%v", rows[0][0]) != "1" {
		t.Fatalf("want winning position 1 in the store, got %v", rows)
	}
}
