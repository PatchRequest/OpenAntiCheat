package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS applications (
	id         INTEGER PRIMARY KEY,
	path       TEXT NOT NULL UNIQUE,
	score      INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_app_path ON applications(path);

CREATE TABLE IF NOT EXISTS events (
	id         INTEGER PRIMARY KEY,
	app_id     INTEGER NOT NULL,
	kind       TEXT,
	info       TEXT,
	delta      INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY(app_id) REFERENCES applications(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_events_appid ON events(app_id);

-- keep applications.score in sync with SUM(events.delta)
CREATE TRIGGER IF NOT EXISTS trg_events_ai AFTER INSERT ON events BEGIN
	UPDATE applications
	SET score = COALESCE((SELECT SUM(delta) FROM events WHERE app_id = NEW.app_id), 0),
	    updated_at = datetime('now')
	WHERE id = NEW.app_id;
END;

CREATE TRIGGER IF NOT EXISTS trg_events_au AFTER UPDATE OF delta, app_id ON events BEGIN
	UPDATE applications
	SET score = COALESCE((SELECT SUM(delta) FROM events WHERE app_id = NEW.app_id), 0),
	    updated_at = datetime('now')
	WHERE id = NEW.app_id;
	UPDATE applications
	SET score = COALESCE((SELECT SUM(delta) FROM events WHERE app_id = OLD.app_id), 0),
	    updated_at = datetime('now')
	WHERE id = OLD.app_id;
END;

CREATE TRIGGER IF NOT EXISTS trg_events_ad AFTER DELETE ON events BEGIN
	UPDATE applications
	SET score = COALESCE((SELECT SUM(delta) FROM events WHERE app_id = OLD.app_id), 0),
	    updated_at = datetime('now')
	WHERE id = OLD.app_id;
END;
`

type App struct {
	ID        int64
	Path      string
	Score     int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Event struct {
	ID        int64
	AppID     int64
	Kind      sql.NullString
	Info      sql.NullString
	Delta     int64
	CreatedAt time.Time
}

func OpenDB(file string) (*sql.DB, error) {
	// file:= "file:db.sqlite?cache=shared&mode=rwc&_pragma=busy_timeout(5000)"
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_pragma=busy_timeout(5000)", file)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func UpsertApp(ctx context.Context, db *sql.DB, path string) (int64, error) {
	if db == nil {
		return 0, errors.New("nil db")
	}
	if path == "" {
		return 0, errors.New("empty path")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Preferred (SQLite ≥3.35): RETURNING gives the correct id even on conflict
	var id int64
	row := tx.QueryRowContext(ctx, `
		INSERT INTO applications(path)
		VALUES (?)
		ON CONFLICT(path) DO UPDATE SET path=excluded.path
		RETURNING id
	`, path)
	if e := row.Scan(&id); e == nil {
		err = tx.Commit()
		return id, err
	}

	// Fallback (older SQLite): INSERT OR IGNORE + SELECT id
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO applications(path) VALUES (?)`, path); err != nil {
		return 0, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM applications WHERE path = ?`, path).Scan(&id); err != nil {
		return 0, err
	}
	err = tx.Commit()
	return id, err
}

func InsertEventByPath(ctx context.Context, db *sql.DB, appPath, kind, info string, delta int64) (int64, error) {
	appID, err := UpsertApp(ctx, db, appPath)
	if err != nil {
		return 0, err
	}
	return InsertEvent(ctx, db, appID, kind, info, delta)
}

func InsertEvent(ctx context.Context, db *sql.DB, appID int64, kind, info string, delta int64) (int64, error) {
	if appID == 0 {
		return 0, errors.New("invalid appID")
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO events(app_id, kind, info, delta) VALUES (?, ?, ?, ?)`,
		appID, nullStr(kind), nullStr(info), delta)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetAppByPath(ctx context.Context, db *sql.DB, path string) (App, error) {
	var a App
	row := db.QueryRowContext(ctx, `
		SELECT id, path, score, created_at, updated_at
		FROM applications
		WHERE path = ?`, path)
	var created, updated string
	err := row.Scan(&a.ID, &a.Path, &a.Score, &created, &updated)
	if err != nil {
		return a, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	a.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return a, nil
}

func GetAppByID(ctx context.Context, db *sql.DB, id int64) (App, error) {
	var a App
	row := db.QueryRowContext(ctx, `
		SELECT id, path, score, created_at, updated_at
		FROM applications
		WHERE id = ?`, id)
	var created, updated string
	err := row.Scan(&a.ID, &a.Path, &a.Score, &created, &updated)
	if err != nil {
		return a, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	a.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return a, nil
}

func ListApps(ctx context.Context, db *sql.DB, limit, offset int) ([]App, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, path, score, created_at, updated_at
		FROM applications
		ORDER BY score DESC, id ASC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		var a App
		var created, updated string
		if err := rows.Scan(&a.ID, &a.Path, &a.Score, &created, &updated); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		a.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, a)
	}
	return out, rows.Err()
}

func ListEventsByPath(ctx context.Context, db *sql.DB, appPath string, limit, offset int) ([]Event, error) {
	var appID int64
	err := db.QueryRowContext(ctx, `SELECT id FROM applications WHERE path = ?`, appPath).Scan(&appID)
	if err != nil {
		return nil, err
	}
	return ListEventsByAppID(ctx, db, appID, limit, offset)
}

func ListEventsByAppID(ctx context.Context, db *sql.DB, appID int64, limit, offset int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, app_id, kind, info, delta, created_at
		FROM events
		WHERE app_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?`, appID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var created string
		if err := rows.Scan(&e.ID, &e.AppID, &e.Kind, &e.Info, &e.Delta, &created); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Helpers

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func rollbackIfNeeded(tx *sql.Tx, errp *error) error {
	if *errp != nil {
		_ = tx.Rollback()
		return *errp
	}
	return nil
}
