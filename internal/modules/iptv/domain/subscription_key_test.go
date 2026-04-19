package domain

import (
	"testing"
	"time"
)

func TestNewSubscriptionKey_TokenLength(t *testing.T) {
	k, err := NewSubscriptionKey("id-1", "sub-1", "cust-1", "pkg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(k.Token) != 64 {
		t.Errorf("token length: want 64, got %d", len(k.Token))
	}
}

func TestNewSubscriptionKey_Unique(t *testing.T) {
	k1, _ := NewSubscriptionKey("id-1", "sub-1", "cust-1", "pkg-1")
	k2, _ := NewSubscriptionKey("id-2", "sub-1", "cust-1", "pkg-1")
	if k1.Token == k2.Token {
		t.Error("two keys should have different tokens")
	}
}

func TestNewSubscriptionKey_MissingSubscriptionID(t *testing.T) {
	_, err := NewSubscriptionKey("id-1", "", "cust-1", "pkg-1")
	if err == nil {
		t.Error("expected error for empty subscription id")
	}
}

func TestNewSubscriptionKey_MissingCustomerID(t *testing.T) {
	_, err := NewSubscriptionKey("id-1", "sub-1", "", "pkg-1")
	if err == nil {
		t.Error("expected error for empty customer id")
	}
}

func TestSubscriptionKey_IsActive_NewKey(t *testing.T) {
	k, _ := NewSubscriptionKey("id-1", "sub-1", "cust-1", "pkg-1")
	if !k.IsActive() {
		t.Error("new key should be active")
	}
}

func TestSubscriptionKey_Revoke(t *testing.T) {
	k, _ := NewSubscriptionKey("id-1", "sub-1", "cust-1", "pkg-1")
	before := time.Now()
	k.Revoke()
	if k.IsActive() {
		t.Error("revoked key should not be active")
	}
	if k.RevokedAt == nil {
		t.Fatal("RevokedAt should be set")
	}
	if k.RevokedAt.Before(before) {
		t.Error("RevokedAt should be >= time before revoke")
	}
}
