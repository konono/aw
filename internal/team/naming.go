package team

import "fmt"

// MemberRole pairs a role name with its input position, used for name generation.
type MemberRole struct {
	Role string
}

// AssignNames takes a list of team members and returns their assigned agent names.
// Names follow the pattern {role}-{N} where N is the occurrence count for that role.
// Example: two developers → developer-1, developer-2
func AssignNames(members []MemberRole) []string {
	counts := make(map[string]int, len(members))
	names := make([]string, len(members))
	for i, m := range members {
		counts[m.Role]++
		names[i] = fmt.Sprintf("%s-%d", m.Role, counts[m.Role])
	}
	return names
}
