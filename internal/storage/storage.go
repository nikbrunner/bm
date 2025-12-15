package storage

import (
	"github.com/nikbrunner/bm/internal/model"
)

// Storage defines the interface for persisting bookmarks.
type Storage interface {
	Load() (*model.Store, error)
	Save(store *model.Store) error
}

// OpenStorage opens the SQLite storage backend.
func OpenStorage() (Storage, error) {
	sqlitePath, err := DefaultSQLitePath()
	if err != nil {
		return nil, err
	}
	return NewSQLiteStorage(sqlitePath)
}
