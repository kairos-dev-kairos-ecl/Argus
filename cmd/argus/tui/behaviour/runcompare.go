package behaviour

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func renderRunCompare(m Model) string {
	var b strings.Builder
	b.WriteString("ARGUS — BEHAVIOUR — COMPARE\n\n")
	if m.CompareA == nil || m.CompareB == nil {
		b.WriteString("Select two runs from the list (press [a] then [b] on highlighted rows)\n")
		return b.String()
	}
	peakByLayerA := layerPeaks(m.CompareA)
	peakByLayerB := layerPeaks(m.CompareB)
	layers := unionLayers(peakByLayerA, peakByLayerB)
	b.WriteString(fmt.Sprintf("A: %s\nB: %s\n\n", m.CompareA.Meta.TraceID, m.CompareB.Meta.TraceID))
	b.WriteString(fmt.Sprintf("%-8s %-12s %-12s %-12s\n", "layer", "A dev", "B dev", "delta"))
	for _, l := range layers {
		a, aok := peakByLayerA[l]
		bv, bok := peakByLayerB[l]
		switch {
		case aok && !bok:
			b.WriteString(fmt.Sprintf("L%-7d %-12.2f %-12s %-12s\n", l, a, "—", "REMOVED"))
		case !aok && bok:
			b.WriteString(fmt.Sprintf("L%-7d %-12s %-12.2f %-12s\n", l, "—", bv, "ADDED"))
		default:
			delta := bv - a
			tag := ""
			if math.Abs(float64(delta)) > 1.0 {
				tag = devColour(float32(math.Abs(float64(delta))), fmt.Sprintf("%+.2f", delta))
			} else {
				tag = fmt.Sprintf("%+.2f", delta)
			}
			b.WriteString(fmt.Sprintf("L%-7d %-12.2f %-12.2f %-12s\n", l, a, bv, tag))
		}
	}
	b.WriteString("\n[r] back to list  [q] quit\n")
	return b.String()
}

func layerPeaks(g *RunGraph) map[int32]float32 {
	out := map[int32]float32{}
	for _, n := range g.Nodes {
		if n.BaselineDeviation > out[n.Layer] {
			out[n.Layer] = n.BaselineDeviation
		}
	}
	return out
}

func unionLayers(a, b map[int32]float32) []int32 {
	seen := map[int32]bool{}
	for l := range a {
		seen[l] = true
	}
	for l := range b {
		seen[l] = true
	}
	out := make([]int32, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

var _ = strings.Repeat
