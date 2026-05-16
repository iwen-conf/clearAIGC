package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

var ErrNotFound = errors.New("not found")
var ErrSessionLocked = errors.New("session locked")

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions (
			id, document_id, document_name, doc_id, origin_path, file_format, file_size_bytes, prompt_profile, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`,
		session.ID, session.DocumentID, session.DocumentName, session.DocID, session.OriginPath, session.FileFormat,
		session.FileSizeBytes, session.PromptProfile, session.Status, session.CreatedAt, session.UpdatedAt,
	)
	return err
}

func (r *SessionRepository) Get(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, document_id, document_name, doc_id, origin_path, file_format, file_size_bytes, prompt_profile, status, created_at, updated_at
		FROM sessions
		WHERE id = $1
	`, sessionID)

	var session domain.Session
	if err := row.Scan(
		&session.ID,
		&session.DocumentID,
		&session.DocumentName,
		&session.DocID,
		&session.OriginPath,
		&session.FileFormat,
		&session.FileSizeBytes,
		&session.PromptProfile,
		&session.Status,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	rounds, err := listRounds(ctx, r.pool, session.ID)
	if err != nil {
		return nil, err
	}
	session.Rounds = rounds
	return &session, nil
}

func (r *SessionRepository) List(ctx context.Context, filter domain.SessionListFilter) ([]domain.Session, int, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.Size
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	whereClauses := make([]string, 0, 2)
	args := []any{}
	if filter.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", len(args)+1))
		args = append(args, filter.Status)
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("document_name ILIKE '%%' || $%d || '%%'", len(args)+1))
		args = append(args, query)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM sessions`+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := sessionListOrderBy(filter.Sort)
	args = append(args, size, (page-1)*size)
	rows, err := r.pool.Query(ctx, `
		SELECT id, document_id, document_name, doc_id, origin_path, file_format, file_size_bytes, prompt_profile, status, created_at, updated_at
		FROM sessions`+whereClause+`
		ORDER BY `+orderBy+`
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	sessions := make([]domain.Session, 0, size)
	for rows.Next() {
		var session domain.Session
		if err := rows.Scan(
			&session.ID,
			&session.DocumentID,
			&session.DocumentName,
			&session.DocID,
			&session.OriginPath,
			&session.FileFormat,
			&session.FileSizeBytes,
			&session.PromptProfile,
			&session.Status,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		rounds, err := listRounds(ctx, r.pool, session.ID)
		if err != nil {
			return nil, 0, err
		}
		session.Rounds = rounds
		sessions = append(sessions, session)
	}

	return sessions, total, rows.Err()
}

func (r *SessionRepository) ListExpired(ctx context.Context, cutoff time.Time, statuses []domain.SessionStatus) ([]domain.Session, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	args := []any{cutoff}
	placeholders := make([]string, 0, len(statuses))
	for _, status := range statuses {
		args = append(args, status)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, document_id, document_name, doc_id, origin_path, file_format, file_size_bytes, prompt_profile, status, created_at, updated_at
		FROM sessions
		WHERE updated_at < $1 AND status IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY updated_at ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]domain.Session, 0, 16)
	for rows.Next() {
		var session domain.Session
		if err := rows.Scan(
			&session.ID,
			&session.DocumentID,
			&session.DocumentName,
			&session.DocID,
			&session.OriginPath,
			&session.FileFormat,
			&session.FileSizeBytes,
			&session.PromptProfile,
			&session.Status,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rounds, err := listRounds(ctx, r.pool, session.ID)
		if err != nil {
			return nil, err
		}
		session.Rounds = rounds
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func sessionListOrderBy(sort string) string {
	switch sort {
	case "created_at":
		return "created_at ASC"
	case "-updated_at":
		return "updated_at DESC"
	case "updated_at":
		return "updated_at ASC"
	default:
		return "created_at DESC"
	}
}

func (r *SessionRepository) Delete(ctx context.Context, sessionID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SessionRepository) UpdateStatus(ctx context.Context, sessionID uuid.UUID, status domain.SessionStatus) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET status = $2, updated_at = $3 WHERE id = $1`, sessionID, status, time.Now().UTC())
	return err
}

func (r *SessionRepository) AcquireLock(ctx context.Context, sessionID uuid.UUID) (func(), error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	lockKey := int64(crc32.ChecksumIEEE([]byte(sessionID.String())))
	var acquired bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&acquired); err != nil {
		conn.Release()
		return nil, err
	}
	if !acquired {
		conn.Release()
		return nil, ErrSessionLocked
	}
	return func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey)
		conn.Release()
	}, nil
}

type RoundRepository struct {
	pool *pgxpool.Pool
}

func NewRoundRepository(pool *pgxpool.Pool) *RoundRepository {
	return &RoundRepository{pool: pool}
}

func (r *RoundRepository) Create(ctx context.Context, round *domain.Round) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO rounds (
			id, session_id, round_number, prompt, prompt_profile, input_path, output_path,
			score_total, chunk_limit, input_segment_count, output_segment_count, checkpoint_id,
			provider_used, total_tokens, status, started_at, completed_at, created_at, recovery_justification
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`,
		round.ID, round.SessionID, round.Number, round.Prompt, round.PromptProfile, round.InputPath, round.OutputPath,
		round.ScoreTotal, round.ChunkLimit, round.InputSegmentCount, round.OutputSegmentCount, round.CheckpointID,
		round.ProviderUsed, round.TotalTokens, round.Status, round.StartedAt, round.CompletedAt, round.CreatedAt,
		round.RecoveryJustification,
	)
	return err
}

