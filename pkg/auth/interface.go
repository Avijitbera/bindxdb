package auth

import (
	"context"
	"time"
)

type AuthProvider interface {
	Name() string
	Authenticate(ctx context.Context, credentials map[string]string) (*AuthResult, error)

	ValidateToken(ctx context.Context, token string) (*AuthResult, error)

	RefreshToken(ctx context.Context, token string) (*AuthResult, error)

	RevokeToken(ctx context.Context, token string) error
}

type AuthResult struct {
	Success bool
	UserID  string

	Username     string
	Email        string
	Roles        []string
	Permissions  []string
	Token        string
	RefreshToken string
	ExpiresAt    time.Time
	Metadata     map[string]interface{}
}

type User struct {
	ID           string
	Username     string
	Email        string
	Roles        []string
	Permissions  []string
	Metadata     map[string]interface{}
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLogin    time.Time
	Enabled      bool
	PasswordHash string
}

type Role struct {
	Name        string
	Permissions []string
	Description string
}

type Permission struct {
	Resource string
	Action   string
	Effect   string
}

type AuthContext struct {
	UserID        string
	Username      string
	Roles         []string
	Permissions   []Permission
	Token         string
	Authenticated bool
	ExpiresAt     time.Time
}

type Authorizer interface {
	Authorize(ctx context.Context, authCtx *AuthContext, resource string, action string) (bool, error)
	GetRole(ctx context.Context, authCtx *AuthContext, role string) ([]Permission, error)
	HasRole(ctx context.Context, authCtx *AuthContext, role string) (bool, error)
}

type UserStore interface {
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)

	GetUserByEmail(ctx context.Context, email string) (*User, error)

	CreateUser(ctx context.Context, email string) error

	UpdateUser(ctx context.Context, user *User) error

	DeleteUser(ctx context.Context, id string) error

	ListUsers(ctx context.Context, offset, limit int) ([]*User, error)
}

type TokenStore interface {
	StoreToken(ctx context.Context, token string, userID string, expiresAt time.Time) error
	ValidateToken(ctx context.Context, token string) (string, error)
	RevokeToken(ctx context.Context, token string) error
	CleanupExpired(ctx context.Context) error
}
