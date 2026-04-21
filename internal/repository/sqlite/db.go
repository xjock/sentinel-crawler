package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open 打开 SQLite 数据库连接
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// Migrate 执行数据库迁移
func Migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			platform TEXT,
			product_type TEXT,
			sensing_date DATETIME,
			ingestion_date DATETIME,
			footprint_type TEXT,
			footprint_coords TEXT,
			size INTEGER,
			download_url TEXT,
			checksum TEXT,
			checksum_algo TEXT,
			metadata TEXT,
			raw_xml TEXT,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_products_platform ON products(platform);`,
		`CREATE INDEX IF NOT EXISTS idx_products_type ON products(product_type);`,
		`CREATE INDEX IF NOT EXISTS idx_products_sensing ON products(sensing_date);`,
		`CREATE INDEX IF NOT EXISTS idx_products_source ON products(source);`,

		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			spec TEXT,
			progress REAL DEFAULT 0,
			error TEXT,
			retries INTEGER DEFAULT 0,
			worker_id TEXT,
			started_at DATETIME,
			ended_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);`,

		`CREATE TABLE IF NOT EXISTS download_states (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			product_id TEXT NOT NULL,
			dest_path TEXT,
			total_bytes INTEGER DEFAULT 0,
			received_bytes INTEGER DEFAULT 0,
			checksum_expected TEXT,
			status TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_download_task ON download_states(task_id);`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
