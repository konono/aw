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
	Team      string
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

// DB returns the underlying database connection for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

// Send inserts a new message and returns its ID and timestamp.
func (s *Store) Send(team, from, to, body string) (int64, time.Time, error) {
	res, err := s.db.Exec(
		`INSERT INTO messages (team, from_agent, to_agent, body) VALUES (?, ?, ?, ?)`,
		team, from, to, body,
	)
	if err != nil {
		return 0, time.Time{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, time.Time{}, err
	}
	var ts string
	_ = s.db.QueryRow(`SELECT created_at FROM messages WHERE id = ?`, id).Scan(&ts)
	t, _ := time.Parse("2006-01-02T15:04:05Z", ts)
	return id, t, nil
}

const previewLen = 200

// ReadInbox returns unread message previews for the given agent in a team.
func (s *Store) ReadInbox(team, agent string) ([]MessagePreview, error) {
	rows, err := s.db.Query(
		`SELECT id, from_agent, body, created_at FROM messages
		 WHERE team = ? AND to_agent = ? AND read_at IS NULL
		 ORDER BY created_at`,
		team, agent,
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
		t, _ := time.Parse("2006-01-02T15:04:05Z", ts)
		previews = append(previews, MessagePreview{
			ID:        id,
			From:      from,
			Preview:   preview,
			CreatedAt: t,
		})
	}
	return previews, rows.Err()
}

// ReadMessage returns a full message by ID without marking it as read.
func (s *Store) ReadMessage(id int64) (*Message, error) {
	var team, from, to, body, createdAt string
	var readAt sql.NullString
	err := s.db.QueryRow(
		`SELECT team, from_agent, to_agent, body, created_at, read_at FROM messages WHERE id = ?`,
		id,
	).Scan(&team, &from, &to, &body, &createdAt, &readAt)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		ID:   id,
		Team: team,
		From: from,
		To:   to,
		Body: body,
	}
	msg.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	if readAt.Valid {
		t, _ := time.Parse("2006-01-02T15:04:05Z", readAt.String)
		msg.ReadAt = &t
	}
	return msg, nil
}

// MarkRead marks a message as read.
func (s *Store) MarkRead(id int64) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := s.db.Exec(
		`UPDATE messages SET read_at = ? WHERE id = ? AND read_at IS NULL`,
		now, id,
	)
	return err
}

// UnreadCount returns the number of unread messages for the given agent in a team.
func (s *Store) UnreadCount(team, agent string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE team = ? AND to_agent = ? AND read_at IS NULL`,
		team, agent,
	).Scan(&count)
	return count, err
}

// ListAgents returns all known agents in a team from message history.
func (s *Store) ListAgents(team string) ([]AgentInfo, error) {
	rows, err := s.db.Query(`
		SELECT agent, MAX(ts) AS last_seen FROM (
			SELECT from_agent AS agent, created_at AS ts FROM messages WHERE team = ?
			UNION ALL
			SELECT to_agent AS agent, created_at AS ts FROM messages WHERE team = ?
		) GROUP BY agent ORDER BY agent`, team, team)
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
		t, _ := time.Parse("2006-01-02T15:04:05Z", ts)
		agents = append(agents, AgentInfo{Name: name, LastSeen: t})
	}
	return agents, rows.Err()
}

// History returns messages for a team, optionally filtered by agent, newest first.
func (s *Store) History(team, agent string, limit int) ([]Message, error) {
	var query string
	var args []interface{}
	if agent != "" {
		query = `SELECT id, from_agent, to_agent, body, created_at, read_at
				 FROM messages WHERE team = ? AND (from_agent = ? OR to_agent = ?)
				 ORDER BY created_at DESC LIMIT ?`
		args = []interface{}{team, agent, agent, limit}
	} else {
		query = `SELECT id, from_agent, to_agent, body, created_at, read_at
				 FROM messages WHERE team = ?
				 ORDER BY created_at DESC LIMIT ?`
		args = []interface{}{team, limit}
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
		msg.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		if readAt.Valid {
			t, _ := time.Parse("2006-01-02T15:04:05Z", readAt.String)
			msg.ReadAt = &t
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// Watch polls for new unread messages for the given agent in a team.
func (s *Store) Watch(team, agent string, interval time.Duration, fn func(Message)) error {
	var lastID int64
	for {
		rows, err := s.db.Query(
			`SELECT id, from_agent, to_agent, body, created_at FROM messages
			 WHERE team = ? AND to_agent = ? AND id > ? ORDER BY id`,
			team, agent, lastID,
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
			msg.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", ts)
			fn(msg)
			lastID = id
		}
		_ = rows.Close()

		time.Sleep(interval)
	}
}

// WatchAll polls for all new messages in a team.
func (s *Store) WatchAll(team string, interval time.Duration, fn func(Message)) error {
	var lastID int64
	for {
		rows, err := s.db.Query(
			`SELECT id, from_agent, to_agent, body, created_at FROM messages
			 WHERE team = ? AND id > ? ORDER BY id`,
			team, lastID,
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
			msg.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", ts)
			fn(msg)
			lastID = id
		}
		_ = rows.Close()

		time.Sleep(interval)
	}
}

// ClearTeam deletes all messages for the given team scope.
func (s *Store) ClearTeam(team string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM messages WHERE team = ?`, team)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ClearAll deletes all messages.
func (s *Store) ClearAll() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM messages`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ClearBefore deletes messages older than the given duration.
func (s *Store) ClearBefore(d time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-d).Format("2006-01-02T15:04:05Z")
	res, err := s.db.Exec(`DELETE FROM messages WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListTeams returns all distinct team scopes in the database.
func (s *Store) ListTeams() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT team FROM messages ORDER BY team`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var teams []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
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
