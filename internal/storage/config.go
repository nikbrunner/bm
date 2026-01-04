package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds application configuration.
type Config struct {
	QuickAddFolder     string   `json:"quickAddFolder"`
	CullExcludeDomains []string `json:"cullExcludeDomains"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		QuickAddFolder:     "Read Later",
		CullExcludeDomains: []string{"github.com", "gitlab.com"},
	}
}

// LoadConfig reads config from the JSON file.
// Creates the file with defaults if it doesn't exist.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			config := DefaultConfig()
			// Create the config file with defaults
			if saveErr := SaveConfig(path, &config); saveErr != nil {
				// Non-fatal: return defaults even if save fails
				return &config, nil
			}
			return &config, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Apply defaults for missing fields
	defaults := DefaultConfig()
	if config.QuickAddFolder == "" {
		config.QuickAddFolder = defaults.QuickAddFolder
	}
	if config.CullExcludeDomains == nil {
		config.CullExcludeDomains = defaults.CullExcludeDomains
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
