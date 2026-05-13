package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"zheng-harness/internal/domain"

	"golang.org/x/crypto/bcrypt"
)

type SQLiteUserStore struct {
	db *sql.DB
}

func NewSQLiteUserStore(dbPath string) (*SQLiteUserStore, error) {
	return NewSQLiteUserStoreWithOptions(dbPath, SQLiteOptions{})
}

func NewSQLiteUserStoreWithOptions(dbPath string, opts SQLiteOptions) (*SQLiteUserStore, error) {
	db, err := openSQLiteWithOptions(dbPath, opts)
	if err != nil {
		return nil, err
	}
	return &SQLiteUserStore{db: db}, nil
}

func (s *SQLiteUserStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteUserStore) Create(ctx context.Context, user domain.User) (domain.User, error) {
	if s == nil || s.db == nil {
		return domain.User{}, errors.New("sqlite user store is not initialized")
	}
	user.ID = strings.TrimSpace(user.ID)
	user.Username = strings.TrimSpace(strings.ToLower(user.Username))
	user.PasswordHash = strings.TrimSpace(user.PasswordHash)
	if user.ID == "" || user.Username == "" || user.PasswordHash == "" || user.CreatedAt.IsZero() {
		return domain.User{}, fmt.Errorf("user is incomplete")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, user.ID, user.Username, user.PasswordHash, user.CreatedAt.UTC())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.User{}, fmt.Errorf("user already exists: %w", err)
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *SQLiteUserStore) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	if s == nil || s.db == nil {
		return domain.User{}, errors.New("sqlite user store is not initialized")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = ?
	`, strings.TrimSpace(strings.ToLower(username)))
	var user domain.User
	var createdAt time.Time
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &createdAt); err != nil {
		return domain.User{}, err
	}
	user.CreatedAt = createdAt.UTC()
	return user, nil
}

const defaultAdminUsername = "admin"
const defaultAdminPassword = "admin123"

// EnsureDefaultAdmin 创建默认管理员账号（如果不存在）
func (s *SQLiteUserStore) EnsureDefaultAdmin() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite user store is not initialized")
	}

	ctx := context.Background()

	// 检查是否已存在
	_, err := s.GetByUsername(ctx, defaultAdminUsername)
	if err == nil {
		// 用户已存在
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("检查默认用户失败: %w", err)
	}

	// 创建默认管理员
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("加密默认密码失败: %w", err)
	}

	user := domain.User{
		ID:           "default-admin",
		Username:     defaultAdminUsername,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now().UTC(),
	}

	_, err = s.Create(ctx, user)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			// 并发情况下可能已被其他实例创建
			return nil
		}
		return fmt.Errorf("创建默认管理员失败: %w", err)
	}

	fmt.Printf("✓ 默认管理员账号已创建: %s / %s\n", defaultAdminUsername, defaultAdminPassword)
	return nil
}
