package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/nikbrunner/bm/internal/model"
)

// Storage defines the interface for persisting bookmarks.
type Storage interface {
	Load() (*model.Store, error)
	Save(store *model.Store) error
}

// JSONStorage implements Storage using a JSON file.
type JSONStorage struct {
	path string
}

// NewJSONStorage creates a new JSONStorage with the given file path.
func NewJSONStorage(path string) *JSONStorage {
	return &JSONStorage{path: path}
}

// Path returns the storage file path.
func (s *JSONStorage) Path() string {
	return s.path
}

// Load reads the store from the JSON file.
// Returns an empty store if the file doesn't exist.
func (s *JSONStorage) Load() (*model.Store, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return empty store for missing file
			return &model.Store{
				Folders:   []model.Folder{},
				Bookmarks: []model.Bookmark{},
			}, nil
		}
		return nil, err
	}

	var store model.Store
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	// Ensure slices are not nil
	if store.Folders == nil {
		store.Folders = []model.Folder{}
	}
	if store.Bookmarks == nil {
		store.Bookmarks = []model.Bookmark{}
	}

	return &store, nil
}

// Save writes the store to the JSON file.
// Creates the directory if it doesn't exist.
func (s *JSONStorage) Save(store *model.Store) error {
	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

// DefaultConfigPath returns the default config path: ~/.config/bm/bookmarks.json
func DefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "bm", "bookmarks.json"), nil
}

// OpenStorage opens the appropriate storage backend.
// Prefers SQLite if the database file exists, otherwise falls back to JSON.
func OpenStorage() (Storage, error) {
	sqlitePath, err := DefaultSQLitePath()
	if err != nil {
		return nil, err
	}

	// If SQLite database exists, use it
	if _, err := os.Stat(sqlitePath); err == nil {
		return NewSQLiteStorage(sqlitePath)
	}

	// Fall back to JSON
	jsonPath, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return NewJSONStorage(jsonPath), nil
}
