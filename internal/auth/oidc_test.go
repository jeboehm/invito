package auth_test

import (
	"testing"

	"github.com/jeboehm/invito/internal/auth"
)

func TestSlugifyUsername(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"alice", "alice"},
		{"Alice.Smith", "alicesmith"},
		{"user@example.com", "userexamplecom"},
		{"hello-world", "hello-world"},
		{"", "user"},
		{"!@#$%", "user"},
		{"müller", "müller"},
		{"123abc", "123abc"},
	}
	for _, tc := range tests {
		got := auth.SlugifyUsername(tc.input)
		if got != tc.want {
			t.Errorf("SlugifyUsername(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
