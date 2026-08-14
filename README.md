# tln-asp

[![CI](https://github.com/opentalon/tln-asp/actions/workflows/ci.yml/badge.svg)](https://github.com/opentalon/tln-asp/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Answer Set Programming (stable-model) plugin for [tln](https://github.com/opentalon/tln-language) — "tln as a new Prolog/ASP front-end."**

tln's core is deterministic: its well-founded resolver yields a single
three-valued model (true / false / undefined). **Stable-model semantics is
different** — a rule set has **zero or many** answer sets (`p :- not q. q :- not p.`
has two) — and is the classic win for combinatorial search, planning, and
configuration. That non-determinism is deliberately kept out of core
([ADR-0008](https://github.com/opentalon/tln-language/blob/master/docs/design/0008-asp-plugin.md));
this plugin owns it.

`tln-asp` is the tln plugin's *solver* leg, alongside
[`tln-db`](https://github.com/opentalon/tln-db) (a **store**) and
[`tln-mcp`](https://github.com/opentalon/tln-mcp) (a **tool**). Core stays a pure
language + planner + SPIs; every edge is a plugin.

## What it does

Write the rule the way you would in tln — a position is **winning** if some move
leads to a position that is *not* winning, and `win` is then used like any other
predicate:

```tln
// `move(x, y)` edges come from the facts; `win` refers back to itself through
// `not` — a cycle tln core rejects as "negation through recursion is not stratifiable".
derive win(x) {
  for records where move(x, y) and not win(y)
}

// Once solved, win(...) drives an ordinary detect (or rule / recommend):
detect "Winning positions" {
  for records where type == "position" and win(pos)
  flag matching items
  label "{item.name}: winning — a move forces the opponent into a loss"
  priority HIGH
}
```

On a draw (a cycle) that rule has **multiple answer sets** — undefined for core's
single well-founded model, but exactly the ASP case. The host builds the rule set
from the public `pkg/factstore` types (no new DSL) and hands it to tln-asp, which
enumerates the answer sets; each feeds back into any FactStore:

```go
import (
    fs "github.com/opentalon/tln-language/pkg/factstore"
    tlnasp "github.com/opentalon/tln-asp"
)

// win(X) :- move(X,Y), not win(Y)
prog := tlnasp.Program{
    Rules: []fs.Rule{{
        Name: "win", Args: []string{"?x"},
        Body: []fs.Clause{
            &fs.Pattern{Entity: fs.Var("e"), Attribute: ":edge/from", Value: fs.Var("x")},
            &fs.Pattern{Entity: fs.Var("e"), Attribute: ":edge/to",   Value: fs.Var("y")},
            &fs.Negation{Name: "win", Args: []fs.Term{fs.Var("y")}},
        },
    }},
    Facts: /* the edge facts (EDB) as []fs.Fact */,
}

sets, _ := tlnasp.New().Solve(ctx, prog)   // 0..N stable models
for _, s := range sets {
    store.Assert(ctx, s.Facts())           // answers feed back as facts
}
```

`AnswerSet.Facts()` returns `[]factstore.Fact`, so answer sets feed back into
**any** tln FactStore — the in-process `MemoryStore`, Datalevin, or **tln-db** —
the boundary is store-agnostic.

## How it works (pure Go)

No clingo, no cgo, no subprocess. The solver:

1. **Grounds** the rules against the program's facts (the same pattern as tln's
   core well-founded grounder): `Pattern`/`Predicate` bind variables, `RuleCall`
   is a positive literal, `Negation` is negation-as-failure.
2. **Enumerates stable models** via the **Gelfond-Lifschitz reduct**: for a
   candidate set `M`, the reduct `P^M` drops rules with a negative literal in `M`
   and strips the rest; `M` is stable iff `leastModel(P^M) == M`. The search
   ranges over the negatively-referenced atoms, so it is `2^|neg-atoms|` — the
   inherent NP-hardness. Good for the "new Prolog" use case; not clingo-scale.

The `Solver` interface leaves room for a clingo-backed solver later without
changing callers.

## Stable vs. well-founded

The 2-cycle `a ⇄ b` under `win(X) :- move(X,Y), not win(Y)` has **two** stable
models (`{win(a)}`, `{win(b)}`) — where tln core's well-founded model leaves both
**undefined**. That's the whole reason ASP is a separate opt-in: it trades the
single auditable answer for search over many.

## Status

Host-driven: the host supplies the rule set (built from `pkg/factstore` types).
Solving rules authored directly in `.tln` `derive` blocks depends on the tracked
recursion/arity-N derive follow-up in tln-language and is out of scope for now.

## License

Apache 2.0 — see [LICENSE](LICENSE).
