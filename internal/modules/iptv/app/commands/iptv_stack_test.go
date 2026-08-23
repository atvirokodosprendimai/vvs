package commands

import (
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "mystack",
			expected: "'mystack'",
		},
		{
			name:     "string with single quote",
			input:    "my'stack",
			expected: "'my'\\''stack'",
		},
		{
			name:     "string with multiple single quotes",
			input:    "my'stack'name",
			expected: "'my'\\''stack'\\''name'",
		},
		{
			name:     "string with spaces",
			input:    "my stack",
			expected: "'my stack'",
		},
		{
			name:     "string with special chars",
			input:    "stack; rm -rf /",
			expected: "'stack; rm -rf /'",
		},
		{
			name:     "string with backticks",
			input:    "stack`whoami`",
			expected: "'stack`whoami`'",
		},
		{
			name:     "string with dollar",
			input:    "stack$(whoami)",
			expected: "'stack$(whoami)'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shellQuote(tt.input)
			if result != tt.expected {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
