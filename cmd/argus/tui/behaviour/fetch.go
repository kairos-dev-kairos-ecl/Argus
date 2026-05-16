package behaviour

import (
	"encoding/json"
	"fmt"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
)

type runsLoadedMsg struct {
	Runs []RecentRun
	Err  error
}

type graphLoadedMsg struct {
	Graph *RunGraph
	Err   error
	Slot  int // Slot: 0=Selected, 1=CompareA, 2=CompareB
}

func fetchRuns(c *http.Client, baseURL, token, appID string) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("%s/api/v1/traces/recent?app_id=%s&limit=50", baseURL, appID)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.Do(req)
		if err != nil {
			return runsLoadedMsg{Err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return runsLoadedMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}
		var runs []RecentRun
		if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
			return runsLoadedMsg{Err: err}
		}
		return runsLoadedMsg{Runs: runs}
	}
}

func fetchGraph(c *http.Client, baseURL, token, traceID string, slot int) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("%s/api/v1/traces/%s/graph", baseURL, traceID)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.Do(req)
		if err != nil {
			return graphLoadedMsg{Err: err, Slot: slot}
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return graphLoadedMsg{Err: fmt.Errorf("status %d", resp.StatusCode), Slot: slot}
		}
		var g RunGraph
		if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
			return graphLoadedMsg{Err: err, Slot: slot}
		}
		return graphLoadedMsg{Graph: &g, Slot: slot}
	}
}
