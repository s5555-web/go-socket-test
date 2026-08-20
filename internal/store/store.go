package store

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct{ DB *sql.DB }

type User struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Avatar      string    `json:"avatar"`
	PublicKey   string    `json:"public_key,omitempty"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}
type Conversation struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	IsGroup     bool       `json:"is_group"`
	LastMessage string     `json:"last_message"`
	LastAt      *time.Time `json:"last_at"`
	Unread      int        `json:"unread"`
}
type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(3 * time.Minute)
	if err = db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err = s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (id BIGINT PRIMARY KEY AUTO_INCREMENT, username VARCHAR(40) NOT NULL UNIQUE, password_hash VARCHAR(100) NOT NULL, display_name VARCHAR(80) NOT NULL, avatar VARCHAR(255) NOT NULL DEFAULT '', public_key TEXT NULL, is_admin BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS conversations (id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(100) NOT NULL DEFAULT '', is_group BOOLEAN NOT NULL DEFAULT FALSE, created_by BIGINT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, INDEX(created_by)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS conversation_members (conversation_id BIGINT NOT NULL, user_id BIGINT NOT NULL, last_read_message_id BIGINT NOT NULL DEFAULT 0, hidden BOOLEAN NOT NULL DEFAULT FALSE, joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(conversation_id,user_id), INDEX(user_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS messages (id BIGINT PRIMARY KEY AUTO_INCREMENT, conversation_id BIGINT NOT NULL, sender_id BIGINT NOT NULL, body TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, INDEX(conversation_id,id), INDEX(sender_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range statements {
		if _, err := s.DB.Exec(q); err != nil {
			return err
		}
	}
	// Existing installations predate end-to-end encryption. Duplicate-column is harmless.
	_, _ = s.DB.Exec(`ALTER TABLE users ADD COLUMN public_key TEXT NULL AFTER avatar`)
	_, _ = s.DB.Exec(`ALTER TABLE conversation_members ADD COLUMN hidden BOOLEAN NOT NULL DEFAULT FALSE AFTER last_read_message_id`)
	return nil
}

func (s *Store) Close() error { return s.DB.Close() }
