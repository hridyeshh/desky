package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// Config holds the three screen widget slots for the single desk display,
// the global power state, and optional per-screen GIF URLs.
type Config struct {
	Screen1    string `json:"screen1"`
	Screen2    string `json:"screen2"`
	Screen3    string `json:"screen3"`
	PowerState string `json:"power_state"`
	GifURL1    string `json:"gif_url_1"`
	GifURL2    string `json:"gif_url_2"`
	GifURL3    string `json:"gif_url_3"`
	// Per-screen countdown timers. TimerEndN is the absolute Unix time (seconds)
	// at which the timer hits zero; 0 means no active timer. PrevN is the widget
	// to revert to after the "TIME'S UP" flash ends.
	TimerEnd1 int64  `json:"timer_end_1"`
	TimerEnd2 int64  `json:"timer_end_2"`
	TimerEnd3 int64  `json:"timer_end_3"`
	Prev1     string `json:"prev_1"`
	Prev2     string `json:"prev_2"`
	Prev3     string `json:"prev_3"`
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

	// Additive migrations for power state and per-screen GIF URLs.
	for _, stmt := range []string{
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS power_state TEXT NOT NULL DEFAULT 'ON'`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS gif_url_1 TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS gif_url_2 TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS gif_url_3 TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS timer_end_1 BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS timer_end_2 BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS timer_end_3 BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS prev_1 TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS prev_2 TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE config ADD COLUMN IF NOT EXISTS prev_3 TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			return fmt.Errorf("migrate config columns: %w", err)
		}
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
		`SELECT screen1, screen2, screen3, power_state, gif_url_1, gif_url_2, gif_url_3,
		        timer_end_1, timer_end_2, timer_end_3, prev_1, prev_2, prev_3
		 FROM config WHERE id = 1`,
	).Scan(&c.Screen1, &c.Screen2, &c.Screen3, &c.PowerState, &c.GifURL1, &c.GifURL2, &c.GifURL3,
		&c.TimerEnd1, &c.TimerEnd2, &c.TimerEnd3, &c.Prev1, &c.Prev2, &c.Prev3)
	if err != nil {
		return c, fmt.Errorf("get config: %w", err)
	}
	return c, nil
}

// UpdateConfig applies partial updates. Any non-nil field is written; nil
// fields are left unchanged. Returns the updated full config.
func (d *DB) UpdateConfig(screen1, screen2, screen3, gifURL1, gifURL2, gifURL3 *string) (Config, error) {
	_, err := d.conn.Exec(`
		UPDATE config SET
			screen1   = COALESCE($1, screen1),
			screen2   = COALESCE($2, screen2),
			screen3   = COALESCE($3, screen3),
			gif_url_1 = COALESCE($4, gif_url_1),
			gif_url_2 = COALESCE($5, gif_url_2),
			gif_url_3 = COALESCE($6, gif_url_3)
		WHERE id = 1
	`, screen1, screen2, screen3, gifURL1, gifURL2, gifURL3)
	if err != nil {
		return Config{}, fmt.Errorf("update config: %w", err)
	}
	return d.GetConfig()
}

// SetPowerState writes the global power state ("ON" or "OFF").
func (d *DB) SetPowerState(state string) (Config, error) {
	_, err := d.conn.Exec(
		`UPDATE config SET power_state = $1 WHERE id = 1`, state,
	)
	if err != nil {
		return Config{}, fmt.Errorf("set power state: %w", err)
	}
	return d.GetConfig()
}

// SetTimer starts a countdown on the given screen (1–3): sets the slot to
// "timer", records the absolute end time, and snapshots the widget to revert
// to when the timer finishes. If the screen already shows a timer, the stored
// prev is preserved so repeated timers don't lose the original widget.
func (d *DB) SetTimer(screen int, endUnix int64, prev string) (Config, error) {
	var screenCol, endCol, prevCol string
	switch screen {
	case 1:
		screenCol, endCol, prevCol = "screen1", "timer_end_1", "prev_1"
	case 2:
		screenCol, endCol, prevCol = "screen2", "timer_end_2", "prev_2"
	case 3:
		screenCol, endCol, prevCol = "screen3", "timer_end_3", "prev_3"
	default:
		return Config{}, fmt.Errorf("invalid screen %d", screen)
	}

	// Only overwrite prev when the slot is not already a timer, so re-arming a
	// timer keeps the original underlying widget.
	query := fmt.Sprintf(`
		UPDATE config SET
			%[3]s = CASE WHEN %[1]s = 'timer' THEN %[3]s ELSE $1 END,
			%[1]s = 'timer',
			%[2]s = $2
		WHERE id = 1
	`, screenCol, endCol, prevCol)

	if _, err := d.conn.Exec(query, prev, endUnix); err != nil {
		return Config{}, fmt.Errorf("set timer: %w", err)
	}
	return d.GetConfig()
}
