package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type AgentSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewAgentSettingsRepository(pool *pgxpool.Pool) *AgentSettingsRepository {
	return &AgentSettingsRepository{pool: pool}
}

func (r *AgentSettingsRepository) List(ctx context.Context) ([]domain.AgentSetting, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT name, display_name, protocol, base_url, api_key, model, updated_at
		FROM agent_settings
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.AgentSetting, 0, 3)
	for rows.Next() {
		var s domain.AgentSetting
		if err := rows.Scan(&s.Name, &s.DisplayName, &s.Protocol, &s.BaseURL, &s.APIKey, &s.Model, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *AgentSettingsRepository) Get(ctx context.Context, name domain.AgentName) (domain.AgentSetting, error) {
	var s domain.AgentSetting
	row := r.pool.QueryRow(ctx, `
		SELECT name, display_name, protocol, base_url, api_key, model, updated_at
		FROM agent_settings WHERE name = $1
	`, name)
	if err := row.Scan(&s.Name, &s.DisplayName, &s.Protocol, &s.BaseURL, &s.APIKey, &s.Model, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AgentSetting{}, ErrNotFound
		}
		return domain.AgentSetting{}, err
	}
	return s, nil
}

func (r *AgentSettingsRepository) Upsert(ctx context.Context, s domain.AgentSetting) error {
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agent_settings (name, display_name, protocol, base_url, api_key, model, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (name) DO UPDATE SET
		    display_name = EXCLUDED.display_name,
		    protocol     = EXCLUDED.protocol,
		    base_url     = EXCLUDED.base_url,
		    api_key      = EXCLUDED.api_key,
		    model        = EXCLUDED.model,
		    updated_at   = EXCLUDED.updated_at
	`, s.Name, s.DisplayName, s.Protocol, s.BaseURL, s.APIKey, s.Model, s.UpdatedAt)
	return err
}

func (r *AgentSettingsRepository) SeedIfEmpty(ctx context.Context, defaults []domain.AgentSetting) error {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM agent_settings`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, s := range defaults {
		if err := r.Upsert(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
