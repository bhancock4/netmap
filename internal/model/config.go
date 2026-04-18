package model

import (
	"fmt"
	"time"
)

// Config holds the runtime configuration for a scan.
type Config struct {
	Target  string
	Depth   int
	Breadth int
	Timeout time.Duration
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Depth:   3,
		Breadth: 10,
		Timeout: 5 * time.Minute,
	}
}

// Validate checks the config for invalid values and clamps to safe ranges.
func (c *Config) Validate() error {
	if c.Target == "" {
		return fmt.Errorf("target is required")
	}
	if c.Depth < 1 {
		c.Depth = 1
	}
	if c.Depth > 5 {
		c.Depth = 5
	}
	if c.Breadth < 1 {
		c.Breadth = 1
	}
	if c.Breadth > 50 {
		c.Breadth = 50
	}
	if c.Timeout < 5*time.Second {
		c.Timeout = 5 * time.Second
	}
	if c.Timeout > 30*time.Minute {
		c.Timeout = 30 * time.Minute
	}
	return nil
}
