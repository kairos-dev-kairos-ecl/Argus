package engine

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// TemporalStore abstracts Redis-backed frequency counting.
type TemporalStore interface {
	AddAndCount(ctx context.Context, key string, ts time.Time, windowSeconds int) (int, error)
}

// Tier3Matches evaluates temporal-frequency conditions.
func Tier3Matches(ctx context.Context, rule Rule, sig *v1.ArgusSignal, store TemporalStore) (bool, error) {
	if sig == nil || store == nil || rule.Conditions.Temporal == nil {
		return false, nil
	}
	tc := rule.Conditions.Temporal
	if tc.CountGte < 1 || tc.WindowSeconds < 1 {
		return false, nil
	}

	appID := ""
	if sig.Source != nil {
		appID = sig.Source.AppId
	}
	key := fmt.Sprintf("det:t3:%s:%s:%s", rule.ID, appID, sig.Category)

	at := time.Now()
	if sig.Timestamp != nil {
		at = sig.Timestamp.AsTime()
	}
	count, err := store.AddAndCount(ctx, key, at, tc.WindowSeconds)
	if err != nil {
		return false, err
	}
	return count >= tc.CountGte, nil
}
