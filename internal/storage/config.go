package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds application configuration.
type Config struct {
	QuickAddFolder string `json:"quickAddFolder"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		QuickAddFolder: "Read Later",
	}
}

// LoadConfig reads config from the JSON file.
// Returns default config if the file doesn't exist.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			config := DefaultConfig()
			return &config, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Apply defaults for missing fields
	if config.QuickAddFolder == "" {
		config.QuickAddFolder = DefaultConfig().QuickAddFolder
	}

	return &config, nil
}

// SaveConfig writes config to the JSON file.
// Creates the directory if it doesn't exist.
func SaveConfig(path string, config *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// DefaultConfigFilePath returns the default config path: ~/.config/bm/config.json
func DefaultConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "bm", "config.json"), nil
}
