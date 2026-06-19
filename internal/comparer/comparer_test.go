package comparer

import (
	"strings"
	"testing"
)

// reassemble rebuilds one side of the diff from the segments.
func reassemble(segs []Segment, keep Kind) string {
	var b strings.Builder
	for _, s := range segs {
		if s.Op == Equal || s.Op == keep {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

func TestDiffLines(t *testing.T) {
	a := "alpha\nbravo\ncharlie\n"
	b := "alpha\ndelta\ncharlie\n"
	segs := DiffLines(a, b)

	if got := reassemble(segs, Delete); got != a {
		t.Errorf("left reassembly = %q, want %q", got, a)
	}
	if got := reassemble(segs, Insert); got != b {
		t.Errorf("right reassembly = %q, want %q", got, b)
	}

	var del, ins bool
	for _, s := range segs {
		if s.Op == Delete && strings.Contains(s.Text, "bravo") {
			del = true
		}
		if s.Op == Insert && strings.Contains(s.Text, "delta") {
			ins = true
		}
	}
	if !del || !ins {
		t.Errorf("expected bravo deleted and delta inserted: %+v", segs)
	}
}

func TestDiffWords(t *testing.T) {
	a := "the quick brown fox"
	b := "the slow brown fox"
	segs := DiffWords(a, b)
	if got := reassemble(segs, Delete); got != a {
		t.Errorf("left = %q, want %q", got, a)
	}
	if got := reassemble(segs, Insert); got != b {
		t.Errorf("right = %q, want %q", got, b)
	}
}

func TestDiff_Identical(t *testing.T) {
	segs := DiffLines("same\ntext\n", "same\ntext\n")
	for _, s := range segs {
		if s.Op != Equal {
			t.Errorf("identical inputs should be all Equal, got %+v", segs)
		}
	}
}

func TestDiff_EmptySide(t *testing.T) {
	segs := DiffLines("", "new\n")
	if len(segs) != 1 || segs[0].Op != Insert || segs[0].Text != "new\n" {
		t.Errorf("empty-vs-content = %+v", segs)
	}
}
