package storage

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// SessionIDSkipIndexDDL adds a bloom-filter skip index on signals.session_id.
// Used by SessionTimeline queries; eliminates granules that cannot contain the target session_id.
const SessionIDSkipIndexDDL = `ALTER TABLE signals ADD INDEX IF NOT EXISTS idx_skip_session_id session_id TYPE bloom_filter(0.01) GRANULARITY 4`

// ConversationIDSkipIndexDDL adds a bloom-filter skip index on signals.conversation_id.
// Used by ConversationBehaviour queries.
const ConversationIDSkipIndexDDL = `ALTER TABLE signals ADD INDEX IF NOT EXISTS idx_skip_conversation_id conversation_id TYPE bloom_filter(0.01) GRANULARITY 4`

// ApplySignalSkipIndexes runs the two Phase 7 skip-index ALTERs idempotently.
// Must be called after the signals table DDL has been applied.
// Returns the first error encountered; both indexes are attempted sequentially.
func ApplySignalSkipIndexes(ctx context.Context, ch driver.Conn) error {
	for _, ddl := range []string{SessionIDSkipIndexDDL, ConversationIDSkipIndexDDL} {
		if err := ch.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("apply skip index: %w", err)
		}
	}
	return nil
}
