// Package comparer produces line- and word-level diffs between two texts using
// a longest-common-subsequence algorithm, for the RedTrace Comparer tool.
package comparer

import (
	"regexp"
	"strings"
)

// Kind labels a diff segment as unchanged, added, or removed.
type Kind string

const (
	Equal  Kind = "equal"
	Insert Kind = "insert"
	Delete Kind = "delete"
)

// Segment is a contiguous run of text with a single diff kind. Concatenating
// the Text of all Equal+Delete segments reproduces the left input; Equal+Insert
// reproduces the right input.
type Segment struct {
	Op   Kind   `json:"op"`
	Text string `json:"text"`
}

var wordRe = regexp.MustCompile(`\s+|\S+`)

// DiffLines compares a and b line by line.
func DiffLines(a, b string) []Segment {
	return diffTokens(splitLines(a), splitLines(b))
}

// DiffWords compares a and b word by word (whitespace runs are tokens too, so
// reassembly is lossless).
func DiffWords(a, b string) []Segment {
	return diffTokens(wordRe.FindAllString(a, -1), wordRe.FindAllString(b, -1))
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.SplitAfter(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	return parts
}

// diffTokens runs an LCS diff over token slices and coalesces adjacent
// same-kind tokens into segments.
// maxCells bounds the O(la*lb) DP table so a diff of two large inputs cannot
// allocate gigabytes. Beyond it (or when a side is empty) we fall back to a
// whole-delete + whole-insert diff. LCS lengths fit comfortably in int32.
const maxCells = 4 << 20 // ~4M cells (~16 MiB at int32)

func diffTokens(a, b []string) []Segment {
	la, lb := len(a), len(b)

	if la == 0 || lb == 0 || int64(la)*int64(lb) > maxCells {
		var segs []Segment
		if s := strings.Join(a, ""); s != "" {
			segs = append(segs, Segment{Op: Delete, Text: s})
		}
		if s := strings.Join(b, ""); s != "" {
			segs = append(segs, Segment{Op: Insert, Text: s})
		}
		return segs
	}

	// dp[i][j] = LCS length of a[i:] and b[j:].
	dp := make([][]int32, la+1)
	for i := range dp {
		dp[i] = make([]int32, lb+1)
	}
	for i := la - 1; i >= 0; i-- {
		for j := lb - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var segs []Segment
	emit := func(op Kind, text string) {
		if n := len(segs); n > 0 && segs[n-1].Op == op {
			segs[n-1].Text += text
			return
		}
		segs = append(segs, Segment{Op: op, Text: text})
	}

	i, j := 0, 0
	for i < la && j < lb {
		switch {
		case a[i] == b[j]:
			emit(Equal, a[i])
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			emit(Delete, a[i])
			i++
		default:
			emit(Insert, b[j])
			j++
		}
	}
	for ; i < la; i++ {
		emit(Delete, a[i])
	}
	for ; j < lb; j++ {
		emit(Insert, b[j])
	}
	return segs
}
