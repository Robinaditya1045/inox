package observability

import (
	"context"
	"fmt"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the persistence interface for historical analytics events and room sessions.
type Repository interface {
	SaveEvent(ctx context.Context, event *domain.AnalyticsEvent) error
	SaveEventsBatch(ctx context.Context, events []*domain.AnalyticsEvent) error
	UpsertRoomSession(ctx context.Context, session *domain.RoomSession) error
	GetRecentEvents(ctx context.Context, roomID string, limit int) ([]*domain.AnalyticsEvent, error)
}

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository initializes a PostgreSQL persistence adapter for analytics.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

// SaveEvent persists a single historical analytics event.
func (r *postgresRepository) SaveEvent(ctx context.Context, e *domain.AnalyticsEvent) error {
	query := `
		INSERT INTO analytics_events (id, event_type, room_id, user_id, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, e.ID, e.EventType, e.RoomID, e.UserID, e.Metadata, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert analytics event: %w", err)
	}
	return nil
}

// SaveEventsBatch efficiently executes batch insertion of analytics events using pgx pipeline batching.
func (r *postgresRepository) SaveEventsBatch(ctx context.Context, events []*domain.AnalyticsEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	query := `
		INSERT INTO analytics_events (id, event_type, room_id, user_id, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`
	for _, e := range events {
		batch.Queue(query, e.ID, e.EventType, e.RoomID, e.UserID, e.Metadata, e.CreatedAt)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(events); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to execute batch insert at index %d: %w", i, err)
		}
	}
	return nil
}

// UpsertRoomSession inserts a new room session or updates its ended_at and peak_participants metrics.
func (r *postgresRepository) UpsertRoomSession(ctx context.Context, s *domain.RoomSession) error {
	query := `
		INSERT INTO room_sessions (id, room_id, started_at, ended_at, peak_participants, total_duration_seconds)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET ended_at = EXCLUDED.ended_at,
		    peak_participants = GREATEST(room_sessions.peak_participants, EXCLUDED.peak_participants),
		    total_duration_seconds = EXCLUDED.total_duration_seconds
	`
	_, err := r.db.Exec(ctx, query, s.ID, s.RoomID, s.StartedAt, s.EndedAt, s.PeakParticipants, s.TotalDurationSeconds)
	if err != nil {
		return fmt.Errorf("failed to upsert room session: %w", err)
	}
	return nil
}

// GetRecentEvents returns the most recent analytics events, optionally filtered by room UUID.
func (r *postgresRepository) GetRecentEvents(ctx context.Context, roomID string, limit int) ([]*domain.AnalyticsEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows pgx.Rows
	var err error
	if roomID != "" {
		query := `
			SELECT id, event_type, room_id, user_id, metadata, created_at
			FROM analytics_events
			WHERE room_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`
		rows, err = r.db.Query(ctx, query, roomID, limit)
	} else {
		query := `
			SELECT id, event_type, room_id, user_id, metadata, created_at
			FROM analytics_events
			ORDER BY created_at DESC
			LIMIT $1
		`
		rows, err = r.db.Query(ctx, query, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query recent events: %w", err)
	}
	defer rows.Close()

	var events []*domain.AnalyticsEvent
	for rows.Next() {
		e := &domain.AnalyticsEvent{}
		if err := rows.Scan(&e.ID, &e.EventType, &e.RoomID, &e.UserID, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan analytics event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating analytics events: %w", err)
	}
	return events, nil
}
