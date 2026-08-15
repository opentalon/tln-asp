package tlnasp

import (
	"fmt"
	"strconv"
	"strings"

	fs "github.com/opentalon/tln-language/pkg/factstore"
)

// Parse reads a small ASP / Datalog text into a [Program] the solver can ground.
// It supports ground facts and rules with positive and `not`-negated body
// literals:
//
//	p :- not q.  q :- not p.                 // the two-answer-set classic
//	move(a, b).  move(b, c).                 // EDB facts
//	win(X) :- move(X, Y), not win(Y).        // a rule
//
// Predicates that appear only as facts are the EDB; each is reified to EAV
// facts (`:name#i` attributes) and its body literals become the Pattern joins
// the grounder consumes. Predicates that appear as rule heads are the IDB;
// their positive body literals become RuleCall, negated ones Negation. Every
// variable must be bound by an EDB literal (the solver's range restriction).
func Parse(src string) (Program, error) {
	raw, err := splitClauses(src)
	if err != nil {
		return Program{}, err
	}

	type clause struct {
		head literal
		body []literal
		rule bool
	}
	var clauses []clause
	idb := map[string]bool{}
	for _, rc := range raw {
		c := clause{}
		if idx := strings.Index(rc, ":-"); idx >= 0 {
			c.rule = true
			h, err := parseLiteral(rc[:idx])
			if err != nil {
				return Program{}, err
			}
			c.head = h
			for _, bl := range splitTopComma(rc[idx+2:]) {
				lit, err := parseLiteral(bl)
				if err != nil {
					return Program{}, err
				}
				c.body = append(c.body, lit)
			}
			idb[c.head.name] = true
		} else {
			h, err := parseLiteral(rc)
			if err != nil {
				return Program{}, err
			}
			c.head = h
		}
		clauses = append(clauses, c)
	}

	prog := Program{}
	ent := 0
	for _, c := range clauses {
		if !c.rule {
			prog.Facts = append(prog.Facts, reifyFact(c.head)...)
			continue
		}
		r := fs.Rule{Name: c.head.name}
		var body []fs.Clause
		for _, a := range c.head.args {
			if isVar(a) {
				r.Args = append(r.Args, "?"+a)
			} else {
				fresh := fmt.Sprintf("?_h%d", len(r.Args))
				r.Args = append(r.Args, fresh)
				body = append(body, &fs.Predicate{Op: "=", Left: fs.Var(fresh), Right: litTerm(a)})
			}
		}
		for _, lit := range c.body {
			switch {
			case lit.neg:
				body = append(body, &fs.Negation{Name: lit.name, Args: argTerms(lit.args)})
			case idb[lit.name]:
				body = append(body, &fs.RuleCall{Name: lit.name, Args: argTerms(lit.args)})
			default:
				e := fmt.Sprintf("?_e%d", ent)
				ent++
				body = append(body, edbPatterns(e, lit)...)
			}
		}
		r.Body = body
		prog.Rules = append(prog.Rules, r)
	}
	return prog, nil
}

type literal struct {
	neg  bool
	name string
	args []string
}

func splitClauses(src string) ([]string, error) {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.IndexByte(line, '%'); i >= 0 {
			line = line[:i] // strip % comment
		}
		b.WriteString(line)
		b.WriteByte(' ')
	}
	var out []string
	for _, part := range strings.Split(b.String(), ".") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

func parseLiteral(s string) (literal, error) {
	s = strings.TrimSpace(s)
	var lit literal
	if s == "not" || strings.HasPrefix(s, "not ") || strings.HasPrefix(s, "not(") {
		lit.neg = true
		s = strings.TrimSpace(s[3:])
	}
	if i := strings.IndexByte(s, '('); i >= 0 {
		if !strings.HasSuffix(s, ")") {
			return lit, fmt.Errorf("tln-asp: malformed literal %q", s)
		}
		lit.name = strings.TrimSpace(s[:i])
		for _, a := range splitTopComma(s[i+1 : len(s)-1]) {
			lit.args = append(lit.args, strings.TrimSpace(a))
		}
	} else {
		lit.name = s
	}
	if lit.name == "" {
		return lit, fmt.Errorf("tln-asp: empty predicate in %q", s)
	}
	return lit, nil
}

// splitTopComma splits on commas that are not inside parentheses.
func splitTopComma(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(s[start:]) != "" {
		out = append(out, s[start:])
	}
	return out
}

func isVar(s string) bool {
	return s != "" && (s[0] == '_' || (s[0] >= 'A' && s[0] <= 'Z'))
}

func litTerm(a string) fs.Term {
	if isVar(a) {
		return fs.Var(a)
	}
	if n, err := strconv.Atoi(a); err == nil {
		return fs.Lit(n)
	}
	return fs.Lit(a)
}

func argTerms(args []string) []fs.Term {
	out := make([]fs.Term, len(args))
	for i, a := range args {
		out[i] = litTerm(a)
	}
	return out
}

func constVal(a string) any {
	if n, err := strconv.Atoi(a); err == nil {
		return n
	}
	return a
}

func attrOf(name string, i int) string { return ":" + name + "#" + strconv.Itoa(i) }

// reifyFact encodes a ground EDB fact as EAV: one cell per argument on a shared
// entity id, plus a marker cell for a nullary predicate.
func reifyFact(h literal) []fs.Fact {
	if len(h.args) == 0 {
		return []fs.Fact{{RecordID: h.name, Attribute: ":" + h.name + "#h", Value: true}}
	}
	key := h.name
	for _, a := range h.args {
		key += "|" + a
	}
	out := make([]fs.Fact, len(h.args))
	for i, a := range h.args {
		out[i] = fs.Fact{RecordID: key, Attribute: attrOf(h.name, i), Value: constVal(a)}
	}
	return out
}

// edbPatterns turns a body EDB literal into the Pattern joins the grounder
// binds against — all sharing one entity var so the cells come from one fact.
func edbPatterns(ent string, lit literal) []fs.Clause {
	if len(lit.args) == 0 {
		return []fs.Clause{&fs.Pattern{Entity: fs.Var(ent), Attribute: ":" + lit.name + "#h", Value: fs.Lit(true)}}
	}
	out := make([]fs.Clause, len(lit.args))
	for i, a := range lit.args {
		out[i] = &fs.Pattern{Entity: fs.Var(ent), Attribute: attrOf(lit.name, i), Value: litTerm(a)}
	}
	return out
}