func (r *RoundRepository) GetBySessionAndNumber(ctx context.Context, sessionID uuid.UUID, number int) (*domain.Round, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, session_id, round_number, prompt, prompt_profile, input_path, output_path,
		       score_total, chunk_limit, input_segment_count, output_segment_count, checkpoint_id,
		       provider_used, total_tokens, status, started_at, completed_at, created_at, recovery_justification
		FROM rounds
		WHERE session_id = $1 AND round_number = $2
	`, sessionID, number)
	return scanRound(row)
}

func (r *RoundRepository) GetActiveBySession(ctx context.Context, sessionID uuid.UUID) (*domain.Round, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, session_id, round_number, prompt, prompt_profile, input_path, output_path,
		       score_total, chunk_limit, input_segment_count, output_segment_count, checkpoint_id,
		       provider_used, total_tokens, status, started_at, completed_at, created_at, recovery_justification
		FROM rounds
		WHERE session_id = $1 AND status IN ('processing','paused')
		ORDER BY round_number DESC
		LIMIT 1
	`, sessionID)
	return scanRound(row)
}

func (r *RoundRepository) Update(ctx context.Context, round *domain.Round) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE rounds
		SET prompt = $2,
		    prompt_profile = $3,
		    input_path = $4,
		    output_path = $5,
		    score_total = $6,
		    chunk_limit = $7,
		    input_segment_count = $8,
		    output_segment_count = $9,
		    checkpoint_id = $10,
		    provider_used = $11,
		    total_tokens = $12,
		    status = $13,
		    started_at = $14,
		    completed_at = $15,
		    recovery_justification = $16
		WHERE id = $1
	`,
		round.ID, round.Prompt, round.PromptProfile, round.InputPath, round.OutputPath, round.ScoreTotal,
		round.ChunkLimit, round.InputSegmentCount, round.OutputSegmentCount, round.CheckpointID,
		round.ProviderUsed, round.TotalTokens, round.Status, round.StartedAt, round.CompletedAt, round.RecoveryJustification,
	)
	return err
}

func (r *RoundRepository) CompleteRound(ctx context.Context, session *domain.Session, round *domain.Round, manifest *domain.Manifest, reports []domain.QualityReport) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	paragraphsJSON, err := json.Marshal(manifest.Paragraphs)
	if err != nil {
		return err
	}
	chunksJSON, err := json.Marshal(manifest.Chunks)
	if err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO manifests (id, round_id, chunk_limit, chunk_metric, paragraph_count, chunk_count, paragraphs, chunks, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (round_id) DO UPDATE SET
			id = EXCLUDED.id,
			chunk_limit = EXCLUDED.chunk_limit,
			chunk_metric = EXCLUDED.chunk_metric,
			paragraph_count = EXCLUDED.paragraph_count,
			chunk_count = EXCLUDED.chunk_count,
			paragraphs = EXCLUDED.paragraphs,
			chunks = EXCLUDED.chunks
	`, manifest.ID, manifest.RoundID, manifest.ChunkLimit, manifest.ChunkMetric, manifest.ParagraphCount, manifest.ChunkCount, paragraphsJSON, chunksJSON, time.Now().UTC()); err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `DELETE FROM quality_reports WHERE round_id = $1`, round.ID); err != nil {
		return err
	}
	for _, report := range reports {
		for _, check := range report.Checks {
			if _, err = tx.Exec(ctx, `
				INSERT INTO quality_reports (
					id, round_id, chunk_id, check_type, passed, reason, recovered, recovery_method, recovery_steps, token_cost, created_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			`,
				uuid.New(), round.ID, report.ChunkID, check.Type, check.Passed, check.Reason,
				report.Recovered, report.RecoveryMethod, report.RecoverySteps, report.TokenCost, time.Now().UTC(),
			); err != nil {
				return err
			}
		}
	}

	if _, err = tx.Exec(ctx, `
		UPDATE rounds
		SET output_path = $2,
		    score_total = $3,
		    input_segment_count = $4,
		    output_segment_count = $5,
		    provider_used = $6,
		    total_tokens = $7,
		    status = $8,
		    completed_at = $9,
		    recovery_justification = $10
		WHERE id = $1
	`,
		round.ID, round.OutputPath, round.ScoreTotal, round.InputSegmentCount, round.OutputSegmentCount,
		round.ProviderUsed, round.TotalTokens, round.Status, round.CompletedAt, round.RecoveryJustification,
	); err != nil {
		return err
	}

	session.Status = domain.DeriveSessionStatus(session)
	if _, err = tx.Exec(ctx, `UPDATE sessions SET status = $2, updated_at = $3 WHERE id = $1`, session.ID, session.Status, time.Now().UTC()); err != nil {
		return err
	}

	err = tx.Commit(ctx)
	return err
}

