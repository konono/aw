package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/konono/aw/internal/messaging"
)

func toolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "send_message",
			"description": "Send a message to another agent in your team. Your identity (from) is set automatically.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"to":   map[string]interface{}{"type": "string", "description": "Recipient agent name (e.g. reviewer-1)"},
					"body": map[string]interface{}{"type": "string", "description": "Message content"},
				},
				"required": []string{"to", "body"},
			},
		},
		{
			"name":        "read_inbox",
			"description": "Check for unread messages. Returns a list of message previews (first 200 chars). Use read_message to get the full content.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "read_message",
			"description": "Read the full content of a specific message by ID. Does NOT mark it as read — use mark_read for that.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "number", "description": "Message ID from read_inbox"},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "mark_read",
			"description": "Mark a message as read.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "number", "description": "Message ID to mark as read"},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "list_agents",
			"description": "List all known agents in your team from message history with their last activity time.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (s *Server) callTool(store *messaging.Store, name string, args json.RawMessage) (interface{}, error) {
	switch name {
	case "send_message":
		return s.toolSendMessage(store, args)
	case "read_inbox":
		return s.toolReadInbox(store)
	case "read_message":
		return s.toolReadMessage(store, args)
	case "mark_read":
		return s.toolMarkRead(store, args)
	case "list_agents":
		return s.toolListAgents(store)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) toolSendMessage(store *messaging.Store, args json.RawMessage) (interface{}, error) {
	var params struct {
		To   string `json:"to"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if params.To == "" {
		return nil, fmt.Errorf("'to' is required")
	}
	if params.Body == "" {
		return nil, fmt.Errorf("'body' is required")
	}

	id, createdAt, err := store.Send(s.teamName, s.agentName, params.To, params.Body)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":         id,
		"created_at": createdAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *Server) toolReadInbox(store *messaging.Store) (interface{}, error) {
	previews, err := store.ReadInbox(s.teamName, s.agentName)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, len(previews))
	for i, p := range previews {
		result[i] = map[string]interface{}{
			"id":         p.ID,
			"from":       p.From,
			"preview":    p.Preview,
			"created_at": p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return result, nil
}

func (s *Server) toolReadMessage(store *messaging.Store, args json.RawMessage) (interface{}, error) {
	var params struct {
		ID float64 `json:"id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	msg, err := store.ReadMessage(int64(params.ID))
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":         msg.ID,
		"from":       msg.From,
		"to":         msg.To,
		"body":       msg.Body,
		"created_at": msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *Server) toolMarkRead(store *messaging.Store, args json.RawMessage) (interface{}, error) {
	var params struct {
		ID float64 `json:"id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if err := store.MarkRead(int64(params.ID)); err != nil {
		return nil, err
	}
	return map[string]interface{}{"ok": true}, nil
}

func (s *Server) toolListAgents(store *messaging.Store) (interface{}, error) {
	agents, err := store.ListAgents(s.teamName)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, len(agents))
	for i, a := range agents {
		result[i] = map[string]interface{}{
			"name":      a.Name,
			"last_seen": a.LastSeen.Format("2006-01-02T15:04:05Z"),
		}
	}
	return result, nil
}
