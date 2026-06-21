// Package sequencer analyzes the randomness of a sample of tokens (e.g. session
// identifiers). It is a lightweight, indicative estimate — per-character
// positional Shannon entropy summed into an effective-bits figure — not a
// full FIPS-140 battery.
package sequencer

import (
	"fmt"
	"math"
)

// Report is the outcome of analyzing a token sample.
type Report struct {
	SampleCount     int       `json:"sampleCount"`
	UniqueCount     int       `json:"uniqueCount"`
	MinLength       int       `json:"minLength"`
	MaxLength       int       `json:"maxLength"`
	AlphabetSize    int       `json:"alphabetSize"`
	PositionEntropy []float64 `json:"positionEntropy"` // bits at each character position
	EffectiveBits   float64   `json:"effectiveBits"`   // sum of positional entropy
	BitsPerChar     float64   `json:"bitsPerChar"`
	Quality         string    `json:"quality"` // poor | reasonable | good | excellent
	Notes           []string  `json:"notes"`
}

// Analyze computes a randomness report for the given tokens.
func Analyze(tokens []string) Report {
	clean := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t != "" {
			clean = append(clean, t)
		}
	}

	r := Report{SampleCount: len(clean), PositionEntropy: []float64{}, Notes: []string{}}
	if len(clean) == 0 {
		r.Quality = "poor"
		r.Notes = append(r.Notes, "No tokens were collected — check the extraction source/selector.")
		return r
	}

	uniq := make(map[string]struct{}, len(clean))
	alphabet := make(map[byte]struct{})
	r.MinLength, r.MaxLength = len(clean[0]), len(clean[0])
	for _, t := range clean {
		uniq[t] = struct{}{}
		if len(t) < r.MinLength {
			r.MinLength = len(t)
		}
		if len(t) > r.MaxLength {
			r.MaxLength = len(t)
		}
		for i := 0; i < len(t); i++ {
			alphabet[t[i]] = struct{}{}
		}
	}
	r.UniqueCount = len(uniq)
	r.AlphabetSize = len(alphabet)

	// Positional Shannon entropy: at each character position, the entropy of the
	// distribution of characters across all tokens long enough to have one.
	r.PositionEntropy = make([]float64, r.MaxLength)
	posCount := make([]int, r.MaxLength) // tokens that actually reach each position
	for pos := 0; pos < r.MaxLength; pos++ {
		counts := make(map[byte]int)
		n := 0
		for _, t := range clean {
			if pos < len(t) {
				counts[t[pos]]++
				n++
			}
		}
		posCount[pos] = n
		r.PositionEntropy[pos] = shannon(counts, n)
		r.EffectiveBits += r.PositionEntropy[pos]
	}
	// Average entropy over the positions present in every token (0..MinLength-1),
	// so a single longer-token outlier (whose tail positions have a tiny sample)
	// can't deflate the per-character figure.
	if r.MinLength > 0 {
		var coreBits float64
		for i := 0; i < r.MinLength; i++ {
			coreBits += r.PositionEntropy[i]
		}
		r.BitsPerChar = coreBits / float64(r.MinLength)
	}

	r.Quality = quality(r.EffectiveBits)
	r.Notes = notes(r, posCount)
	return r
}

// shannon returns the Shannon entropy (bits) of a character distribution.
func shannon(counts map[byte]int, n int) float64 {
	if n == 0 {
		return 0
	}
	var h float64
	for _, c := range counts {
		p := float64(c) / float64(n)
		h -= p * math.Log2(p)
	}
	return h
}

func quality(bits float64) string {
	switch {
	case bits < 40:
		return "poor"
	case bits < 72:
		return "reasonable"
	case bits < 112:
		return "good"
	default:
		return "excellent"
	}
}

func notes(r Report, posCount []int) []string {
	var out []string
	if r.SampleCount < 100 {
		out = append(out, fmt.Sprintf("Small sample (%d tokens); collect more for a reliable estimate.", r.SampleCount))
	}
	if r.UniqueCount < r.SampleCount {
		out = append(out, fmt.Sprintf("%d of %d tokens are duplicates — a serious randomness weakness.", r.SampleCount-r.UniqueCount, r.SampleCount))
	}
	if r.MinLength != r.MaxLength {
		out = append(out, "Tokens vary in length, which can indicate structure rather than raw randomness.")
	}
	// A position is only "constant across all tokens" if every token reaches it
	// (full coverage) and there are at least two tokens to compare; a sparse tail
	// position present in one outlier-length token is not a fixed prefix.
	if r.SampleCount >= 2 {
		constant := 0
		for i, h := range r.PositionEntropy {
			if h == 0 && posCount[i] == r.SampleCount {
				constant++
			}
		}
		if constant > 0 {
			out = append(out, fmt.Sprintf("%d character position(s) are constant across all tokens (fixed prefix/format).", constant))
		}
	}
	out = append(out, "Effective entropy is an indicative estimate (positional Shannon entropy), not a FIPS-140 result.")
	return out
}
