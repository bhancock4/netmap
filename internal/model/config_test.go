package model

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		check   func(Config) bool
	}{
		{
			name:    "empty target",
			cfg:     Config{Target: "", Depth: 3, Breadth: 10, Timeout: 5 * time.Minute},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg:  Config{Target: "example.com", Depth: 3, Breadth: 10, Timeout: 5 * time.Minute},
		},
		{
			name: "negative depth clamped to 1",
			cfg:  Config{Target: "example.com", Depth: -5, Breadth: 10, Timeout: 5 * time.Minute},
			check: func(c Config) bool {
				return c.Depth == 1
			},
		},
		{
			name: "excessive depth clamped to 5",
			cfg:  Config{Target: "example.com", Depth: 100, Breadth: 10, Timeout: 5 * time.Minute},
			check: func(c Config) bool {
				return c.Depth == 5
			},
		},
		{
			name: "zero breadth clamped to 1",
			cfg:  Config{Target: "example.com", Depth: 3, Breadth: 0, Timeout: 5 * time.Minute},
			check: func(c Config) bool {
				return c.Breadth == 1
			},
		},
		{
			name: "excessive breadth clamped to 50",
			cfg:  Config{Target: "example.com", Depth: 3, Breadth: 200, Timeout: 5 * time.Minute},
			check: func(c Config) bool {
				return c.Breadth == 50
			},
		},
		{
			name: "tiny timeout clamped to 5s",
			cfg:  Config{Target: "example.com", Depth: 3, Breadth: 10, Timeout: 1 * time.Second},
			check: func(c Config) bool {
				return c.Timeout == 5*time.Second
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.check != nil && !tt.check(tt.cfg) {
				t.Errorf("post-validation check failed for config: %+v", tt.cfg)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Depth != 3 || cfg.Breadth != 10 || cfg.Timeout != 5*time.Minute {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}
