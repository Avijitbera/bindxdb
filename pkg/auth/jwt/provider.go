package jwt

import (
	"bindxdb/pkg/auth"
	"bindxdb/pkg/config"
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTProvider struct {
	name          string
	secretKey     []byte
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	signingMethod jwt.SigningMethod
	issuer        string
	audience      string
	expiration    time.Duration
	refreshExp    time.Duration
	tokenStore    auth.TokenStore
	userStore     auth.UserStore
	config        *config.ConfigManager
}

type JWTConfig struct {
	Name       string        `json:"name"`
	SecretKey  string        `json:"secret_key"`
	PrivateKey string        `json:"private_key"`
	PublicKey  string        `json:"public_key"`
	Algorithm  string        `json:"algorithm"`
	Issuer     string        `json:"issuer"`
	Audience   string        `json:"audience"`
	Expiration time.Duration `json:"expiration"`
	RefreshExp time.Duration `json:"refresh_exp"`
}

func NewJWTProvider(cfg *JWTConfig, userStore auth.UserStore, tokenStore auth.TokenStore,
	config *config.ConfigManager) (*JWTProvider, error) {
	provider := &JWTProvider{
		name:       cfg.Name,
		issuer:     cfg.Issuer,
		audience:   cfg.Audience,
		expiration: cfg.Expiration,
		refreshExp: cfg.RefreshExp,
		userStore:  userStore,
		tokenStore: tokenStore,
		config:     config,
	}
	switch cfg.Algorithm {
	case "HS256":
		provider.signingMethod = jwt.SigningMethodES256
		provider.secretKey = []byte(cfg.SecretKey)
	case "HS384":
		provider.signingMethod = jwt.SigningMethodHS384
		provider.secretKey = []byte(cfg.SecretKey)
	case "HS512":
		provider.signingMethod = jwt.SigningMethodHS512
		provider.secretKey = []byte(cfg.SecretKey)
	case "RS256":
		provider.signingMethod = jwt.SigningMethodRS256
		privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		provider.privateKey = privateKey
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}

		provider.publicKey = publicKey
	default:
		return nil, fmt.Errorf("unsupported signing method: %s", cfg.Algorithm)

	}
	return provider, nil
}

func (p *JWTProvider) Name() string {
	return p.name
}

func (p *JWTProvider) Authenticate(ctx context.Context, credentials map[string]string) (*auth.AuthResult, error) {
	username, ok := credentials["username"]
	if !ok {
		return nil, errors.New("username required")
	}
	password, ok := credentials["password"]
	if !ok {
		return nil, errors.New("password required")
	}

	user, err := p.userStore.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.Enabled {
		return nil, errors.New("user is disabled")
	}

	if !p.verifyPassword(password, user.PasswordHash) {
		return nil, errors.New("invalid password")
	}

	accessToken, err := p.generateToken(user, p.expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := p.generateToken(user, p.refreshExp)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	if err := p.tokenStore.StoreToken(ctx, accessToken, user.ID, time.Now().Add(p.expiration)); err != nil {
		return nil, fmt.Errorf("failed to store access token: %w", err)
	}
	if err := p.tokenStore.StoreToken(ctx, refreshToken, user.ID, time.Now().Add(p.refreshExp)); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}
	user.LastLogin = time.Now()
	p.userStore.UpdateUser(ctx, user)

	return &auth.AuthResult{
		Success:      true,
		UserID:       user.ID,
		Username:     user.Username,
		Email:        user.Email,
		Roles:        user.Roles,
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(p.expiration),
		Metadata: map[string]interface{}{
			"provider": p.name,
		},
	}, nil
}

func (p *JWTProvider) ValidateToken(ctx context.Context, tokenString string) (*auth.AuthResult, error) {
	userID, err := p.tokenStore.ValidateToken(ctx, tokenString)

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != p.signingMethod.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		if p.secretKey != nil {
			return p.secretKey, nil
		}
		return p.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	user, err := p.userStore.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.Enabled {
		return nil, errors.New("user is disabled")
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, errors.New("invalid expiration")
	}
	return &auth.AuthResult{
		Success:   true,
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Roles:     user.Roles,
		Token:     tokenString,
		ExpiresAt: time.Unix(int64(exp), 0),
	}, nil
}

func (p *JWTProvider) RefreshToken(ctx context.Context, tokenString string) (*auth.AuthResult, error) {
	result, err := p.ValidateToken(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	user, err := p.userStore.GetUserByID(ctx, result.UserID)
	if err != nil {
		return nil, err
	}
	p.tokenStore.RevokeToken(ctx, tokenString)

	accessToken, err := p.generateToken(user, p.expiration)
	if err != nil {
		return nil, err
	}

	refreshToken, err := p.generateToken(user, p.refreshExp)
	if err != nil {
		return nil, err
	}

	p.tokenStore.StoreToken(ctx, accessToken, user.ID, time.Now().Add(p.expiration))
	p.tokenStore.StoreToken(ctx, refreshToken, user.ID, time.Now().Add(p.refreshExp))

	return &auth.AuthResult{
		Success:      true,
		UserID:       user.ID,
		Username:     user.Username,
		Email:        user.Email,
		Roles:        user.Roles,
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(p.expiration),
	}, nil

}

func (p *JWTProvider) RevokeToken(ctx context.Context, tokenString string) error {
	return p.tokenStore.RevokeToken(ctx, tokenString)

}

func (p *JWTProvider) generateToken(user *auth.User, expiration time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"email":    user.Email,
		"roles":    user.Roles,
		"iat":      now.Unix(),
		"exp":      now.Add(expiration).Unix(),
		"iss":      p.issuer,
	}

	if p.audience != "" {
		claims["aud"] = p.audience

	}

	token := jwt.NewWithClaims(p.signingMethod, claims)

	var tokenString string
	var err error
	if p.secretKey != nil {
		tokenString, err = token.SignedString(p.secretKey)

	} else {
		tokenString, err = token.SignedString(p.privateKey)
	}

	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (p *JWTProvider) verifyPassword(password, hash string) bool {
	return password == hash
}
