package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadPProfConfiguration(t *testing.T) {
	t.Setenv("ENV_FILE", t.TempDir()+"/missing.env")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PPROF_ENABLED", "true")
	t.Setenv("PPROF_PORT", "7070")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.PProfEnabled {
		t.Fatal("PProfEnabled = false, want true")
	}
	if cfg.PProfPort != 7070 {
		t.Fatalf("PProfPort = %d, want 7070", cfg.PProfPort)
	}
	if cfg.PProfAddr() != ":7070" {
		t.Fatalf("PProfAddr() = %q, want %q", cfg.PProfAddr(), ":7070")
	}
}

func TestLoadRejectsInvalidPProfBoolean(t *testing.T) {
	t.Setenv("ENV_FILE", t.TempDir()+"/missing.env")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PPROF_ENABLED", "sometimes")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PPROF_ENABLED must be a boolean") {
		t.Fatalf("Load() error = %v, want PPROF_ENABLED boolean error", err)
	}
}

func TestValidatePProfConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "port outside range",
			cfg:  validConfig(70000),
			want: "PPROF_PORT must be between 1 and 65535",
		},
		{
			name: "same as HTTP port",
			cfg:  validConfig(8080),
			want: "PPROF_PORT must differ from HTTP_PORT",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.cfg.validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func validConfig(pprofPort int) Config {
	return Config{
		AppMode:      AppModeTest,
		HTTPPort:     8080,
		DatabaseURL:  "postgres://example",
		RedisTTL:     time.Minute,
		PProfEnabled: true,
		PProfPort:    pprofPort,
	}
}
