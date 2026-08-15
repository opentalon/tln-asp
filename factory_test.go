package tlnasp_test

import (
	"context"
	"testing"

	tlnasp "github.com/opentalon/tln-asp"
	"github.com/opentalon/tln-language/pkg/tln"
)

func TestFactory_SatisfiesPluginFactory(t *testing.T) {
	var _ tln.PluginFactory = tlnasp.Factory
}

// TestFactory_SolveReturnsModels runs an ASP program through the ToolResolver
// adapter and checks it returns one row per stable model.
func TestFactory_SolveReturnsModels(t *testing.T) {
	r, err := tlnasp.Factory(tln.ConnectorSpec{Name: "solver", Plugin: "asp"})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	res, err := r.Call(context.Background(), "solver", "solve", map[string]any{
		"program": `p :- not q.  q :- not p.`,
	})
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	models, ok := res.([]any)
	if !ok {
		t.Fatalf("result type = %T, want []any", res)
	}
	if len(models) != 2 {
		t.Fatalf("want 2 answer sets, got %d: %#v", len(models), models)
	}
}

func TestFactory_MissingProgramErrors(t *testing.T) {
	r, _ := tlnasp.Factory(tln.ConnectorSpec{Name: "solver"})
	if _, err := r.Call(context.Background(), "solver", "solve", nil); err == nil {
		t.Fatal("expected error when program is missing")
	}
}

func TestFactory_UnknownTool(t *testing.T) {
	r, _ := tlnasp.Factory(tln.ConnectorSpec{Name: "solver"})
	if _, err := r.Call(context.Background(), "solver", "explode", nil); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
