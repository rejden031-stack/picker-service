package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"picker-service/internal/application/ports/store"
	"picker-service/internal/domain/audit"
)

// AuditStore — журнал аудита в PostgreSQL.
type AuditStore struct {
	pool *pgxpool.Pool
}

func NewAuditStore(pool *pgxpool.Pool) *AuditStore {
	return &AuditStore{pool: pool}
}

func (s *AuditStore) Append(ctx context.Context, e audit.Entry) error {
	const q = `INSERT INTO audit_events (actor_id, actor_name, action, entity_id, details) VALUES ($1,$2,$3,$4,$5)`
	if _, err := s.pool.Exec(ctx, q,
		e.ActorID, e.ActorName, string(e.Action), e.EntityID, e.Details); err != nil {
		return fmt.Errorf("audit append: %w", err)
	}
	return nil
}

func (s *AuditStore) List(ctx context.Context, limit, offset int) ([]audit.Entry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	const q = `SELECT id, ts, actor_id, actor_name, action, entity_id, details
	           FROM audit_events ORDER BY ts DESC, id DESC LIMIT $1 OFFSET $2`

	rows, err := s.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	defer rows.Close()

	entries := make([]audit.Entry, 0, limit)
	for rows.Next() {
		var e audit.Entry
		var action string
		if err := rows.Scan(&e.ID, &e.Time, &e.ActorID, &e.ActorName, &action, &e.EntityID, &e.Details); err != nil {
			return nil, fmt.Errorf("audit list scan: %w", err)
		}
		e.Action = audit.Action(action)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

var _ store.AuditStore = (*AuditStore)(nil)
