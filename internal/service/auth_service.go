package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/store"
)

const bcryptCost = 12

type AuthService struct {
	users     *store.SQLiteUserStore
	jwtSecret string
	clock     func() time.Time
}

type AuthDependencies struct {
	UserStore *store.SQLiteUserStore
	JWTSecret string
	Clock     func() time.Time
}

type AuthResult struct {
	Token string      `json:"token"`
	User  domain.User `json:"user"`
}

func NewAuthService(deps AuthDependencies) *AuthService {
	return &AuthService{users: deps.UserStore, jwtSecret: strings.TrimSpace(deps.JWTSecret), clock: deps.Clock}
}

func (s *AuthService) Register(ctx context.Context, username, password string) (*AuthResult, error) {
	if s == nil {
		return nil, errors.New("auth service is nil")
	}
	username = normalizeUsername(username)
	password = strings.TrimSpace(password)
	if username == "" {
		return nil, &ValidationError{Message: "username is required"}
	}
	if len(username) < 3 {
		return nil, &ValidationError{Message: "username must be at least 3 characters"}
	}
	if password == "" {
		return nil, &ValidationError{Message: "password is required"}
	}
	if len(password) < 8 {
		return nil, &ValidationError{Message: "password must be at least 8 characters"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	user := domain.User{ID: uuid.NewString(), Username: username, PasswordHash: string(hash), CreatedAt: s.now().UTC()}
	created, err := s.users.Create(ctx, user)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, &ConflictError{Message: "username already exists", Err: err}
		}
		return nil, err
	}
	token, err := s.issueToken(created.Username)
	if err != nil {
		return nil, err
	}
	created.PasswordHash = ""
	return &AuthResult{Token: token, User: created}, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*AuthResult, error) {
	if s == nil {
		return nil, errors.New("auth service is nil")
	}
	username = normalizeUsername(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, &ValidationError{Message: "username and password are required"}
	}
	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ValidationError{Message: "invalid username or password"}
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, &ValidationError{Message: "invalid username or password"}
	}
	token, err := s.issueToken(user.Username)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return &AuthResult{Token: token, User: user}, nil
}

func (s *AuthService) issueToken(subject string) (string, error) {
	if strings.TrimSpace(s.jwtSecret) == "" {
		return "", errors.New("jwt secret is not configured")
	}
	now := s.now()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(map[string]any{"sub": strings.TrimSpace(subject), "iat": now.Unix(), "exp": now.Add(8 * time.Hour).Unix()})
	if err != nil {
		return "", fmt.Errorf("marshal jwt payload: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *AuthService) now() time.Time {
	if s != nil && s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(strings.ToLower(username))
}
