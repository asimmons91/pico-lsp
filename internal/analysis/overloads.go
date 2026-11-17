package analysis

import (
	"math"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
)

// candidateFunctions returns the overload signatures in display order. A
// primary signature participates only when it is explicitly annotated.
func candidateFunctions(sym *Symbol) []*types.Function {
	if sym == nil {
		return nil
	}
	primary, hasPrimary := types.AsFunction(sym.Type)
	if len(sym.Overloads) == 0 {
		if hasPrimary {
			return []*types.Function{primary}
		}
		return nil
	}
	out := make([]*types.Function, 0, len(sym.Overloads)+1)
	out = append(out, sym.Overloads...)
	if hasPrimary && sym.TypeExplicit {
		out = append(out, primary)
	}
	if len(out) == 0 && hasPrimary {
		out = append(out, primary)
	}
	return out
}

// selectFunctionCandidate picks the unique best overload for args. Ambiguous
// or non-matching calls return nil so callers can fall back without noise.
func selectFunctionCandidate(candidates []*types.Function, args []types.Type) *types.Function {
	bestScore := math.MaxInt
	var best *types.Function
	ambiguous := false
	matches := 0
	for _, fn := range candidates {
		if fn == nil {
			continue
		}
		specialized := types.BindFunction(fn, args)
		score, ok := overloadScore(specialized, args)
		if !ok {
			continue
		}
		matches++
		if score < bestScore {
			best = specialized
			bestScore = score
			ambiguous = false
		} else if score == bestScore && best != nil && types.Display(specialized) != types.Display(best) {
			ambiguous = true
		}
	}
	if matches == 0 || ambiguous {
		return nil
	}
	return best
}

func overloadScore(fn *types.Function, args []types.Type) (int, bool) {
	if fn == nil {
		return 0, false
	}
	score := 0
	for i, arg := range args {
		if i < len(fn.Params) {
			param := fn.Params[i]
			target := param.Type
			if param.Optional {
				target = types.Optional(target)
			}
			if types.IsAny(target) || types.IsAny(arg) {
				score += 4
				continue
			}
			if !types.Assignable(arg, target, false) {
				return 0, false
			}
			if !types.Identical(arg, target) {
				score++
			}
			continue
		}
		if fn.Vararg != nil {
			if !types.IsAny(fn.Vararg) && !types.Assignable(arg, fn.Vararg, false) {
				return 0, false
			}
			score += 2
			continue
		}
		// PICO-8 ignores extra arguments, but a signature that consumes them
		// is a better match.
		score += 8
	}
	for i := len(args); i < len(fn.Params); i++ {
		if fn.Params[i].Optional {
			score++
			continue
		}
		if !types.IsAny(fn.Params[i].Type) {
			score += 8
		}
	}
	return score, true
}

// soleArityCandidate returns the only overload whose required arity matches the
// call. It lets call checking report a clear type mismatch without guessing
// between overloads.
func soleArityCandidate(candidates []*types.Function, args []types.Type) *types.Function {
	var found *types.Function
	for _, fn := range candidates {
		if fn == nil || arityDistance(fn, len(args)) != 0 {
			continue
		}
		if found != nil {
			return nil
		}
		found = fn
	}
	return found
}

func arityDistance(fn *types.Function, count int) int {
	if fn == nil {
		return count
	}
	required := 0
	optional := 0
	for _, param := range fn.Params {
		if param.Optional {
			optional++
			continue
		}
		required++
	}
	if count < required {
		return required - count
	}
	if count <= required+optional {
		return 0
	}
	if fn.Vararg != nil {
		return 0
	}
	return count - (required + optional)
}

// FunctionCandidates returns the signatures exposed for hover and signature
// help, including explicit overloads.
func (s *Symbol) FunctionCandidates() []*types.Function {
	return candidateFunctions(s)
}
