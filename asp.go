// Package tlnasp is the Answer Set Programming (stable-model) plugin for tln —
// "tln as a new Prolog/ASP front-end."
//
// tln core is deterministic: its well-founded resolver yields a single
// three-valued model. Stable-model semantics is different — a rule set has
// ZERO or MANY answer sets (`p :- not q. q :- not p.` has two) — and is the
// classic win for combinatorial search, planning, and configuration. That
// non-determinism is deliberately kept out of core; this plugin owns it.
//
// A Program is expressed with the public github.com/opentalon/tln-language/pkg/factstore
// rule types (Rule bodies of Pattern/Predicate/RuleCall/Negation, plus Fact for
// the EDB) — no new DSL. The solver is pure Go: it grounds the rules and
// enumerates stable models via the Gelfond-Lifschitz reduct. No clingo, no cgo.
package tlnasp

import (
	"context"

	"github.com/opentalon/tln-language/pkg/factstore"
)

// Program is a normal logic program: a rule set plus the EDB (facts the rules
// are grounded against). Rules and Facts use the public tln factstore types.
type Program struct {
	Rules []factstore.Rule
	Facts []factstore.Fact
}

// Atom is a ground atom in an answer set: a predicate name and its arguments.
type Atom struct {
	Name string
	Args []any
}

// AnswerSet is one stable model — the set of atoms true in it. Atoms are sorted
// for deterministic output.
type AnswerSet struct {
	Atoms []Atom
}

// Solver computes the stable models of a Program. The interface leaves room for
// a future clingo-backed solver without changing callers.
type Solver interface {
	Solve(ctx context.Context, p Program) ([]AnswerSet, error)
}

// GoSolver is the pure-Go stable-model solver.
type GoSolver struct {
	maxModels int // 0 = all
}

// Option configures a GoSolver.
type Option func(*GoSolver)

// WithMaxModels caps how many answer sets to return (0 = all). Useful because
// the number of stable models can be exponential.
func WithMaxModels(n int) Option { return func(s *GoSolver) { s.maxModels = n } }

// New builds a pure-Go solver.
func New(opts ...Option) *GoSolver {
	s := &GoSolver{}
	for _, o := range opts {
		o(s)
	}
	return s
}

var _ Solver = (*GoSolver)(nil)

// Solve grounds the program and returns its stable models. Answer sets are
// returned in a deterministic order.
func (s *GoSolver) Solve(ctx context.Context, p Program) ([]AnswerSet, error) {
	g := ground(p)
	return solveStable(ctx, g, s.maxModels)
}
