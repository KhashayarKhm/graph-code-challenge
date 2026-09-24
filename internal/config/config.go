package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type AppMode string

const (
	AppModeProduction  AppMode = "production"
	AppModeDevelopment AppMode = "development"
	AppModeTest        AppMode = "test"
)

func (am AppMode) IsValid() bool {
	switch am {
	case AppModeDevelopment, AppModeProduction, AppModeTest:
		return true
	default:
		return false
	}
}

const (
	DefaultEnvFile   = ".env"
	DefaultRedisTTL  = time.Minute
	DefaultPProfPort = 6060
)

type Config struct {
	AppMode      AppMode
	HTTPPort     int
	DatabaseURL  string
	RedisURL     string
	RedisTTL     time.Duration
	PProfEnabled bool
	PProfPort    int
}

func Load() (Config, error) {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envFile = DefaultEnvFile
	}

	if err := godotenv.Load(envFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("config: load %s: %w", envFile, err)
	}

	mode, err := envAppMode("APP_MODE", AppModeDevelopment)
	if err != nil {
		return Config{}, err
	}

	port, err := envInt("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}

	redisTTL, err := envDuration("REDIS_TTL", DefaultRedisTTL)
	if err != nil {
		return Config{}, err
	}

	pprofEnabled, err := envBool("PPROF_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	pprofPort, err := envInt("PPROF_PORT", DefaultPProfPort)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppMode:      mode,
		HTTPPort:     port,
		DatabaseURL:  envStr("DATABASE_URL", ""),
		RedisURL:     envStr("REDIS_URL", ""),
		RedisTTL:     redisTTL,
		PProfEnabled: pprofEnabled,
		PProfPort:    pprofPort,
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.HTTPPort)
}

func (c Config) PProfAddr() string {
	return fmt.Sprintf(":%d", c.PProfPort)
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return errors.New("config: DATABASE_URL is required (set it in .env or the environment)")
	}

	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		return fmt.Errorf("config: HTTP_PORT must be between 1 and 65535, got %d", c.HTTPPort)
	}

	if c.RedisURL != "" && c.RedisTTL <= 0 {
		return fmt.Errorf("config: REDIS_TTL must be positive, got %s", c.RedisTTL)
	}

	if c.PProfEnabled && (c.PProfPort < 1 || c.PProfPort > 65535) {
		return fmt.Errorf("config: PPROF_PORT must be between 1 and 65535, got %d", c.PProfPort)
	}

	if c.PProfEnabled && c.PProfPort == c.HTTPPort {
		return errors.New("config: PPROF_PORT must differ from HTTP_PORT")
	}

	return nil
}

func envAppMode(key string, def AppMode) (AppMode, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	mode := AppMode(raw)
	if !mode.IsValid() {
		return "", fmt.Errorf("config: %s must be one of development, production, test, got %q", key, raw)
	}

	return mode, nil
}

func envStr(key, def string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return def
}

func envInt(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q", key, raw)
	}

	return value, nil
}

func envBool(key string, def bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("config: %s must be a boolean, got %q", key, raw)
	}

	return value, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be a duration such as 60s or 5m, got %q", key, raw)
	}

	return value, nil
}
