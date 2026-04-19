package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iwen-conf/Naturalize/internal/agent"
	"github.com/iwen-conf/Naturalize/internal/domain"
	illm "github.com/iwen-conf/Naturalize/internal/infra/llm"
	ipostgres "github.com/iwen-conf/Naturalize/internal/infra/postgres"
)

type agentSettingsRepo interface {
	List(ctx context.Context) ([]domain.AgentSetting, error)
	Get(ctx context.Context, name domain.AgentName) (domain.AgentSetting, error)
	Upsert(ctx context.Context, s domain.AgentSetting) error
}

type AgentSettingsService struct {
	repo     agentSettingsRepo
	registry *agent.Registry
}

func NewAgentSettingsService(repo agentSettingsRepo, registry *agent.Registry) *AgentSettingsService {
	return &AgentSettingsService{repo: repo, registry: registry}
}

type AgentUpdateInput struct {
	Protocol domain.AgentProtocol
	BaseURL  string
	APIKey   string
	Model    string
}

func (s *AgentSettingsService) List(ctx context.Context) ([]domain.AgentSetting, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].APIKey = maskAPIKey(items[i].APIKey)
	}
	return items, nil
}

func (s *AgentSettingsService) Update(ctx context.Context, name domain.AgentName, input AgentUpdateInput) (domain.AgentSetting, error) {
	if !name.Valid() {
		return domain.AgentSetting{}, ErrAgentNotFound
	}
	if !input.Protocol.Valid() {
		return domain.AgentSetting{}, fmt.Errorf("%w: protocol must be responses or chat", ErrInvalidRequest)
	}
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.Model = strings.TrimSpace(input.Model)
	if input.BaseURL == "" {
		return domain.AgentSetting{}, fmt.Errorf("%w: base url required", ErrInvalidRequest)
	}

	current, err := s.repo.Get(ctx, name)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return domain.AgentSetting{}, ErrAgentNotFound
		}
		return domain.AgentSetting{}, err
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		apiKey = current.APIKey
	}

	next := domain.AgentSetting{
		Name:        current.Name,
		DisplayName: current.DisplayName,
		Protocol:    input.Protocol,
		BaseURL:     input.BaseURL,
		APIKey:      apiKey,
		Model:       input.Model,
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Upsert(ctx, next); err != nil {
		return domain.AgentSetting{}, err
	}
	s.registry.Update(next)

	response := next
	response.APIKey = maskAPIKey(response.APIKey)
	return response, nil
}

type FetchModelsInput struct {
	BaseURL string
	APIKey  string
}

func (s *AgentSettingsService) FetchModels(ctx context.Context, name domain.AgentName, input FetchModelsInput) ([]string, error) {
	if !name.Valid() {
		return nil, ErrAgentNotFound
	}
	current, err := s.repo.Get(ctx, name)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		baseURL = current.BaseURL
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		apiKey = current.APIKey
	}
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("%w: base url and api key required", ErrInvalidRequest)
	}
	return illm.ListModels(ctx, baseURL, apiKey, "", "", 30*time.Second)
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}
