package loader

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/argusxdr/argus/internal/detection/engine"
)

// querier abstracts pgxpool.Pool for testing.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// DBRuleLoader polls detection_rules every interval using MAX(version) to
// short-circuit when nothing has changed, then hot-swaps via RuleStore.ReplaceAll.
type DBRuleLoader struct {
	pool        querier
	store       *engine.RuleStore
	interval    time.Duration
	lastVersion int64
	log         *zap.Logger
}

// New constructs a DBRuleLoader. If interval is <= 0, it defaults to 45s (D-08).
func New(pool *pgxpool.Pool, store *engine.RuleStore, interval time.Duration, log *zap.Logger) *DBRuleLoader {
	return newWithQuerier(pool, store, interval, log)
}

// newWithQuerier is the internal constructor accepting the querier interface (used by tests).
func newWithQuerier(q querier, store *engine.RuleStore, interval time.Duration, log *zap.Logger) *DBRuleLoader {
	if interval <= 0 {
		interval = 45 * time.Second
	}
	return &DBRuleLoader{
		pool:        q,
		store:       store,
		interval:    interval,
		lastVersion: -1,
		log:         log,
	}
}

// Run performs an initial load then polls on interval until ctx is cancelled.
func (l *DBRuleLoader) Run(ctx context.Context) {
	l.reload(ctx)
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.reload(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// Interval returns the configured polling interval (for testing).
func (l *DBRuleLoader) Interval() time.Duration {
	return l.interval
}

// reload checks MAX(version) and hot-swaps rules only when the version has increased.
func (l *DBRuleLoader) reload(ctx context.Context) {
	var maxVer int64
	err := l.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM detection_rules WHERE enabled = true`,
	).Scan(&maxVer)
	if err != nil {
		l.log.Warn("rule reload version check failed", zap.Error(err))
		return
	}
	if maxVer <= l.lastVersion {
		return
	}

	rows, err := l.pool.Query(ctx,
		`SELECT yaml_config FROM detection_rules WHERE enabled = true`,
	)
	if err != nil {
		l.log.Warn("rule reload query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	var rules []engine.Rule
	for rows.Next() {
		var yamlStr string
		if err := rows.Scan(&yamlStr); err != nil {
			l.log.Warn("scan failed", zap.Error(err))
			continue
		}
		r, err := engine.ParseRuleYAML([]byte(yamlStr))
		if err != nil {
			l.log.Warn("rule yaml parse failed", zap.Error(err))
			continue
		}
		rules = append(rules, r)
	}

	l.store.ReplaceAll(rules)
	l.lastVersion = maxVer
	l.log.Info("rules reloaded", zap.Int("count", len(rules)), zap.Int64("version", maxVer))
}
