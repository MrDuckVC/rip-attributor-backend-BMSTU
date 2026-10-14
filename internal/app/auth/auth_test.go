package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	blacklist "attributor/internal/app/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "a-private-test-secret-with-more-than-32-bytes"

func TestJWTAndBlacklist(t *testing.T) {
	r := miniredis.RunT(t)
	ctx := context.Background()
	store, err := blacklist.New(ctx, r.Addr(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s, err := New(testSecret, "attributor", time.Hour, store)
	if err != nil {
		t.Fatal(err)
	}
	raw, ttl, err := s.Issue(42)
	if err != nil || ttl != 3600 {
		t.Fatalf("issue: %d %v", ttl, err)
	}
	id, claims, err := s.Validate(ctx, raw)
	if err != nil || id != 42 {
		t.Fatalf("subject: %d %v", id, err)
	}
	second, _, _ := s.Issue(42)
	if raw == second {
		t.Fatal("login tokens must be unique")
	}
	if err := s.Logout(ctx, raw, claims.ExpiresAt.Time); err != nil {
		t.Fatal(err)
	}
	remaining := r.TTL(blacklist.Prefix + raw)
	if remaining <= 0 || remaining > time.Hour {
		t.Fatalf("TTL: %v", remaining)
	}
	if _, _, err := s.Validate(ctx, raw); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked: %v", err)
	}
	if _, _, err := s.Validate(ctx, second); err != nil {
		t.Fatalf("other login revoked: %v", err)
	}
	// New server instance still sees the same blacklist.
	restarted, _ := New(testSecret, "attributor", time.Hour, store)
	if _, _, err := restarted.Validate(ctx, raw); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("revocation lost")
	}
	r.FastForward(time.Hour)
	if r.Exists(blacklist.Prefix + raw) {
		t.Fatal("expired blacklist entry remains")
	}
	r.Close()
	if _, _, err := s.Validate(ctx, second); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Redis failure: %v", err)
	}
}

func TestRejectInvalidJWT(t *testing.T) {
	r := miniredis.RunT(t)
	store, err := blacklist.New(context.Background(), r.Addr(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s, _ := New(testSecret, "attributor", time.Hour, store)
	for _, name := range []string{"expired", "missing_exp", "issuer", "subject", "future_iat", "algorithm", "signature", "missing_jti"} {
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			claims := jwt.RegisteredClaims{Issuer: "attributor", Subject: "1", ID: "unique", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}
			method := jwt.SigningMethodHS256
			secret := testSecret
			switch name {
			case "expired":
				claims.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Second))
			case "missing_exp":
				claims.ExpiresAt = nil
			case "issuer":
				claims.Issuer = "other"
			case "subject":
				claims.Subject = "0"
			case "future_iat":
				claims.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour))
			case "algorithm":
				method = jwt.SigningMethodHS384
			case "signature":
				secret = "another-private-secret-with-32-bytes"
			case "missing_jti":
				claims.ID = ""
			}
			raw, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = s.Validate(context.Background(), raw); !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("accepted %s: %v", name, err)
			}
		})
	}
}
