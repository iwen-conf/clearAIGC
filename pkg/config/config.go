package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig
	HTTP      HTTPConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Storage   StorageConfig
	Scoring   ScoringConfig
	Providers ProviderSettings
}

type AppConfig struct {
	Name        string
	Environment string
}

type HTTPConfig struct {
	Listen         string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	ShutdownGrace  time.Duration
	AllowedOrigins []string
}

type DatabaseConfig struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

type RedisConfig struct {
	Addr          string
	Password      string
	DB            int
	CheckpointTTL time.Duration
	PubSubPrefix  string
}

type StorageConfig struct {
	RootDir string
}

type ScoringConfig struct {
	CalibrationPath     string
	CalibrationMinPairs int
	CalibrationWindow   int
	DetectorCacheTTL    time.Duration
	SemanticThreshold   float64
	ReadabilityMaxScore float64
}

type ProviderSettings struct {
	Rewrite    RewriteProviderConfig
	Fallback   ChatProviderConfig
	Agent      ChatProviderConfig
	Detector   ChatProviderConfig
	Embeddings EmbeddingProviderConfig
}

type RewriteProviderConfig struct {
	Name         string
	BaseURL      string
	APIKey       string
	Model        string
	Organization string
	Project      string
	Store        bool
	Timeout      time.Duration
	RPMLimit     int
	TPMLimit     int
}

type ChatProviderConfig struct {
	Name         string
	BaseURL      string
	APIKey       string
	Model        string
	Organization string
	Project      string
	Timeout      time.Duration
	RPMLimit     int
	TPMLimit     int
}

