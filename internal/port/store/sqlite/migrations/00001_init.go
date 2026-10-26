package migrations

import (
	"context"
	"database/sql"

	goose "github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upInit, downInit)
}

var initStatements = []string{
	`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		status     TEXT NOT NULL CHECK (status IN ('active','closed','dismissed')),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS rounds (
		session_id   TEXT NOT NULL,
		round_number INTEGER NOT NULL,
		status       TEXT NOT NULL CHECK (status IN ('pending','submitted')),
		payload      TEXT NOT NULL,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id),
		PRIMARY KEY (session_id, round_number)
	)`,
	`CREATE TABLE IF NOT EXISTS questions (
		session_id        TEXT NOT NULL,
		round_number      INTEGER NOT NULL,
		question_index    INTEGER NOT NULL,
		title             TEXT NOT NULL,
		abstract          TEXT NOT NULL,
		options           TEXT NOT NULL,
		recommended_index INTEGER NOT NULL DEFAULT 0,
		reminders         TEXT NOT NULL DEFAULT '[]',
		created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id, round_number) REFERENCES rounds(session_id, round_number) ON DELETE CASCADE,
		PRIMARY KEY (session_id, round_number, question_index)
	)`,
	`CREATE TABLE IF NOT EXISTS answers (
		session_id     TEXT NOT NULL,
		round_number   INTEGER NOT NULL,
		question_index INTEGER NOT NULL,
		chosen_option  INTEGER,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id, round_number, question_index) REFERENCES questions(session_id, round_number, question_index) ON DELETE CASCADE,
		PRIMARY KEY (session_id, round_number, question_index)
	)`,
	`CREATE TABLE IF NOT EXISTS comments (
		session_id     TEXT NOT NULL,
		round_number   INTEGER NOT NULL,
		question_index INTEGER NOT NULL,
		comment_seq    INTEGER NOT NULL,
		body           TEXT NOT NULL,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id, round_number, question_index) REFERENCES questions(session_id, round_number, question_index) ON DELETE CASCADE,
		PRIMARY KEY (session_id, round_number, question_index, comment_seq)
	)`,
	`CREATE TABLE IF NOT EXISTS legends (
		session_id   TEXT PRIMARY KEY,
		payload      TEXT NOT NULL,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_rounds_session ON rounds(session_id, round_number)`,
	`CREATE INDEX IF NOT EXISTS idx_questions_session ON questions(session_id, round_number)`,
	`CREATE INDEX IF NOT EXISTS idx_answers_session ON answers(session_id, round_number)`,
	`CREATE INDEX IF NOT EXISTS idx_comments_session ON comments(session_id, round_number)`,
}

func upInit(ctx context.Context, db *sql.Tx) error {
	for _, stmt := range initStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func downInit(ctx context.Context, db *sql.Tx) error {
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS idx_comments_session`,
		`DROP INDEX IF EXISTS idx_answers_session`,
		`DROP INDEX IF EXISTS idx_questions_session`,
		`DROP INDEX IF EXISTS idx_rounds_session`,
		`DROP TABLE IF EXISTS comments`,
		`DROP TABLE IF EXISTS answers`,
		`DROP TABLE IF EXISTS questions`,
		`DROP TABLE IF EXISTS legends`,
		`DROP TABLE IF EXISTS rounds`,
		`DROP TABLE IF EXISTS sessions`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
