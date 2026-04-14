package http

import (
	"testing"
)

func TestParseMoneyInput_ValidFloat(t *testing.T) {
	got, err := parseMoneyInput("29.99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2999 {
		t.Errorf("expected 2999 cents; got %d", got)
	}
}

func TestParseMoneyInput_Integer(t *testing.T) {
	got, err := parseMoneyInput("30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 3000 {
		t.Errorf("expected 3000 cents; got %d", got)
	}
}

func TestParseMoneyInput_EmptyString(t *testing.T) {
	got, err := parseMoneyInput("")
	if err != nil {
		t.Fatalf("empty string should parse as 0, got error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0 cents for empty string; got %d", got)
	}
}

func TestParseMoneyInput_Zero(t *testing.T) {
	got, err := parseMoneyInput("0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0; got %d", got)
	}
}
