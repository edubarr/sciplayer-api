package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	"sciplayer-api/internal/store"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_foreign_keys=on&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := migrate(db); err != nil {
		err := db.Close()
		if err != nil {
			return nil, err
		}
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateDevice(ctx context.Context, deviceID string) (bool, error) {
	const query = `
        INSERT INTO devices (device_identifier)
        VALUES (?)
        ON CONFLICT(device_identifier) DO NOTHING;
    `

	res, err := s.db.ExecContext(ctx, query, deviceID)
	if err != nil {
		return false, fmt.Errorf("inserting device: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking insert result: %w", err)
	}

	return affected > 0, nil
}

func (s *Store) AddPlaylist(ctx context.Context, deviceID, name, playlistURL string) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				err = fmt.Errorf("rolling back transaction: %v (original error: %w)", rollbackErr, err)
			}
		}
	}()

	const deviceCheck = `
        SELECT 1 FROM devices WHERE device_identifier = ?;
    `

	if err = tx.QueryRowContext(ctx, deviceCheck, deviceID).Scan(new(int)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrDeviceNotFound
		}
		return fmt.Errorf("checking device existence: %w", err)
	}

	const insertPlaylist = `
        INSERT INTO playlists (device_identifier, name, url)
        VALUES (?, ?, ?);
    `

	if _, err = tx.ExecContext(ctx, insertPlaylist, deviceID, name, playlistURL); err != nil {
		return fmt.Errorf("inserting playlist: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing playlist insert: %w", err)
	}

	return nil
}

func (s *Store) ListPlaylists(ctx context.Context, deviceID string) ([]store.Playlist, error) {
	const deviceCheck = `
        SELECT 1 FROM devices WHERE device_identifier = ?;
    `

	if err := s.db.QueryRowContext(ctx, deviceCheck, deviceID).Scan(new(int)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrDeviceNotFound
		}
		return nil, fmt.Errorf("checking device existence: %w", err)
	}

	const query = `
        SELECT name, url, created_at
        FROM playlists
        WHERE device_identifier = ?
        ORDER BY created_at ASC, id ASC;
    `

	rows, err := s.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("fetching playlists: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	playlists := make([]store.Playlist, 0)
	for rows.Next() {
		var pl store.Playlist
		if err := rows.Scan(&pl.Name, &pl.URL, &pl.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning playlist: %w", err)
		}
		playlists = append(playlists, pl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating playlists: %w", err)
	}

	return playlists, nil
}

func migrate(db *sql.DB) error {
	const createDevicesTable = `
        CREATE TABLE IF NOT EXISTS devices (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            device_identifier TEXT NOT NULL UNIQUE,
            created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
    `

	const createPlaylistsTable = `
        CREATE TABLE IF NOT EXISTS playlists (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            device_identifier TEXT NOT NULL,
            name TEXT NOT NULL,
            url TEXT NOT NULL,
            created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (device_identifier) REFERENCES devices(device_identifier) ON DELETE CASCADE
        );
    `

	if _, err := db.Exec(createDevicesTable); err != nil {
		return fmt.Errorf("creating devices table: %w", err)
	}

	if _, err := db.Exec(createPlaylistsTable); err != nil {
		return fmt.Errorf("creating playlists table: %w", err)
	}

	return nil
}
