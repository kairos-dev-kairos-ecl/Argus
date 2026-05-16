package behaviour

import (
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	ViewRunList ViewState = iota
	ViewRunDetail
	ViewRunCompare
)

// TUI-side mirrors (decoupled from internal/trace to keep TUI buildable standalone)
type RunNode struct {
	SignalID          string                 `json:"signal_id"`
	SpanID            string                 `json:"span_id"`
	ParentSpanID      string                 `json:"parent_span_id"`
	Layer             int32                  `json:"layer"`
	Category          string                 `json:"category"`
	Severity          int32                  `json:"severity"`
	Timestamp         time.Time              `json:"timestamp"`
	DurationMS        float32                `json:"duration_ms"`
	BaselineDeviation float32                `json:"baseline_deviation"`
	IsAnomaly         bool                   `json:"is_anomaly"`
	IsOrphan          bool                   `json:"is_orphan"`
	CtxSummary        map[string]interface{} `json:"ctx_summary"`
}

type RunMeta struct {
	TraceID       string    `json:"trace_id"`
	LayersPresent []int32   `json:"layers_present"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	PeakDeviation float32   `json:"peak_deviation"`
	SignalCount   int       `json:"signal_count"`
}

type RunEdge struct {
	FromSpanID string `json:"from_span_id"`
	ToSpanID   string `json:"to_span_id"`
	Type       string `json:"type"`
}

type RunGraph struct {
	Meta  RunMeta    `json:"meta"`
	Nodes []*RunNode `json:"nodes"`
	Edges []RunEdge  `json:"edges"`
}

type RecentRun struct {
	TraceID       string    `json:"trace_id"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	LayersPresent []int32   `json:"layers_present"`
	SignalCount   int32     `json:"signal_count"`
	PeakDeviation float32   `json:"peak_deviation"`
	DurationMS    int64     `json:"duration_ms"`
}

type Model struct {
	CurrentView ViewState
	AppID       string
	BaseURL     string
	Token       string
	Client      *http.Client

	Runs   []RecentRun
	Cursor int

	Selected      *RunGraph
	CompareA      *RunGraph
	CompareB      *RunGraph
	ExpandedSpans map[string]bool

	Loading bool
	Err     error
}

func New(baseURL, token, appID string) Model {
	return Model{
		CurrentView:   ViewRunList,
		AppID:         appID,
		BaseURL:       baseURL,
		Token:         token,
		Client:        &http.Client{Timeout: 10 * time.Second},
		ExpandedSpans: map[string]bool{},
	}
}

func (m Model) Init() tea.Cmd {
	return fetchRuns(m.Client, m.BaseURL, m.Token, m.AppID)
}
