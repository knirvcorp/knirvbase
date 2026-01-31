package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Permission string

const (
	PermissionReadOnly  Permission = "read"
	PermissionReadWrite Permission = "write"
	PermissionAdmin     Permission = "admin"
)

type Claims struct {
	UserID      string       `json:"user_id"`
	WalletAddr  string       `json:"wallet_addr"`
	Permissions []Permission `json:"permissions"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

func NewTokenManager(secretKey string) *TokenManager {
	return &TokenManager{
		secretKey:     []byte(secretKey),
		tokenDuration: 1 * time.Hour,
	}
}

// GenerateToken creates a new JWT token
func (tm *TokenManager) GenerateToken(
	userID, walletAddr string,
	permissions []Permission,
) (string, error) {
	claims := Claims{
		UserID:      userID,
		WalletAddr:  walletAddr,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tm.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secretKey)
}

// ValidateToken verifies and parses a JWT token
func (tm *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return tm.secretKey, nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RefreshToken generates a new token with extended expiration
func (tm *TokenManager) RefreshToken(oldToken string) (string, error) {
	claims, err := tm.ValidateToken(oldToken)
	if err != nil {
		return "", err
	}

	return tm.GenerateToken(claims.UserID, claims.WalletAddr, claims.Permissions)
}

// HasPermission checks if claims contain required permission
func (c *Claims) HasPermission(required Permission) bool {
	for _, p := range c.Permissions {
		if p == required || p == PermissionAdmin {
			return true
		}
	}
	return false
}

// Middleware for HTTP authentication
type AuthMiddleware struct {
	tokenManager *TokenManager
}

func NewAuthMiddleware(tokenManager *TokenManager) *AuthMiddleware {
	return &AuthMiddleware{tokenManager: tokenManager}
}

type contextKey string

const claimsKey contextKey = "claims"

func (am *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			http.Error(w, "invalid authorization format", http.StatusUnauthorized)
			return
		}

		tokenString := authHeader[7:]
		claims, err := am.tokenManager.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	return claims, ok
}