package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Limits shared with the scan command.
const (
	MaxEngines = 213
	MaxTimeout = 300
)

// Config holds the saved defaults that `administer scan` uses when the
// matching flag is not given.
type Config struct {
	Engines int     `json:"engines"`
	Timeout int     `json:"timeout_seconds"`
	Rate    float64 `json:"rate_per_second"`
	Color   bool    `json:"color"`
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		Engines: 20,
		Timeout: 10,
		Rate:    0,
		Color:   true,
	}
}

// Path returns where the config file lives.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "administer", "config.json"), nil
}

// Load reads the saved config. A missing file means defaults, and any field
// missing from the file keeps its default value. Out-of-range values fall
// back to the default instead of reaching the scan.
func Load() (Config, error) {
	cfg := Default()
	path, err := Path()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Default(), err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg.normalized(), nil
}

func (c Config) normalized() Config {
	d := Default()
	if c.Engines < 1 || c.Engines > MaxEngines {
		c.Engines = d.Engines
	}
	if c.Timeout < 1 || c.Timeout > MaxTimeout {
		c.Timeout = d.Timeout
	}
	if c.Rate < 0 {
		c.Rate = d.Rate
	}
	return c
}

// Set returns a copy of the config with one key changed, or an error that
// explains what a valid value looks like.
func (c Config) Set(key, value string) (Config, error) {
	switch key {
	case "engines":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > MaxEngines {
			return c, fmt.Errorf("engines must be a whole number from 1 to %d", MaxEngines)
		}
		c.Engines = n
	case "timeout":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > MaxTimeout {
			return c, fmt.Errorf("timeout must be a whole number of seconds from 1 to %d", MaxTimeout)
		}
		c.Timeout = n
	case "rate":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil || !(f >= 0) || f > 100000 {
			return c, fmt.Errorf("rate must be requests per second from 0 to 100000 (0 = unlimited)")
		}
		c.Rate = f
	case "color":
		switch strings.ToLower(value) {
		case "on", "true", "yes", "1":
			c.Color = true
		case "off", "false", "no", "0":
			c.Color = false
		default:
			return c, fmt.Errorf("color must be on or off")
		}
	default:
		return c, fmt.Errorf("unknown key %q (valid keys: engines, timeout, rate, color)", key)
	}
	return c, nil
}

// Save writes the config atomically (write to a temp file, then
// rename) so a crash mid-write can never leave a corrupt config file.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
