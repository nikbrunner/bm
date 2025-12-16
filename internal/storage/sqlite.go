package storage

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nikbrunner/bm/internal/model"
)

const currentSchemaVersion = 2

// SQLiteStorage implements Storage using a SQLite database.
type SQLiteStorage struct {
	db   *sql.DB
	path string
}

// NewSQLiteStorage creates a new SQLiteStorage with the given database path.
func NewSQLiteStorage(path string) (*SQLiteStorage, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable foreign keys and set pragmas for performance
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}

	s := &SQLiteStorage{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Path returns the database file path.
func (s *SQLiteStorage) Path() string {
	return s.path
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// migrate runs database migrations.
func (s *SQLiteStorage) migrate() error {
	// Check current schema version
	var version int
	err := s.db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&version)
	if err != nil {
		// Table doesn't exist or is empty, start fresh
		version = 0
	}

	if version < 1 {
		if err := s.migrateV1(); err != nil {
			return err
		}
	}

	if version < 2 {
		if err := s.migrateV2(); err != nil {
			return err
		}
	}

	return nil
}

// migrateV1 creates the initial schema.
func (s *SQLiteStorage) migrateV1() error {
	schema := `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		);

		CREATE TABLE IF NOT EXISTS folders (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			parent_id TEXT,
			pinned INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (parent_id) REFERENCES folders(id) ON DELETE SET NULL
		);

		CREATE INDEX IF NOT EXISTS idx_folders_parent_id ON folders(parent_id);
		CREATE INDEX IF NOT EXISTS idx_folders_pinned ON folders(pinned) WHERE pinned = 1;

		CREATE TABLE IF NOT EXISTS bookmarks (
			id TEXT PRIMARY KEY NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			folder_id TEXT,
			tags TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			visited_at TEXT,
			pinned INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (folder_id) REFERENCES folders(id) ON DELETE SET NULL
		);

		CREATE INDEX IF NOT EXISTS idx_bookmarks_folder_id ON bookmarks(folder_id);
		CREATE INDEX IF NOT EXISTS idx_bookmarks_url ON bookmarks(url);
		CREATE INDEX IF NOT EXISTS idx_bookmarks_pinned ON bookmarks(pinned) WHERE pinned = 1;

		INSERT OR REPLACE INTO schema_version (version) VALUES (1);
	`
	_, err := s.db.Exec(schema)
	return err
}

// migrateV2 adds pin_order column for pinned item ordering.
func (s *SQLiteStorage) migrateV2() error {
	migration := `
		ALTER TABLE folders ADD COLUMN pin_order INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE bookmarks ADD COLUMN pin_order INTEGER NOT NULL DEFAULT 0;
		UPDATE schema_version SET version = 2;
	`
	_, err := s.db.Exec(migration)
	return err
}

// Load reads the store from the SQLite database.
func (s *SQLiteStorage) Load() (*model.Store, error) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	// Load folders
	rows, err := s.db.Query(`
		SELECT id, name, parent_id, pinned, pin_order
		FROM folders
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var f model.Folder
		var parentID sql.NullString
		var pinned int

		if err := rows.Scan(&f.ID, &f.Name, &parentID, &pinned, &f.PinOrder); err != nil {
			return nil, err
		}

		if parentID.Valid {
			f.ParentID = &parentID.String
		}
		f.Pinned = pinned == 1

		store.Folders = append(store.Folders, f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load bookmarks
	rows, err = s.db.Query(`
		SELECT id, title, url, folder_id, tags, created_at, visited_at, pinned, pin_order
		FROM bookmarks
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var b model.Bookmark
		var folderID sql.NullString
		var tagsJSON string
		var createdAtStr string
		var visitedAtStr sql.NullString
		var pinned int

		if err := rows.Scan(
			&b.ID, &b.Title, &b.URL, &folderID,
			&tagsJSON, &createdAtStr, &visitedAtStr, &pinned, &b.PinOrder,
		); err != nil {
			return nil, err
		}

		if folderID.Valid {
			b.FolderID = &folderID.String
		}

		if err := json.Unmarshal([]byte(tagsJSON), &b.Tags); err != nil {
			b.Tags = []string{}
		}

		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)

		if visitedAtStr.Valid {
			t, err := time.Parse(time.RFC3339, visitedAtStr.String)
			if err == nil {
				b.VisitedAt = &t
			}
		}

		b.Pinned = pinned == 1

		store.Bookmarks = append(store.Bookmarks, b)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return store, nil
}

// Save writes the store to the SQLite database.
// Uses a transaction for atomicity - all or nothing.
func (s *SQLiteStorage) Save(store *model.Store) error {
	// Temporarily disable foreign key checks for bulk insert
	// (folders may reference parents that haven't been inserted yet)
	// Note: PRAGMA foreign_keys cannot be changed inside a transaction
	if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.db.Exec("PRAGMA foreign_keys = ON")
		return err
	}
	defer tx.Rollback()

	// Clear existing data
	if _, err := tx.Exec("DELETE FROM bookmarks"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM folders"); err != nil {
		return err
	}

	// Insert folders
	folderStmt, err := tx.Prepare(`
		INSERT INTO folders (id, name, parent_id, pinned, pin_order)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer folderStmt.Close()

	for _, f := range store.Folders {
		pinned := 0
		if f.Pinned {
			pinned = 1
		}
		if _, err := folderStmt.Exec(f.ID, f.Name, f.ParentID, pinned, f.PinOrder); err != nil {
			return err
		}
	}

	// Insert bookmarks
	bookmarkStmt, err := tx.Prepare(`
		INSERT INTO bookmarks (id, title, url, folder_id, tags, created_at, visited_at, pinned, pin_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer bookmarkStmt.Close()

	for _, b := range store.Bookmarks {
		tagsJSON, _ := json.Marshal(b.Tags)
		if b.Tags == nil {
			tagsJSON = []byte("[]")
		}
		createdAt := b.CreatedAt.Format(time.RFC3339)

		var visitedAt *string
		if b.VisitedAt != nil {
			v := b.VisitedAt.Format(time.RFC3339)
			visitedAt = &v
		}

		pinned := 0
		if b.Pinned {
			pinned = 1
		}

		if _, err := bookmarkStmt.Exec(
			b.ID, b.Title, b.URL, b.FolderID,
			string(tagsJSON), createdAt, visitedAt, pinned, b.PinOrder,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Re-enable foreign key checks
	_, _ = s.db.Exec("PRAGMA foreign_keys = ON")

	return nil
}

// DefaultSQLitePath returns the default SQLite database path: ~/.config/bm/bookmarks.db
func DefaultSQLitePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "bm", "bookmarks.db"), nil
}
