package behaviour

import (
	"fmt"
	"sort"
	"strings"
)

func renderRunDetail(m Model) string {
	var b strings.Builder
	b.WriteString("ARGUS — BEHAVIOUR — RUN DETAIL\n\n")
	if m.Selected == nil {
		b.WriteString("(no run loaded)\n")
		return b.String()
	}
	g := m.Selected
	b.WriteString(fmt.Sprintf("trace_id: %s  signals: %d  peak_dev: %.2f\n",
		g.Meta.TraceID, g.Meta.SignalCount, g.Meta.PeakDeviation))
	b.WriteString(fmt.Sprintf("start: %s  end: %s\n\n",
		g.Meta.StartTime.Format("15:04:05.000"), g.Meta.EndTime.Format("15:04:05.000")))

	// Build parent->children map
	childMap := map[string][]*RunNode{}
	bySpan := map[string]*RunNode{}
	for _, n := range g.Nodes {
		bySpan[n.SpanID] = n
	}
	var roots []*RunNode
	for _, n := range g.Nodes {
		if n.ParentSpanID == "" || bySpan[n.ParentSpanID] == nil {
			roots = append(roots, n)
		} else {
			childMap[n.ParentSpanID] = append(childMap[n.ParentSpanID], n)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Timestamp.Before(roots[j].Timestamp) })
	for _, r := range roots {
		renderSpan(&b, r, childMap, 0)
	}

	b.WriteString("\n[r] back to list  [c] compare  [q] quit\n")
	return b.String()
}

func renderSpan(b *strings.Builder, n *RunNode, children map[string][]*RunNode, depth int) {
	indent := strings.Repeat("  ", depth)
	orphan := ""
	if n.IsOrphan {
		orphan = " [orphan]"
	}
	line := fmt.Sprintf("%s[L%d] %s (%.0fms) [dev=%.2f]%s\n",
		indent, n.Layer, n.Category, n.DurationMS, n.BaselineDeviation, orphan)
	b.WriteString(devColour(n.BaselineDeviation, line))
	kids := children[n.SpanID]
	sort.Slice(kids, func(i, j int) bool { return kids[i].Timestamp.Before(kids[j].Timestamp) })
	for _, c := range kids {
		renderSpan(b, c, children, depth+1)
	}
}
