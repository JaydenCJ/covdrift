package pathnorm

import "strings"

// Match reports whether a slash-separated glob pattern matches path.
// Supported syntax, chosen to feel like .gitignore:
//
//   - `*` matches any run of characters within one path segment
//   - `?` matches exactly one character within a segment
//   - `**` as a full segment matches zero or more whole segments
//   - a pattern with no `/` matches the file's base name in any directory
func Match(pattern, path string) bool {
	if !strings.Contains(pattern, "/") {
		base := path
		if i := strings.LastIndexByte(path, '/'); i >= 0 {
			base = path[i+1:]
		}
		return matchSegments([]string{pattern}, []string{base})
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

// matchSegments matches pattern segments against path segments, handling
// `**` by trying every possible number of consumed segments.
func matchSegments(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		// `**` may swallow zero segments (so `a/**/b` matches `a/b`)
		// or any positive number of them.
		for skip := 0; skip <= len(segs); skip++ {
			if matchSegments(pat[1:], segs[skip:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	return matchSegment(pat[0], segs[0]) && matchSegments(pat[1:], segs[1:])
}

// matchSegment matches one glob segment (with * and ?) against one path
// segment, via iterative backtracking on the last-seen star.
func matchSegment(pat, s string) bool {
	pi, si := 0, 0
	star, starSi := -1, 0
	for si < len(s) {
		switch {
		case pi < len(pat) && (pat[pi] == '?' || pat[pi] == s[si]):
			pi++
			si++
		case pi < len(pat) && pat[pi] == '*':
			star, starSi = pi, si
			pi++
		case star >= 0:
			// Backtrack: let the star absorb one more character.
			starSi++
			pi, si = star+1, starSi
		default:
			return false
		}
	}
	for pi < len(pat) && pat[pi] == '*' {
		pi++
	}
	return pi == len(pat)
}
