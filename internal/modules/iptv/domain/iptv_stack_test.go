package domain

import (
	"testing"
)

func TestValidateStackName(t *testing.T) {
	tests := []struct {
		name      string
		stackName string
		wantErr   bool
	}{
		{
			name:      "valid alphanumeric",
			stackName: "mystack123",
			wantErr:   false,
		},
		{
			name:      "valid with hyphens",
			stackName: "my-stack-name",
			wantErr:   false,
		},
		{
			name:      "valid with underscores",
			stackName: "my_stack_name",
			wantErr:   false,
		},
		{
			name:      "valid mixed",
			stackName: "Stack_Name-123",
			wantErr:   false,
		},
		{
			name:      "empty string",
			stackName: "",
			wantErr:   true,
		},
		{
			name:      "too long",
			stackName: "a123456789012345678901234567890123456789012345678901234567890123456",
			wantErr:   true,
		},
		{
			name:      "contains space",
			stackName: "my stack",
			wantErr:   true,
		},
		{
			name:      "contains semicolon (command injection)",
			stackName: "stack; rm -rf /",
			wantErr:   true,
		},
		{
			name:      "contains backtick (command injection)",
			stackName: "stack`whoami`",
			wantErr:   true,
		},
		{
			name:      "contains dollar (command injection)",
			stackName: "stack$(whoami)",
			wantErr:   true,
		},
		{
			name:      "contains pipe (command injection)",
			stackName: "stack | cat /etc/passwd",
			wantErr:   true,
		},
		{
			name:      "contains ampersand (command injection)",
			stackName: "stack & whoami",
			wantErr:   true,
		},
		{
			name:      "contains single quote",
			stackName: "stack'test",
			wantErr:   true,
		},
		{
			name:      "contains double quote",
			stackName: "stack\"test",
			wantErr:   true,
		},
		{
			name:      "contains slash",
			stackName: "stack/test",
			wantErr:   true,
		},
		{
			name:      "contains backslash",
			stackName: "stack\\test",
			wantErr:   true,
		},
		{
			name:      "contains newline",
			stackName: "stack\ntest",
			wantErr:   true,
		},
		{
			name:      "contains tab",
			stackName: "stack\ttest",
			wantErr:   true,
		},
		{
			name:      "contains asterisk",
			stackName: "stack*",
			wantErr:   true,
		},
		{
			name:      "contains question mark",
			stackName: "stack?",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStackName(tt.stackName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStackName(%q) error = %v, wantErr %v", tt.stackName, err, tt.wantErr)
			}
		})
	}
}
