package baseline

// ComputeSessionDrift returns the normalised Levenshtein distance between two
// layer-enum sequences. 0.0 = identical, 1.0 = completely different.
//
// Normalisation: editDistance(actual, baseline) / max(len(actual), len(baseline)).
// If both inputs are empty, returns 0.0 (no drift computable, sessions are equivalent).
// If one input is empty and the other is not, returns 1.0 (maximum drift).
//
// Pure function: no I/O, no state. Safe for concurrent use.
func ComputeSessionDrift(actual, baseline []int32) float64 {
	if len(actual) == 0 && len(baseline) == 0 {
		return 0.0
	}
	if len(actual) == 0 || len(baseline) == 0 {
		return 1.0
	}
	d := levenshtein(actual, baseline)
	maxLen := len(actual)
	if len(baseline) > maxLen {
		maxLen = len(baseline)
	}
	return float64(d) / float64(maxLen)
}

// levenshtein computes the Levenshtein edit distance between two int32 slices.
// Uses a two-row DP table to keep memory usage O(n).
func levenshtein(a, b []int32) int {
	m, n := len(a), len(b)
	prev := make([]int, n+1)
	curr := make([]int, n+1)

	for j := 0; j <= n; j++ {
		prev[j] = j
	}

	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
