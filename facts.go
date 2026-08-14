package tlnasp

import (
	"fmt"

	"github.com/opentalon/tln-language/pkg/factstore"
)

// Facts encodes an answer set as EAV facts so a host can assert them back into
// a tln FactStore or inspect them. The encoding is:
//
//	nullary  p        -> {RecordID: "p",        Attribute: ":asp/holds",  Value: true}
//	unary    p(a)     -> {RecordID: "a",        Attribute: ":asp/p",      Value: true}
//	n-ary    p(a,b,…) -> {RecordID: "p|a|b|…",  Attribute: ":asp/p",      Value: []any{a,b,…}}
//
// Unary atoms map to the natural EAV shape (entity = the argument); higher
// arities keep the full tuple under the atom's canonical id.
func (a AnswerSet) Facts() []factstore.Fact {
	out := make([]factstore.Fact, 0, len(a.Atoms))
	for _, at := range a.Atoms {
		switch len(at.Args) {
		case 0:
			out = append(out, factstore.Fact{RecordID: at.Name, Attribute: ":asp/holds", Value: true})
		case 1:
			out = append(out, factstore.Fact{RecordID: fmt.Sprintf("%v", at.Args[0]), Attribute: ":asp/" + at.Name, Value: true})
		default:
			out = append(out, factstore.Fact{RecordID: atomKey(at.Name, at.Args), Attribute: ":asp/" + at.Name, Value: append([]any(nil), at.Args...)})
		}
	}
	return out
}

// Has reports whether the answer set contains the atom name(args...).
func (a AnswerSet) Has(name string, args ...any) bool {
	want := atomKey(name, args)
	for _, at := range a.Atoms {
		if atomKey(at.Name, at.Args) == want {
			return true
		}
	}
	return false
}
