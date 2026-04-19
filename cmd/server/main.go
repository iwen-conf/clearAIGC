package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iwen-conf/Naturalize/internal/agent"
	"github.com/iwen-conf/Naturalize/internal/api"
	apihandler "github.com/iwen-conf/Naturalize/internal/api/handler"
	"github.com/iwen-conf/Naturalize/internal/domain"
	illm "github.com/iwen-conf/Naturalize/internal/infra/llm"
	ipostgres "github.com/iwen-conf/Naturalize/internal/infra/postgres"
	iredis "github.com/iwen-conf/Naturalize/internal/infra/redis"
	"github.com/iwen-conf/Naturalize/internal/service"
	"github.com/iwen-conf/Naturalize/internal/workflow"
	"github.com/iwen-conf/Naturalize/internal/workflow/nodes"
	"github.com/iwen-conf/Naturalize/pkg/config"
	"github.com/iwen-conf/Naturalize/pkg/storage"
)

var version = "dev"
var buildTime = "unknown"

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	layout := storage.NewLayout(cfg.Storage.RootDir)
	if err := layout.Ensure(); err != nil {
		log.Fatalf("prepare storage: %v", err)
	}

	db, err := connectDatabaseWithRetry(ctx, cfg.Database, 45*time.Second)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()
	if err := ipostgres.RunMigrations(ctx, db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	redisClient, err := connectRedisWithRetry(ctx, cfg.Redis, 30*time.Second)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	checkpointStore := iredis.NewCheckpointStore(redisClient, cfg.Redis.CheckpointTTL)
	progressPublisher := iredis.NewProgressPublisher(redisClient, cfg.Redis.PubSubPrefix)

	builder := workflow.NewPromptBuilder()
	sessionRepo := ipostgres.NewSessionRepository(db)
	roundRepo := ipostgres.NewRoundRepository(db)
	manifestRepo := ipostgres.NewManifestRepository(db)
	qualityRepo := ipostgres.NewQualityRepository(db)
	agentSettingsRepo := ipostgres.NewAgentSettingsRepository(db)

	if err := agentSettingsRepo.SeedIfEmpty(ctx, defaultAgentSettings(cfg)); err != nil {
		log.Fatalf("seed agent settings: %v", err)
	}
	agentSettings, err := agentSettingsRepo.List(ctx)
	if err != nil {
		log.Fatalf("load agent settings: %v", err)
	}
	agentRegistry := agent.NewRegistry(agentSettings)

	rewriteClient := illm.NewResponsesClient(domain.ProviderConfig{
		Name:         cfg.Providers.Rewrite.Name,
		Mode:         domain.ProviderModeResponses,
		BaseURL:      cfg.Providers.Rewrite.BaseURL,
		APIKey:       cfg.Providers.Rewrite.APIKey,
		Model:        cfg.Providers.Rewrite.Model,
		Organization: cfg.Providers.Rewrite.Organization,
		Project:      cfg.Providers.Rewrite.Project,
		Timeout:      cfg.Providers.Rewrite.Timeout,
		RPMLimit:     cfg.Providers.Rewrite.RPMLimit,
		TPMLimit:     cfg.Providers.Rewrite.TPMLimit,
		Store:        cfg.Providers.Rewrite.Store,
	})
	fallbackClient := illm.NewChatClient(domain.ProviderConfig{
		Name:         cfg.Providers.Fallback.Name,
		Mode:         domain.ProviderModeChat,
		BaseURL:      cfg.Providers.Fallback.BaseURL,
		APIKey:       cfg.Providers.Fallback.APIKey,
		Model:        cfg.Providers.Fallback.Model,
		Organization: cfg.Providers.Fallback.Organization,
		Project:      cfg.Providers.Fallback.Project,
		Timeout:      cfg.Providers.Fallback.Timeout,
		RPMLimit:     cfg.Providers.Fallback.RPMLimit,
		TPMLimit:     cfg.Providers.Fallback.TPMLimit,
	})
	providerChain := illm.NewProviderChain(rewriteClient, fallbackClient)

	manager := service.NewExecutionManager()
	recoverySupervisor := agent.NewSupervisor(einoopenai.ChatModelConfig{
		APIKey:  cfg.Providers.Agent.APIKey,
		BaseURL: cfg.Providers.Agent.BaseURL,
		Model:   cfg.Providers.Agent.Model,
		Timeout: cfg.Providers.Agent.Timeout,
	}, providerChain, builder)

	lexicalAgent := agent.NewLexicalMutator(agentRegistry)
	syntaxAgent := agent.NewSyntaxRebuilder(agentRegistry)
	coordinator := agent.NewCoordinator(agentRegistry, lexicalAgent, syntaxAgent)

	exporter := nodes.NewExporter()
	pipeline, err := workflow.NewPipeline(
		builder,
		nodes.NewParser(),
		nodes.NewChunker(),
		nodes.NewQualityGate(),
		exporter,
		providerChain,
		coordinator,
		recoverySupervisor,
		checkpointStore,
		progressPublisher,
		manager,
	)
	if err != nil {
		log.Fatalf("build pipeline: %v", err)
	}

	svc := service.NewService(
		sessionRepo,
		roundRepo,
		manifestRepo,
		qualityRepo,
		pipeline,
		progressPublisher,
		checkpointStore,
		exporter,
		layout,
		manager,
		builder,
	)

	agentSettingsService := service.NewAgentSettingsService(agentSettingsRepo, agentRegistry)

	healthHandler := api.NewHealthHandler(db, redisClient, version, buildTime)
	handler := apihandler.New(svc, agentSettingsService, healthHandler)
	router := api.NewRouter(handler, cfg.HTTP.AllowedOrigins)

	server := &http.Server{
		Addr:         cfg.HTTP.Listen,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownGrace)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
}

func defaultAgentSettings(cfg config.Config) []domain.AgentSetting {
	return []domain.AgentSetting{
		{
			Name:        domain.AgentCoordinator,
			DisplayName: "Rewrite Coordinator",
			Protocol:    domain.AgentProtocolChat,
			BaseURL:     cfg.Providers.Agent.BaseURL,
			APIKey:      cfg.Providers.Agent.APIKey,
			Model:       cfg.Providers.Agent.Model,
		},
		{
			Name:        domain.AgentLexicalMutator,
			DisplayName: "Lexical Mutator",
			Protocol:    domain.AgentProtocolChat,
		},
		{
			Name:        domain.AgentSyntaxRebuilder,
			DisplayName: "Syntax Rebuilder",
			Protocol:    domain.AgentProtocolChat,
		},
	}
}

func connectDatabaseWithRetry(ctx context.Context, cfg config.DatabaseConfig, timeout time.Duration) (*pgxpool.Pool, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := ipostgres.Connect(ctx, cfg)
		if err == nil {
			return db, nil
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("database not ready within %s: %w", timeout, lastErr)
}

func connectRedisWithRetry(ctx context.Context, cfg config.RedisConfig, timeout time.Duration) (*iredis.Client, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		client := iredis.Connect(cfg)
		if err := client.Ping(ctx).Err(); err == nil {
			return client, nil
		} else {
			lastErr = err
			_ = client.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("redis not ready within %s: %w", timeout, lastErr)
}
