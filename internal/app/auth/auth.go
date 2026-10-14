package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrUnauthorized = errors.New("invalid credentials or token")
	ErrUnavailable  = errors.New("blacklist unavailable")
)

type Blacklist interface {
	Contains(context.Context, string) (bool, error)
	Revoke(context.Context, string, time.Time) error
}

type Service struct {
	secret    []byte
	issuer    string
	ttl       time.Duration
	blacklist Blacklist
}

func New(secret, issuer string, ttl time.Duration, blacklist Blacklist) (*Service, error) {
	if len(secret) < 32 || issuer == "" || ttl < time.Second || blacklist == nil {
		return nil, errors.New("JWT needs a secret of at least 32 bytes, issuer, positive TTL and blacklist")
	}
	return &Service{[]byte(secret), issuer, ttl, blacklist}, nil
}

func (s *Service) Issue(userID uint) (string, int, error) {
	if userID == 0 {
		return "", 0, ErrUnauthorized
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject: strconv.FormatUint(uint64(userID), 10), Issuer: s.issuer,
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)), ID: uuid.NewString(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return token, int(s.ttl / time.Second), err
}

func (s *Service) Validate(ctx context.Context, raw string) (uint, *jwt.RegisteredClaims, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(_ *jwt.Token) (interface{}, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(s.issuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || claims.ID == "" || claims.IssuedAt == nil {
		return 0, nil, ErrUnauthorized
	}
	id, err := strconv.ParseUint(claims.Subject, 10, 31)
	if err != nil || id == 0 {
		return 0, nil, ErrUnauthorized
	}
	revoked, err := s.blacklist.Contains(ctx, raw)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if revoked {
		return 0, nil, ErrUnauthorized
	}
	return uint(id), claims, nil
}

func (s *Service) Logout(ctx context.Context, raw string, expiresAt time.Time) error {
	if err := s.blacklist.Revoke(ctx, raw, expiresAt); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}
