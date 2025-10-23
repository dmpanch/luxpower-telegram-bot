package main

import (
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

// SubscriberStore wraps persistence for Telegram chat IDs.
type SubscriberStore struct {
	db *sql.DB
}

// NewSubscriberStore opens (or creates) the SQLite database located at dsn
// and ensures the subscribers table is ready for use.
func NewSubscriberStore(dsn string) (*SubscriberStore, error) {
	if dsn == "" {
		return nil, errors.New("empty SQLite DSN")
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// SQLite works best with a single writer connection in this use-case.
	db.SetMaxOpenConns(1)

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SubscriberStore{db: db}, nil
}

// initSchema ensures the subscribers table exists.
func initSchema(db *sql.DB) error {
	const createTable = `
CREATE TABLE IF NOT EXISTS subscribers (
	chat_id INTEGER PRIMARY KEY
);`
	_, err := db.Exec(createTable)
	return err
}

// Close releases underlying resources.
func (s *SubscriberStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Save records the given chat ID if it has not been stored yet.
func (s *SubscriberStore) Save(chatID int64) error {
	if s == nil {
		return errors.New("nil SubscriberStore")
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO subscribers (chat_id) VALUES (?)`, chatID)
	return err
}

// List returns all known chat IDs.
func (s *SubscriberStore) List() ([]int64, error) {
	if s == nil {
		return nil, errors.New("nil SubscriberStore")
	}

	rows, err := s.db.Query(`SELECT chat_id FROM subscribers ORDER BY chat_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chatIDs, nil
}
