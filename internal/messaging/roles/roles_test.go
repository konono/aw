package roles

import (
	"strings"
	"testing"
)

func TestRenderDeveloper(t *testing.T) {
	data := TemplateData{
		TeamName:  "test-team",
		AgentName: "developer-1",
		Members: []MemberData{
			{AgentName: "developer-1", Role: "developer", IsSelf: true},
			{AgentName: "reviewer-1", Role: "reviewer", IsSelf: false},
		},
	}

	out, err := Render("developer", data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "developer-1") {
		t.Error("expected output to contain agent name")
	}
	if !strings.Contains(out, "test-team") {
		t.Error("expected output to contain team name")
	}
}

func TestRenderAllRoles(t *testing.T) {
	roles := []string{"developer", "reviewer", "lead", "partner"}
	data := TemplateData{
		TeamName:  "t",
		AgentName: "agent-1",
		Members: []MemberData{
			{AgentName: "agent-1", Role: "developer", IsSelf: true},
		},
	}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			out, err := Render(role, data)
			if err != nil {
				t.Fatalf("Render(%q): %v", role, err)
			}
			if out == "" {
				t.Fatalf("Render(%q) returned empty", role)
			}
		})
	}
}

func TestRenderWithTask(t *testing.T) {
	data := TemplateData{
		TeamName:  "test-team",
		AgentName: "developer-1",
		Members:   []MemberData{{AgentName: "developer-1", Role: "developer", IsSelf: true}},
		Task:      "Implement FizzBuzz",
	}

	out, err := Render("developer", data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "### Task") {
		t.Error("expected output to contain ### Task section")
	}
	if !strings.Contains(out, "Implement FizzBuzz") {
		t.Error("expected output to contain task description")
	}
}

func TestRenderWithoutTask(t *testing.T) {
	data := TemplateData{
		TeamName:  "test-team",
		AgentName: "developer-1",
		Members:   []MemberData{{AgentName: "developer-1", Role: "developer", IsSelf: true}},
	}

	out, err := Render("developer", data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "### Task") {
		t.Error("expected output to NOT contain ### Task section when task is empty")
	}
}

func TestRenderUnknownRole(t *testing.T) {
	data := TemplateData{TeamName: "t", AgentName: "a"}
	_, err := Render("nonexistent", data)
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
}
