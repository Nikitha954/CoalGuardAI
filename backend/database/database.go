package database

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

// PostgreSQL is the active database backend for this project. The app uses the
// standard Postgres driver while keeping a compatibility shim so older SQL built
// with MySQL-style ? placeholders still runs without per-file rewrites.
type DBWrapper struct {
	*sql.DB
}

func (db *DBWrapper) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.Query(Rebind(query), args...)
}

func (db *DBWrapper) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRow(Rebind(query), args...)
}

func (db *DBWrapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(Rebind(query), args...)
}

func (db *DBWrapper) Begin() (*TxWrapper, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &TxWrapper{tx}, nil
}

type TxWrapper struct {
	*sql.Tx
}

func (t *TxWrapper) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.Tx.Query(Rebind(query), args...)
}

func (t *TxWrapper) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.Tx.QueryRow(Rebind(query), args...)
}

func (t *TxWrapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.Tx.Exec(Rebind(query), args...)
}

// Rebind converts MySQL-style ? placeholders to Postgres $1, $2, ... while
// leaving quoted SQL strings alone.
func Rebind(query string) string {
	if !strings.Contains(query, "?") {
		return query
	}

	var out strings.Builder
	paramIndex := 0
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(query); i++ {
		ch := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}

		switch {
		case inLineComment:
			out.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
		case inBlockComment:
			out.WriteByte(ch)
			if ch == '*' && next == '/' {
				out.WriteByte(next)
				i++
				inBlockComment = false
			}
		case inSingleQuote:
			out.WriteByte(ch)
			if ch == '\'' && next == '\'' {
				out.WriteByte(next)
				i++
			} else if ch == '\'' {
				inSingleQuote = false
			}
		case inDoubleQuote:
			out.WriteByte(ch)
			if ch == '"' && next == '"' {
				out.WriteByte(next)
				i++
			} else if ch == '"' {
				inDoubleQuote = false
			}
		case ch == '-' && next == '-':
			out.WriteByte(ch)
			out.WriteByte(next)
			i++
			inLineComment = true
		case ch == '/' && next == '*':
			out.WriteByte(ch)
			out.WriteByte(next)
			i++
			inBlockComment = true
		case ch == '\'':
			inSingleQuote = true
			out.WriteByte(ch)
		case ch == '"':
			inDoubleQuote = true
			out.WriteByte(ch)
		case ch == '?':
			paramIndex++
			out.WriteString("$" + strconv.Itoa(paramIndex))
		default:
			out.WriteByte(ch)
		}
	}

	return out.String()
}

// DB is the globally accessible wrapped database instance.
var DB *DBWrapper

// Connect initializes the database connection, verifies it, and runs
// startup migrations. Kept as (dbURL string) error to match how main.go
// already calls this (database.Connect(cfg.DatabaseURL)) — no other files
// need to change because of this fix.
func Connect(dbURL string) error {
	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		if !strings.Contains(err.Error(), "does not exist") {
			conn.Close()
			return err
		}
		if err := ensureDatabaseExists(dbURL); err != nil {
			conn.Close()
			return err
		}
		if err := conn.Ping(); err != nil {
			conn.Close()
			return err
		}
	}

	DB = &DBWrapper{DB: conn}

	log.Println("Connected to PostgreSQL database successfully")
	RunMigrations()
	return nil
}

func ensureDatabaseExists(dbURL string) error {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return fmt.Errorf("parse db url: %w", err)
	}

	if parsed.Path == "" || parsed.Path == "/" {
		return fmt.Errorf("database name is missing in DATABASE_URL")
	}

	dbName := strings.TrimPrefix(parsed.Path, "/")
	baseURL := *parsed
	baseURL.Path = "/postgres"
	baseURL.RawPath = "/postgres"

	adminConn, err := sql.Open("postgres", baseURL.String())
	if err != nil {
		return err
	}
	defer adminConn.Close()

	if err := adminConn.Ping(); err != nil {
		return fmt.Errorf("connect to default postgres database to create %s: %w", dbName, err)
	}

	quotedName := pq.QuoteIdentifier(dbName)
	if _, err := adminConn.Exec("SELECT 1 FROM pg_database WHERE datname = $1", dbName); err == nil {
		var exists bool
		if err := adminConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists); err == nil && exists {
			return nil
		}
	}

	if _, err := adminConn.Exec("CREATE DATABASE " + quotedName); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create database %s: %w", dbName, err)
		}
	}

	log.Printf("Created PostgreSQL database %s if it was missing", dbName)
	return nil
}

// RunMigrations checks and adds required workflow and OCR columns to the
// documents table. Safe to run even when schema_postgres.sql already
// created these columns — "IF NOT EXISTS" makes every statement a no-op.
func RunMigrations() {
	columns := []string{
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS workflow_status VARCHAR(50) DEFAULT 'PENDING_REVIEW'",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS mine_code VARCHAR(50) NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS inspector_name VARCHAR(100) NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS inspection_date DATE NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS compliance_status VARCHAR(50) DEFAULT 'COMPLIANT'",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS violation_details TEXT NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS risk_level VARCHAR(30) DEFAULT 'LOW'",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS corrective_action TEXT NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS due_date DATE NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS regulatory_reference VARCHAR(255) NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS ocr_data_json JSONB NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS reviewed_by INT NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS approved_by INT NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS verified_by INT NULL",
		"ALTER TABLE documents ADD COLUMN IF NOT EXISTS verified_at TIMESTAMP NULL",
	}

	for _, stmt := range columns {
		if _, err := DB.Exec(stmt); err != nil {
			log.Printf("Migration notice (or error): %v", err)
		}
	}
	log.Println("Database schema migrations verified.")
}
