package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUnauthenticated = errors.New("unauthenticated")

type User struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	DisplayName     string `json:"display_name"`
	ExperienceLevel string `json:"experience_level"`
}

type Store struct {
	pool *pgxpool.Pool
	ttl  time.Duration
}

func NewStore(pool *pgxpool.Pool, ttl time.Duration) *Store {
	return &Store{pool: pool, ttl: ttl}
}

// UpsertDevUser is for development only. Real sign-in will use OIDC.
func (s *Store) UpsertDevUser(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name := email
	if i := strings.Index(email, "@"); i > 0 {
		name = email[:i]
	}

	const q = `
		INSERT INTO users (oidc_subject, email, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (oidc_subject) DO UPDATE SET email = EXCLUDED.email
		RETURNING id::text, coalesce(email, ''), coalesce(display_name, ''), experience_level`

	var u User
	err := s.pool.QueryRow(ctx, q, "dev:"+email, email, name).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.ExperienceLevel)
	if err != nil {
		return User{}, fmt.Errorf("upsert dev user: %w", err)
	}
	return u, nil
}

// Create makes a session and returns the raw token (only ever sent to the browser).
func (s *Store) Create(ctx context.Context, userID string) (string, time.Time, error) {
	token, hash, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(s.ttl)

	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		hash, userID, expires)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return token, expires, nil
}

// Lookup returns the user for a valid, unexpired session token.
func (s *Store) Lookup(ctx context.Context, token string) (User, error) {
	const q = `
		SELECT u.id::text, coalesce(u.email, ''), coalesce(u.display_name, ''), u.experience_level
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.deleted_at IS NULL`

	var u User
	err := s.pool.QueryRow(ctx, q, hashToken(token)).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.ExperienceLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup session: %w", err)
	}
	return u, nil
}

func (s *Store) Delete(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

func newToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Request-context helpers so handlers can read the signed-in user.
type ctxKey struct{}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}
