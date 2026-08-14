package tlnasp

import (
	"fmt"
	"sort"

	"github.com/opentalon/tln-language/pkg/factstore"
)

// groundRule is one variable-free instance of a Rule: a head atom that holds
// when every positive dependency holds and no negative dependency does. Atoms
// are canonical string keys (see atomKey).
type groundRule struct {
	head string
	pos  []string
	neg  []string
}

// grounding is the ground program plus a key→atom map so answer sets can be
// reconstructed with names and argument values.
type grounding struct {
	rules []groundRule
	atoms map[string]Atom
}

// ground instantiates every rule against the program's facts. It mirrors the
// core well-founded grounder (internal/factstore/wellfounded.go): Pattern and
// `=`/comparison predicates are the binding generators; RuleCall clauses become
// positive dependencies and Negation clauses negative ones. Range restriction
// is enforced — an instance whose head or dependency args aren't fully bound is
// dropped.
func ground(p Program) *grounding {
	byAttr := map[string][]factstore.Fact{}
	for _, f := range p.Facts {
		byAttr[f.Attribute] = append(byAttr[f.Attribute], f)
	}
	g := &grounding{atoms: map[string]Atom{}}
	for _, r := range p.Rules {
		groundOne(r, byAttr, g)
	}
	return g
}

func groundOne(r factstore.Rule, byAttr map[string][]factstore.Fact, g *grounding) {
	var gens []factstore.Clause
	var pos []*factstore.RuleCall
	var neg []*factstore.Negation
	for _, c := range r.Body {
		switch cc := c.(type) {
		case *factstore.Pattern:
			gens = append(gens, cc)
		case *factstore.Predicate:
			gens = append(gens, cc)
		case *factstore.RuleCall:
			pos = append(pos, cc)
		case *factstore.Negation:
			neg = append(neg, cc)
		}
	}

	enumGen(gens, 0, byAttr, map[string]any{}, func(b map[string]any) {
		headArgs, ok := groundVars(r.Args, b)
		if !ok {
			return
		}
		headKey := atomKey(r.Name, headArgs)
		if _, seen := g.atoms[headKey]; !seen {
			g.atoms[headKey] = Atom{Name: r.Name, Args: headArgs}
		}
		gr := groundRule{head: headKey}
		for _, pc := range pos {
			a, ok := groundTerms(pc.Args, b)
			if !ok {
				return
			}
			gr.pos = append(gr.pos, atomKey(pc.Name, a))
		}
		for _, nc := range neg {
			a, ok := groundTerms(nc.Args, b)
			if !ok {
				return
			}
			gr.neg = append(gr.neg, atomKey(nc.Name, a))
		}
		g.rules = append(g.rules, gr)
	})
}

// enumGen walks the binding-generator clauses (Pattern, Predicate) against the
// facts, yielding one binding map per solution.
func enumGen(gens []factstore.Clause, i int, byAttr map[string][]factstore.Fact, b map[string]any, yield func(map[string]any)) {
	if i == len(gens) {
		yield(b)
		return
	}
	switch c := gens[i].(type) {
	case *factstore.Pattern:
		for _, f := range byAttr[c.Attribute] {
			next := cloneBindings(b)
			if !unify(c.Entity, f.RecordID, next) {
				continue
			}
			if !unify(c.Value, f.Value, next) {
				continue
			}
			enumGen(gens, i+1, byAttr, next, yield)
		}
	case *factstore.Predicate:
		l := resolveTerm(c.Left, b)
		r := resolveTerm(c.Right, b)
		switch c.Op {
		case "=", "==":
			switch {
			case l != nil && r != nil:
				if equalValues(l, r) {
					enumGen(gens, i+1, byAttr, b, yield)
				}
			case c.Left.IsVar() && r != nil:
				next := cloneBindings(b)
				next[c.Left.Var] = r
				enumGen(gens, i+1, byAttr, next, yield)
			case c.Right.IsVar() && l != nil:
				next := cloneBindings(b)
				next[c.Right.Var] = l
				enumGen(gens, i+1, byAttr, next, yield)
			}
		default:
			// A comparison (<, <=, >, >=, !=) is a filter: both sides must
			// already be bound.
			if l != nil && r != nil && compareOp(c.Op, l, r) {
				enumGen(gens, i+1, byAttr, b, yield)
			}
		}
	}
}

func groundVars(vars []string, b map[string]any) ([]any, bool) {
	out := make([]any, len(vars))
	for i, v := range vars {
		val, ok := b[v]
		if !ok {
			return nil, false
		}
		out[i] = val
	}
	return out, true
}

func groundTerms(terms []factstore.Term, b map[string]any) ([]any, bool) {
	out := make([]any, len(terms))
	for i, t := range terms {
		switch {
		case t.IsWildcard():
			return nil, false
		case t.IsVar():
			val, ok := b[t.Var]
			if !ok {
				return nil, false
			}
			out[i] = val
		default:
			out[i] = t.Literal
		}
	}
	return out, true
}

func unify(t factstore.Term, val any, b map[string]any) bool {
	switch {
	case t.IsWildcard():
		return true
	case t.IsVar():
		if existing, bound := b[t.Var]; bound {
			return equalValues(existing, val)
		}
		b[t.Var] = val
		return true
	default:
		return equalValues(t.Literal, val)
	}
}

func resolveTerm(t factstore.Term, b map[string]any) any {
	if t.IsVar() {
		return b[t.Var] // nil if unbound
	}
	return t.Literal
}

func cloneBindings(b map[string]any) map[string]any {
	out := make(map[string]any, len(b))
	for k, v := range b {
		out[k] = v
	}
	return out
}

func equalValues(a, b any) bool { return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b) }

func compareOp(op string, l, r any) bool {
	if op == "!=" {
		return !equalValues(l, r)
	}
	lf, lok := toFloat(l)
	rf, rok := toFloat(r)
	if !lok || !rok {
		return false
	}
	switch op {
	case "<":
		return lf < rf
	case "<=":
		return lf <= rf
	case ">":
		return lf > rf
	case ">=":
		return lf >= rf
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// atomKey is the canonical string identity of a ground atom.
func atomKey(name string, args []any) string {
	key := name
	for _, a := range args {
		key += fmt.Sprintf("|%v", a)
	}
	return key
}

// sortedKeys returns the map keys sorted, for deterministic iteration.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
