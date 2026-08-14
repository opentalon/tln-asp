package tlnasp

import (
	"context"
	"sort"
)

// solveStable enumerates the stable models of a ground program via the
// Gelfond-Lifschitz reduct.
//
// A stable model M is a fixpoint of: take the reduct P^M (drop every rule with
// a negative body literal in M; strip the remaining negative literals), then M
// must equal the least model of that definite program. Since the reduct depends
// on M only through the *negated* atoms, we search over subsets S of the atoms
// that appear negated: for each S, compute the reduct's least model LM, and
// accept LM as a stable model iff its negated-atom projection is exactly S. This
// makes each stable model appear once and bounds the search at 2^|negAtoms| —
// the inherent NP-hardness lives here.
func solveStable(ctx context.Context, g *grounding, maxModels int) ([]AnswerSet, error) {
	// The atoms whose truth the search branches on.
	negSet := map[string]bool{}
	for _, r := range g.rules {
		for _, n := range r.neg {
			negSet[n] = true
		}
	}
	negAtoms := sortedKeys(negSet)

	var out []AnswerSet
	total := 1 << uint(len(negAtoms))
	for mask := 0; mask < total; mask++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		S := map[string]bool{}
		for i, a := range negAtoms {
			if mask&(1<<uint(i)) != 0 {
				S[a] = true
			}
		}
		lm := leastModel(g.rules, S)
		// Accept iff the negated-atom projection of the least model equals the
		// guess S — the stability check.
		if projectionEquals(lm, negSet, S) {
			out = append(out, answerSetFrom(lm, g))
			if maxModels > 0 && len(out) >= maxModels {
				break
			}
		}
	}

	sortAnswerSets(out)
	return out, nil
}

// leastModel computes the least model of the reduct P^S: rules whose negative
// literals are all outside S, evaluated as a definite (negation-free) program.
func leastModel(rules []groundRule, S map[string]bool) map[string]bool {
	// Keep the reduct rules (no negative literal in S).
	reduct := rules[:0:0]
	for _, r := range rules {
		blocked := false
		for _, n := range r.neg {
			if S[n] {
				blocked = true
				break
			}
		}
		if !blocked {
			reduct = append(reduct, r)
		}
	}

	derived := map[string]bool{}
	for {
		changed := false
		for _, r := range reduct {
			if derived[r.head] {
				continue
			}
			ok := true
			for _, p := range r.pos {
				if !derived[p] {
					ok = false
					break
				}
			}
			if ok {
				derived[r.head] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return derived
}

// projectionEquals reports whether {a ∈ lm : a ∈ negSet} == S.
func projectionEquals(lm map[string]bool, negSet, S map[string]bool) bool {
	count := 0
	for a := range lm {
		if negSet[a] {
			if !S[a] {
				return false
			}
			count++
		}
	}
	return count == len(S)
}

// answerSetFrom reconstructs an AnswerSet (named, argumented atoms) from a set
// of true atom keys.
func answerSetFrom(lm map[string]bool, g *grounding) AnswerSet {
	as := AnswerSet{}
	for k := range lm {
		if a, ok := g.atoms[k]; ok {
			as.Atoms = append(as.Atoms, a)
		}
	}
	sort.Slice(as.Atoms, func(i, j int) bool {
		return atomKey(as.Atoms[i].Name, as.Atoms[i].Args) < atomKey(as.Atoms[j].Name, as.Atoms[j].Args)
	})
	return as
}

// sortAnswerSets orders the models deterministically by their atom signature.
func sortAnswerSets(sets []AnswerSet) {
	sort.Slice(sets, func(i, j int) bool { return sig(sets[i]) < sig(sets[j]) })
}

func sig(a AnswerSet) string {
	s := ""
	for _, at := range a.Atoms {
		s += atomKey(at.Name, at.Args) + ";"
	}
	return s
}
