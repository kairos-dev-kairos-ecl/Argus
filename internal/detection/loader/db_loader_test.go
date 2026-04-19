package loader

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/argusxdr/argus/internal/detection/engine"
)

// --- fakeQuerier helpers ---

type fakeRow struct {
	ver int64
	err error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("fakeRow: expected 1 dest")
	}
	*dest[0].(*int64) = r.ver
	return nil
}

type fakeRows struct {
	yamls []string
	idx   int
	err   error
}

func (r *fakeRows) Next() bool {
	if r.err != nil {
		return false
	}
	r.idx++
	return r.idx <= len(r.yamls)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*string) = r.yamls[r.idx-1]
	return nil
}

func (r *fakeRows) Close()                      {}
func (r *fakeRows) Err() error                  { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)      { return nil, nil }
func (r *fakeRows) RawValues() [][]byte         { return nil }
func (r *fakeRows) Conn() *pgx.Conn             { return nil }

type fakeQuerier struct {
	// versionSeq is returned on successive QueryRow calls.
	versionSeq []int64
	versionIdx int
	yamls      []string
	// replaceCount tracks how many times ReplaceAll was called (via store).
}

func (q *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	idx := q.versionIdx
	if idx >= len(q.versionSeq) {
		idx = len(q.versionSeq) - 1
	}
	q.versionIdx++
	return &fakeRow{ver: q.versionSeq[idx]}
}

func (q *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &fakeRows{yamls: q.yamls}, nil
}

// --- valid minimal rule YAML ---

const validRuleYAML = `
id: "rule-001"
name: "Test Rule"
tier: 1
enabled: true
severity: 3
conditions:
  layer: "L6_SAFETY"
action:
  title: "Test Alert"
`

const malformedYAML = `:::not yaml:::`

const invalidRuleYAML = `
id: ""
name: ""
tier: 0
severity: 0
action:
  title: ""
`

// --- tests ---

// TestDBRuleLoader_RespectsIntervalDefault asserts that passing 0 sets interval to 45s.
func TestDBRuleLoader_RespectsIntervalDefault(t *testing.T) {
	store := engine.NewRuleStore()
	log := zap.NewNop()
	q := &fakeQuerier{versionSeq: []int64{0}}
	l := newWithQuerier(q, store, 0, log)
	assert.Equal(t, 45*time.Second, l.Interval())
}

// TestDBRuleLoader_LoadsOnVersionBump verifies rules are loaded when MAX(version) increases.
func TestDBRuleLoader_LoadsOnVersionBump(t *testing.T) {
	store := engine.NewRuleStore()
	log := zap.NewNop()
	q := &fakeQuerier{
		versionSeq: []int64{1},
		yamls:      []string{validRuleYAML},
	}
	l := newWithQuerier(q, store, 45*time.Second, log)

	// First reload — version 1 > lastVersion (-1) → should load rules.
	l.reload(context.Background())

	require.Equal(t, 1, store.Count(), "store should have 1 rule after version bump")
	assert.Equal(t, int64(1), l.lastVersion)
}

// TestDBRuleLoader_SkipsWhenVersionUnchanged verifies ReplaceAll is not called twice when version stays the same.
func TestDBRuleLoader_SkipsWhenVersionUnchanged(t *testing.T) {
	store := engine.NewRuleStore()
	log := zap.NewNop()
	q := &fakeQuerier{
		versionSeq: []int64{1, 1}, // version 1 returned twice
		yamls:      []string{validRuleYAML},
	}
	l := newWithQuerier(q, store, 45*time.Second, log)

	// First reload: version 1 > -1 → loads.
	l.reload(context.Background())
	assert.Equal(t, 1, store.Count())

	// Manually track store state between reloads to detect if ReplaceAll zeroes it.
	// Second reload: version 1 == lastVersion (1) → should skip.
	// Add a sentinel rule to detect if ReplaceAll is called (it would wipe it).
	store.Add(engine.Rule{
		ID: "sentinel", Name: "Sentinel", Tier: 1, Severity: 3,
		Action: engine.Action{Title: "X"},
	})
	assert.Equal(t, 2, store.Count())

	l.reload(context.Background())
	// If ReplaceAll was called, sentinel would be gone. If skipped, we'd have 2.
	assert.Equal(t, 2, store.Count(), "ReplaceAll should not be called when version unchanged")
}

// TestDBRuleLoader_SkipsMalformedYAML verifies bad YAML rows are skipped and others loaded.
func TestDBRuleLoader_SkipsMalformedYAML(t *testing.T) {
	store := engine.NewRuleStore()
	log := zap.NewNop()
	q := &fakeQuerier{
		versionSeq: []int64{1},
		yamls:      []string{malformedYAML, validRuleYAML},
	}
	l := newWithQuerier(q, store, 45*time.Second, log)

	l.reload(context.Background())

	// malformedYAML should be skipped; validRuleYAML should be loaded.
	assert.Equal(t, 1, store.Count(), "only valid rules should be loaded")
}
