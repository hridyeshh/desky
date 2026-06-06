package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// Config holds the three screen widget slots for the single desk display.
type Config struct {
	Screen1 string `json:"screen1"`
	Screen2 string `json:"screen2"`
	Screen3 string `json:"screen3"`
}

// DB wraps the sql.DB connection.
type DB struct {
	conn *sql.DB
}

// Connect opens a Postgres connection using the given DATABASE_URL.
func Connect(databaseURL string) (*DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the underlying connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Migrate creates the config table if it does not exist and seeds a default
// config row on first run. There is only ever one config row (id = 1).
func (d *DB) Migrate() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			id      INTEGER PRIMARY KEY DEFAULT 1,
			screen1 TEXT NOT NULL,
			screen2 TEXT NOT NULL,
			screen3 TEXT NOT NULL,
			CONSTRAINT single_row CHECK (id = 1)
		)
	`)
	if err != nil {
		return fmt.Errorf("create config table: %w", err)
	}

	// Seed default config only if the table is empty.
	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM config`).Scan(&count); err != nil {
		return fmt.Errorf("count config rows: %w", err)
	}
	if count == 0 {
		_, err := d.conn.Exec(
			`INSERT INTO config (id, screen1, screen2, screen3) VALUES (1, $1, $2, $3)`,
			"clock", "gif", "music",
		)
		if err != nil {
			return fmt.Errorf("seed default config: %w", err)
		}
		log.Println("seeded default config: clock / gif / music")
	}
	return nil
}

// GetConfig returns the single config row.
func (d *DB) GetConfig() (Config, error) {
	var c Config
	err := d.conn.QueryRow(
		`SELECT screen1, screen2, screen3 FROM config WHERE id = 1`,
	).Scan(&c.Screen1, &c.Screen2, &c.Screen3)
	if err != nil {
		return c, fmt.Errorf("get config: %w", err)
	}
	return c, nil
}

// UpdateConfig applies partial updates. Any non-nil field is written; nil
// fields are left unchanged. Returns the updated full config.
func (d *DB) UpdateConfig(screen1, screen2, screen3 *string) (Config, error) {
	_, err := d.conn.Exec(`
		UPDATE config SET
			screen1 = COALESCE($1, screen1),
			screen2 = COALESCE($2, screen2),
			screen3 = COALESCE($3, screen3)
		WHERE id = 1
	`, screen1, screen2, screen3)
	if err != nil {
		return Config{}, fmt.Errorf("update config: %w", err)
	}
	return d.GetConfig()
}
