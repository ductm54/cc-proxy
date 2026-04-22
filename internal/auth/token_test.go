package auth

import (
	"testing"
	"time"
)

func TestIssueAndValidate(t *testing.T) {
	ts := NewTokenService("test-secret-key", time.Hour)

	token, exp, err := ts.Issue("user@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if time.Until(exp) < 59*time.Minute {
		t.Fatalf("unexpected expiry: %v", exp)
	}

	email, err := ts.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("got email %q, want user@example.com", email)
	}
}

func TestValidateExpired(t *testing.T) {
	ts := NewTokenService("test-secret-key", -time.Second)

	token, _, err := ts.Issue("user@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = ts.Validate(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateWrongSecret(t *testing.T) {
	ts1 := NewTokenService("secret-1", time.Hour)
	ts2 := NewTokenService("secret-2", time.Hour)

	token, _, err := ts1.Issue("user@example.com")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = ts2.Validate(token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateGarbage(t *testing.T) {
	ts := NewTokenService("test-secret-key", time.Hour)
	_, err := ts.Validate("not-a-jwt")
	if err == nil {
		t.Fatal("expected error for garbage token")
	}
}
