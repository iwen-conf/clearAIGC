package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type SessionStateRepository struct {
	pool *pgxpool.Pool
}

func NewSessionStateRepository(pool *pgxpool.Pool) *SessionStateRepository {
	return &SessionStateRepository{pool: pool}
}

func (r *SessionStateRepository) UpsertProgress(ctx context.Context, snapshot *domain.SessionProgressSnapshot) error {
	if snapshot == nil {
		return nil
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO session_progress (
			session_id, round_number, phase, completed_chunks, total_chunks, percent,
			chunk_id, paragraph_index, chunk_index, provider_used, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (session_id) DO UPDATE SET
			round_number = EXCLUDED.round_number,
			phase = EXCLUDED.phase,
			completed_chunks = EXCLUDED.completed_chunks,
			total_chunks = EXCLUDED.total_chunks,
			percent = EXCLUDED.percent,
			chunk_id = EXCLUDED.chunk_id,
			paragraph_index = EXCLUDED.paragraph_index,
			chunk_index = EXCLUDED.chunk_index,
			provider_used = EXCLUDED.provider_used,
			updated_at = EXCLUDED.updated_at
	`,
		snapshot.SessionID,
		snapshot.Round,
		snapshot.Phase,
		snapshot.CompletedChunks,
		snapshot.TotalChunks,
		snapshot.Percent,
		snapshot.ChunkID,
		snapshot.ParagraphIndex,
		snapshot.ChunkIndex,
		snapshot.ProviderUsed,
		snapshot.UpdatedAt,
	)
	return err
}

func (r *SessionStateRepository) GetProgress(ctx context.Context, sessionID uuid.UUID) (*domain.SessionProgressSnapshot, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT session_id, round_number, phase, completed_chunks, total_chunks, percent,
		       chunk_id, paragraph_index, chunk_index, provider_used, updated_at
		FROM session_progress
		WHERE session_id = $1
	`, sessionID)

	var snapshot domain.SessionProgressSnapshot
	if err := row.Scan(
		&snapshot.SessionID,
		&snapshot.Round,
		&snapshot.Phase,
		&snapshot.CompletedChunks,
		&snapshot.TotalChunks,
		&snapshot.Percent,
		&snapshot.ChunkID,
		&snapshot.ParagraphIndex,
		&snapshot.ChunkIndex,
		&snapshot.ProviderUsed,
		&snapshot.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &snapshot, nil
}

func (r *SessionStateRepository) AppendTimeline(ctx context.Context, entry *domain.SessionTimelineEntry) error {
	if entry == nil {
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"tone":   entry.Tone,
		"title":  entry.Title,
		"detail": entry.Detail,
	})
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_log (session_id, action, actor, detail, created_at)
		VALUES ($1, $2, 'system', $3, $4)
	`,
		entry.SessionID,
		"timeline.entry",
		payload,
		time.Now().UTC(),
	)
	return err
}

func (r *SessionStateRepository) ListTimeline(ctx context.Context, sessionID uuid.UUID, limit int) ([]domain.SessionTimelineEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, detail, created_at
		FROM audit_log
		WHERE session_id = $1 AND action = 'timeline.entry'
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]domain.SessionTimelineEntry, 0, limit)
	for rows.Next() {
		var id int64
		var detailJSON []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &detailJSON, &createdAt); err != nil {
			return nil, err
		}

		var detail struct {
			Tone   domain.TimelineTone `json:"tone"`
			Title  string              `json:"title"`
			Detail string              `json:"detail"`
		}
		if err := json.Unmarshal(detailJSON, &detail); err != nil {
			return nil, err
		}

		entries = append(entries, domain.SessionTimelineEntry{
			ID:        strconv.FormatInt(id, 10),
			SessionID: sessionID,
			Tone:      detail.Tone,
			Title:     detail.Title,
			Detail:    detail.Detail,
			Timestamp: createdAt.UnixMilli(),
		})
	}

	return entries, rows.Err()
}

func (r *SessionStateRepository) DeleteForSession(ctx context.Context, sessionID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err = tx.Exec(ctx, `DELETE FROM session_progress WHERE session_id = $1`, sessionID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM audit_log WHERE session_id = $1 AND action = 'timeline.entry'`, sessionID); err != nil {
		return err
	}

	err = tx.Commit(ctx)
	return err
}