type ManifestRepository struct {
	pool *pgxpool.Pool
}

func NewManifestRepository(pool *pgxpool.Pool) *ManifestRepository {
	return &ManifestRepository{pool: pool}
}

func (r *ManifestRepository) Create(ctx context.Context, manifest *domain.Manifest) error {
	paragraphsJSON, err := json.Marshal(manifest.Paragraphs)
	if err != nil {
		return err
	}
	chunksJSON, err := json.Marshal(manifest.Chunks)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO manifests (id, round_id, chunk_limit, chunk_metric, paragraph_count, chunk_count, paragraphs, chunks, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (round_id) DO UPDATE SET
			id = EXCLUDED.id,
			chunk_limit = EXCLUDED.chunk_limit,
			chunk_metric = EXCLUDED.chunk_metric,
			paragraph_count = EXCLUDED.paragraph_count,
			chunk_count = EXCLUDED.chunk_count,
			paragraphs = EXCLUDED.paragraphs,
			chunks = EXCLUDED.chunks
	`, manifest.ID, manifest.RoundID, manifest.ChunkLimit, manifest.ChunkMetric, manifest.ParagraphCount, manifest.ChunkCount, paragraphsJSON, chunksJSON, time.Now().UTC())
	return err
}

func (r *ManifestRepository) GetByRoundID(ctx context.Context, roundID uuid.UUID) (*domain.Manifest, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, round_id, chunk_limit, chunk_metric, paragraph_count, chunk_count, paragraphs, chunks
		FROM manifests
		WHERE round_id = $1
	`, roundID)
	var manifest domain.Manifest
	var paragraphsJSON []byte
	var chunksJSON []byte
	if err := row.Scan(
		&manifest.ID, &manifest.RoundID, &manifest.ChunkLimit, &manifest.ChunkMetric,
		&manifest.ParagraphCount, &manifest.ChunkCount, &paragraphsJSON, &chunksJSON,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(paragraphsJSON, &manifest.Paragraphs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(chunksJSON, &manifest.Chunks); err != nil {
		return nil, err
	}
	return &manifest, nil
}

type QualityRepository struct {
	pool *pgxpool.Pool
}

func NewQualityRepository(pool *pgxpool.Pool) *QualityRepository {
	return &QualityRepository{pool: pool}
}

func (r *QualityRepository) ReplaceForRound(ctx context.Context, roundID uuid.UUID, reports []domain.QualityReport) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err = tx.Exec(ctx, `DELETE FROM quality_reports WHERE round_id = $1`, roundID); err != nil {
		return err
	}
	for _, report := range reports {
		for _, check := range report.Checks {
			if _, err = tx.Exec(ctx, `
				INSERT INTO quality_reports (
					id, round_id, chunk_id, check_type, passed, reason, recovered, recovery_method, recovery_steps, token_cost, created_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			`,
				uuid.New(), roundID, report.ChunkID, check.Type, check.Passed, check.Reason, report.Recovered,
				report.RecoveryMethod, report.RecoverySteps, report.TokenCost, time.Now().UTC(),
			); err != nil {
				return err
			}
		}
	}
	err = tx.Commit(ctx)
	return err
}

func (r *QualityRepository) GetStatsByRound(ctx context.Context, roundID uuid.UUID) (*domain.QualityStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT chunk_id, check_type, passed, recovered
		FROM quality_reports
		WHERE round_id = $1
	`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &domain.QualityStats{ByCheckType: map[domain.CheckType]int{}}
	chunkStatus := map[string]struct {
		passed    bool
		recovered bool
	}{}

	for rows.Next() {
		var chunkID string
		var checkType domain.CheckType
		var passed bool
		var recovered bool
		if err := rows.Scan(&chunkID, &checkType, &passed, &recovered); err != nil {
			return nil, err
		}
		state := chunkStatus[chunkID]
		if _, ok := chunkStatus[chunkID]; !ok {
			state.passed = true
		}
		if !passed {
			state.passed = false
			stats.ByCheckType[checkType]++
		}
		if recovered {
			state.recovered = true
		}
		chunkStatus[chunkID] = state
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	stats.TotalChunks = len(chunkStatus)
	for _, state := range chunkStatus {
		switch {
		case state.passed:
			stats.PassedChunks++
		case state.recovered:
			stats.RecoveredChunks++
		default:
			stats.FailedChunks++
		}
	}
	if stats.TotalChunks > 0 {
		stats.PassRate = float64(stats.PassedChunks+stats.RecoveredChunks) / float64(stats.TotalChunks)
		if failed := stats.FailedChunks + stats.RecoveredChunks; failed > 0 {
			stats.RecoveryRate = float64(stats.RecoveredChunks) / float64(failed)
		}
	}
	return stats, nil
}

func listRounds(ctx context.Context, querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, sessionID uuid.UUID) ([]domain.Round, error) {
	rows, err := querier.Query(ctx, `
		SELECT id, session_id, round_number, prompt, prompt_profile, input_path, output_path,
		       score_total, chunk_limit, input_segment_count, output_segment_count, checkpoint_id,
		       provider_used, total_tokens, status, started_at, completed_at, created_at, recovery_justification
		FROM rounds
		WHERE session_id = $1
		ORDER BY round_number ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rounds := make([]domain.Round, 0, 4)
	for rows.Next() {
		round, err := scanRound(rows)
		if err != nil {
			return nil, err
		}
		rounds = append(rounds, *round)
	}
	return rounds, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRound(row rowScanner) (*domain.Round, error) {
	var round domain.Round
	if err := row.Scan(
		&round.ID,
		&round.SessionID,
		&round.Number,
		&round.Prompt,
		&round.PromptProfile,
		&round.InputPath,
		&round.OutputPath,
		&round.ScoreTotal,
		&round.ChunkLimit,
		&round.InputSegmentCount,
		&round.OutputSegmentCount,
		&round.CheckpointID,
		&round.ProviderUsed,
		&round.TotalTokens,
		&round.Status,
		&round.StartedAt,
		&round.CompletedAt,
		&round.CreatedAt,
		&round.RecoveryJustification,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &round, nil
}
