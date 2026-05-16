package behaviour

import (
	"net/http"
	"testing"
)

// TestHandlerHasServeMethods verifies the BehaviourHandler exposes all required
// http.HandlerFunc signatures. This is a compile-time smoke test.
func TestHandlerHasServeMethods(t *testing.T) {
	var _ http.HandlerFunc = (&BehaviourHandler{}).ServeTraceGraph
	var _ http.HandlerFunc = (&BehaviourHandler{}).ServeSessionTimeline
	var _ http.HandlerFunc = (&BehaviourHandler{}).ServeConversationBehaviour
	var _ http.HandlerFunc = (&BehaviourHandler{}).ServeAlertChain
	var _ http.HandlerFunc = (&BehaviourHandler{}).ServeRecentRuns
}
