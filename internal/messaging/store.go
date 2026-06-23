package messaging

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Message represents a single inter-agent message.
type Message struct {
	ID        int64
	From      string
	To        string
	Body      string
	CreatedAt time.Time
	ReadAt    *time.Time
}

// MessagePreview is a summary returned by ReadInbox.
type MessagePreview struct {
	ID        int64
	From      string
	Preview   string
	CreatedAt time.Time
}

// AgentInfo describes a known agent from message history.
type AgentInfo struct {
	Name     string
	LastSeen time.Time
}

// Store manages the SQLite message database.
type Store struct {
	db *sql.DB
}

// OpenStore opens (or creates) the message database at the given path.
func OpenStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating messaging dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("opening message db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating message db: %w", err)
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

// Send inserts a new message and returns its ID.
func (s *Store) Send(from, to, body string) (int64, time.Time, error) {
	now := time.Now().UTC()
	ts := now.Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`INSERT INTO messages (from_agent, to_agent, body, created_at) VALUES (?, ?, ?, ?)`,
		from, to, body, ts,
	)
	if err != nil {
		return 0, time.Time{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, time.Time{}, err
	}
	return id, now, nil
}

const previewLen = 200

// ReadInbox returns unread message previews for the given agent.
func (s *Store) ReadInbox(agent string) ([]MessagePreview, error) {
	rows, err := s.db.Query(
		`SELECT id, from_agent, body, created_at FROM messages
		 WHERE to_agent = ? AND read_at IS NULL
		 ORDER BY created_at`,
		agent,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var previews []MessagePreview
	for rows.Next() {
		var id int64
		var from, body, ts string
		if err := rows.Scan(&id, &from, &body, &ts); err != nil {
			return nil, err
		}
		preview := body
		if len(preview) > previewLen {
			preview = preview[:previewLen] + "..."
		}
		t, _ := time.Parse(time.RFC3339Nano, ts)
		previews = append(previews, MessagePreview{
			ID:        id,
			From:      from,
			Preview:   preview,
			CreatedAt: t,
		})
	}
	return previews, rows.Err()
}

// ReadMessage returns a full message by ID and marks it as read.
func (s *Store) ReadMessage(id int64) (*Message, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`UPDATE messages SET read_at = ? WHERE id = ? AND read_at IS NULL`,
		now, id,
	)
	if err != nil {
		return nil, err
	}

	var from, to, body, createdAt string
	var readAt sql.NullString
	err = s.db.QueryRow(
		`SELECT from_agent, to_agent, body, created_at, read_at FROM messages WHERE id = ?`,
		id,
	).Scan(&from, &to, &body, &createdAt, &readAt)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		ID:   id,
		From: from,
		To:   to,
		Body: body,
	}
	msg.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if readAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, readAt.String)
		msg.ReadAt = &t
	}
	return msg, nil
}

// MarkRead marks a message as read.
func (s *Store) MarkRead(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`UPDATE messages SET read_at = ? WHERE id = ? AND read_at IS NULL`,
		now, id,
	)
	return err
}

// UnreadCount returns the number of unread messages for the given agent.
func (s *Store) UnreadCount(agent string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE to_agent = ? AND read_at IS NULL`,
		agent,
	).Scan(&count)
	return count, err
}

// ListAgents returns all known agents from message history.
func (s *Store) ListAgents() ([]AgentInfo, error) {
	rows, err := s.db.Query(`
		SELECT agent, MAX(ts) AS last_seen FROM (
			SELECT from_agent AS agent, created_at AS ts FROM messages
			UNION ALL
			SELECT to_agent AS agent, created_at AS ts FROM messages
		) GROUP BY agent ORDER BY agent`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var agents []AgentInfo
	for rows.Next() {
		var name, ts string
		if err := rows.Scan(&name, &ts); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339Nano, ts)
		agents = append(agents, AgentInfo{Name: name, LastSeen: t})
	}
	return agents, rows.Err()
}

// History returns messages, optionally filtered by agent, newest first.
func (s *Store) History(agent string, limit int) ([]Message, error) {
	var query string
	var args []interface{}
	if agent != "" {
		query = `SELECT id, from_agent, to_agent, body, created_at, read_at
				 FROM messages WHERE from_agent = ? OR to_agent = ?
				 ORDER BY created_at DESC LIMIT ?`
		args = []interface{}{agent, agent, limit}
	} else {
		query = `SELECT id, from_agent, to_agent, body, created_at, read_at
				 FROM messages ORDER BY created_at DESC LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var msgs []Message
	for rows.Next() {
		var id int64
		var from, to, body, createdAt string
		var readAt sql.NullString
		if err := rows.Scan(&id, &from, &to, &body, &createdAt, &readAt); err != nil {
			return nil, err
		}
		msg := Message{ID: id, From: from, To: to, Body: body}
		msg.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if readAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, readAt.String)
			msg.ReadAt = &t
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// Watch polls for new unread messages for the given agent.
// It calls fn for each new message. Blocks until ctx is cancelled.
func (s *Store) Watch(agent string, interval time.Duration, fn func(Message)) error {
	var lastID int64
	for {
		rows, err := s.db.Query(
			`SELECT id, from_agent, to_agent, body, created_at FROM messages
			 WHERE to_agent = ? AND id > ? ORDER BY id`,
			agent, lastID,
		)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var from, to, body, ts string
			if err := rows.Scan(&id, &from, &to, &body, &ts); err != nil {
				_ = rows.Close()
				return err
			}
			msg := Message{ID: id, From: from, To: to, Body: body}
			msg.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
			fn(msg)
			lastID = id
		}
		_ = rows.Close()

		time.Sleep(interval)
	}
}

// WatchAll polls for all new messages (not filtered by agent).
func (s *Store) WatchAll(interval time.Duration, fn func(Message)) error {
	var lastID int64
	for {
		rows, err := s.db.Query(
			`SELECT id, from_agent, to_agent, body, created_at FROM messages
			 WHERE id > ? ORDER BY id`,
			lastID,
		)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var from, to, body, ts string
			if err := rows.Scan(&id, &from, &to, &body, &ts); err != nil {
				_ = rows.Close()
				return err
			}
			msg := Message{ID: id, From: from, To: to, Body: body}
			msg.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
			fn(msg)
			lastID = id
		}
		_ = rows.Close()

		time.Sleep(interval)
	}
}

// truncate returns s truncated to maxLen with "..." appended if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatPreview returns a short single-line preview of a message body.
func FormatPreview(body string) string {
	firstLine := body
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	return truncate(firstLine, 80)
}
