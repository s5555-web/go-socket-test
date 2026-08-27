package store

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct{ DB *sql.DB }

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	About        string    `json:"about"`
	Avatar       string    `json:"avatar"`
	Relationship string    `json:"relationship,omitempty"`
	PublicKey    string    `json:"public_key,omitempty"`
	KeyBackup    string    `json:"key_backup,omitempty"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
}
type Conversation struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	IsGroup      bool       `json:"is_group"`
	LastMessage  string     `json:"last_message"`
	LastAt       *time.Time `json:"last_at"`
	Unread       int        `json:"unread"`
	ManualUnread bool       `json:"manual_unread"`
	Pinned       bool       `json:"pinned"`
	Archived     bool       `json:"archived"`
	MutedUntil   *time.Time `json:"muted_until,omitempty"`
	Muted        bool       `json:"muted"`
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
		`CREATE TABLE IF NOT EXISTS users (id BIGINT PRIMARY KEY AUTO_INCREMENT, username VARCHAR(40) NOT NULL UNIQUE, password_hash VARCHAR(100) NOT NULL, display_name VARCHAR(80) NOT NULL, avatar VARCHAR(255) NOT NULL DEFAULT '', public_key TEXT NULL, encrypted_private_key MEDIUMTEXT NULL, is_admin BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS friendships (user_low_id BIGINT NOT NULL, user_high_id BIGINT NOT NULL, requested_by BIGINT NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'pending', created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY(user_low_id,user_high_id), INDEX(requested_by), INDEX(status)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS conversations (id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(100) NOT NULL DEFAULT '', is_group BOOLEAN NOT NULL DEFAULT FALSE, created_by BIGINT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, INDEX(created_by)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS conversation_members (conversation_id BIGINT NOT NULL, user_id BIGINT NOT NULL, last_read_message_id BIGINT NOT NULL DEFAULT 0, manual_unread BOOLEAN NOT NULL DEFAULT FALSE, pinned BOOLEAN NOT NULL DEFAULT FALSE, archived BOOLEAN NOT NULL DEFAULT FALSE, muted_until DATETIME NULL, hidden BOOLEAN NOT NULL DEFAULT FALSE, joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(conversation_id,user_id), INDEX(user_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS messages (id BIGINT PRIMARY KEY AUTO_INCREMENT, conversation_id BIGINT NOT NULL, sender_id BIGINT NOT NULL, body MEDIUMTEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, INDEX(conversation_id,id), INDEX(sender_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS encrypted_attachments (id CHAR(32) PRIMARY KEY, conversation_id BIGINT NOT NULL, uploader_id BIGINT NOT NULL, message_id BIGINT NULL, cipher_size BIGINT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, INDEX(conversation_id), INDEX(uploader_id), INDEX(message_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS push_subscriptions (endpoint_hash CHAR(64) PRIMARY KEY, user_id BIGINT NOT NULL, endpoint MEDIUMTEXT NOT NULL, p256dh VARCHAR(255) NOT NULL, auth VARCHAR(255) NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, INDEX(user_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range statements {
		if _, err := s.DB.Exec(q); err != nil {
			return err
		}
	}
	// Existing installations predate end-to-end encryption. Duplicate-column is harmless.
	_, _ = s.DB.Exec(`ALTER TABLE users ADD COLUMN public_key TEXT NULL AFTER avatar`)
	_, _ = s.DB.Exec(`ALTER TABLE users ADD COLUMN encrypted_private_key MEDIUMTEXT NULL AFTER public_key`)
	_, _ = s.DB.Exec(`ALTER TABLE users ADD COLUMN about VARCHAR(160) NOT NULL DEFAULT '' AFTER display_name`)
	_, _ = s.DB.Exec(`ALTER TABLE conversation_members ADD COLUMN hidden BOOLEAN NOT NULL DEFAULT FALSE AFTER last_read_message_id`)
	_, _ = s.DB.Exec(`ALTER TABLE conversation_members ADD COLUMN manual_unread BOOLEAN NOT NULL DEFAULT FALSE AFTER last_read_message_id`)
	_, _ = s.DB.Exec(`ALTER TABLE conversation_members ADD COLUMN pinned BOOLEAN NOT NULL DEFAULT FALSE AFTER manual_unread`)
	_, _ = s.DB.Exec(`ALTER TABLE conversation_members ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE AFTER pinned`)
	_, _ = s.DB.Exec(`ALTER TABLE conversation_members ADD COLUMN muted_until DATETIME NULL AFTER archived`)
	// Message bodies are opaque E2EE envelopes. MEDIUMTEXT avoids truncating padded
	// ciphertext while the API continues to enforce a much smaller request limit.
	_, _ = s.DB.Exec(`ALTER TABLE messages MODIFY COLUMN body MEDIUMTEXT NOT NULL`)
	// Existing direct conversations are already trusted contacts; preserve them as friends.
	_, _ = s.DB.Exec(`INSERT IGNORE INTO friendships(user_low_id,user_high_id,requested_by,status) SELECT cm1.user_id,cm2.user_id,x.created_by,'accepted' FROM conversations x JOIN conversation_members cm1 ON cm1.conversation_id=x.id JOIN conversation_members cm2 ON cm2.conversation_id=x.id AND cm1.user_id<cm2.user_id WHERE x.is_group=FALSE`)
	return nil
}

func (s *Store) Close() error { return s.DB.Close() }