type EmbeddingProviderConfig struct {
	Name         string
	BaseURL      string
	APIKey       string
	Model        string
	Organization string
	Project      string
	Timeout      time.Duration
	RPMLimit     int
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./deploy")
	v.SetEnvPrefix("NATURALIZE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)
	_ = v.ReadInConfig()

	cfg := Config{
		App: AppConfig{
			Name:        v.GetString("app.name"),
			Environment: v.GetString("app.environment"),
		},
		HTTP: HTTPConfig{
			Listen:         v.GetString("http.listen"),
			ReadTimeout:    v.GetDuration("http.read_timeout"),
			WriteTimeout:   v.GetDuration("http.write_timeout"),
			ShutdownGrace:  v.GetDuration("http.shutdown_grace"),
			AllowedOrigins: v.GetStringSlice("http.allowed_origins"),
		},
		Database: DatabaseConfig{
			DSN:             v.GetString("database.dsn"),
			MaxConns:        int32(v.GetInt("database.max_conns")),
			MinConns:        int32(v.GetInt("database.min_conns")),
			MaxConnLifetime: v.GetDuration("database.max_conn_lifetime"),
		},
		Redis: RedisConfig{
			Addr:          v.GetString("redis.addr"),
			Password:      v.GetString("redis.password"),
			DB:            v.GetInt("redis.db"),
			CheckpointTTL: v.GetDuration("redis.checkpoint_ttl"),
			PubSubPrefix:  v.GetString("redis.pubsub_prefix"),
		},
		Storage: StorageConfig{
			RootDir: v.GetString("storage.root_dir"),
		},
		Scoring: ScoringConfig{
			CalibrationPath:     v.GetString("scoring.calibration_path"),
			CalibrationMinPairs: v.GetInt("scoring.calibration_min_pairs"),
			CalibrationWindow:   v.GetInt("scoring.calibration_window"),
			DetectorCacheTTL:    v.GetDuration("scoring.detector_cache_ttl"),
			SemanticThreshold:   v.GetFloat64("scoring.semantic_threshold"),
			ReadabilityMaxScore: v.GetFloat64("scoring.readability_max_score"),
		},
		Providers: ProviderSettings{
			Rewrite: RewriteProviderConfig{
				Name:         v.GetString("providers.rewrite.name"),
				BaseURL:      v.GetString("providers.rewrite.base_url"),
				APIKey:       v.GetString("providers.rewrite.api_key"),
				Model:        v.GetString("providers.rewrite.model"),
				Organization: v.GetString("providers.rewrite.organization"),
				Project:      v.GetString("providers.rewrite.project"),
				Store:        v.GetBool("providers.rewrite.store"),
				Timeout:      v.GetDuration("providers.rewrite.timeout"),
				RPMLimit:     v.GetInt("providers.rewrite.rpm_limit"),
				TPMLimit:     v.GetInt("providers.rewrite.tpm_limit"),
			},
			Fallback: ChatProviderConfig{
				Name:         v.GetString("providers.fallback.name"),
				BaseURL:      v.GetString("providers.fallback.base_url"),
				APIKey:       v.GetString("providers.fallback.api_key"),
				Model:        v.GetString("providers.fallback.model"),
				Organization: v.GetString("providers.fallback.organization"),
				Project:      v.GetString("providers.fallback.project"),
				Timeout:      v.GetDuration("providers.fallback.timeout"),
				RPMLimit:     v.GetInt("providers.fallback.rpm_limit"),
				TPMLimit:     v.GetInt("providers.fallback.tpm_limit"),
			},
			Agent: ChatProviderConfig{
				Name:         v.GetString("providers.agent.name"),
				BaseURL:      v.GetString("providers.agent.base_url"),
				APIKey:       v.GetString("providers.agent.api_key"),
				Model:        v.GetString("providers.agent.model"),
				Organization: v.GetString("providers.agent.organization"),
				Project:      v.GetString("providers.agent.project"),
				Timeout:      v.GetDuration("providers.agent.timeout"),
				RPMLimit:     v.GetInt("providers.agent.rpm_limit"),
				TPMLimit:     v.GetInt("providers.agent.tpm_limit"),
			},
			Detector: ChatProviderConfig{
				Name:         v.GetString("providers.detector.name"),
				BaseURL:      v.GetString("providers.detector.base_url"),
				APIKey:       v.GetString("providers.detector.api_key"),
				Model:        v.GetString("providers.detector.model"),
				Organization: v.GetString("providers.detector.organization"),
				Project:      v.GetString("providers.detector.project"),
				Timeout:      v.GetDuration("providers.detector.timeout"),
				RPMLimit:     v.GetInt("providers.detector.rpm_limit"),
				TPMLimit:     v.GetInt("providers.detector.tpm_limit"),
			},
			Embeddings: EmbeddingProviderConfig{
				Name:         v.GetString("providers.embeddings.name"),
				BaseURL:      v.GetString("providers.embeddings.base_url"),
				APIKey:       v.GetString("providers.embeddings.api_key"),
				Model:        v.GetString("providers.embeddings.model"),
				Organization: v.GetString("providers.embeddings.organization"),
				Project:      v.GetString("providers.embeddings.project"),
				Timeout:      v.GetDuration("providers.embeddings.timeout"),
				RPMLimit:     v.GetInt("providers.embeddings.rpm_limit"),
			},
		},
	}

	if strings.TrimSpace(cfg.Scoring.CalibrationPath) == "" {
		cfg.Scoring.CalibrationPath = filepath.Join(cfg.Storage.RootDir, "calibration", "detector-state.json")
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	missing := make([]string, 0, 8)
	if c.Database.DSN == "" {
		missing = append(missing, "database.dsn")
	}
	if c.Redis.Addr == "" {
		missing = append(missing, "redis.addr")
	}
	if c.Storage.RootDir == "" {
		missing = append(missing, "storage.root_dir")
	}
	if c.Providers.Rewrite.APIKey == "" {
		missing = append(missing, "providers.rewrite.api_key")
	}
	if c.Providers.Rewrite.Model == "" {
		missing = append(missing, "providers.rewrite.model")
	}
	if c.Providers.Fallback.APIKey == "" {
		missing = append(missing, "providers.fallback.api_key")
	}
	if c.Providers.Fallback.Model == "" {
		missing = append(missing, "providers.fallback.model")
	}
	if c.Providers.Agent.APIKey == "" {
		missing = append(missing, "providers.agent.api_key")
	}
	if c.Providers.Agent.Model == "" {
		missing = append(missing, "providers.agent.model")
	}
	if err := validateOptionalChatProvider("providers.detector", c.Providers.Detector); err != nil {
		return err
	}
	if err := validateOptionalEmbeddingProvider("providers.embeddings", c.Providers.Embeddings); err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "Naturalize")
	v.SetDefault("app.environment", "development")

	v.SetDefault("http.listen", ":8080")
	v.SetDefault("http.read_timeout", 30*time.Second)
	v.SetDefault("http.write_timeout", 0*time.Second)
	v.SetDefault("http.shutdown_grace", 10*time.Second)
	v.SetDefault("http.allowed_origins", []string{"*"})

	v.SetDefault("database.max_conns", 10)
	v.SetDefault("database.min_conns", 1)
	v.SetDefault("database.max_conn_lifetime", time.Hour)

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.checkpoint_ttl", 24*time.Hour)
	v.SetDefault("redis.pubsub_prefix", "naturalize")

	v.SetDefault("storage.root_dir", "./var/data")
	v.SetDefault("scoring.calibration_min_pairs", 8)
	v.SetDefault("scoring.calibration_window", 128)
	v.SetDefault("scoring.detector_cache_ttl", 24*time.Hour)
	v.SetDefault("scoring.semantic_threshold", 0.82)
	v.SetDefault("scoring.readability_max_score", 0.24)

	v.SetDefault("providers.rewrite.name", "openai-responses")
	v.SetDefault("providers.rewrite.base_url", "https://api.openai.com")
	v.SetDefault("providers.rewrite.store", false)
	v.SetDefault("providers.rewrite.timeout", 90*time.Second)
	v.SetDefault("providers.rewrite.rpm_limit", 60)
	v.SetDefault("providers.rewrite.tpm_limit", 200000)

	v.SetDefault("providers.fallback.name", "openai-chat")
	v.SetDefault("providers.fallback.base_url", "https://api.openai.com")
	v.SetDefault("providers.fallback.timeout", 90*time.Second)
	v.SetDefault("providers.fallback.rpm_limit", 60)
	v.SetDefault("providers.fallback.tpm_limit", 200000)

	v.SetDefault("providers.agent.name", "openai-agent")
	v.SetDefault("providers.agent.base_url", "https://api.openai.com")
	v.SetDefault("providers.agent.timeout", 90*time.Second)
	v.SetDefault("providers.agent.rpm_limit", 60)
	v.SetDefault("providers.agent.tpm_limit", 200000)

	v.SetDefault("providers.detector.name", "openai-detector")
	v.SetDefault("providers.detector.base_url", "https://api.openai.com")
	v.SetDefault("providers.detector.timeout", 60*time.Second)
	v.SetDefault("providers.detector.rpm_limit", 30)
	v.SetDefault("providers.detector.tpm_limit", 120000)

	v.SetDefault("providers.embeddings.name", "openai-embeddings")
	v.SetDefault("providers.embeddings.base_url", "https://api.openai.com")
	v.SetDefault("providers.embeddings.timeout", 30*time.Second)
	v.SetDefault("providers.embeddings.rpm_limit", 120)
}

func validateOptionalChatProvider(prefix string, provider ChatProviderConfig) error {
	fields := map[string]string{
		"api_key": provider.APIKey,
		"model":   provider.Model,
	}
	return validateOptionalProvider(prefix, fields)
}

func validateOptionalEmbeddingProvider(prefix string, provider EmbeddingProviderConfig) error {
	fields := map[string]string{
		"api_key": provider.APIKey,
		"model":   provider.Model,
	}
	return validateOptionalProvider(prefix, fields)
}

func validateOptionalProvider(prefix string, fields map[string]string) error {
	enabled := false
	missing := make([]string, 0, len(fields))
	for name, value := range fields {
		if strings.TrimSpace(value) != "" {
			enabled = true
			continue
		}
		missing = append(missing, prefix+"."+name)
	}
	if !enabled {
		return nil
	}
	if len(missing) > 0 {
		return fmt.Errorf("incomplete optional provider configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}
