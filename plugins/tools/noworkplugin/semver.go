package main

import (
	"strconv"
	"strings"
)

// compareSemver orders two versions (1 = a newer); a pre-release sorts before its release,
// numeric pre-release identifiers compare numerically, before non-numeric identifiers.
// Keep aligned with Backend/internal/modules/pluginmarket/pluginstd/engines.go.
func compareSemver(a, b string) int {
	na, pa := splitSemver(a)
	nb, pb := splitSemver(b)
	for i := 0; i < 3; i++ {
		if na[i] != nb[i] {
			if na[i] > nb[i] {
				return 1
			}
			return -1
		}
	}
	switch {
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	}
	aParts, bParts := strings.Split(pa, "."), strings.Split(pb, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aPart, bPart := aParts[i], bParts[i]
		if aPart == bPart {
			continue
		}
		aNum, bNum := numericIdentifier(aPart), numericIdentifier(bPart)
		if aNum != bNum {
			if aNum {
				return -1
			}
			return 1
		}
		// Comparing lengths first avoids overflowing integers on long identifiers.
		if aNum && len(aPart) != len(bPart) {
			if len(aPart) < len(bPart) {
				return -1
			}
			return 1
		}
		if aPart < bPart {
			return -1
		}
		return 1
	}
	if len(aParts) < len(bParts) {
		return -1
	}
	return 1
}

func numericIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func splitSemver(v string) ([3]int, string) {
	var out [3]int
	core, pre, _ := strings.Cut(strings.TrimSpace(v), "-")
	for i, part := range strings.SplitN(core, ".", 3) {
		out[i], _ = strconv.Atoi(part)
	}
	return out, pre
}
