package update

import (
	"strconv"
	"strings"
)

func compareDottedVersion(a string, b string) int {
	aa := versionParts(a)
	bb := versionParts(b)
	maxLen := len(aa)
	if len(bb) > maxLen {
		maxLen = len(bb)
	}
	for i := 0; i < maxLen; i++ {
		var av, bv int
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func versionParts(version string) []int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	raw := strings.Split(version, ".")
	parts := make([]int, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			parts = append(parts, 0)
			continue
		}
		if idx := strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }); idx >= 0 {
			part = part[:idx]
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			value = 0
		}
		parts = append(parts, value)
	}
	return parts
}
