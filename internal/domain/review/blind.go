package review

// Blind merges two independent passes of the same lenses over the same
// diff. A finding both passes prove on the same hunk, through the same
// lens, is corroborated: it becomes deterministic and needs no refuter. A
// finding only one pass reports stays as it is, but a severe one is made
// inferential so the refuter has to confirm it before it can block. IDs of
// the second pass get a "b" suffix, so the two never collide.
func Blind(first, second []Finding, d Diff) []Finding {
	used := make([]bool, len(second))
	var out []Finding
	for _, f := range first {
		match := -1
		for j, g := range second {
			if !used[j] && g.Lens == f.Lens && sameHunk(f, g, d) {
				match = j
				break
			}
		}
		if match >= 0 {
			used[match] = true
			f.Evidence = Deterministic
		} else {
			f = alone(f)
		}
		out = append(out, f)
	}
	for j, g := range second {
		if !used[j] {
			g.ID += "b"
			out = append(out, alone(g))
		}
	}
	return out
}

// alone marks a finding only one pass reported.
func alone(f Finding) Finding {
	if f.Severity.Severe() && f.Evidence == Deterministic {
		f.Evidence = Inferential
	}
	return f
}

// sameHunk reports whether two findings prove something on a common hunk.
func sameHunk(a, b Finding, d Diff) bool {
	for _, h := range d.Hunks {
		if provesIn(a, h) && provesIn(b, h) {
			return true
		}
	}
	return false
}

func provesIn(f Finding, h Hunk) bool {
	for _, p := range f.Proof {
		if p.Kind == ProofChangedHunk && h.Contains(p.Path, p.Line) {
			return true
		}
		if p.Kind == ProofNewFile && p.Path == h.Path {
			return true
		}
	}
	return false
}
