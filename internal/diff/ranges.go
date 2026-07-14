package diff

import (
	"fmt"
	"sort"
	"strings"
)

// LineRange is an inclusive run of consecutive line numbers.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// String renders "12" or "12-15".
func (r LineRange) String() string {
	if r.Start == r.End {
		return fmt.Sprintf("%d", r.Start)
	}
	return fmt.Sprintf("%d-%d", r.Start, r.End)
}

// Count is the number of lines in the range.
func (r LineRange) Count() int { return r.End - r.Start + 1 }

// Ranges groups a set of line numbers into sorted inclusive ranges:
// [5 1 2 3 9] → [1-3, 5, 9]. Duplicates are tolerated.
func Ranges(lines []int) []LineRange {
	if len(lines) == 0 {
		return nil
	}
	sorted := append([]int(nil), lines...)
	sort.Ints(sorted)
	var out []LineRange
	cur := LineRange{Start: sorted[0], End: sorted[0]}
	for _, ln := range sorted[1:] {
		switch {
		case ln == cur.End || ln == cur.End+1:
			cur.End = ln
		default:
			out = append(out, cur)
			cur = LineRange{Start: ln, End: ln}
		}
	}
	return append(out, cur)
}

// TotalLines sums the line counts of all ranges.
func TotalLines(ranges []LineRange) int {
	n := 0
	for _, r := range ranges {
		n += r.Count()
	}
	return n
}

// FormatRanges renders at most max ranges, eliding the rest:
// "12-15, 22, 31 (+3 more ranges)".
func FormatRanges(ranges []LineRange, max int) string {
	if len(ranges) == 0 {
		return ""
	}
	shown := ranges
	extra := 0
	if max > 0 && len(ranges) > max {
		shown = ranges[:max]
		extra = len(ranges) - max
	}
	parts := make([]string, len(shown))
	for i, r := range shown {
		parts[i] = r.String()
	}
	s := strings.Join(parts, ", ")
	if extra > 0 {
		s += fmt.Sprintf(" (+%d more ranges)", extra)
	}
	return s
}
