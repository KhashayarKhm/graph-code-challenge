package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

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

const DefaultEnvFile = ".env"

type Config struct {
	AppMode     AppMode
	HTTPPort    int
	DatabaseURL string
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

	cfg := Config{
		AppMode:     mode,
		HTTPPort:    port,
		DatabaseURL: envStr("DATABASE_URL", ""),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.HTTPPort)
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return errors.New("config: DATABASE_URL is required (set it in .env or the environment)")
	}

	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		return fmt.Errorf("config: HTTP_PORT must be between 1 and 65535, got %d", c.HTTPPort)
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
