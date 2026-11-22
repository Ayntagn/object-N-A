// Package shortsearch stores a derived disk index for literal substring candidates.
package shortsearch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
)

const (
	applicationID = 0x53485352
	schemaVersion = "1"
	schema        = `
CREATE TABLE documents (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE CHECK (path <> ''),
    version TEXT NOT NULL
);
CREATE VIRTUAL TABLE grams USING fts5(
    terms, tokenize='ascii', content='', contentless_delete=1, detail=none
);`
)

var (
	// ErrUnfilterable indicates that a query has fewer than two folded code points.
	ErrUnfilterable = errors.New("shortsearch: query has no bigram")
	// ErrIncompatible indicates that a file belongs to another format or schema version.
	ErrIncompatible = errors.New("shortsearch: incompatible database")
)

// Store is a disk-backed candidate index. Its methods may be called concurrently.
type Store struct {
	db *sql.DB
}

// Document supplies a nonempty path key, an opaque version and saved UTF-8 text.
type Document struct {
	Path, Version, Text string
}

// CandidateSet describes one atomic view of the index. Versions includes every
// indexed path. Matches may contain false positives, which require exact matching.
type CandidateSet struct {
	Versions map[string]string
	Matches  map[string]struct{}
}

// Open opens or creates a derived index at filename. The parent directory must
// exist. An incompatible database is left intact.
func Open(ctx context.Context, filename string) (*Store, error) {
	if filename == "" || strings.ContainsRune(filename, 0) {
		return nil, errors.New("shortsearch: invalid database path")
	}
	// An absolute filesystem path cannot be interpreted as a SQLite file: URI.
	filename, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("shortsearch: resolve database path: %w", err)
	}
	db, err := driver.Open(filename, func(conn *sqlite3.Conn) error {
		if err := fts5.Register(conn); err != nil {
			return err
		}
		return conn.Exec(`PRAGMA busy_timeout=500; PRAGMA cache_size=-4096; PRAGMA mmap_size=0`)
	})
	if err != nil {
		return nil, fmt.Errorf("shortsearch: open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := initialize(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("shortsearch: initialize: %w", err)
	}
	return &Store{db: db}, nil
}

func initialize(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var appID int
	var version string
	if err := tx.QueryRowContext(ctx, `SELECT application_id, user_version FROM pragma_application_id(), pragma_user_version()`).Scan(&appID, &version); err != nil {
		return err
	}
	if appID == applicationID && version == schemaVersion {
		return tx.Commit()
	}
	if appID != 0 || version != "0" {
		return fmt.Errorf("%w (application_id=%d, user_version=%s)", ErrIncompatible, appID, version)
	}
	var hasObjects bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sqlite_schema)`).Scan(&hasObjects); err != nil {
		return err
	}
	if hasObjects {
		return ErrIncompatible
	}
	versionSQL := fmt.Sprintf("PRAGMA application_id=%d; PRAGMA user_version=%s", applicationID, schemaVersion)
	if _, err := tx.ExecContext(ctx, schema+versionSQL); err != nil {
		return err
	}
	return tx.Commit()
}

// Close releases the store's SQLite connection.
func (s *Store) Close() error { return s.db.Close() }

// Versions returns the stored version of every indexed path.
func (s *Store) Versions(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, version FROM documents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make(map[string]string)
	for rows.Next() {
		var path, version string
		if err := rows.Scan(&path, &version); err != nil {
			return nil, err
		}
		versions[path] = version
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

// Update removes the given paths and then upserts documents as one transaction.
// Callers should batch large sets to limit memory use and time in the write lock.
func (s *Store) Update(ctx context.Context, documents []Document, removedPaths []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(documents) == 0 && len(removedPaths) == 0 {
		return nil
	}
	terms := make([]string, len(documents))
	for i, document := range documents {
		value, err := documentTerms(ctx, document.Text)
		if err != nil {
			return err
		}
		terms[i] = value
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, path := range removedPaths {
		if _, err := tx.ExecContext(ctx, `DELETE FROM grams WHERE rowid = (SELECT id FROM documents WHERE path = ?)`, path); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE path = ?`, path); err != nil {
			return err
		}
	}
	for i, document := range documents {
		var id int64
		row := tx.QueryRowContext(ctx, `
INSERT INTO documents(path, version) VALUES (?, ?)
ON CONFLICT(path) DO UPDATE SET version = excluded.version RETURNING id`, document.Path, document.Version)
		if err := row.Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO grams(rowid, terms) VALUES (?, ?)`, id, terms[i]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Lookup returns candidates for a trimmed literal query. The caller supplies
// its own query trimming and minimum grapheme policy. On any error, the caller
// must ignore the index and read bodies through its regular search path.
func (s *Store) Lookup(ctx context.Context, query string) (CandidateSet, error) {
	if err := ctx.Err(); err != nil {
		return CandidateSet{}, err
	}
	terms := queryTerms(query)
	if terms == "" {
		return CandidateSet{}, ErrUnfilterable
	}
	// One statement keeps the versions and postings in the same read snapshot.
	rows, err := s.db.QueryContext(ctx, `
SELECT path, version, id IN (SELECT rowid FROM grams WHERE grams MATCH ?)
FROM documents`, terms)
	if err != nil {
		return CandidateSet{}, err
	}
	defer rows.Close()
	set := CandidateSet{Versions: make(map[string]string), Matches: make(map[string]struct{})}
	for rows.Next() {
		var path, version string
		var candidate bool
		if err := rows.Scan(&path, &version, &candidate); err != nil {
			return CandidateSet{}, err
		}
		set.Versions[path] = version
		if candidate {
			set.Matches[path] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return CandidateSet{}, err
	}
	return set, nil
}
