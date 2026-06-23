package team

import "testing"

func TestAssignNames(t *testing.T) {
	tests := []struct {
		name    string
		members []MemberRole
		want    []string
	}{
		{
			name:    "single member",
			members: []MemberRole{{Role: "developer"}},
			want:    []string{"developer-1"},
		},
		{
			name: "two same role",
			members: []MemberRole{
				{Role: "developer"},
				{Role: "developer"},
			},
			want: []string{"developer-1", "developer-2"},
		},
		{
			name: "mixed roles",
			members: []MemberRole{
				{Role: "lead"},
				{Role: "developer"},
				{Role: "reviewer"},
				{Role: "developer"},
			},
			want: []string{"lead-1", "developer-1", "reviewer-1", "developer-2"},
		},
		{
			name:    "empty",
			members: nil,
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssignNames(tt.members)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d names, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("name[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
