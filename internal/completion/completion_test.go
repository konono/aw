package completion

import (
	"testing"

	"github.com/posener/complete"
)

func TestToolPredictor_Predict(t *testing.T) {
	p := ToolPredictor{}
	results := p.Predict(complete.Args{})
	expected := map[string]bool{
		"claude": false, "codex": false, "opencode": false, "cursor": false,
	}
	for _, name := range results {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected tool name %q not found in results", name)
		}
	}
}

func TestProfilePredictor_Predict_NoPanic(t *testing.T) {
	p := ProfilePredictor{}
	results := p.Predict(complete.Args{})
	_ = results
}

func TestTeamPredictor_Predict_NoPanic(t *testing.T) {
	p := TeamPredictor{}
	results := p.Predict(complete.Args{})
	_ = results
}
