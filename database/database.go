package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func InitDB() (*sql.DB, error) {

	return Open("velora.db")
}

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		account_number TEXT NOT NULL UNIQUE,
		balance REAL NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL REFERENCES users(id),
		reference TEXT NOT NULL UNIQUE,
		kind TEXT NOT NULL,
		amount_kobo INTEGER NOT NULL CHECK (amount_kobo > 0),
		status TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS admins (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS user_profiles (
		user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
		avatar TEXT NOT NULL CHECK (avatar IN ('classic', 'leaf', 'sun', 'mountain'))
	);
	CREATE INDEX IF NOT EXISTS transactions_user_date
		ON transactions (user_id, created_at DESC, id DESC);
	CREATE TRIGGER IF NOT EXISTS users_nonnegative_balance_insert
		BEFORE INSERT ON users WHEN NEW.balance < 0
		BEGIN SELECT RAISE(ABORT, 'negative balance'); END;
	CREATE TRIGGER IF NOT EXISTS users_nonnegative_balance_update
		BEFORE UPDATE OF balance ON users WHEN NEW.balance < 0
		BEGIN SELECT RAISE(ABORT, 'negative balance'); END;
	`

	err = applySchema(db, createTable)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func applySchema(db *sql.DB, schema string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schema); err != nil {
		return err
	}
	return tx.Commit()
}
