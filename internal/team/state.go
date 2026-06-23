package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TeamState represents the persisted state of a running team.
type TeamState struct {
	Name      string        `json:"name"`
	StartedAt string        `json:"started_at"`
	Members   []MemberState `json:"members"`
}

// MemberState represents the persisted state of a single team member.
type MemberState struct {
	AgentName     string `json:"agent_name"`
	Profile       string `json:"profile"`
	Role          string `json:"role"`
	ContainerName string `json:"container_name"`
	Foreground    bool   `json:"foreground"`
	Status        string `json:"status"` // "running", "stopped"
}

// StateDir returns the directory used for team state files (~/.config/aw/teams/).
func StateDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to $HOME/.config if UserConfigDir fails.
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "aw", "teams")
}

func stateFilePath(teamName string) string {
	return filepath.Join(StateDir(), teamName+".state.json")
}

// SaveState persists a TeamState to disk.
func SaveState(state TeamState) error {
	dir := StateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling team state: %w", err)
	}
	path := stateFilePath(state.Name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing state file %s: %w", path, err)
	}
	return nil
}

// LoadState reads a TeamState from disk by team name.
func LoadState(teamName string) (*TeamState, error) {
	path := stateFilePath(teamName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading state file %s: %w", path, err)
	}
	var state TeamState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshalling state file %s: %w", path, err)
	}
	return &state, nil
}

// RemoveState deletes the state file for the given team.
func RemoveState(teamName string) error {
	path := stateFilePath(teamName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing state file %s: %w", path, err)
	}
	return nil
}

// ListStates returns all persisted team states.
func ListStates() ([]TeamState, error) {
	dir := StateDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state directory %s: %w", dir, err)
	}
	var states []TeamState
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		teamName := e.Name()[:len(e.Name())-len(".state.json")]
		state, err := LoadState(teamName)
		if err != nil {
			continue // skip corrupt state files
		}
		states = append(states, *state)
	}
	return states, nil
}
