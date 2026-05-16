package behaviour

import (
	"fmt"
	"strings"
)

func renderRunList(m Model) string {
	var b strings.Builder
	b.WriteString("ARGUS — BEHAVIOUR — RECENT RUNS\n\n")
	if m.Loading {
		b.WriteString("loading...\n")
		return b.String()
	}
	if m.Err != nil {
		b.WriteString("error: " + m.Err.Error() + "\n")
		return b.String()
	}
	if len(m.Runs) == 0 {
		b.WriteString("(no runs found for app_id=" + m.AppID + ")\n")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("%-3s %-20s %-25s %-20s %-10s %-10s\n", "  ", "trace_id", "last_seen", "layers", "peak_dev", "dur_ms"))
	for i, r := range m.Runs {
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}
		tid := r.TraceID
		if len(tid) > 18 {
			tid = tid[:18]
		}
		layers := layerCompact(r.LayersPresent)
		dev := devColour(r.PeakDeviation, fmt.Sprintf("%.2f", r.PeakDeviation))
		b.WriteString(fmt.Sprintf("%s%-20s %-25s %-20s %-10s %-10d\n",
			cursor, tid, r.LastSeenAt.Format("2006-01-02 15:04:05"), layers, dev, r.DurationMS))
	}
	b.WriteString("\n[↑/↓] navigate  [enter] open run  [c] compare  [q] quit\n")
	return b.String()
}

func layerCompact(ls []int32) string {
	parts := make([]string, 0, len(ls))
	for _, l := range ls {
		parts = append(parts, fmt.Sprintf("L%d", l))
	}
	return strings.Join(parts, " ")
}

// ANSI 16-colour codes: 32 green, 33 yellow, 31 red
func devColour(dev float32, text string) string {
	var code int
	switch {
	case dev < 1.0:
		code = 32
	case dev < 2.0:
		code = 33
	default:
		code = 31
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", code, text)
}
