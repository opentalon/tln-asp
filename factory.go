package tlnasp

import (
	"context"
	"fmt"

	"github.com/opentalon/tln-language/pkg/tln"
)

// Factory builds a ToolResolver that solves ASP programs as tool calls, so
// tln-asp can be loaded by name via mod.tln + a connector (ADR 0012/0013):
//
//	connector "solver" via asp { }
//	tool "solver" "solve" { program "p :- not q.  q :- not p." }
//
// The "solve" tool parses `program`, enumerates its stable models, and returns
// one row per answer set — each a list of the atoms in that set, projected to
// facts (record_id / attribute / value) via [AnswerSet.Facts].
func Factory(spec tln.ConnectorSpec) (tln.ToolResolver, error) {
	return resolver{}, nil
}

// Factory satisfies tln.PluginFactory.
var _ tln.PluginFactory = Factory

type resolver struct{}

func (resolver) Call(ctx context.Context, server, tool string, args map[string]any) (any, error) {
	if tool != "solve" && tool != "stable-models" {
		return nil, fmt.Errorf("tln-asp: unknown tool %q on server %q (want \"solve\")", tool, server)
	}
	src, _ := args["program"].(string)
	if src == "" {
		return nil, fmt.Errorf("tln-asp: \"solve\" requires a \"program\" argument")
	}
	prog, err := Parse(src)
	if err != nil {
		return nil, err
	}
	sets, err := New().Solve(ctx, prog)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(sets))
	for i, s := range sets {
		facts := s.Facts()
		rows := make([]map[string]any, len(facts))
		for j, f := range facts {
			rows[j] = map[string]any{"record_id": f.RecordID, "attribute": f.Attribute, "value": f.Value}
		}
		out[i] = rows
	}
	return out, nil
}
