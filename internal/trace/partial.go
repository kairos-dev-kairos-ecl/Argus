package trace

// attachOrphans marks each orphan IsOrphan=true and adds a synthetic temporal edge
// from the first root's span_id (if any) so the graph remains connected.
// If no roots exist, the earliest-timestamped orphan is promoted to the attachment point
// (it is not itself added to edges, just used as the anchor).
func attachOrphans(roots []*RunNode, orphans []*RunNode, edges *[]RunEdge) {
	if len(orphans) == 0 {
		return
	}
	var attach *RunNode
	if len(roots) > 0 {
		attach = roots[0]
	} else {
		// Promote the earliest orphan as the implicit root.
		attach = orphans[0]
		for _, o := range orphans[1:] {
			if o.Timestamp.Before(attach.Timestamp) {
				attach = o
			}
		}
	}
	for _, o := range orphans {
		o.IsOrphan = true
		if o == attach {
			// The promoted root is still an orphan structurally, but we don't add a self-edge.
			continue
		}
		*edges = append(*edges, RunEdge{
			FromSpanID: attach.SpanID,
			ToSpanID:   o.SpanID,
			Type:       EdgeTemporal,
		})
	}
}
